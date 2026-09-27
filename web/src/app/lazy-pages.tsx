import { lazy } from 'react'

import { loadUploadPage, loadImageListPage, loadImageDetailPage, loadMigrationImagePage, loadAliasPage, loadApiTokenPage, loadSettingsPage, loadSystemPage } from './page-loaders'

export const UploadPage = lazy(loadUploadPage)
export const ImageListPage = lazy(loadImageListPage)
export const ImageDetailPage = lazy(loadImageDetailPage)
export const MigrationImagePage = lazy(loadMigrationImagePage)
export const AliasPage = lazy(loadAliasPage)
export const ApiTokenPage = lazy(loadApiTokenPage)
export const SettingsPage = lazy(loadSettingsPage)
export const SystemPage = lazy(loadSystemPage)
