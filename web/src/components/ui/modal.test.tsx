import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Modal } from './modal'

describe('Modal', () => {
  afterEach(cleanup)

  it('keeps the background locked during an overlapping exit and restores it after the last dialog', async () => {
    const close = vi.fn()
    const outside = document.createElement('button')
    document.body.append(outside)
    outside.focus()
    const dialogs = (first: boolean, second: boolean) => <>
      <Modal open={first} onClose={close} title="Navigation" closeLabel="Close navigation"><button>Destination</button></Modal>
      <Modal open={second} onClose={close} title="Search" closeLabel="Close search"><input aria-label="Query" /></Modal>
    </>
    const view = render(dialogs(true, false))
    try {
      await waitFor(() => expect(screen.getByRole('button', { name: 'Destination' })).toHaveFocus())
      view.rerender(dialogs(false, true))
      await waitFor(() => expect(screen.getByRole('textbox', { name: 'Query' })).toHaveFocus())
      await waitFor(() => expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1))
      expect(view.container.inert).toBe(true)
      expect(view.container).toHaveAttribute('aria-hidden', 'true')
      expect(document.body.style.overflow).toBe('hidden')
      expect(screen.getByRole('textbox', { name: 'Query' })).toHaveFocus()
      view.rerender(dialogs(false, false))
      await waitFor(() => expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(0))
      expect(view.container.inert).toBe(false)
      expect(view.container).not.toHaveAttribute('aria-hidden')
      expect(document.body.style.overflow).toBe('')
      expect(outside).toHaveFocus()
    } finally {
      view.unmount()
      outside.remove()
    }
  })

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
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true })
    expect(last).toHaveFocus()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(close).toHaveBeenCalledOnce()

    view.unmount()
    expect(view.container.inert).toBe(false)
  })
  it('keeps notifications accessible and includes their close action in the modal focus cycle', async () => {
    const host = document.createElement('div')
    host.id = 'imagesilo-notifications'
    host.setAttribute('data-modal-notifications', '')
    host.innerHTML = '<div data-sonner-toast><span role="alert">Operation failed</span><button>Dismiss notification</button></div>'
    document.body.append(host)
    const close = vi.fn()
    const view = render(<Modal open onClose={close} title="Confirm" closeLabel="Close"><button>Retry</button></Modal>)
    try {
      const retry = screen.getByRole('button', { name: 'Retry' })
      await waitFor(() => expect(retry).toHaveFocus())
      expect(view.container.inert).toBe(true)
      expect(host.inert).not.toBe(true)
      expect(host).not.toHaveAttribute('aria-hidden')
      expect(screen.getByRole('alert')).toHaveTextContent('Operation failed')
      expect(screen.getByRole('dialog')).toHaveAttribute('aria-owns', host.id)
      fireEvent.keyDown(retry, { key: 'Tab' })
      const dismiss = screen.getByRole('button', { name: 'Dismiss notification' })
      expect(dismiss).toHaveFocus()
      fireEvent.keyDown(dismiss, { key: 'Escape' })
      expect(close).not.toHaveBeenCalled()
      fireEvent.keyDown(dismiss, { key: 'Tab' })
      expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()
    } finally { view.unmount(); host.remove() }
  })

})
