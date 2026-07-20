import { beforeEach, describe, expect, it } from 'vitest'

import { updateFavicon } from '@/utils/branding'

describe('updateFavicon', () => {
  beforeEach(() => {
    document.head.innerHTML = '<link rel="icon" href="/logo.png">'
  })

  it('replaces the default favicon with the configured logo', () => {
    updateFavicon('https://example.com/custom-logo.png')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.href).toBe('https://example.com/custom-logo.png')
  })

  it('supports relative and image data URLs', () => {
    updateFavicon('/uploads/logo.svg')
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/uploads/logo.svg')
    expect(link?.type).toBe('image/svg+xml')

    updateFavicon('data:image/png;base64,abc')
    link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('data:image/png;base64,abc')
  })

  it.each(['javascript:alert(1)', '//example.com/logo.png'])('ignores unsafe logo URL %s', (logoUrl) => {
    updateFavicon(logoUrl)

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.png')
  })
})
