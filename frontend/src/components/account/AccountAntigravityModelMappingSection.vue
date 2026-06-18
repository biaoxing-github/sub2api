<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AccountModelMappingList from '@/components/account/AccountModelMappingList.vue'

interface ModelMapping {
  from: string
  to: string
}

interface PresetMapping {
  label: string
  from: string
  to: string
  color: string
}

const props = withDefaults(defineProps<{
  modelMappings: ModelMapping[]
  presetMappings?: PresetMapping[]
  showSyncButton?: boolean
  syncLoading?: boolean
  syncDisabled?: boolean
}>(), {
  presetMappings: () => [],
  showSyncButton: false,
  syncLoading: false,
  syncDisabled: false
})

const emit = defineEmits<{
  'update:modelMappings': [value: ModelMapping[]]
  sync: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
    <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>
    <AccountModelMappingList
      :model-mappings="props.modelMappings"
      :preset-mappings="props.presetMappings"
      :sync-loading="props.syncLoading"
      :sync-disabled="props.syncDisabled"
      :show-sync-button="props.showSyncButton"
      validate-wildcard
      @update:model-mappings="emit('update:modelMappings', $event)"
      @sync="emit('sync')"
    />
  </div>
</template>
