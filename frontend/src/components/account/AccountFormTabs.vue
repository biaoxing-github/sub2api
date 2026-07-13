<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

export type AccountFormTab = 'basic' | 'advanced' | 'models'

const props = defineProps<{
  modelValue: AccountFormTab
}>()

const emit = defineEmits<{
  'update:modelValue': [value: AccountFormTab]
}>()

const { t } = useI18n()
const tabButtons = ref<HTMLButtonElement[]>([])

const tabs: Array<{ key: AccountFormTab; labelKey: string }> = [
  { key: 'basic', labelKey: 'admin.accounts.formTabs.basic' },
  { key: 'advanced', labelKey: 'admin.accounts.formTabs.advanced' },
  { key: 'models', labelKey: 'admin.accounts.formTabs.models' }
]

// 键盘切换后同步聚焦目标 Tab，保持弹窗表单可完整使用键盘操作。
const selectTab = async (tab: AccountFormTab, focusIndex?: number) => {
  emit('update:modelValue', tab)
  if (focusIndex === undefined) return
  await nextTick()
  tabButtons.value[focusIndex]?.focus()
}

const handleKeydown = (event: KeyboardEvent, currentIndex: number) => {
  let nextIndex: number | undefined
  if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
    nextIndex = (currentIndex + 1) % tabs.length
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
    nextIndex = (currentIndex - 1 + tabs.length) % tabs.length
  } else if (event.key === 'Home') {
    nextIndex = 0
  } else if (event.key === 'End') {
    nextIndex = tabs.length - 1
  }

  if (nextIndex === undefined) return
  event.preventDefault()
  void selectTab(tabs[nextIndex].key, nextIndex)
}
</script>

<template>
  <div
    role="tablist"
    :aria-label="t('admin.accounts.formTabs.label')"
    class="grid grid-cols-3 gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-700"
  >
    <button
      v-for="(tab, index) in tabs"
      :ref="(element) => { if (element) tabButtons[index] = element as HTMLButtonElement }"
      :key="tab.key"
      type="button"
      role="tab"
      :aria-selected="props.modelValue === tab.key"
      :tabindex="props.modelValue === tab.key ? 0 : -1"
      :data-testid="`account-form-tab-${tab.key}`"
      :class="[
        'min-h-9 rounded-md px-3 py-2 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-1 dark:focus:ring-offset-dark-700',
        props.modelValue === tab.key
          ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white'
          : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200'
      ]"
      @click="selectTab(tab.key)"
      @keydown="handleKeydown($event, index)"
    >
      {{ t(tab.labelKey) }}
    </button>
  </div>
</template>
