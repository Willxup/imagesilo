import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { CopyLinkControl } from './copy-link-control'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const image = { originalName: 'coast.jpg', standardUrl: '/image/coast.jpg' }

describe('copy feedback', () => {
  afterEach(() => { cleanup(); vi.useRealTimers(); localStorage.clear(); vi.unstubAllGlobals() })

  it('waits for the clipboard, prevents duplicate clicks and clears success without changing its label', async () => {
    vi.useFakeTimers()
    let resolve!: () => void
    const writeText = vi.fn(() => new Promise<void>((done) => { resolve = done }))
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const onCopied = vi.fn()
    const { container } = render(<CopyLinkControl image={image} onCopied={onCopied} />)
    const button = screen.getByRole('button', { name: 'images.copySelectedFormat' })
    const label = button.textContent
    fireEvent.click(button)
    expect(button).toBeDisabled()
    expect(button).toHaveAttribute('aria-busy', 'true')
    fireEvent.click(button)
    expect(writeText).toHaveBeenCalledTimes(1)
    await act(async () => resolve())
    expect(button).toBeEnabled()
    expect(container.firstChild).toHaveAttribute('data-copied', 'true')
    expect(button.textContent).toBe(label)
    expect(onCopied).toHaveBeenCalledWith('direct')
    act(() => vi.advanceTimersByTime(1600))
    expect(container.firstChild).not.toHaveAttribute('data-copied')
  })

  it('does not retain a success marker for another image or after a failed copy', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const { container, rerender } = render(<CopyLinkControl image={image} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'images.copySelectedFormat' })))
    expect(container.firstChild).toHaveAttribute('data-copied', 'true')
    rerender(<CopyLinkControl image={{ ...image, standardUrl: '/image/other.jpg' }} />)
    expect(container.firstChild).not.toHaveAttribute('data-copied')
    writeText.mockRejectedValueOnce(new Error('Clipboard denied'))
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'images.copySelectedFormat' })))
    expect(container.firstChild).not.toHaveAttribute('data-copied')
    expect(screen.getByRole('button', { name: 'images.copySelectedFormat' })).toBeEnabled()
  })
})
