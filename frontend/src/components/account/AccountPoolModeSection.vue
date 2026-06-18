<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  enabled: boolean
  retryCount: number
  defaultRetryCount: number
  maxRetryCount: number
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:retryCount': [value: number]
}>()

const { t } = useI18n()
</script>

<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.poolMode') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.poolModeHint') }}
        </p>
      </div>
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
          props.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
        ]"
        @click="emit('update:enabled', !props.enabled)"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            props.enabled ? 'translate-x-5' : 'translate-x-0'
          ]"
        />
      </button>
    </div>
    <div v-if="props.enabled" class="rounded-lg bg-slate-50 p-3 dark:bg-dark-700/60">
      <p class="text-xs text-slate-600 dark:text-gray-300">
        <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
        {{ t('admin.accounts.poolModeInfo') }}
      </p>
    </div>
    <div v-if="props.enabled" class="mt-3">
      <label class="input-label">{{ t('admin.accounts.poolModeRetryCount') }}</label>
      <input
        :value="props.retryCount"
        type="number"
        min="0"
        :max="props.maxRetryCount"
        step="1"
        class="input"
        @input="emit('update:retryCount', Number(($event.target as HTMLInputElement).value))"
      />
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{
          t('admin.accounts.poolModeRetryCountHint', {
            default: props.defaultRetryCount,
            max: props.maxRetryCount
          })
        }}
      </p>
    </div>
  </div>
</template>
