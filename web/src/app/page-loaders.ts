export const loadUploadPage = () => import('../features/upload/upload-page').then((module) => ({ default: module.UploadPage }))

export const loadImageListPage = () => import('../features/images/image-list-page').then((module) => ({ default: module.ImageListPage }))

export const loadImageDetailPage = () => import('../features/images/image-detail-page').then((module) => ({ default: module.ImageDetailPage }))

export const loadMigrationImagePage = () => import('../features/migrations/migration-image-page').then((module) => ({ default: module.MigrationImagePage }))

export const loadAliasPage = () => import('../features/aliases/alias-page').then((module) => ({ default: module.AliasPage }))

export const loadApiTokenPage = () => import('../features/api-tokens/api-token-page').then((module) => ({ default: module.ApiTokenPage }))

export const loadSettingsPage = () => import('../features/settings/settings-page').then((module) => ({ default: module.SettingsPage }))

export const loadSystemPage = () => import('../features/system/system-page').then((module) => ({ default: module.SystemPage }))

// Warm only the destination the user points to or focuses; no API/data prefetch.
const pageLoaders: Record<string, () => Promise<unknown>> = {
  '/admin/upload': loadUploadPage,
  '/admin/images': loadImageListPage,
  '/admin/migrations': loadMigrationImagePage,
  '/admin/aliases': loadAliasPage,
  '/admin/api-tokens': loadApiTokenPage,
  '/admin/settings': loadSettingsPage,
  '/admin/system': loadSystemPage,
}
export function preloadAdminPage(path: string) {
  const load = path.startsWith('/admin/images/') ? loadImageDetailPage : pageLoaders[path]
  // A failed speculative load must never prevent normal navigation or show an error.
  if (load) void load().catch(() => undefined)
}
