import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  mountNewapiCheckinLegacyTool,
  newapiCheckinLegacyBodyHtml,
  newapiCheckinLegacyStyles
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
    confirm: vi.fn(() => true),
    prompt: vi.fn(() => 'owner@example.com')
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

  it('keeps overview cards readable when the embedded tool becomes narrow', () => {
    expect(newapiCheckinLegacyStyles).toMatch(
      /\.overview-site-grid,[\s\S]*?grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*300px\),\s*1fr\)\)/
    )
    expect(newapiCheckinLegacyStyles).toMatch(
      /\.overview-top\s*\{[^}]*flex-wrap:\s*wrap/
    )
    expect(newapiCheckinLegacyStyles).toMatch(
      /\.overview-metrics\s*\{[^}]*grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(92px,\s*1fr\)\)/
    )
    expect(newapiCheckinLegacyStyles).toMatch(
      /\.overview-metric-value\s*\{[^}]*white-space:\s*nowrap/
    )
    expect(newapiCheckinLegacyStyles).not.toMatch(
      /\.overview-metric-value\s*\{[^}]*word-break:\s*break-all/
    )
  })

  it('opens the platform directory modal and manages generated NewAPI keys and groups', async () => {
    let revealPayload: Record<string, unknown> | null = null
    let groupPayload: Record<string, unknown> | null = null
    let linkPayload: Record<string, unknown> | null = null
	let syncMethod = ''
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
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
            {
              site: 'demo', provider: 'newapi', user_id: '1001', status: 'ready',
              group_status: 'ready', group_message: '已读取上游完整可用分组', available_groups: ['codex-team', 'default'],
              api_keys: [
                {
                  id: 8, name: 'other', masked_key: 'sk-zz***yyyy', group: 'default',
                  referenced_accounts: [],
                  target_accounts: [{ id: 91, name: 'demo-main', referenced: false }]
                },
                {
                  id: 7, name: 'codex', masked_key: 'sk-ab***5678', group: 'codex-team',
                  referenced_accounts: [{ id: 91, name: 'demo-main', referenced: true }],
                  target_accounts: [{ id: 91, name: 'demo-main', referenced: true }]
                }
              ]
            },
            { site: 'demo', provider: 'newapi', user_id: '1002', status: 'missing', group_status: 'partial', group_message: '仅展示已知分组', available_groups: ['default'], api_keys: [] }
          ]
        })
      }
	  if (url.endsWith('/api-keys/sync')) {
		syncMethod = String(init?.method || 'GET')
		return jsonResponse({ account_count: 2, available_count: 1, refreshed_count: 2, accounts: [] })
	  }
      if (url.endsWith('/reveal-api-key')) {
        revealPayload = JSON.parse(String(init?.body || '{}')) as Record<string, unknown>
        return jsonResponse({ site: 'demo', user_id: '1001', api_key_id: 7, name: 'codex', key: 'sk-abcdef12345678', masked_key: 'sk-ab***5678', group: 'codex-team' })
      }
      if (url.endsWith('/api-key-group')) {
        groupPayload = JSON.parse(String(init?.body || '{}')) as Record<string, unknown>
        return jsonResponse({ site: 'demo', user_id: '1001', api_key_id: 7, name: 'codex', group: 'default', message: 'updated' })
      }
      if (url.endsWith('/link-api-key')) {
        linkPayload = JSON.parse(String(init?.body || '{}')) as Record<string, unknown>
        return jsonResponse({ target_account_name: 'demo-main', key_count: 2, operation: 'append' })
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
    const directoryButton = root.querySelector('#openPlatformDirectoryBtn') as HTMLButtonElement
    directoryButton.click()
    expect(root.querySelector('#platformDirectoryDialog')?.hasAttribute('open')).toBe(true)
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('未生成')
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('已引用 · demo-main')
    expect(root.querySelector('.api-key-item.referenced')).toBeTruthy()
    expect(root.querySelector('.api-key-item')?.textContent).toContain('sk-ab***5678')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/newapi-checkin/api-keys',
      expect.any(Object)
    )
    const revealButton = Array.from(root.querySelectorAll('[data-reveal-generated-key]'))[0] as HTMLButtonElement
    revealButton.click()
    await vi.waitFor(() => expect(root.querySelector('#configTableWrap')?.textContent).toContain('sk-abcdef12345678'))
    expect(revealPayload).toEqual({ site: 'demo', user_id: '1001', api_key_id: 7 })

    const groupSelect = root.querySelector('[data-group-select="7"]') as HTMLSelectElement
    groupSelect.value = 'default'
    const updateButton = root.querySelector('[data-update-generated-group="7"]') as HTMLButtonElement
    updateButton.click()
    await vi.waitFor(() => expect(groupPayload).toEqual({ site: 'demo', user_id: '1001', api_key_id: 7, group: 'default', group_id: 0 }))

    ;(root.querySelector('[data-link-generated-key="7"][data-operation="append"]') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(linkPayload).toEqual({
      site: 'demo', user_id: '1001', api_key_id: 7, target_account_id: 91, operation: 'append'
    }))

    ;(root.querySelector('#refreshDirectoryKeysBtn') as HTMLButtonElement).click()
	await vi.waitFor(() => expect(syncMethod).toBe('POST'))

    cleanup()
  })

  it('renders sub2api subscription data and disables every checkin action', async () => {
    let savedIdentityPayload: Record<string, string> | null = null
    let testedLoginPayload: Record<string, string> | null = null
    let updatedGroupPayload: Record<string, unknown> | null = null
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.endsWith('/config')) {
        return jsonResponse({
          all_site_count: 1,
          enabled_site_count: 1,
          all_account_count: 1,
          enabled_account_count: 1,
          sites: [{
            name: 'sub2-demo',
            provider: 'sub2api',
            enabled: true,
            base_url: 'https://sub2.example',
            accounts: [{ name: 'primary', label: 'primary', user_id: 'primary', access_key: 'sk-test-full-key', access_key_masked: 'sk-90***84cd', login_username: 'owner@example.com', has_login_password: true }]
          }]
        })
      }
      if (url.endsWith('/api-keys')) return jsonResponse({
        account_count: 1,
        available_count: 1,
        accounts: [{
          site: 'sub2-demo', provider: 'sub2api', user_id: 'primary', status: 'ready',
          group_status: 'ready', group_message: '登录成功，已读取 2 个可用分组',
          available_groups: ['尝鲜套餐', 'codex--pro'],
          available_group_options: [{ id: 22, name: '尝鲜套餐' }, { id: 26, name: 'codex--pro' }],
          api_keys: [{ id: 1067, name: 'codex', masked_key: 'sk-90***84cd', group: '尝鲜套餐', group_id: 22 }]
        }]
      })
      if (url.endsWith('/test-login-credentials')) {
        testedLoginPayload = JSON.parse(String(init?.body || '{}')) as Record<string, string>
        return jsonResponse({ login_ok: true, message: '登录验证成功', api_key_count: 1, group_count: 2 })
      }
      if (url.endsWith('/api-key-group')) {
        updatedGroupPayload = JSON.parse(String(init?.body || '{}')) as Record<string, unknown>
        return jsonResponse({ site: 'sub2-demo', user_id: 'primary', api_key_id: 1067, name: 'codex', group: 'codex--pro', group_id: 26, message: 'updated' })
      }
      if (url.endsWith('/account-display-name')) {
        savedIdentityPayload = JSON.parse(String(init?.body || '{}')) as Record<string, string>
        return jsonResponse({
          all_site_count: 1,
          enabled_site_count: 1,
          all_account_count: 1,
          enabled_account_count: 1,
          sites: [{
            name: 'sub2-demo',
            provider: 'sub2api',
            enabled: true,
            base_url: 'https://sub2.example',
            accounts: [{ name: 'primary', display_name: 'owner@example.com', label: 'owner@example.com', user_id: 'primary', access_key: 'sk-test-full-key', access_key_masked: 'sk-90***84cd' }]
          }]
        })
      }
      if (url.endsWith('/last-run')) return jsonResponse({})
      if (url.endsWith('/balances')) {
        return jsonResponse({
          generated_at: '2026-07-15 12:00:00',
          site_count: 1,
          account_count: 1,
          overall: { display_totals: [] },
          site_summaries: [],
          site_statuses: {},
          accounts: [{
            site: 'sub2-demo',
            provider: 'sub2api',
            account: 'primary',
            label: 'primary',
            user_id: 'primary',
            status: '只读数据正常',
            quota_display: '$25',
            used_quota_display: '$0.0199075',
            provider_data: {
              plan_name: '尝鲜套餐',
              expires_at: '2026-08-12T13:55:02+08:00',
              usage: { total: { requests: 6, total_tokens: 7648 } },
              models: ['gpt-5.6-terra', 'gpt-5.6-sol']
            }
          }]
        })
      }
      if (url.endsWith('/history')) return jsonResponse({ entries: [], daily_summaries: [], site_summaries: [], account_summaries: [] })
      if (url.includes('/monthly?')) return jsonResponse({ records: [], site_summaries: [], account_summaries: [], site_daily_summaries: [], account_daily_summaries: [], available_months: [], sync_state: {} })
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
    await vi.waitFor(() => expect(root.querySelector('#configTableWrap')?.textContent).toContain('sub2api 只读'))
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('sk-90***84cd')
    expect(root.querySelector('#configTableWrap')?.textContent).not.toContain('sk-test-full-key')
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('登录 owner@example.com')
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('完整分组已读取')
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('$25')
    expect(root.querySelector('#configTableWrap')?.textContent).toContain('累计已用 $0.02')

    const revealKeyButton = Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === '显示')
    expect(revealKeyButton).toBeTruthy()
    revealKeyButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await vi.waitFor(() => expect(root.querySelector('#configTableWrap')?.textContent).toContain('sk-test-full-key'))

    const hideKeyButton = Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === '隐藏')
    expect(hideKeyButton).toBeTruthy()
    hideKeyButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await vi.waitFor(() => expect(root.querySelector('#configTableWrap')?.textContent).not.toContain('sk-test-full-key'))
    expect(root.querySelector('#balanceAccountTable')?.textContent).toContain('尝鲜套餐')
    expect(root.querySelector('#balanceAccountTable')?.textContent).toContain('7648 Token')
    expect(root.querySelector('#balanceAccountTable')?.textContent).toContain('2 个模型')
    expect((root.querySelector('#singleRunBtn') as HTMLButtonElement).disabled).toBe(true)
    expect((root.querySelector('#syncSiteNamesBtn') as HTMLButtonElement).disabled).toBe(true)
    expect((root.querySelector('#syncMonthlySiteBtn') as HTMLButtonElement).disabled).toBe(true)

    const editIdentityButton = Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === '编辑标识')
    expect(editIdentityButton).toBeTruthy()
    editIdentityButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await vi.waitFor(() => expect(savedIdentityPayload).toEqual({
      site: 'sub2-demo',
      user_id: 'primary',
      display_name: 'owner@example.com'
    }))
    await vi.waitFor(() => expect(root.querySelector('#configTableWrap')?.textContent).toContain('owner@example.com'))

    const loginButton = root.querySelector('[data-edit-login-credential="primary"]') as HTMLButtonElement
    loginButton.click()
    expect(root.querySelector('#loginCredentialDialog')?.hasAttribute('open')).toBe(true)
    expect((root.querySelector('#loginUsernameInput') as HTMLInputElement).value).toBe('owner@example.com')
    ;(root.querySelector('#loginPasswordInput') as HTMLInputElement).value = 'new-fixture-password'
    ;(root.querySelector('#testLoginCredentialBtn') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(testedLoginPayload).toEqual({
      site: 'sub2-demo',
      user_id: 'primary',
      login_username: 'owner@example.com',
      login_password: 'new-fixture-password'
    }))
    await vi.waitFor(() => expect(root.querySelector('#loginCredentialStatus')?.textContent).toContain('分组 2 个'))

    const sub2GroupSelect = root.querySelector('[data-group-select="1067"]') as HTMLSelectElement
    sub2GroupSelect.value = '26'
    ;(root.querySelector('[data-update-generated-group="1067"]') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(updatedGroupPayload).toEqual({
      site: 'sub2-demo', user_id: 'primary', api_key_id: 1067, group: 'codex--pro', group_id: 26
    }))

    cleanup()
  })
})
