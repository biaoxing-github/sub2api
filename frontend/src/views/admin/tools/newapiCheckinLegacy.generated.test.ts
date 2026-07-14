import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  mountNewapiCheckinLegacyTool,
  newapiCheckinLegacyBodyHtml
} from './newapiCheckinLegacy.generated'
import type { LegacyDocumentFacade, LegacyToolScope, LegacyWindowFacade } from './legacyToolRuntime'

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

function jsonResponse(data: unknown) {
  return Promise.resolve(new Response(JSON.stringify({ code: 0, data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  }))
}

describe('mountNewapiCheckinLegacyTool', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
  })

  it('shows masked API keys with sk prefix and missing state in the account catalog', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.endsWith('/config')) {
        return jsonResponse({
          all_site_count: 1,
          enabled_site_count: 1,
          all_account_count: 2,
          enabled_account_count: 2,
          sites: [{
            name: 'demo',
            enabled: true,
            disabled_reason: '',
            base_url: 'https://demo.example',
            accounts: [
              { name: 'alpha', username: 'alpha', display_name: 'Alpha', label: 'Alpha', user_id: '1001', ip_profile: 'slot-a' },
              { name: 'beta', username: 'beta', display_name: 'Beta', label: 'Beta', user_id: '1002', ip_profile: 'slot-b' }
            ]
          }]
        })
      }
      if (url.endsWith('/api-keys')) {
        return jsonResponse({
          account_count: 2,
          available_count: 1,
          missing_count: 1,
          error_count: 0,
          accounts: [
            { site: 'demo', user_id: '1001', status: 'ready', api_keys: [{ name: 'codex', masked_key: 'sk-ab***5678' }] },
            { site: 'demo', user_id: '1002', status: 'missing', api_keys: [] }
          ]
        })
      }
      if (url.endsWith('/last-run')) return jsonResponse({})
      if (url.endsWith('/balances')) return jsonResponse({ site_statuses: {}, accounts: [] })
      if (url.endsWith('/history')) return jsonResponse({ entries: [], daily_summaries: [], site_summaries: [], account_summaries: [] })
      if (url.includes('/monthly?')) {
        return jsonResponse({
          records: [],
          site_summaries: [],
          account_summaries: [],
          site_daily_summaries: [],
          account_daily_summaries: [],
          available_months: [],
          sync_state: {}
        })
      }
      throw new Error(`unexpected request: ${url}`)
    })

    const root = document.createElement('div')
    root.innerHTML = newapiCheckinLegacyBodyHtml
    document.body.appendChild(root)

    const scope: LegacyToolScope = {
      document: createDocumentFacade(root),
      window: createWindowFacade(),
      services: {},
      localStorage,
      fetch: fetchMock as unknown as typeof fetch,
      Headers,
      CSS: window.CSS,
      structuredClone,
      console,
      cleanup: vi.fn()
    }

    const cleanup = mountNewapiCheckinLegacyTool(scope)

    await vi.waitFor(() => {
      expect(root.querySelector('#configTableWrap')?.textContent).toContain('sk-ab***5678')
    })
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('未生成')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/newapi-checkin/api-keys',
      expect.any(Object)
    )

    cleanup()
  })
})
