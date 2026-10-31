import { useCallback, useState, type ImgHTMLAttributes } from 'react'

// Cached thumbnails appear immediately; newly decoded images fade into reserved space.
// An error settles too, so alternative text is never trapped behind opacity: 0.
export function PreviewImage({ src, className = '', ...props }: Omit<ImgHTMLAttributes<HTMLImageElement>, 'onLoad' | 'onError'> & { src: string }) {
  const [settledSource, setSettledSource] = useState<string | null>(null)
  const imageRef = useCallback((image: HTMLImageElement | null) => {
    if (image?.complete && image.naturalWidth > 0) setSettledSource(src)
  }, [src])

  return (
    <img
      {...props}
      ref={imageRef}
      src={src}
      decoding="async"
      className={`preview-image ${className}`}
      data-ready={settledSource === src || undefined}
      onLoad={() => setSettledSource(src)}
      onError={() => setSettledSource(src)}
    />
  )
}
