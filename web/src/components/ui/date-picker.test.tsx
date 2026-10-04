import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import { DatePicker } from './date-picker'

function Harness({ min }: { min?: string }) {
  const [value, setValue] = useState('2026-10-03')
  return <><input aria-label="Outside" /><DatePicker value={value} onChange={setValue} min={min} locale="en-US" ariaLabel="Date" placeholder="Choose" clearLabel="Clear" todayLabel="Today" previousMonthLabel="Previous" nextMonthLabel="Next" /><output>{value}</output></>
}

describe('DatePicker', () => {
  afterEach(cleanup)
  it('focuses the selected day, crosses months with arrows and commits a date', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: 'Date' }))
    const day = screen.getByRole('button', { name: 'Saturday, October 3, 2026' })
    expect(day).toHaveFocus()
    fireEvent.keyDown(day, { key: 'ArrowUp' })
    const previous = screen.getByRole('button', { name: 'Saturday, September 26, 2026' })
    expect(previous).toHaveFocus()
    fireEvent.click(previous)
    expect(screen.getByRole('status')).toHaveTextContent('2026-09-26')
    expect(screen.getByRole('button', { name: 'Date' })).toHaveFocus()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it('respects the minimum and clears without losing the trigger', () => {
    render(<Harness min="2026-10-03" />)
    const trigger = screen.getByRole('button', { name: 'Date' })
    fireEvent.click(trigger)
    expect(screen.getByRole('button', { name: 'Friday, October 2, 2026' })).toBeDisabled()
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft' })
    expect(screen.getByRole('button', { name: 'Saturday, October 3, 2026' })).toHaveFocus()
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
    expect(trigger).toHaveFocus()
  })
  it('restores focus for internal Escape but never steals it after leaving', () => {
    render(<Harness />)
    const trigger = screen.getByRole('button', { name: 'Date' })
    fireEvent.click(trigger)
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(trigger).toHaveFocus()
    fireEvent.click(trigger)
    const outside = screen.getByRole('textbox', { name: 'Outside' })
    act(() => outside.focus())
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    fireEvent.keyDown(outside, { key: 'Escape' })
    expect(outside).toHaveFocus()
  })
})
