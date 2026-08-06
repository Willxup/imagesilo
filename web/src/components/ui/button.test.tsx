import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Button } from './button'

describe('Button', () => {
  it('uses the login gradient for the default primary action', () => {
    render(<Button>Primary action</Button>)

    expect(screen.getByRole('button', { name: 'Primary action' })).toHaveClass(
      '[background:var(--silo-gradient)]',
      'hover:opacity-90',
    )
  })

  it('keeps the explicit solid brand variant available', () => {
    render(<Button variant="brandSolid">Solid action</Button>)

    expect(screen.getByRole('button', { name: 'Solid action' })).toHaveClass(
      'bg-brand-500',
      'hover:bg-brand-600',
    )
  })
})
