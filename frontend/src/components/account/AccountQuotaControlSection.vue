<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import QuotaLimitCard from '@/components/account/QuotaLimitCard.vue'
import type { QuotaResetMode, QuotaThresholdType } from '@/constants/account'

const props = withDefaults(defineProps<{
  hintKey: string
  totalLimit: number | null
  dailyLimit: number | null
  weeklyLimit: number | null
  dailyResetMode: QuotaResetMode | null
  dailyResetHour: number | null
  weeklyResetMode: QuotaResetMode | null
  weeklyResetDay: number | null
  weeklyResetHour: number | null
  resetTimezone: string | null
  quotaNotifyGlobalEnabled?: boolean
  quotaNotifyDailyEnabled?: boolean | null
  quotaNotifyDailyThreshold?: number | null
  quotaNotifyDailyThresholdType?: QuotaThresholdType | null
  quotaNotifyWeeklyEnabled?: boolean | null
  quotaNotifyWeeklyThreshold?: number | null
  quotaNotifyWeeklyThresholdType?: QuotaThresholdType | null
  quotaNotifyTotalEnabled?: boolean | null
  quotaNotifyTotalThreshold?: number | null
  quotaNotifyTotalThresholdType?: QuotaThresholdType | null
}>(), {
  quotaNotifyGlobalEnabled: false,
  quotaNotifyDailyEnabled: null,
  quotaNotifyDailyThreshold: null,
  quotaNotifyDailyThresholdType: null,
  quotaNotifyWeeklyEnabled: null,
  quotaNotifyWeeklyThreshold: null,
  quotaNotifyWeeklyThresholdType: null,
  quotaNotifyTotalEnabled: null,
  quotaNotifyTotalThreshold: null,
  quotaNotifyTotalThresholdType: null
})

const emit = defineEmits<{
  'update:totalLimit': [value: number | null]
  'update:dailyLimit': [value: number | null]
  'update:weeklyLimit': [value: number | null]
  'update:dailyResetMode': [value: QuotaResetMode | null]
  'update:dailyResetHour': [value: number | null]
  'update:weeklyResetMode': [value: QuotaResetMode | null]
  'update:weeklyResetDay': [value: number | null]
  'update:weeklyResetHour': [value: number | null]
  'update:resetTimezone': [value: string | null]
  'update:quotaNotifyDailyEnabled': [value: boolean | null]
  'update:quotaNotifyDailyThreshold': [value: number | null]
  'update:quotaNotifyDailyThresholdType': [value: QuotaThresholdType | null]
  'update:quotaNotifyWeeklyEnabled': [value: boolean | null]
  'update:quotaNotifyWeeklyThreshold': [value: number | null]
  'update:quotaNotifyWeeklyThresholdType': [value: QuotaThresholdType | null]
  'update:quotaNotifyTotalEnabled': [value: boolean | null]
  'update:quotaNotifyTotalThreshold': [value: number | null]
  'update:quotaNotifyTotalThresholdType': [value: QuotaThresholdType | null]
}>()

const { t } = useI18n()
</script>

<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600 space-y-4">
    <div class="mb-3">
      <h3 class="input-label mb-0 text-base font-semibold">
        {{ t('admin.accounts.quotaControl.title') }}
      </h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t(props.hintKey) }}
      </p>
    </div>
    <QuotaLimitCard
      :total-limit="props.totalLimit"
      :daily-limit="props.dailyLimit"
      :weekly-limit="props.weeklyLimit"
      :quota-notify-global-enabled="props.quotaNotifyGlobalEnabled"
      :quota-notify-daily-enabled="props.quotaNotifyDailyEnabled"
      :quota-notify-daily-threshold="props.quotaNotifyDailyThreshold"
      :quota-notify-daily-threshold-type="props.quotaNotifyDailyThresholdType"
      :quota-notify-weekly-enabled="props.quotaNotifyWeeklyEnabled"
      :quota-notify-weekly-threshold="props.quotaNotifyWeeklyThreshold"
      :quota-notify-weekly-threshold-type="props.quotaNotifyWeeklyThresholdType"
      :quota-notify-total-enabled="props.quotaNotifyTotalEnabled"
      :quota-notify-total-threshold="props.quotaNotifyTotalThreshold"
      :quota-notify-total-threshold-type="props.quotaNotifyTotalThresholdType"
      :daily-reset-mode="props.dailyResetMode"
      :daily-reset-hour="props.dailyResetHour"
      :weekly-reset-mode="props.weeklyResetMode"
      :weekly-reset-day="props.weeklyResetDay"
      :weekly-reset-hour="props.weeklyResetHour"
      :reset-timezone="props.resetTimezone"
      @update:total-limit="emit('update:totalLimit', $event)"
      @update:daily-limit="emit('update:dailyLimit', $event)"
      @update:weekly-limit="emit('update:weeklyLimit', $event)"
      @update:quota-notify-daily-enabled="emit('update:quotaNotifyDailyEnabled', $event)"
      @update:quota-notify-daily-threshold="emit('update:quotaNotifyDailyThreshold', $event)"
      @update:quota-notify-daily-threshold-type="emit('update:quotaNotifyDailyThresholdType', $event)"
      @update:quota-notify-weekly-enabled="emit('update:quotaNotifyWeeklyEnabled', $event)"
      @update:quota-notify-weekly-threshold="emit('update:quotaNotifyWeeklyThreshold', $event)"
      @update:quota-notify-weekly-threshold-type="emit('update:quotaNotifyWeeklyThresholdType', $event)"
      @update:quota-notify-total-enabled="emit('update:quotaNotifyTotalEnabled', $event)"
      @update:quota-notify-total-threshold="emit('update:quotaNotifyTotalThreshold', $event)"
      @update:quota-notify-total-threshold-type="emit('update:quotaNotifyTotalThresholdType', $event)"
      @update:daily-reset-mode="emit('update:dailyResetMode', $event)"
      @update:daily-reset-hour="emit('update:dailyResetHour', $event)"
      @update:weekly-reset-mode="emit('update:weeklyResetMode', $event)"
      @update:weekly-reset-day="emit('update:weeklyResetDay', $event)"
      @update:weekly-reset-hour="emit('update:weeklyResetHour', $event)"
      @update:reset-timezone="emit('update:resetTimezone', $event)"
    />
  </div>
</template>
