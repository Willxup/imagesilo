import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { Icon } from '../../components/ui/icon'
import type { IconName } from '../../components/ui/icon'
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
          <Button className="standard-action-button" variant="outline" type="button" disabled={inspection.isPending} onClick={() => inspection.mutate()}><Icon name={inspection.isPending ? 'loader' : 'search'} className={inspection.isPending ? 'animate-spin' : ''} />{inspection.isPending ? t('common.working') : t('settings.inspectNow')}</Button>
          <Button className="standard-action-button" variant="destructive" type="button" disabled={rebuild.isPending} onClick={() => setRebuildOpen(true)}><Icon name={rebuild.isPending ? 'loader' : 'refresh'} className={rebuild.isPending ? 'animate-spin' : ''} />{rebuild.isPending ? t('common.working') : t('settings.rebuildIndexes')}</Button>
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
  const { t } = useTranslation()
  const rows: Array<Array<{ icon: IconName, label: string, value: string }>> = [
    [
      { icon: 'images', label: t('settings.imageCount'), value: String(value.imageCount) },
      { icon: 'history', label: t('settings.aliasCount'), value: String(value.aliasCount) },
      { icon: 'server', label: t('settings.storageUsed'), value: formatBytes(value.storedBytes) },
      { icon: 'refresh', label: t('settings.migrationStorageUsed'), value: formatBytes(value.migrationStoredBytes) },
    ],
    [
      { icon: 'server', label: t('settings.rss'), value: formatBytes(value.rssBytes) },
      { icon: 'activity', label: t('settings.heap'), value: formatBytes(value.heapAllocBytes) },
      { icon: 'server', label: t('settings.heapSys'), value: formatBytes(value.heapSysBytes) },
      { icon: 'activity', label: t('settings.goroutines'), value: String(value.goroutines) },
    ],
  ]
  return (
    <div className="mt-5">
      <div className="grid gap-3">
        {rows.map((metrics) => (
          <Card className="p-5 md:p-6" key={metrics[0].label}>
            <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
              {metrics.map((metric) => (
                <div className="flex min-w-0 items-center gap-3" key={metric.label}>
                  <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-white/90">
                    <Icon name={metric.icon} className="h-5 w-5" />
                  </span>
                  <div className="min-w-0">
                    <p className="text-sm text-gray-500 dark:text-gray-400">{metric.label}</p>
                    <p className="mt-1 truncate text-title-sm font-bold text-gray-800 dark:text-white/90" title={metric.value}>{metric.value}</p>
                  </div>
                </div>
              ))}
            </div>
          </Card>
        ))}
      </div>
      <div className="system-index-status" data-consistent={value.indexConsistent || undefined}>
        <Icon name={value.indexConsistent ? 'check' : 'activity'} />
        <div>
          <p className={value.indexConsistent ? 'text-green' : 'text-danger'}>{value.indexConsistent ? t('settings.indexConsistent') : t('settings.indexDifferent')}</p>
          <p className="mt-1 text-xs leading-[18px] text-muted-foreground">{t('settings.indexCounts', value.indexes)}</p>
        </div>
      </div>
      {value.missingImageCount > 0 ? (
        <div className="mt-4 rounded-xl bg-danger-soft p-4 text-sm text-danger" role="alert">
          <p className="font-medium">{t('settings.missingImages', { count: value.missingImageCount })}</p>
          {value.missingImageIds.length > 0 ? <code className="mt-2 block break-all text-xs">{value.missingImageIds.join(', ')}</code> : null}
        </div>
      ) : null}
    </div>
  )
}
