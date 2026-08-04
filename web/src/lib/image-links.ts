export type LinkFormat = 'direct' | 'markdown' | 'html' | 'bbcode'

export type LinkableImage = {
  originalName: string
  standardUrl: string
}

export function imageLinks(image: LinkableImage): Record<LinkFormat, string> {
  const direct = new URL(image.standardUrl, window.location.origin).toString()
  const alt = image.originalName.replaceAll(']', '\\]')
  return {
    direct,
    markdown: `![${alt}](${direct})`,
    html: `<img src="${escapeHTML(direct)}" alt="${escapeHTML(image.originalName)}">`,
    bbcode: `[img]${direct}[/img]`,
  }
}

export async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
}

export function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MiB`
  return `${(value / 1024 / 1024 / 1024).toFixed(2)} GiB`
}

function escapeHTML(value: string) {
  return value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}
