import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { InspectionResult, RebuildResult, SystemOverview } from '../../lib/api-types'

export function SystemOverviewPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['overview'], queryFn: () => apiRequest<SystemOverview>('/api/v1/overview') })
  const inspection = useMutation({
    mutationFn: () => apiRequest<InspectionResult>('/api/v1/maintenance/inspect', { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['overview'] }),
  })
  const rebuild = useMutation({
    mutationFn: () => apiRequest<RebuildResult>('/api/v1/maintenance/rebuild', { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['overview'] }),
  })
  return (
    <section className="rounded-2xl border border-line bg-panel p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div><h2 className="text-xl font-semibold">{t('settings.overview')}</h2><p className="mt-2 text-sm text-muted">{t('settings.overviewHelp')}</p></div>
        <div className="flex flex-wrap gap-2">
          <button className="button-secondary" type="button" disabled={inspection.isPending} onClick={() => inspection.mutate()}>{inspection.isPending ? t('common.working') : t('settings.inspectNow')}</button>
          <button className="button-secondary" type="button" disabled={rebuild.isPending} onClick={() => {
            if (window.confirm(t('settings.confirmRebuild'))) rebuild.mutate()
          }}>{rebuild.isPending ? t('common.working') : t('settings.rebuildIndexes')}</button>
        </div>
      </div>
      {query.isLoading ? <p className="mt-4 text-muted">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-4 text-danger">{t('settings.overviewFailed')}</p> : null}
      {query.data ? <Overview value={query.data} /> : null}
      {(inspection.isError || rebuild.isError) ? <p className="mt-4 text-danger">{t('settings.maintenanceFailed')}</p> : null}
    </section>
  )
}

function Overview({ value }: { value: SystemOverview }) {
  const { t } = useTranslation()
  const cards = [
    [t('settings.imageCount'), String(value.imageCount)], [t('settings.storageUsed'), formatBytes(value.storedBytes)],
    [t('settings.aliasCount'), String(value.aliasCount)], [t('settings.rss'), formatBytes(value.rssBytes)],
    [t('settings.heap'), formatBytes(value.heapAllocBytes)], [t('settings.goroutines'), String(value.goroutines)],
  ]
  return (
    <div className="mt-5">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map(([label, number]) => <div className="rounded-xl bg-canvas p-4" key={label}><p className="text-sm text-muted">{label}</p><p className="mt-1 text-2xl font-semibold">{number}</p></div>)}
      </div>
      <div className="mt-4 rounded-xl bg-canvas p-4 text-sm">
        <p className={value.indexConsistent ? 'text-accent' : 'text-danger'}>{value.indexConsistent ? t('settings.indexConsistent') : t('settings.indexDifferent')}</p>
        <p className="mt-2 text-muted">{t('settings.indexCounts', value.indexes)}</p>
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <MaintenanceSummary title={t('settings.lastInspection')} value={value.lastInspection ? [new Date(value.lastInspection.checkedAt).toLocaleString(), t('settings.inspectionSummary', value.lastInspection)] : null} />
        <MaintenanceSummary title={t('settings.lastRebuild')} value={value.lastRebuild ? [new Date(value.lastRebuild.completedAt).toLocaleString(), t('settings.rebuildSummary', value.lastRebuild)] : null} />
      </div>
    </div>
  )
}

function MaintenanceSummary({ title, value }: { title: string; value: [string, string] | null }) {
  const { t } = useTranslation()
  return <div className="rounded-xl border border-line p-4"><h3 className="font-semibold">{title}</h3>{value ? <><p className="mt-2 text-sm">{value[0]}</p><p className="mt-1 text-sm text-muted">{value[1]}</p></> : <p className="mt-2 text-sm text-muted">{t('settings.notRun')}</p>}</div>
}
