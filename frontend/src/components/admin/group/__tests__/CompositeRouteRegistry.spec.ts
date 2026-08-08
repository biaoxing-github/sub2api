import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import CompositeRouteRegistry from '../CompositeRouteRegistry.vue'
import groupsAPI from '@/api/admin/groups'
import type { AdminGroup, CompositeModelRoute } from '@/types'

vi.mock('@/api/admin/groups', () => ({
  default: {
    listCompositeRoutes: vi.fn(),
    createCompositeRoute: vi.fn(),
    updateCompositeRoute: vi.fn(),
    deleteCompositeRoute: vi.fn(),
    previewCompositeRoute: vi.fn()
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'en-US' } })
}))

const group = {
  id: 42,
  name: 'Composite Main',
  platform: 'composite'
} as AdminGroup

const route: CompositeModelRoute = {
  id: 7,
  group_id: 42,
  public_model: 'vendor/gpt',
  match_type: 'prefix',
  target_platform: 'openai',
  upstream_model: 'gpt-5.5',
  endpoint: 'responses',
  priority: 3,
  enabled: true,
  notes: 'primary'
}

function mountRegistry() {
  return mount(CompositeRouteRegistry, {
    props: { show: true, group },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><slot /></div>'
        },
        PlatformIcon: true,
        Icon: true
      }
    }
  })
}

describe('CompositeRouteRegistry', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(groupsAPI.listCompositeRoutes).mockResolvedValue([route])
  })

  it('loads routes and creates a normalized route', async () => {
    vi.mocked(groupsAPI.createCompositeRoute).mockResolvedValue({ ...route, id: 8 })
    const wrapper = mountRegistry()
    await flushPromises()

    expect(groupsAPI.listCompositeRoutes).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('vendor/gpt')

    await wrapper.find('#composite-route-public-model').setValue('  alias/model  ')
    await wrapper.find('#composite-route-upstream-model').setValue('  upstream/model  ')

    await wrapper.find('[data-testid="route-match-type"]').setValue('prefix')
    await wrapper.find('[data-testid="route-endpoint"]').setValue('responses')
    await wrapper.find('[data-testid="route-target-platform"]').setValue('anthropic')
    await wrapper.find('[data-testid="composite-route-form"]').trigger('submit')
    await flushPromises()

    expect(groupsAPI.createCompositeRoute).toHaveBeenCalledWith(
      42,
      expect.objectContaining({
        public_model: 'alias/model',
        upstream_model: 'upstream/model',
        match_type: 'prefix',
        endpoint: 'responses',
        target_platform: 'anthropic',
        enabled: true
      })
    )
  })

  it('edits, deletes and previews a route', async () => {
    vi.mocked(groupsAPI.updateCompositeRoute).mockResolvedValue(route)
    vi.mocked(groupsAPI.deleteCompositeRoute).mockResolvedValue({ message: 'ok' })
    vi.mocked(groupsAPI.previewCompositeRoute).mockResolvedValue({
      matched: true,
      source: 'route',
      group_id: 42,
      public_model: 'vendor/gpt',
      target_platform: 'openai',
      upstream_model: 'gpt-5.5',
      endpoint: 'responses',
      route
    })
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true)
    const wrapper = mountRegistry()
    await flushPromises()

    await wrapper.find('[data-testid="edit-composite-route-7"]').trigger('click')
    await wrapper.find('[data-testid="composite-route-form"]').trigger('submit')
    await flushPromises()
    expect(groupsAPI.updateCompositeRoute).toHaveBeenCalledWith(
      42,
      7,
      expect.objectContaining({ public_model: 'vendor/gpt' })
    )

    await wrapper.find('#composite-route-preview-model').setValue('vendor/gpt')
    await wrapper.find('[data-testid="preview-route-endpoint"]').setValue('responses')
    await wrapper.find('[data-testid="preview-composite-route"]').trigger('click')
    await flushPromises()
    expect(groupsAPI.previewCompositeRoute).toHaveBeenCalledWith(42, {
      model: 'vendor/gpt',
      endpoint: 'responses'
    })
    expect(wrapper.find('[data-testid="composite-route-preview-result"]').text()).toContain('gpt-5.5')

    await wrapper.find('[data-testid="delete-composite-route-7"]').trigger('click')
    await flushPromises()
    expect(groupsAPI.deleteCompositeRoute).toHaveBeenCalledWith(42, 7)
  })
})
