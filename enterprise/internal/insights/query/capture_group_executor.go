package query

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/aggregation"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/compression"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/gitserver"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/query/streaming"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/timeseries"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CaptureGroupExecutor struct {
	previewExecutor
	computeSearch func(ctx context.Context, query string) ([]GroupedResults, error)

	logger     log.Logger
	postgresDB database.DB
}

func NewCaptureGroupExecutor(postgres database.DB, clock func() time.Time) *CaptureGroupExecutor {
	return &CaptureGroupExecutor{
		previewExecutor: previewExecutor{
			repoStore: postgres.Repos(),
			// filter:    compression.NewHistoricalFilter(true, clock().Add(time.Hour*24*365*-1), insightsDb),
			filter: &compression.NoopFilter{},
			clock:  clock,
		},
		computeSearch: streamCompute,
		logger:        log.Scoped("CaptureGroupExecutor", ""),
		postgresDB:    postgres,
	}
}

func streamCompute(ctx context.Context, query string) ([]GroupedResults, error) {
	decoder, streamResults := streaming.MatchContextComputeDecoder()
	err := streaming.ComputeMatchContextStream(ctx, query, decoder)
	if err != nil {
		return nil, err
	}
	if len(streamResults.Errors) > 0 {
		return nil, errors.Errorf("compute streaming search: errors: %v", streamResults.Errors)
	}
	if len(streamResults.Alerts) > 0 {
		return nil, errors.Errorf("compute streaming search: alerts: %v", streamResults.Alerts)
	}
	return computeTabulationResultToGroupedResults(streamResults), nil
}

func (c *CaptureGroupExecutor) searchWithAggregator(ctx context.Context, query, repo, revision string) ([]*aggregation.Aggregate, error) {
	searchTimelimit := 60

	// If a search includes a timeout it reports as completing succesfully with the timeout is hit
	// This includes a timeout in the search that is a second longer than the context we will cancel as a fail safe
	modified, err := querybuilder.SingleRepoQuery(querybuilder.BasicQuery(query), repo, revision, querybuilder.CodeInsightsQueryDefaults(false))
	if err != nil {
		return nil, err
	}

	fmt.Println(fmt.Sprintf("query: %v", modified.String()))

	aggregationBufferSize := 100000000
	cappedAggregator := aggregation.NewLimitedAggregator(aggregationBufferSize)
	var tabulationErrors []error
	tabulationFunc := func(amr *aggregation.AggregationMatchResult, err error) {
		if err != nil {
			// r.getLogger().Debug("unable to aggregate results", log.Error(err))
			tabulationErrors = append(tabulationErrors, err)
			return
		}
		cappedAggregator.Add(amr.Key.Group, int32(amr.Count))
	}

	patternType := "regexp"

	countingFunc, err := aggregation.GetCountFuncForMode(query, patternType, types.CAPTURE_GROUP_AGGREGATION_MODE, types.FilePath)
	if err != nil {
		// r.getLogger().Debug("no aggregation counting function for mode", log.String("mode", string(aggregationMode)), log.Error(err))
		return nil, err
	}

	requestContext, cancelReqContext := context.WithTimeout(ctx, time.Second*time.Duration(searchTimelimit))
	defer cancelReqContext()
	searchClient := streaming.NewInsightsSearchClient(c.postgresDB, jobutil.NewUnimplementedEnterpriseJobs())
	searchResultsAggregator := aggregation.NewSearchResultsAggregatorWithContext(requestContext, tabulationFunc, countingFunc, c.postgresDB, types.CAPTURE_GROUP_AGGREGATION_MODE)

	_, err = searchClient.Search(requestContext, string(modified), &patternType, searchResultsAggregator)
	if err != nil || requestContext.Err() != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return nil, err
		} else {
			return nil, err
		}
	}

	return cappedAggregator.SortAggregate(), nil

}

