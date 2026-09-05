import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ReAuthAccountModal from '../ReAuthAccountModal.vue'
import type { Account } from '@/types'

const state = vi.hoisted(() => ({
  client: {
    authUrl: { value: '' }, sessionId: { value: '' }, loading: { value: false }, error: { value: '' },
    validateRefreshToken: vi.fn(), buildCredentials: vi.fn(), buildExtraInfo: vi.fn(), resetState: vi.fn(),
  },
  update: vi.fn(), clearError: vi.fn(), success: vi.fn(), error: vi.fn(),
}))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: state.success, showError: state.error }) }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { update: state.update, clearError: state.clearError } } }))
vi.mock('@/composables/useAccountOAuth', () => ({ useAccountOAuth: () => state.client }))
vi.mock('@/composables/useOpenAIOAuth', () => ({ useOpenAIOAuth: () => state.client }))
vi.mock('@/composables/useAntigravityOAuth', () => ({ useAntigravityOAuth: () => state.client }))
vi.mock('@/composables/useGeminiOAuth', () => ({ useGeminiOAuth: () => state.client }))
vi.mock('@/composables/useGrokOAuth', () => ({ useGrokOAuth: () => state.client }))

function mountModal(platform: string) {
  return mount(ReAuthAccountModal, {
    props: { show: true, account: { id: 42, platform, type: 'oauth', proxy_id: 7 } as Account },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true, OAuthAuthorizationFlow: true } },
  })
}

describe('refresh token reauthorization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    state.client.validateRefreshToken.mockResolvedValue({ access_token: 'new-access' })
    state.client.buildCredentials.mockReturnValue({ access_token: 'new-access', refresh_token: 'new-refresh' })
    state.client.buildExtraInfo.mockReturnValue({ plan_type: 'plus' })
    state.update.mockResolvedValue({ id: 42 })
    state.clearError.mockResolvedValue({ id: 42, status: 'active' })
  })

  it.each(['openai', 'antigravity'])('updates the existing %s account before clearing its error', async (platform) => {
    const wrapper = mountModal(platform)
    const flow = wrapper.findComponent({ name: 'OAuthAuthorizationFlow' })
    expect(flow.props('showRefreshTokenOption')).toBe(true)
    flow.vm.$emit('validate-refresh-token', '  new-refresh \nignored-second-token')
    await flushPromises()
    expect(state.client.validateRefreshToken).toHaveBeenCalledWith('new-refresh', 7)
    expect(state.update).toHaveBeenCalledWith(42, expect.objectContaining({ type: 'oauth', credentials: { access_token: 'new-access', refresh_token: 'new-refresh' } }))
    expect(state.clearError).toHaveBeenCalledWith(42)
    expect(state.update.mock.invocationCallOrder[0]).toBeLessThan(state.clearError.mock.invocationCallOrder[0]!)
    expect(wrapper.emitted('reauthorized')).toEqual([[{ id: 42, status: 'active' }]])
  })

  it('keeps the error state when credential persistence fails', async () => {
    state.update.mockRejectedValue(new Error('save failed'))
    const wrapper = mountModal('openai')
    wrapper.findComponent({ name: 'OAuthAuthorizationFlow' }).vm.$emit('validate-refresh-token', 'refresh')
    await flushPromises()
    expect(state.clearError).not.toHaveBeenCalled()
    expect(wrapper.emitted('reauthorized')).toBeUndefined()
    expect(state.error).toHaveBeenCalledWith('save failed')
  })
})
