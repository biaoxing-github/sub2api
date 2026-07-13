<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

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
  platform: string
  accountId?: number
  mode: 'whitelist' | 'mapping'
  allowedModels: string[]
  modelMappings: ModelMapping[]
  presetMappings: PresetMapping[]
  disabled?: boolean
  disabledHintKey?: string
  supportsAllRequiresEmptyMappings?: boolean
  allowDuplicatePresets?: boolean
  fromPlaceholderKey?: string
  toPlaceholderKey?: string
}>(), {
  accountId: undefined,
  disabled: false,
  disabledHintKey: '',
  supportsAllRequiresEmptyMappings: false,
  allowDuplicatePresets: false,
  fromPlaceholderKey: 'admin.accounts.requestModel',
  toPlaceholderKey: 'admin.accounts.actualModel'
})

const emit = defineEmits<{
  'update:mode': [value: 'whitelist' | 'mapping']
  'update:allowedModels': [value: string[]]
  'update:modelMappings': [value: ModelMapping[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const showSupportsAll = computed(() => {
  if (props.allowedModels.length > 0) return false
  return !props.supportsAllRequiresEmptyMappings || props.modelMappings.length === 0
})

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
  <div
    data-testid="account-model-restriction-section"
    class="border-t border-gray-200 pt-4 dark:border-dark-600"
  >
    <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>

    <div
      v-if="props.disabled"
      class="mb-3 rounded-lg bg-amber-50 p-3 dark:bg-amber-900/20"
    >
      <p class="text-xs text-amber-700 dark:text-amber-400">
        {{ props.disabledHintKey ? t(props.disabledHintKey) : t('admin.accounts.modelRestriction') }}
      </p>
    </div>

    <template v-else>
      <div class="mb-4 flex gap-2">
        <button
          type="button"
          data-testid="model-mode-whitelist"
          :class="[
            'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
            props.mode === 'whitelist'
              ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500'
          ]"
          @click="emit('update:mode', 'whitelist')"
        >
          <Icon name="checkCircle" size="sm" class="mr-1.5 inline" :stroke-width="2" />
          {{ t('admin.accounts.modelWhitelist') }}
        </button>
        <button
          type="button"
          data-testid="model-mode-mapping"
          :class="[
            'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
            props.mode === 'mapping'
              ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500'
          ]"
          @click="emit('update:mode', 'mapping')"
        >
          <Icon name="swap" size="sm" class="mr-1.5 inline" :stroke-width="2" />
          {{ t('admin.accounts.modelMapping') }}
        </button>
      </div>

      <div v-if="props.mode === 'whitelist'">
        <ModelWhitelistSelector
          :model-value="props.allowedModels"
          :platform="props.platform"
          :account-id="props.accountId"
          @update:model-value="emit('update:allowedModels', $event)"
        />
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.selectedModels', { count: props.allowedModels.length }) }}
          <span v-if="showSupportsAll">{{ t('admin.accounts.supportsAllModels') }}</span>
        </p>
      </div>

      <div v-else>
        <div class="mb-3 rounded-lg bg-slate-50 p-3 dark:bg-dark-700/60">
          <p class="text-xs text-slate-600 dark:text-gray-300">
            <Icon name="infoCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
            {{ t('admin.accounts.mapRequestModels') }}
          </p>
        </div>

        <div v-if="props.modelMappings.length > 0" class="mb-3 space-y-2">
          <div
            v-for="(mapping, index) in props.modelMappings"
            :key="index"
            class="flex items-center gap-2"
          >
            <input
              :value="mapping.from"
              type="text"
              class="input flex-1"
              :placeholder="t(props.fromPlaceholderKey)"
              :data-testid="`mapping-from-${index}`"
              @input="updateMapping(index, 'from', ($event.target as HTMLInputElement).value)"
            />
            <Icon name="arrowRight" size="sm" class="h-4 w-4 flex-shrink-0 text-gray-400" :stroke-width="2" />
            <input
              :value="mapping.to"
              type="text"
              class="input flex-1"
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
        </div>

        <button
          type="button"
          data-testid="add-model-mapping"
          class="mb-3 w-full rounded-lg border-2 border-dashed border-gray-300 px-4 py-2 text-gray-600 transition-colors hover:border-gray-400 hover:text-gray-700 dark:border-dark-500 dark:text-gray-400 dark:hover:border-dark-400 dark:hover:text-gray-300"
          @click="addMapping"
        >
          <Icon name="plus" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.addMapping') }}
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
  </div>
</template>
