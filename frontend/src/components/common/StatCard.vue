<template>
  <div class="card p-4">
    <div class="flex items-center gap-3">
      <div :class="['rounded-lg p-2', toneClasses[tone]]">
        <Icon :name="icon" size="md" :stroke-width="2" />
      </div>
      <div class="min-w-0 flex-1">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ label }}
        </p>
        <p class="text-xl font-bold text-gray-900 dark:text-white">
          {{ value }}
        </p>
        <p v-if="sub" class="text-xs text-gray-500 dark:text-gray-400">
          {{ sub }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '../icons/Icon.vue'

type IconName = InstanceType<typeof Icon>['$props']['name']

interface Props {
  icon: IconName
  label: string
  value: string | number
  sub?: string
  tone?: 'neutral' | 'success' | 'warning'
}

withDefaults(defineProps<Props>(), {
  tone: 'neutral'
})

const toneClasses = {
  neutral: 'bg-slate-100 text-slate-600 dark:bg-slate-900/30 dark:text-slate-400',
  success: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400',
  warning: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400'
}
</script>
