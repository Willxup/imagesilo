type BrandLogoProps = {
  className?: string
  imageClassName?: string
}

export function BrandLogo({
  className = '',
  imageClassName = 'h-10 w-auto',
}: BrandLogoProps) {
  return (
    <span className={`brand-logo-surface inline-flex items-center ${className}`.trim()} role="img" aria-label="ImageSilo">
      <img
        src="/brand/imagesilo-logo.png"
        alt=""
        aria-hidden="true"
        className={`brand-logo-light shrink-0 ${imageClassName}`.trim()}
        width="1377"
        height="385"
      />
      <span className={`brand-logo-dark brand-logo-dark-composite shrink-0 ${imageClassName}`.trim()} aria-hidden="true">
        <img className="brand-logo-dark-base" src="/brand/imagesilo-logo.png" alt="" />
        <img className="brand-logo-dark-wordmark" src="/brand/imagesilo-logo.png" alt="" />
      </span>
    </span>
  )
}
