<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AccountModelMappingList from '@/components/account/AccountModelMappingList.vue'
import Select from '@/components/common/Select.vue'
import type { OpenAICompactMode, SelectOption } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

const props = defineProps<{
  compactMode: OpenAICompactMode
  compactModeOptions: SelectOption[]
  compactModelMappings: ModelMapping[]
  statusKey?: string
  lastCheckedText?: string
}>()

const emit = defineEmits<{
  'update:compactMode': [value: OpenAICompactMode]
  'update:compactModelMappings': [value: ModelMapping[]]
}>()

const { t } = useI18n()

const updateCompactMode = (value: string | number | boolean | null) => {
  emit('update:compactMode', (value || 'auto') as OpenAICompactMode)
}
</script>

<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600 space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.openai.compactMode') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.openai.compactModeDesc') }}
        </p>
      </div>
      <div class="w-44">
        <Select
          :model-value="props.compactMode"
          :options="props.compactModeOptions"
          @update:model-value="updateCompactMode"
        />
      </div>
    </div>
    <div
      v-if="props.statusKey"
      class="rounded-lg bg-slate-50 p-3 text-xs text-slate-600 dark:bg-dark-700/60 dark:text-gray-300"
    >
      <span class="font-medium">{{ t(props.statusKey) }}</span>
      <span v-if="props.lastCheckedText" class="ml-2 text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.openai.compactLastChecked') }}:
        {{ props.lastCheckedText }}
      </span>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.openai.compactModelMapping') }}</label>
      <p class="input-hint">{{ t('admin.accounts.openai.compactModelMappingDesc') }}</p>
      <AccountModelMappingList
        :model-mappings="props.compactModelMappings"
        :preset-mappings="[]"
        description-key="admin.accounts.openai.compactModelMappingDesc"
        from-placeholder-key="admin.accounts.fromModel"
        to-placeholder-key="admin.accounts.toModel"
        @update:model-mappings="emit('update:compactModelMappings', $event)"
      />
    </div>
  </div>
</template>
