import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { Icon } from '../../components/ui/icon'
import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { InspectionResult, RebuildResult, SystemOverview } from '../../lib/api-types'

export function SystemOverviewPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [rebuildOpen, setRebuildOpen] = useState(false)
  const query = useQuery({ queryKey: ['overview'], queryFn: () => apiRequest<SystemOverview>('/api/v1/overview') })
  const inspection = useMutation({
    mutationFn: () => apiRequest<InspectionResult>('/api/v1/maintenance/inspect', { method: 'POST' }),
    onMutate: () => toast.loading(t('toast.inspectionRunning'), { id: 'system-inspection' }),
    onSuccess: (result) => { toast.success(t('toast.inspectionComplete'), { id: 'system-inspection', description: t('settings.inspectionSummary', result) }); return queryClient.invalidateQueries({ queryKey: ['overview'] }) },
	onError: () => toast.error(t('settings.maintenanceFailed'), { id: 'system-inspection' }),
  })
  const rebuild = useMutation({
    mutationFn: () => apiRequest<RebuildResult>('/api/v1/maintenance/rebuild', { method: 'POST' }),
    onMutate: () => toast.loading(t('toast.rebuildRunning'), { id: 'system-rebuild' }),
    onSuccess: (result) => { setRebuildOpen(false); toast.success(t('toast.rebuildComplete'), { id: 'system-rebuild', description: t('settings.rebuildSummary', result) }); return queryClient.invalidateQueries({ queryKey: ['overview'] }) },
	onError: () => toast.error(t('settings.maintenanceFailed'), { id: 'system-rebuild' }),
  })
  return (
    <section className="surface-panel system-overview-panel p-6">
      <div className="system-overview-header">
        <div><h2 className="text-xl font-semibold">{t('settings.overview')}</h2><p className="mt-2 text-xs leading-[18px] text-muted-foreground">{t('settings.overviewHelp')}</p></div>
        <div className="flex flex-wrap gap-2">
          <Button className="standard-action-button" variant="outline" type="button" disabled={rebuild.isPending} onClick={() => setRebuildOpen(true)}><Icon name={rebuild.isPending ? 'loader' : 'refresh'} className={rebuild.isPending ? 'animate-spin' : ''} />{rebuild.isPending ? t('common.working') : t('settings.rebuildIndexes')}</Button>
          <Button className="standard-action-button" variant="outline" type="button" disabled={inspection.isPending} onClick={() => inspection.mutate()}><Icon name={inspection.isPending ? 'loader' : 'search'} className={inspection.isPending ? 'animate-spin' : ''} />{inspection.isPending ? t('common.working') : t('settings.inspectNow')}</Button>
        </div>
      </div>
      {query.isLoading ? <p className="mt-4 text-muted-foreground">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-4 text-danger">{t('settings.overviewFailed')}</p> : null}
      {query.data ? <Overview value={query.data} /> : null}
      <ConfirmDialog open={rebuildOpen} onOpenChange={setRebuildOpen} title={t('settings.rebuildTitle')} description={t('settings.confirmRebuild')} confirmLabel={t('settings.rebuildIndexes')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} destructive pending={rebuild.isPending} onConfirm={() => rebuild.mutate()} />
    </section>
  )
}

function Overview({ value }: { value: SystemOverview }) {
  const { t, i18n } = useTranslation()
  const maintenance = [
    { label: t('settings.lastInspection'), at: value.lastInspection?.checkedAt, summary: value.lastInspection ? t('settings.inspectionSummary', value.lastInspection) : t('settings.notRun') },
    { label: t('settings.lastRebuild'), at: value.lastRebuild?.completedAt, summary: value.lastRebuild ? t('settings.rebuildSummary', value.lastRebuild) : t('settings.notRun') },
    { label: t('settings.lastDaily'), at: value.lastDaily?.completedAt, summary: value.lastDaily ? t('settings.dailySummary', value.lastDaily) : t('settings.notRun') },
  ]
  return (
    <div>
      <div className="system-metrics">
        <div className="system-metric"><span className="system-metric-label">{t('settings.imageCount')}</span><strong>{value.imageCount}</strong></div>
        <div className="system-metric"><span className="system-metric-label">{t('settings.storageUsed')}</span><strong>{formatBytes(value.storedBytes)}</strong></div>
        <div className="system-metric"><span className="system-metric-label">{t('settings.migrationStorageUsed')}</span><strong>{formatBytes(value.migrationStoredBytes)}</strong></div>
      </div>
      <div className="system-detail-grid">
        <section className="system-detail-panel"><h3>{t('settings.runtimeResources')}</h3><dl>
          <dt>{t('settings.rss')}</dt><dd>{formatBytes(value.rssBytes)}</dd>
          <dt>{t('settings.heap')}</dt><dd>{formatBytes(value.heapAllocBytes)}</dd>
          <dt>{t('settings.heapSys')}</dt><dd>{formatBytes(value.heapSysBytes)}</dd>
          <dt>{t('settings.goroutines')}</dt><dd>{value.goroutines}</dd>
          <dt>{t('settings.aliasCount')}</dt><dd>{value.aliasCount}</dd>
        </dl></section>
        <section className="system-detail-panel"><h3>{t('settings.indexHealth')}</h3>
          <div className="system-index-status" data-consistent={value.indexConsistent || undefined}>
            <Icon name={value.indexConsistent ? 'check' : 'activity'} />
            <div><p className={value.indexConsistent ? 'text-green' : 'text-danger'}>{value.indexConsistent ? t('settings.indexConsistent') : t('settings.indexDifferent')}</p><p className="mt-1 text-xs leading-[18px] text-muted-foreground">{t('settings.indexCounts', value.indexes)}</p></div>
          </div>
          {value.missingImageCount > 0 ? <div className="mt-4 rounded-xl bg-danger-soft p-4 text-sm text-danger" role="alert"><p className="font-medium">{t('settings.missingImages', { count: value.missingImageCount })}</p>{value.missingImageIds.length > 0 ? <code className="mt-2 block break-all text-xs">{value.missingImageIds.join(', ')}</code> : null}</div> : null}
        </section>
      </div>
      <section className="system-timeline"><h3>{t('settings.maintenanceHistory')}</h3>
        {maintenance.map((item) => <div className="system-timeline-row" key={item.label}><div><strong>{item.label} {item.at ? <time dateTime={item.at}>{new Date(item.at).toLocaleString(i18n.resolvedLanguage ?? i18n.language)}</time> : null}</strong><p>{item.summary}</p></div></div>)}
      </section>
    </div>
  )
}
