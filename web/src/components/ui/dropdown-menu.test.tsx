import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { DropdownItem, DropdownMenu } from './dropdown-menu'

function MenuHarness({ onSelect = () => undefined }: { onSelect?: () => void }) {
  const [open, setOpen] = useState(false)
  return <DropdownMenu open={open} onOpenChange={setOpen} trigger={<button type="button" onClick={() => setOpen((value) => !value)}>Formats</button>}>
    <DropdownItem onClick={() => { onSelect(); setOpen(false) }}>Direct</DropdownItem>
    <DropdownItem onClick={() => { onSelect(); setOpen(false) }}>Markdown</DropdownItem>
  </DropdownMenu>
}

describe('DropdownMenu', () => {
  afterEach(cleanup)
  it('wraps ArrowUp to the last item and restores trigger focus on selection', async () => {
    const onSelect = vi.fn()
    render(<MenuHarness onSelect={onSelect} />)
    const trigger = screen.getByRole('button', { name: 'Formats' })
    trigger.focus()
    fireEvent.click(trigger)
    fireEvent.keyDown(trigger, { key: 'ArrowUp' })
    const last = screen.getByRole('menuitem', { name: 'Markdown' })
    expect(last).toHaveFocus()
    fireEvent.click(last)
    expect(onSelect).toHaveBeenCalledOnce()
    expect(trigger).toHaveFocus()
    expect(document.querySelector('.ui-dropdown-panel')).toHaveAttribute('data-state', 'closed')
    await waitFor(() => expect(document.querySelector('.ui-dropdown-panel')).not.toBeInTheDocument())
  })

  it('closes with Escape and returns focus to the trigger', () => {
    render(<MenuHarness />)
    const trigger = screen.getByRole('button', { name: 'Formats' })
    trigger.focus()
    fireEvent.click(trigger)
    fireEvent.keyDown(trigger, { key: 'Escape' })
    expect(trigger).toHaveFocus()
    expect(document.querySelector('.ui-dropdown-panel')).toHaveAttribute('data-state', 'closed')
  })
  it('supports arrow/home/end navigation and closes on keyboard focus leaving', () => {
    render(<><button>Outside</button><MenuHarness /></>)
    const trigger = screen.getByRole('button', { name: 'Formats' })
    fireEvent.click(trigger)
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    expect(screen.getByRole('menuitem', { name: 'Direct' })).toHaveFocus()
    fireEvent.keyDown(document.activeElement!, { key: 'End' })
    expect(screen.getByRole('menuitem', { name: 'Markdown' })).toHaveFocus()
    fireEvent.keyDown(document.activeElement!, { key: 'Home' })
    expect(screen.getByRole('menuitem', { name: 'Direct' })).toHaveFocus()
    act(() => screen.getByRole('button', { name: 'Outside' }).focus())
    expect(document.querySelector('.ui-dropdown-panel')).toHaveAttribute('data-state', 'closed')
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(screen.getByRole('button', { name: 'Outside' })).toHaveFocus()
  })

})
