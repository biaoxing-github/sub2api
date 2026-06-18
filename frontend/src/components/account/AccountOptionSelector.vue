<script setup lang="ts">
import { computed } from 'vue'

import Icon from '@/components/icons/Icon.vue'

export type AccountOptionIconName = InstanceType<typeof Icon>['$props']['name']

export interface AccountOptionItem {
  value: string
  label: string
  description?: string
  icon: AccountOptionIconName
  disabled?: boolean
  testId?: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  options: AccountOptionItem[]
  variant?: 'segmented' | 'cards'
  columns?: 2 | 3 | 4
  dataTour?: string
}>(), {
  variant: 'cards',
  columns: 2,
  dataTour: undefined
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const containerClass = computed(() => {
  if (props.variant === 'segmented') {
    return 'mt-2 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700'
  }
  const columnsClass = {
    2: 'grid-cols-2',
    3: 'grid-cols-3',
    4: 'grid-cols-2 sm:grid-cols-4'
  }[props.columns]
  return `mt-2 grid ${columnsClass} gap-3`
})

const optionButtonClass = (item: AccountOptionItem) => {
  const selected = props.modelValue === item.value
  if (props.variant === 'segmented') {
    return [
      'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
      selected
        ? 'bg-white text-slate-700 shadow-sm dark:bg-dark-600 dark:text-gray-100'
        : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200',
      item.disabled ? 'cursor-not-allowed opacity-60' : ''
    ]
  }

  return [
    'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
    selected
      ? 'border-slate-400 bg-slate-50 dark:border-dark-400 dark:bg-dark-700/70'
      : 'border-gray-200 hover:border-gray-300 dark:border-dark-600 dark:hover:border-dark-500',
    item.disabled ? 'cursor-not-allowed opacity-60' : ''
  ]
}

const optionIconClass = (item: AccountOptionItem) => [
  'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
  props.modelValue === item.value
    ? 'bg-slate-700 text-white dark:bg-slate-200 dark:text-dark-900'
    : 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-400'
]

const selectOption = (item: AccountOptionItem) => {
  if (item.disabled) return
  emit('update:modelValue', item.value)
}
</script>

<template>
  <div :class="containerClass" :data-tour="props.dataTour">
    <button
      v-for="item in props.options"
      :key="item.value"
      type="button"
      :disabled="item.disabled"
      :data-testid="item.testId"
      :class="optionButtonClass(item)"
      @click="selectOption(item)"
    >
      <span v-if="props.variant === 'cards'" :class="optionIconClass(item)">
        <Icon :name="item.icon" size="sm" />
      </span>
      <Icon v-else :name="item.icon" size="sm" />
      <span v-if="props.variant === 'cards'" class="min-w-0">
        <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ item.label }}</span>
        <span v-if="item.description" class="text-xs text-gray-500 dark:text-gray-400">
          {{ item.description }}
        </span>
      </span>
      <span v-else>{{ item.label }}</span>
    </button>
  </div>
</template>
