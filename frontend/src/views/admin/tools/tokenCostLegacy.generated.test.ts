import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  mountTokenCostLegacyTool,
  tokenCostLegacyBodyHtml
} from './tokenCostLegacy.generated'
import type { LegacyDocumentFacade, LegacyToolScope, LegacyWindowFacade } from './legacyToolRuntime'

const storageKey = 'token-api-cost-live-calculator:v1'

function createDocumentFacade(root: HTMLElement): LegacyDocumentFacade {
  return {
    getElementById(elementId: string) {
      return root.querySelector(`#${elementId}`) as HTMLElement | null
    },
    querySelector(selectors: string) {
      return root.querySelector(selectors)
    },
    querySelectorAll(selectors: string) {
      return root.querySelectorAll(selectors)
    },
    addEventListener(type: string, listener: EventListenerOrEventListenerObject, options?: boolean | AddEventListenerOptions) {
      document.addEventListener(type, listener, options)
    },
    removeEventListener(type: string, listener: EventListenerOrEventListenerObject, options?: boolean | EventListenerOptions) {
      document.removeEventListener(type, listener, options)
    },
    get activeElement() {
      return document.activeElement
    },
    startViewTransition(callback: () => void) {
      callback()
      return undefined
    }
  }
}

function createWindowFacade(): LegacyWindowFacade {
  return {
    CSS: window.CSS,
    location: {
      protocol: 'http:',
      href: 'http://127.0.0.1:8080/admin/tools'
    },
    setTimeout: window.setTimeout.bind(window),
    clearTimeout: window.clearTimeout.bind(window),
    setInterval: window.setInterval.bind(window),
    clearInterval: window.clearInterval.bind(window),
    requestAnimationFrame: window.requestAnimationFrame.bind(window),
    cancelAnimationFrame: window.cancelAnimationFrame.bind(window),
    confirm: vi.fn(() => true)
  }
}

function createState(platformCount: number) {
  return {
    version: 1,
    updatedAt: '2026-07-08T00:00:00+08:00',
    rankMode: 'plus',
    personalRechargeR: 400,
    platforms: Array.from({ length: platformCount }, (_, index) => ({
      id: `platform-${index + 1}`,
      name: `平台${index + 1}`,
      balanceUsd: index + 1,
      calcBalanceUsd: null,
      rateR: 1,
      rateUsd: 1,
      plus: 0.1,
      proMin: 0.2,
      proMax: null,
      note: ''
    })),
    history: [],
    events: []
  }
}

function htmlStateResponse(state: unknown): Response {
  const encoded = JSON.stringify(state)
    .replace(/&/g, '&amp;')
    .replace(/'/g, '&#39;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return new Response(`<!doctype html><html><body data-state='${encoded}'></body></html>`, {
    status: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' }
  })
}

describe('mountTokenCostLegacyTool', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
  })

  it('refreshes an expired admin token and replaces stale local platform data from SQL state', async () => {
    localStorage.setItem('auth_token', 'expired-token')
    localStorage.setItem('refresh_token', 'refresh-token')
    localStorage.setItem(storageKey, JSON.stringify(createState(16)))

    const serverState = createState(21)
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/admin/token-cost/health') && fetchMock.mock.calls.length === 1) {
        return new Response(JSON.stringify({ code: 401, message: 'expired' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' }
        })
      }
      if (url.includes('/auth/refresh')) {
        return new Response(JSON.stringify({
          code: 0,
          data: {
            access_token: 'fresh-token',
            refresh_token: 'fresh-refresh-token',
            expires_in: 3600
          }
        }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        })
      }
      if (url.includes('/admin/token-cost/health')) {
        return new Response(JSON.stringify({ ok: true, state: 'sql', storage: 'sql' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        })
      }
      if (url.includes('/admin/token-cost/state?view=page')) {
        return htmlStateResponse(serverState)
      }
      return new Response('', { status: 404 })
    }) as unknown as typeof fetch

    const root = document.createElement('div')
    root.innerHTML = tokenCostLegacyBodyHtml
    document.body.appendChild(root)

    const scope: LegacyToolScope = {
      document: createDocumentFacade(root),
      window: createWindowFacade(),
      localStorage,
      fetch: fetchMock,
      Headers,
      CSS: window.CSS,
      structuredClone: structuredClone,
      console,
      cleanup: vi.fn()
    }

    const cleanup = mountTokenCostLegacyTool(scope)

    await vi.waitFor(() => {
      const state = JSON.parse(localStorage.getItem(storageKey) || '{}')
      expect(state.platforms).toHaveLength(21)
    })
    expect(localStorage.getItem('auth_token')).toBe('fresh-token')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/token-cost/state?view=page',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer fresh-token' })
      })
    )

    cleanup()
  })
})
