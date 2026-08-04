import { lazy } from 'react'

export const UploadPage = lazy(() => import('../features/upload/upload-page').then((module) => ({ default: module.UploadPage })))
export const ImageListPage = lazy(() => import('../features/images/image-list-page').then((module) => ({ default: module.ImageListPage })))
export const ImageDetailPage = lazy(() => import('../features/images/image-detail-page').then((module) => ({ default: module.ImageDetailPage })))
export const MigrationImagePage = lazy(() => import('../features/migrations/migration-image-page').then((module) => ({ default: module.MigrationImagePage })))
export const AliasPage = lazy(() => import('../features/aliases/alias-page').then((module) => ({ default: module.AliasPage })))
export const ApiTokenPage = lazy(() => import('../features/api-tokens/api-token-page').then((module) => ({ default: module.ApiTokenPage })))
export const SettingsPage = lazy(() => import('../features/settings/settings-page').then((module) => ({ default: module.SettingsPage })))
export const SystemPage = lazy(() => import('../features/system/system-page').then((module) => ({ default: module.SystemPage })))
