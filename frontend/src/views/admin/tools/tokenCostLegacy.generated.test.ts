import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  mountTokenCostLegacyTool,
  tokenCostLegacyBodyHtml
} from './tokenCostLegacy.generated'
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

describe('mountTokenCostLegacyTool', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
  })

  it('loads latest SQL state from injected service instead of stale local data', async () => {
    const serverState = createState(21)
    localStorage.setItem('token-api-cost-live-calculator:v1', JSON.stringify(createState(16)))
    localStorage.setItem('token-api-cost-live-calculator:v2', JSON.stringify(createState(16)))
    const tokenCostService = {
      health: vi.fn(async () => ({ ok: true, state: 'sql:token-cost', storage: 'sql:token-cost' })),
      loadState: vi.fn(async () => serverState),
      saveState: vi.fn(async (state: unknown) => state)
    }

    const root = document.createElement('div')
    root.innerHTML = tokenCostLegacyBodyHtml
    document.body.appendChild(root)

    const scope: LegacyToolScope = {
      document: createDocumentFacade(root),
      window: createWindowFacade(),
      services: {
        tokenCost: tokenCostService
      },
      localStorage,
      fetch: vi.fn() as unknown as typeof fetch,
      Headers,
      CSS: window.CSS,
      structuredClone: structuredClone,
      console,
      cleanup: vi.fn()
    }

    const cleanup = mountTokenCostLegacyTool(scope)

    await vi.waitFor(() => {
      expect(root.querySelectorAll('.platform-card')).toHaveLength(6)
    })
    expect(tokenCostService.health).toHaveBeenCalledTimes(1)
    expect(tokenCostService.loadState).toHaveBeenCalledTimes(1)
    expect(root.querySelector('#tableMeta')?.textContent).toContain('21 个平台')
    expect(localStorage.getItem('token-api-cost-live-calculator:v1')).toBe(JSON.stringify(createState(16)))
    expect(localStorage.getItem('token-api-cost-live-calculator:v2')).toBe(JSON.stringify(createState(16)))

    cleanup()
  })
})
