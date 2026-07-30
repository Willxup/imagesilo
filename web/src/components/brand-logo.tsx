type BrandLogoProps = {
  className?: string
  imageClassName?: string
}

export function BrandLogo({
  className = '',
  imageClassName = 'h-10 w-auto',
}: BrandLogoProps) {
  return (
    <span className={`brand-logo-surface inline-flex items-center ${className}`.trim()}>
      <img
        src="/brand/imagesilo-logo.png"
        alt="ImageSilo"
        className={`shrink-0 ${imageClassName}`.trim()}
        width="1377"
        height="385"
      />
    </span>
  )
}
