import { FC, useMemo } from 'react'

import classNames from 'classnames'

import { FilterType, NegatedFilters, resolveFilter } from '@sourcegraph/shared/src/search/query/filters'
import { findFilters } from '@sourcegraph/shared/src/search/query/query'
import { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
import type { Filter } from '@sourcegraph/shared/src/search/query/token'
import { omitFilter, succeedScan, updateFilter } from '@sourcegraph/shared/src/search/query/transformer'
import type { Filter as ResultFilter, SearchMatch } from '@sourcegraph/shared/src/search/stream'
import { Panel } from '@sourcegraph/wildcard'

import {
    authorFilter,
    commitDateFilter,
    languageFilter,
    repoFilter,
    SearchDynamicFilter,
    symbolFilter,
    utilityFilter,
} from './components/dynamic-filter/SearchDynamicFilter'
import {
    FilterTypeList,
    resolveFilterTypeValue,
    toSearchSyntaxTypeFilter,
} from './components/filter-type-list/FilterTypeList'
import { FiltersDocFooter } from './components/filters-doc-footer/FiltersDocFooter'
import { useFilterQuery } from './hooks'
import { COMMIT_DATE_FILTERS, SearchFilterType, SYMBOL_KIND_FILTERS } from './types'
import { generateAuthorFilters } from './utils'

import styles from './NewSearchFilters.module.scss'

interface NewSearchFiltersProps {
    query: string
    results: SearchMatch[] | undefined
    filters?: ResultFilter[]
    className?: string
    onQueryChange: (nextQuery: string) => void
}

export const NewSearchFilters: FC<NewSearchFiltersProps> = props => {
    const { query, results, filters, className, onQueryChange } = props

    const [filterQuery, setFilterQuery] = useFilterQuery()

    const type = useMemo(() => {
        const tokens = scanSearchQuery(query)

        if (tokens.type === 'success') {
            const filters = tokens.term.filter(token => token.type === 'filter') as Filter[]
            const typeFilters = filters.filter(filter => resolveFilter(filter.field.value)?.type === 'type')

            if (typeFilters.length === 0 || typeFilters.length > 1) {
                return SearchFilterType.Code
            }

            return resolveFilterTypeValue(typeFilters[0].value?.value)
        }

        return SearchFilterType.Code
    }, [query])

    const authorFilters = useMemo(() => generateAuthorFilters(results ?? []), [results])

    const handleFilterTypeChange = (filterType: SearchFilterType): void => {
        switch (filterType) {
            case SearchFilterType.Code: {
                const filters = findFilters(succeedScan(query), FilterType.type)

                const newQuery = filters.reduce((query, filter) => omitFilter(query, filter), query)
                onQueryChange(newQuery)
                break
            }
            default: {
                const filters = findFilters(succeedScan(query), FilterType.type)
                const newQuery = filters.reduce((query, filter) => omitFilter(query, filter), query)

                onQueryChange(updateFilter(newQuery, FilterType.type, toSearchSyntaxTypeFilter(filterType)))
            }
        }
    }

    return (
        <Panel
            defaultSize={250}
            minSize={200}
            position="left"
            storageKey="filter-sidebar"
            ariaLabel="Filters sidebar"
            className={classNames(styles.root, className)}
        >
            <aside className={styles.scrollWrapper}>
                <FilterTypeList value={type} onSelect={handleFilterTypeChange} />

                {type === SearchFilterType.Symbols && (
                    <SearchDynamicFilter
                        filterType={FilterType.select}
                        filters={SYMBOL_KIND_FILTERS}
                        exclusive={true}
                        staticFilters={true}
                        filterQuery={filterQuery}
                        renderItem={symbolFilter}
                        onFilterQueryChange={setFilterQuery}
                    />
                )}

                {type === SearchFilterType.Commits && (
                    <>
                        <SearchDynamicFilter
                            filterType={FilterType.author}
                            filters={authorFilters}
                            exclusive={true}
                            filterQuery={filterQuery}
                            renderItem={authorFilter}
                            onFilterQueryChange={setFilterQuery}
                        />

                        <SearchDynamicFilter
                            filterType={[FilterType.after, FilterType.before]}
                            filters={COMMIT_DATE_FILTERS}
                            exclusive={true}
                            staticFilters={true}
                            filterQuery={filterQuery}
                            renderItem={commitDateFilter}
                            onFilterQueryChange={setFilterQuery}
                        />
                    </>
                )}

                <SearchDynamicFilter
                    filterType={FilterType.lang}
                    filters={filters}
                    filterQuery={filterQuery}
                    renderItem={languageFilter}
                    onFilterQueryChange={setFilterQuery}
                />

                <SearchDynamicFilter
                    filterType={FilterType.repo}
                    filters={filters}
                    filterQuery={filterQuery}
                    renderItem={repoFilter}
                    onFilterQueryChange={setFilterQuery}
                />

                <SearchDynamicFilter
                    filterType={FilterType.file}
                    filterAlias={NegatedFilters.file}
                    filters={filters}
                    filterQuery={filterQuery}
                    onFilterQueryChange={setFilterQuery}
                />

                <SearchDynamicFilter
                    filterType="utility"
                    filterAlias={[FilterType.archived, FilterType.fork]}
                    filters={filters}
                    filterQuery={filterQuery}
                    renderItem={utilityFilter}
                    onFilterQueryChange={setFilterQuery}
                />

                <FiltersDocFooter className={styles.footer} />
            </aside>
        </Panel>
    )
}
