import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { PreviewImage } from './preview-image'

describe('PreviewImage', () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks() })

  it('reveals loaded and failed images and resets readiness for a new source', () => {
    const { rerender } = render(<PreviewImage src="/first.webp" alt="Coast" />)
    const image = screen.getByRole('img')
    expect(image).not.toHaveAttribute('data-ready')
    fireEvent.load(image)
    expect(image).toHaveAttribute('data-ready', 'true')
    rerender(<PreviewImage src="/second.webp" alt="Coast" />)
    expect(image).not.toHaveAttribute('data-ready')
    fireEvent.error(image)
    expect(image).toHaveAttribute('data-ready', 'true')
    expect(image).toHaveAttribute('alt', 'Coast')
  })

  it('shows a cached thumbnail immediately without waiting for a second load event', () => {
    vi.spyOn(HTMLImageElement.prototype, 'complete', 'get').mockReturnValue(true)
    vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockReturnValue(512)
    render(<PreviewImage src="/cached.webp" alt="Cached coast" />)
    expect(screen.getByRole('img')).toHaveAttribute('data-ready', 'true')
  })
})
