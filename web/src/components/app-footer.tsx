const repositoryURL = 'https://github.com/Willxup/imagesilo'
const profileURL = 'https://github.com/Willxup'
const buildVersion = import.meta.env.VITE_APP_VERSION?.trim() || 'dev'

export function AppFooter({ version = buildVersion }: { version?: string }) {
  const normalizedVersion = version.trim() || 'dev'
  const versionURL = normalizedVersion === 'dev'
    ? `${repositoryURL}/releases`
    : `${repositoryURL}/releases/tag/${encodeURIComponent(normalizedVersion)}`

  return (
    <footer className="app-footer">
      <div className="app-footer-line">
        <span>© 2026</span>
        <a href={repositoryURL} target="_blank" rel="noreferrer">ImageSilo</a>
        <span aria-hidden="true">·</span>
        <a href={`${repositoryURL}/blob/main/LICENSE`} target="_blank" rel="noreferrer">License</a>
      </div>
      <div className="app-footer-line app-footer-powered">
        <span>Powered By</span>
        <a href={profileURL} target="_blank" rel="noreferrer" aria-label="Willxup GitHub profile">
          <svg aria-hidden="true" className="app-footer-github" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 .7a11.5 11.5 0 0 0-3.64 22.4c.58.11.79-.25.79-.56v-2.23c-3.22.7-3.9-1.37-3.9-1.37-.53-1.34-1.29-1.69-1.29-1.69-1.05-.72.08-.71.08-.71 1.17.08 1.78 1.2 1.78 1.2 1.04 1.78 2.72 1.27 3.38.97.1-.75.4-1.27.74-1.56-2.57-.29-5.27-1.28-5.27-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.47.11-3.05 0 0 .96-.31 3.16 1.18A11 11 0 0 1 12 6.12c.98 0 1.95.13 2.87.39 2.2-1.49 3.16-1.18 3.16-1.18.63 1.58.23 2.76.12 3.05.73.81 1.17 1.83 1.17 3.09 0 4.41-2.71 5.38-5.29 5.67.42.36.78 1.06.78 2.14v3.26c0 .31.21.68.8.56A11.5 11.5 0 0 0 12 .7Z" />
          </svg>
          <span>Willxup</span>
        </a>
        <span aria-hidden="true">·</span>
        <a className="app-footer-version" href={versionURL} target="_blank" rel="noreferrer">Version: {normalizedVersion}</a>
      </div>
    </footer>
  )
}
