import type { LinkFormat } from '../../lib/image-links'

// Original optical-size glyphs: one 24px grid and rounded 1.75px strokes.
// Keep format marks distinct without file outlines or miniature text labels.
export function CopyGlyph({ name, className }: { name: LinkFormat | 'copy'; className?: string }) {
  return (
    <svg className={className} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false" data-copy-glyph={name}>
      {name === 'copy' && <><rect x="8" y="8" width="12" height="13" rx="2.5" /><path d="M15 4.5V4a1 1 0 0 0-1-1H5.5A2.5 2.5 0 0 0 3 5.5V14a1 1 0 0 0 1 1h.5" /></>}
      {name === 'direct' && <><path d="m10 8 2.5-2.5a4.25 4.25 0 0 1 6 6L16 14M8 10l-2.5 2.5a4.25 4.25 0 0 0 6 6L14 16" /><path d="m9 15 6-6" /></>}
      {name === 'markdown' && <><path d="M3 17V7l4.5 5L12 7v10M18.5 7v10m-3-3 3 3 3-3" /></>}
      {name === 'bbcode' && <><path d="M5 5H3v14h2M19 5h2v14h-2M9 7v10h3.5a2.5 2.5 0 0 0 0-5H9m0-5h3a2.5 2.5 0 0 1 0 5" /></>}
      {name === 'html' && <><path d="m7 7-5 5 5 5m10-10 5 5-5 5M14 4l-4 16" /></>}
    </svg>
  )
}
