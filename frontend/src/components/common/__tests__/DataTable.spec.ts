import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import DataTable from '../DataTable.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('DataTable sorting', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn()
      })
    })
  })

  it('uses the column default order on the first click', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name' },
          { key: 'cost', label: 'Cost', sortable: true, defaultSortOrder: 'desc' }
        ],
        data: [{ id: 1, name: 'A', cost: 3 }],
        serverSideSort: true
      },
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('th:nth-child(2)').trigger('click')

    expect(wrapper.emitted('sort')).toEqual([['cost', 'desc']])
  })
})