func (c *CaptureGroupExecutor) Execute(ctx context.Context, query string, repositories []string, interval timeseries.TimeInterval) ([]GeneratedTimeSeries, error) {
	repoIds := make(map[string]api.RepoID)
	for _, repository := range repositories {
		repo, err := c.repoStore.GetByName(ctx, api.RepoName(repository))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch repository information for repository name: %s", repository)
		}
		repoIds[repository] = repo.ID
	}
	c.logger.Debug("Generated repoIds", log.String("repoids", fmt.Sprintf("%v", repoIds)))

	sampleTimes := timeseries.BuildSampleTimes(7, interval, c.clock())
	pivoted := make(map[string]timeCounts)

	for _, repository := range repositories {
		firstCommit, err := gitserver.GitFirstEverCommit(ctx, api.RepoName(repository))
		if err != nil {
			if errors.Is(err, gitserver.EmptyRepoErr) {
				continue
			} else {
				return nil, errors.Wrapf(err, "FirstEverCommit")
			}
		}
		// uncompressed plan for now, because there is some complication between the way compressed plans are generated and needing to resolve revhashes
		plan := c.filter.Filter(ctx, sampleTimes, api.RepoName(repository))

		// we need to perform the pivot from time -> {label, count} to label -> {time, count}
		for _, execution := range plan.Executions {
			if execution.RecordingTime.Before(firstCommit.Committer.Date) {
				// this logic is faulty, but works for now. If the plan was compressed (these executions had children) we would have to
				// iterate over the children to ensure they are also all before the first commit date. Otherwise, we would have to promote
				// that child to the new execution, and all of the remaining children (after the promoted one) become children of the new execution.
				// since we are using uncompressed plans (to avoid this problem and others) right now, each execution is standalone
				continue
			}
			commits, err := gitserver.NewGitCommitClient().RecentCommits(ctx, api.RepoName(repository), execution.RecordingTime, "")
			if err != nil {
				return nil, errors.Wrap(err, "git.Commits")
			} else if len(commits) < 1 {
				// there is no commit so skip this execution. Once again faulty logic for the same reasons as above.
				continue
			}

			// modifiedQuery, err := querybuilder.SingleRepoQuery(querybuilder.BasicQuery(query), repository, string(commits[0].ID), querybuilder.CodeInsightsQueryDefaults(false))
			// if err != nil {
			// 	return nil, errors.Wrap(err, "query validation")
			// }
			//
			// c.logger.Debug("executing query", log.String("query", modifiedQuery.String()))
			// grouped, err := c.computeSearch(ctx, modifiedQuery.String())
			// if err != nil {
			// 	errorMsg := "failed to execute capture group search for repository:" + repository
			// 	if execution.Revision != "" {
			// 		errorMsg += " commit:" + execution.Revision
			// 	}
			// 	return nil, errors.Wrap(err, errorMsg)
			// }
			//
			// sort.Slice(grouped, func(i, j int) bool {
			// 	return grouped[i].Value < grouped[j].Value
			// })

			aggResults, err := c.searchWithAggregator(ctx, query, repository, string(commits[0].ID))
			if err != nil {
				return nil, err
			}

			var grouped []GroupedResults
			for _, aggregate := range aggResults {
				grouped = append(grouped, GroupedResults{
					Value: aggregate.Label,
					Count: int(aggregate.Count),
				})
			}

			fmt.Println(fmt.Sprintf("got results: %v", grouped))

			for _, timeGroupElement := range grouped {
				value := timeGroupElement.Value
				if _, ok := pivoted[value]; !ok {
					pivoted[value] = generateTimes(plan)
				}
				pivoted[value][execution.RecordingTime] += timeGroupElement.Count
				for _, children := range execution.SharedRecordings {
					pivoted[value][children] += timeGroupElement.Count
				}
			}
		}
	}

	calculated := makeTimeSeries(pivoted)
	return calculated, nil
}

func makeTimeSeries(pivoted map[string]timeCounts) []GeneratedTimeSeries {
	var calculated []GeneratedTimeSeries
	seriesCount := 1
	for value, timeCounts := range pivoted {
		var ts []TimeDataPoint

		for key, val := range timeCounts {
			ts = append(ts, TimeDataPoint{
				Time:  key,
				Count: val,
			})
		}

		sort.Slice(ts, func(i, j int) bool {
			return ts[i].Time.Before(ts[j].Time)
		})

		calculated = append(calculated, GeneratedTimeSeries{
			Label:    value,
			Points:   ts,
			SeriesId: fmt.Sprintf("dynamic-series-%d", seriesCount),
		})
		seriesCount++
	}
	return calculated
}

func computeTabulationResultToGroupedResults(result *streaming.ComputeTabulationResult) []GroupedResults {
	var grouped []GroupedResults
	for _, match := range result.RepoCounts {
		for value, count := range match.ValueCounts {
			grouped = append(grouped, GroupedResults{
				Value: value,
				Count: count,
			})
		}
	}
	return grouped
}
