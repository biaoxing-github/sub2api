import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBasicInfoFields from '../AccountBasicInfoFields.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountBasicInfoFields', () => {
  it('emits account name and notes updates', async () => {
    const wrapper = mount(AccountBasicInfoFields, {
      props: {
        name: 'old-name',
        notes: 'old-notes',
        nameLabelKey: 'common.name',
        nameTour: 'account-name-tour'
      }
    })

    const nameInput = wrapper.get('input')
    const notesTextarea = wrapper.get('textarea')

    expect(nameInput.attributes('data-tour')).toBe('account-name-tour')
    expect(wrapper.text()).toContain('admin.accounts.notesHint')

    await nameInput.setValue('new-name')
    await notesTextarea.setValue('new-notes')

    expect(wrapper.emitted('update:name')?.[0]).toEqual(['new-name'])
    expect(wrapper.emitted('update:notes')?.[0]).toEqual(['new-notes'])
  })
})
