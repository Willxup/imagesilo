import { useTranslation } from 'react-i18next'

import { SystemOverviewPanel } from './system-overview-panel'

export function SystemPage() {
  const { t } = useTranslation()
  return <section><h1 className="page-title">{t('system.title')}</h1><p className="page-description">{t('system.description')}</p><div className="mt-8"><SystemOverviewPanel /></div></section>
}
