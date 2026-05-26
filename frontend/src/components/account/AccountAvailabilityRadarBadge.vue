<template>
  <div v-if="advice" class="inline-flex max-w-full flex-wrap items-center gap-1.5">
    <span
      :class="[
        'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium leading-4',
        radarClass
      ]"
      :title="title"
    >
      <span :class="['h-1.5 w-1.5 rounded-full', dotClass]" />
      {{ label }}
    </span>
    <span
      v-if="advice.suggested_load_factor"
      class="inline-flex items-center rounded-md border border-slate-200 bg-slate-50 px-1.5 py-0.5 text-[11px] font-medium leading-4 text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
      :title="suggestionTitle"
    >
      {{ suggestionLabel }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AccountLoadFactorAdvice, AccountAvailabilityRadarStatus } from '@/types'

const props = defineProps<{
  advice?: AccountLoadFactorAdvice | null
}>()

const advice = computed(() => props.advice ?? null)
const status = computed<AccountAvailabilityRadarStatus>(() => advice.value?.availability_radar.status ?? 'needs_probe')
const label = computed(() => advice.value?.availability_radar.label || statusLabel(status.value))
const reasons = computed(() => advice.value?.reasons ?? advice.value?.availability_radar.reasons ?? [])
const title = computed(() => reasons.value.length > 0 ? reasons.value.join('\n') : label.value)
const suggestionLabel = computed(() => `建议 ${advice.value?.suggested_load_factor}`)
const suggestionTitle = computed(() => reasons.value.length > 0 ? reasons.value.join('\n') : suggestionLabel.value)

const radarClass = computed(() => {
  switch (status.value) {
    case 'fast_stable':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'slow_usable':
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'
    case 'unstable':
      return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-800 dark:bg-rose-900/20 dark:text-rose-300'
    case 'balance_risk':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'
    case 'cooldown':
      return 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-900/20 dark:text-violet-300'
    case 'needs_probe':
    default:
      return 'border-gray-200 bg-gray-50 text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'
  }
})

const dotClass = computed(() => {
  switch (status.value) {
    case 'fast_stable':
      return 'bg-emerald-500'
    case 'slow_usable':
      return 'bg-sky-500'
    case 'unstable':
      return 'bg-rose-500'
    case 'balance_risk':
      return 'bg-amber-500'
    case 'cooldown':
      return 'bg-violet-500'
    case 'needs_probe':
    default:
      return 'bg-gray-400'
  }
})

function statusLabel(value: AccountAvailabilityRadarStatus): string {
  switch (value) {
    case 'fast_stable':
      return '快且稳'
    case 'slow_usable':
      return '可用偏慢'
    case 'unstable':
      return '不稳定'
    case 'balance_risk':
      return '余额风险'
    case 'cooldown':
      return '冷却中'
    case 'needs_probe':
    default:
      return '待探测'
  }
}
</script>
