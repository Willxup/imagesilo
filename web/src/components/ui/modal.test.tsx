import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Modal } from './modal'

describe('Modal', () => {
  afterEach(cleanup)

  it('makes the background inert, selects useful initial focus, and traps keyboard focus', async () => {
    const close = vi.fn()
    const view = render(
      <div data-testid="background">
        <button type="button">Outside</button>
        <Modal open onClose={close} title="Dialog" closeLabel="Close">
          <button type="button">First action</button>
          <button type="button">Last action</button>
        </Modal>
      </div>,
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'First action' })).toHaveFocus())
    expect(view.container).toHaveAttribute('aria-hidden', 'true')
    expect(view.container.inert).toBe(true)

    const last = screen.getByRole('button', { name: 'Last action' })
    last.focus()
    fireEvent.keyDown(document, { key: 'Tab' })
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(close).toHaveBeenCalledOnce()

    view.unmount()
    expect(view.container.inert).toBe(false)
  })
})
