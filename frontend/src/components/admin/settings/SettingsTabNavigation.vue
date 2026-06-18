<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'

export type SettingsTabNavigationKey =
  | 'general'
  | 'agreement'
  | 'features'
  | 'security'
  | 'users'
  | 'gateway'
  | 'payment'
  | 'email'
  | 'backup'

type SettingsTabIconName = InstanceType<typeof Icon>['$props']['name']

export interface SettingsTabNavigationItem {
  key: SettingsTabNavigationKey
  icon: SettingsTabIconName
}

const props = defineProps<{
  tabs: SettingsTabNavigationItem[]
  activeTab: SettingsTabNavigationKey
  ariaLabel: string
}>()

const emit = defineEmits<{
  select: [tab: SettingsTabNavigationKey]
}>()

const { t } = useI18n()

const tabKeyboardActions = {
  ArrowLeft: -1,
  ArrowUp: -1,
  ArrowRight: 1,
  ArrowDown: 1,
  Home: 'first',
  End: 'last'
} as const

function focusTab(tab: SettingsTabNavigationKey): void {
  window.requestAnimationFrame(() => {
    document.getElementById(`settings-tab-${tab}`)?.focus()
  })
}

function handleTabKeydown(event: KeyboardEvent, tab: SettingsTabNavigationKey): void {
  const action = tabKeyboardActions[event.key as keyof typeof tabKeyboardActions]
  if (action === undefined) {
    return
  }

  event.preventDefault()
  const currentIndex = props.tabs.findIndex((item) => item.key === tab)
  let nextIndex = currentIndex < 0 ? 0 : currentIndex

  if (action === 'first') {
    nextIndex = 0
  } else if (action === 'last') {
    nextIndex = props.tabs.length - 1
  } else {
    nextIndex = (nextIndex + action + props.tabs.length) % props.tabs.length
  }

  const nextTab = props.tabs[nextIndex]?.key
  if (!nextTab) {
    return
  }

  emit('select', nextTab)
  focusTab(nextTab)
}
</script>

<template>
  <div class="settings-tabs-shell">
    <nav
      class="settings-tabs-scroll"
      role="tablist"
      :aria-label="props.ariaLabel"
    >
      <div class="settings-tabs">
        <button
          v-for="tab in props.tabs"
          :key="tab.key"
          :id="`settings-tab-${tab.key}`"
          type="button"
          role="tab"
          :aria-selected="props.activeTab === tab.key"
          :tabindex="props.activeTab === tab.key ? 0 : -1"
          :class="[
            'settings-tab',
            props.activeTab === tab.key && 'settings-tab-active'
          ]"
          @click="emit('select', tab.key)"
          @keydown="handleTabKeydown($event, tab.key)"
        >
          <span class="settings-tab-icon">
            <Icon :name="tab.icon" size="sm" />
          </span>
          <span class="settings-tab-label">
            {{ t(`admin.settings.tabs.${tab.key}`) }}
          </span>
        </button>
      </div>
    </nav>
  </div>
</template>
