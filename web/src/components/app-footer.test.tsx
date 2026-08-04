import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { AppFooter } from './app-footer'

describe('AppFooter', () => {
  it('renders the project, license, author, and injected version links', () => {
    render(<AppFooter version="v0.2.0" />)

    expect(screen.getByRole('contentinfo')).toHaveTextContent(/© 2026ImageSilo·License/)
    expect(screen.getByRole('link', { name: 'ImageSilo' })).toHaveAttribute('href', 'https://github.com/Willxup/imagesilo')
    expect(screen.getByRole('link', { name: 'License' })).toHaveAttribute('href', 'https://github.com/Willxup/imagesilo/blob/main/LICENSE')
    expect(screen.getByRole('link', { name: 'Willxup GitHub profile' })).toHaveAttribute('href', 'https://github.com/Willxup')
    expect(screen.getByRole('link', { name: 'Version: v0.2.0' })).toHaveAttribute('href', 'https://github.com/Willxup/imagesilo/releases/tag/v0.2.0')
  })

  it('uses the releases page for a development build', () => {
    render(<AppFooter version="" />)

    expect(screen.getByRole('link', { name: 'Version: dev' })).toHaveAttribute('href', 'https://github.com/Willxup/imagesilo/releases')
  })
})
