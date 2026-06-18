<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { isValidWildcardPattern } from '@/composables/useModelWhitelist'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'

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
  validateWildcard?: boolean
  showSyncButton?: boolean
  syncLoading?: boolean
  syncDisabled?: boolean
  allowDuplicatePresets?: boolean
  fromPlaceholderKey?: string
  toPlaceholderKey?: string
  descriptionKey?: string
  addLabelKey?: string
}>(), {
  presetMappings: () => [],
  validateWildcard: false,
  showSyncButton: false,
  syncLoading: false,
  syncDisabled: false,
  allowDuplicatePresets: false,
  fromPlaceholderKey: 'admin.accounts.requestModel',
  toPlaceholderKey: 'admin.accounts.actualModel',
  descriptionKey: 'admin.accounts.mapRequestModels',
  addLabelKey: 'admin.accounts.addMapping'
})

const emit = defineEmits<{
  'update:modelMappings': [value: ModelMapping[]]
  sync: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const getModelMappingKey = createStableObjectKeyResolver<ModelMapping>('account-model-mapping')

const cloneMappings = () => props.modelMappings.map(mapping => ({ ...mapping }))

const updateMapping = (index: number, field: keyof ModelMapping, value: string) => {
  const mappings = cloneMappings()
  if (!mappings[index]) return
  mappings[index][field] = value
  emit('update:modelMappings', mappings)
}

const addMapping = () => {
  emit('update:modelMappings', [...cloneMappings(), { from: '', to: '' }])
}

const removeMapping = (index: number) => {
  emit('update:modelMappings', cloneMappings().filter((_, currentIndex) => currentIndex !== index))
}

const addPresetMapping = (preset: PresetMapping) => {
  if (!props.allowDuplicatePresets && props.modelMappings.some(mapping => mapping.from === preset.from)) {
    appStore.showInfo(t('admin.accounts.mappingExists', { model: preset.from }))
    return
  }
  emit('update:modelMappings', [...cloneMappings(), { from: preset.from, to: preset.to }])
}
</script>

<template>
  <div>
    <div class="mb-3 rounded-lg bg-slate-50 p-3 dark:bg-dark-700/60">
      <p class="text-xs text-slate-600 dark:text-gray-300">
        {{ t(props.descriptionKey) }}
      </p>
    </div>

    <div v-if="props.showSyncButton" class="mb-3 flex flex-wrap gap-2">
      <button
        type="button"
        data-testid="sync-upstream-models"
        :disabled="props.syncLoading || props.syncDisabled"
        class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm text-slate-600 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-500 dark:text-gray-300 dark:hover:bg-dark-600"
        @click="emit('sync')"
      >
        {{ props.syncLoading ? t('admin.accounts.syncUpstreamModelsLoading') : t('admin.accounts.syncUpstreamModels') }}
      </button>
    </div>

    <div v-if="props.modelMappings.length > 0" class="mb-3 space-y-2">
      <div
        v-for="(mapping, index) in props.modelMappings"
        :key="getModelMappingKey(mapping)"
        class="space-y-1"
      >
        <div class="flex items-center gap-2">
          <input
            :value="mapping.from"
            type="text"
            :class="[
              'input flex-1',
              props.validateWildcard && !isValidWildcardPattern(mapping.from)
                ? 'border-red-500 dark:border-red-500'
                : ''
            ]"
            :placeholder="t(props.fromPlaceholderKey)"
            :data-testid="`mapping-from-${index}`"
            @input="updateMapping(index, 'from', ($event.target as HTMLInputElement).value)"
          />
          <Icon name="arrowRight" size="sm" class="h-4 w-4 flex-shrink-0 text-gray-400" :stroke-width="2" />
          <input
            :value="mapping.to"
            type="text"
            :class="[
              'input flex-1',
              props.validateWildcard && mapping.to.includes('*')
                ? 'border-red-500 dark:border-red-500'
                : ''
            ]"
            :placeholder="t(props.toPlaceholderKey)"
            :data-testid="`mapping-to-${index}`"
            @input="updateMapping(index, 'to', ($event.target as HTMLInputElement).value)"
          />
          <button
            type="button"
            :data-testid="`remove-model-mapping-${index}`"
            class="rounded-lg p-2 text-red-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
            @click="removeMapping(index)"
          >
            <Icon name="trash" size="sm" :stroke-width="2" />
          </button>
        </div>
        <p
          v-if="props.validateWildcard && !isValidWildcardPattern(mapping.from)"
          class="text-xs text-red-500"
        >
          {{ t('admin.accounts.wildcardOnlyAtEnd') }}
        </p>
        <p
          v-if="props.validateWildcard && mapping.to.includes('*')"
          class="text-xs text-red-500"
        >
          {{ t('admin.accounts.targetNoWildcard') }}
        </p>
      </div>
    </div>

    <button
      type="button"
      data-testid="add-model-mapping"
      class="mb-3 w-full rounded-lg border-2 border-dashed border-gray-300 px-4 py-2 text-gray-600 transition-colors hover:border-gray-400 hover:text-gray-700 dark:border-dark-500 dark:text-gray-400 dark:hover:border-dark-400 dark:hover:text-gray-300"
      @click="addMapping"
    >
      <Icon name="plus" size="sm" class="mr-1 inline" :stroke-width="2" />
      {{ t(props.addLabelKey) }}
    </button>

    <div v-if="props.presetMappings.length > 0" class="flex flex-wrap gap-2">
      <button
        v-for="preset in props.presetMappings"
        :key="`${preset.from}-${preset.to}`"
        type="button"
        :data-testid="`preset-mapping-${preset.from}`"
        :class="['rounded-lg px-3 py-1 text-xs transition-colors', preset.color]"
        @click="addPresetMapping(preset)"
      >
        + {{ preset.label }}
      </button>
    </div>
  </div>
</template>
