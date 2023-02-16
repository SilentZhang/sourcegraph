package lsifstore

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	deleteLsifDataByUploadIds                *observation.Operation
	idsWithMeta                              *observation.Operation
	reconcileCandidates                      *observation.Operation
	getUploadDocumentsForPath                *observation.Operation
	scanDocuments                            *observation.Operation
	createDefinitionsAndReferencesForRanking *observation.Operation
	setDefinitionsForRanking                 *observation.Operation
	setReferencesForRanking                  *observation.Operation
	getRankingReferencesByUploadID           *observation.Operation
	insertPathCountInputs                    *observation.Operation
	insertPathRanks                          *observation.Operation
	insertMetadata                           *observation.Operation
	writeMeta                                *observation.Operation
	writeDocuments                           *observation.Operation
	writeResultChunks                        *observation.Operation
	writeDefinitions                         *observation.Operation
	writeReferences                          *observation.Operation
	writeImplementations                     *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_uploads_lsifstore",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.uploads.lsifstore.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		deleteLsifDataByUploadIds:                op("DeleteLsifDataByUploadIds"),
		idsWithMeta:                              op("IDsWithMeta"),
		reconcileCandidates:                      op("ReconcileCandidates"),
		getUploadDocumentsForPath:                op("GetUploadDocumentsForPath"),
		scanDocuments:                            op("ScanDocuments"),
		createDefinitionsAndReferencesForRanking: op("CreateDefinitionsAndReferencesForRanking"),
		setDefinitionsForRanking:                 op("SetDefinitionsForRanking"),
		setReferencesForRanking:                  op("SetReferencesForRanking"),
		getRankingReferencesByUploadID:           op("GetRankingReferencesByUploadID"),
		insertPathCountInputs:                    op("InsertPathCountInputs"),
		insertPathRanks:                          op("InsertPathRanks"),
		insertMetadata:                           op("InsertMetadata"),
		writeMeta:                                op("WriteMeta"),
		writeDocuments:                           op("WriteDocuments"),
		writeResultChunks:                        op("WriteResultChunks"),
		writeDefinitions:                         op("WriteDefinitions"),
		writeReferences:                          op("WriteReferences"),
		writeImplementations:                     op("WriteImplementations"),
	}
}
