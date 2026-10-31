import { describe, expect, it } from 'vitest'
import { imageLinks } from './image-links'

describe('imageLinks', () => {
  it('escapes brackets and existing backslashes in Markdown image labels', () => {
    const links = imageLinks({ originalName: 'screen[1]\\draft.png', standardUrl: '/image/example' })

    expect(links.markdown).toBe(`![screen\\[1\\]\\\\draft.png](${window.location.origin}/image/example)`)
    expect(links.direct).toBe(`${window.location.origin}/image/example`)
    expect(links.html).toBe(`<img src="${window.location.origin}/image/example" alt="screen[1]\\draft.png">`)
  })
})
