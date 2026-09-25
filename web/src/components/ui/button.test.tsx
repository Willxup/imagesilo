import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Button } from './button'

describe('Button', () => {
  it('uses the original brand gradient for the default primary action', () => {
    render(<Button>Primary action</Button>)

    expect(screen.getByRole('button', { name: 'Primary action' })).toHaveClass(
      '[background:var(--silo-gradient)]',
      'text-white',
      'hover:opacity-90',
    )
  })

  it('keeps the explicit solid variant on the same accessible palette', () => {
    render(<Button variant="brandSolid">Solid action</Button>)

    expect(screen.getByRole('button', { name: 'Solid action' })).toHaveClass(
      '[background-color:var(--accent)]',
      '[color:var(--accent-foreground)]',
    )
  })
})
