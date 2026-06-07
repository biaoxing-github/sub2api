<template>
  <section class="mt-3 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 md:flex-row md:items-center md:justify-between">
      <div class="flex min-w-0 items-center gap-2">
        <span class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">
          <Icon name="chart" size="sm" :stroke-width="2" />
        </span>
        <div class="min-w-0">
          <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
            {{ t('admin.accounts.usageSummary.title') }}
          </h2>
          <p class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.usageSummary.openaiScope') }}
          </p>
        </div>
      </div>
      <div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
        <span v-if="summary">{{ t('admin.accounts.usageSummary.generatedAt', { time: formatTime(summary.generated_at) }) }}</span>
        <span v-if="loading" class="inline-flex items-center gap-1">
          <Icon name="refresh" size="xs" class="animate-spin" />
          {{ t('common.loading') }}
        </span>
        <button
          type="button"
          class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-gray-100"
          @click="expanded = !expanded"
        >
          {{ expanded ? t('admin.accounts.usageSummary.collapse') : t('admin.accounts.usageSummary.expand') }}
          <Icon :name="expanded ? 'chevronUp' : 'chevronDown'" size="xs" />
        </button>
      </div>
    </div>

    <div v-if="error && expanded" class="border-b border-rose-100 bg-rose-50 px-4 py-2 text-sm text-rose-700 dark:border-rose-900/40 dark:bg-rose-900/20 dark:text-rose-200">
      {{ error }}
    </div>

    <div v-if="!expanded && summary" class="grid gap-0 md:grid-cols-7">
      <MetricCell :label="t('admin.accounts.usageSummary.accounts')" :value="formatNumber(summary.total_accounts)" />
      <MetricCell :label="t('admin.accounts.usageSummary.fiveHourRemaining')" :value="formatWindowPercent(summary.five_hour.remaining_percent_sum, summary.five_hour)" :title="t('admin.accounts.usageSummary.fiveHourTooltip')" />
      <MetricCell :label="t('admin.accounts.usageSummary.sevenDayRemaining')" :value="formatWindowPercent(summary.seven_day.remaining_percent_sum, summary.seven_day)" :title="t('admin.accounts.usageSummary.sevenDayTooltip')" />
      <MetricCell :label="t('admin.accounts.usageSummary.upstreamActualBalance')" :value="formatCost(summary.upstream_balance?.available || 0)" tone="blue" />
      <MetricCell :label="t('admin.accounts.usageSummary.upstreamUsableBalance')" :value="formatCost(upstreamUsableBalance(summary.upstream_balance))" tone="emerald" />
      <MetricCell :label="t('admin.accounts.usageSummary.missingCodexSnapshots')" :value="formatNumber(missingCodexSnapshots(summary))" tone="slate" />
      <MetricCell :label="t('admin.accounts.usageSummary.missingUpstreamBalanceSnapshots')" :value="formatNumber(missingUpstreamBalanceSnapshots(summary))" tone="amber" />
    </div>

    <div v-if="expanded && !summary && loading" class="grid gap-3 p-4 md:grid-cols-4">
      <div v-for="index in 4" :key="index" class="h-16 animate-pulse rounded-md bg-gray-100 dark:bg-gray-700" />
    </div>

    <div v-else-if="expanded && summary" class="max-h-[42vh] overflow-y-auto divide-y divide-gray-100 dark:divide-gray-700">
      <div class="grid gap-0 md:grid-cols-5">
        <MetricCell :label="t('admin.accounts.usageSummary.accounts')" :value="formatNumber(summary.total_accounts)" />
        <MetricCell :label="t('admin.accounts.usageSummary.schedulable')" :value="formatNumber(summary.schedulable_accounts)" />
        <MetricCell :label="t('admin.accounts.usageSummary.rateLimited')" :value="formatNumber(summary.rate_limited_accounts)" tone="amber" />
        <MetricCell :label="t('admin.accounts.usageSummary.missingCodexSnapshots')" :value="formatNumber(missingCodexSnapshots(summary))" tone="slate" />
        <MetricCell :label="t('admin.accounts.usageSummary.missingUpstreamBalanceSnapshots')" :value="formatNumber(missingUpstreamBalanceSnapshots(summary))" tone="amber" />
      </div>

      <div class="grid gap-0 lg:grid-cols-2">
        <WindowBlock :title="t('admin.accounts.usageSummary.fiveHour')" :tooltip="t('admin.accounts.usageSummary.fiveHourTooltip')" :window="summary.five_hour" />
        <WindowBlock :title="t('admin.accounts.usageSummary.sevenDay')" :tooltip="t('admin.accounts.usageSummary.sevenDayTooltip')" :window="summary.seven_day" />
      </div>

      <div class="grid gap-0 md:grid-cols-5">
        <MetricCell :label="t('admin.accounts.usageSummary.upstreamActualBalance')" :value="formatCost(summary.upstream_balance?.available || 0)" tone="blue" />
        <MetricCell :label="t('admin.accounts.usageSummary.upstreamUsableBalance')" :value="formatCost(upstreamUsableBalance(summary.upstream_balance))" tone="emerald" />
        <MetricCell :label="t('admin.accounts.usageSummary.upstreamUsed')" :value="formatCost(summary.upstream_balance?.used || 0)" tone="slate" />
        <MetricCell :label="t('admin.accounts.usageSummary.upstreamTotal')" :value="formatCost(summary.upstream_balance?.total || 0)" tone="slate" />
        <MetricCell :label="t('admin.accounts.usageSummary.upstreamKeys')" :value="formatUpstreamKeys(summary.upstream_balance)" :tone="(summary.upstream_balance?.failed_key_count || 0) > 0 ? 'amber' : 'blue'" />
      </div>

      <div v-if="formatConvertedGroups(summary.upstream_balance?.converted_available_by_group)" class="px-4 py-2 text-xs text-emerald-700 dark:text-emerald-300">
        {{ t('admin.accounts.usageSummary.convertedUpstreamBalance') }}: {{ formatConvertedGroups(summary.upstream_balance?.converted_available_by_group) }}
      </div>

      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-gray-700">
          <thead class="bg-gray-50 text-xs font-medium text-gray-500 dark:bg-gray-900/30 dark:text-gray-400">
            <tr>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.usageSummary.plan') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.usageSummary.accounts') }}</th>
              <th class="px-4 py-2 text-left" :title="t('admin.accounts.usageSummary.fiveHourTooltip')">{{ t('admin.accounts.usageSummary.fiveHour') }}</th>
              <th class="px-4 py-2 text-left" :title="t('admin.accounts.usageSummary.sevenDayTooltip')">{{ t('admin.accounts.usageSummary.sevenDay') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.usageSummary.upstreamBalance') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.usageSummary.typeBreakdown') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr v-for="plan in visiblePlans" :key="plan.plan_type" class="align-top">
              <td class="px-4 py-3">
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ plan.plan_label }}</div>
                <div v-if="plan.oldest_updated_at" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.accounts.usageSummary.oldestSnapshot') }} {{ formatTime(plan.oldest_updated_at) }}
                </div>
              </td>
              <td class="px-4 py-3 text-gray-600 dark:text-gray-300">
                <div>{{ formatNumber(plan.account_count) }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.accounts.usageSummary.schedulableShort', { count: formatNumber(plan.schedulable_count) }) }}
                </div>
              </td>
              <td class="px-4 py-3">
                <WindowMini :window="plan.five_hour" />
              </td>
              <td class="px-4 py-3">
                <WindowMini :window="plan.seven_day" />
              </td>
              <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                <div class="font-medium text-blue-700 dark:text-blue-300">
                  {{ t('admin.accounts.usageSummary.actualShort') }} {{ formatCost(plan.upstream_balance?.available || 0) }}
                </div>
                <div class="mt-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-300">
                  {{ t('admin.accounts.usageSummary.usableShort') }} {{ formatCost(upstreamUsableBalance(plan.upstream_balance)) }}
                </div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ formatUpstreamKeys(plan.upstream_balance) }}
                </div>
              </td>
              <td class="px-4 py-3">
                <div class="flex min-w-[14rem] flex-wrap gap-1.5">
                  <span
                    v-for="typeGroup in plan.types || []"
                    :key="`${plan.plan_type}-${typeGroup.account_type}`"
                    class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2 py-1 text-xs text-gray-600 dark:border-gray-600 dark:text-gray-300"
                    :title="formatTypeTitle(typeGroup)"
                  >
                    <span class="font-medium">{{ typeGroup.account_type_label || typeGroup.account_type }}</span>
                    <span class="text-gray-400">{{ formatNumber(typeGroup.account_count) }}</span>
                    <span class="text-emerald-600 dark:text-emerald-300">{{ formatTypeChipUsage(typeGroup) }}</span>
                  </span>
                </div>
              </td>
            </tr>
            <tr v-if="visiblePlans.length === 0">
              <td colspan="5" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.usageSummary.noData') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="expanded" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.accounts.usageSummary.noData') }}
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type {
  AccountPoolUsageSummary,
  AccountPoolUsageSummaryGroup,
  AccountPoolUsageSummaryWindow,
  UpstreamBalanceSummary,
} from '@/types'

const props = defineProps<{
  summary: AccountPoolUsageSummary | null
  loading?: boolean
  error?: string | null
}>()

const { t, locale } = useI18n()
const expanded = ref(false)

const visiblePlans = computed(() => props.summary?.plans ?? [])

const missingCodexSnapshots = (summary: AccountPoolUsageSummary) => {
  return summary.missing_codex_snapshot_accounts ?? summary.missing_snapshot_accounts ?? 0
}

const missingUpstreamBalanceSnapshots = (summary: AccountPoolUsageSummary) => {
  return summary.upstream_balance?.missing_accounts ?? 0
}

const formatNumber = (value: number | string) => {
  const numeric = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(numeric)) return '0'
  return numeric.toLocaleString()
}

const formatCost = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '$0.0000'
  return `$${value.toFixed(4)}`
}

const formatPercent = (value: number) => {
  if (!Number.isFinite(value)) return '0%'
  if (value >= 100) return `${value.toFixed(0)}%`
  if (value <= -100) return `${value.toFixed(0)}%`
  return `${value.toFixed(1)}%`
}

const hasWindowData = (window: AccountPoolUsageSummaryWindow) => {
  return window.accounts_in_window > 0
}

const formatWindowPercent = (value: number, window: AccountPoolUsageSummaryWindow) => {
  return hasWindowData(window) ? formatPercent(value) : t('admin.accounts.usageSummary.notApplicable')
}

const formatWindowUsedPercent = (window: AccountPoolUsageSummaryWindow) => {
  return formatWindowPercent(window.used_percent_sum, window)
}

const windowProgressWidth = (window: AccountPoolUsageSummaryWindow) => {
  if (!hasWindowData(window) || window.accounts_in_window <= 0) return '0%'
  const capacity = window.accounts_in_window * 100
  return `${Math.max(0, Math.min(100, window.used_percent_sum / capacity * 100))}%`
}

const formatTime = (value?: string | null) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale.value)
}

const formatUpstreamKeys = (summary?: UpstreamBalanceSummary | null) => {
  if (!summary || summary.key_count <= 0) return t('admin.accounts.usageSummary.notApplicable')
  return `${formatNumber(summary.ok_key_count)} / ${formatNumber(summary.key_count)}`
}

const upstreamUsableBalance = (summary?: UpstreamBalanceSummary | null) => {
  const converted = summary?.converted_available_by_group
  if (!converted) return summary?.available || 0
  const total = Object.values(converted).reduce((sum, value) => {
    const numeric = Number(value)
    return Number.isFinite(numeric) ? sum + numeric : sum
  }, 0)
  return total > 0 ? total : summary?.available || 0
}

const formatConvertedGroups = (groups?: Record<string, number> | null) => {
  if (!groups) return ''
  const items = Object.entries(groups)
    .filter(([, value]) => Number.isFinite(Number(value)))
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(0, 5)
    .map(([name, value]) => `${name} ${formatCost(value)}`)
  return items.join(' / ')
}

const formatTypeTitle = (group: AccountPoolUsageSummaryGroup) => {
  return [
    `${t('admin.accounts.usageSummary.accounts')}: ${formatNumber(group.account_count)}`,
    `${t('admin.accounts.usageSummary.fiveHour')}: ${formatWindowUsedPercent(group.five_hour)} / ${formatWindowPercent(group.five_hour.remaining_percent_sum, group.five_hour)}`,
    `${t('admin.accounts.usageSummary.sevenDay')}: ${formatWindowUsedPercent(group.seven_day)} / ${formatWindowPercent(group.seven_day.remaining_percent_sum, group.seven_day)}`,
  ].join(' | ')
}

const formatTypeChipUsage = (group: AccountPoolUsageSummaryGroup) => {
  if (hasWindowData(group.five_hour)) {
    return `${t('admin.accounts.usageSummary.fiveHourShort')} ${formatWindowUsedPercent(group.five_hour)}`
  }
  if (hasWindowData(group.seven_day)) {
    return `${t('admin.accounts.usageSummary.sevenDayShort')} ${formatWindowUsedPercent(group.seven_day)}`
  }
  return t('admin.accounts.usageSummary.notApplicable')
}

const MetricCell = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    title: { type: String, default: '' },
    tone: { type: String as PropType<'emerald' | 'amber' | 'slate' | 'blue'>, default: 'emerald' },
  },
  setup(cellProps) {
    const toneClass = computed(() => ({
      emerald: 'text-emerald-700 dark:text-emerald-300',
      amber: 'text-amber-700 dark:text-amber-300',
      slate: 'text-gray-700 dark:text-gray-200',
      blue: 'text-blue-700 dark:text-blue-300',
    }[cellProps.tone]))

    return () => h('div', { class: 'border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-gray-700 md:border-b-0 md:border-r md:last:border-r-0', title: cellProps.title || undefined }, [
      h('div', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, cellProps.label),
      h('div', { class: ['mt-1 text-xl font-semibold', toneClass.value] }, cellProps.value),
    ])
  }
})

const WindowMini = defineComponent({
  props: {
    window: { type: Object as PropType<AccountPoolUsageSummaryWindow>, required: true },
  },
  setup(windowProps) {
    return () => h('div', { class: 'min-w-[9rem]' }, [
      h('div', { class: 'flex items-baseline justify-between gap-3' }, [
        h('span', { class: 'font-medium text-gray-900 dark:text-gray-100' }, formatWindowUsedPercent(windowProps.window)),
        h('span', { class: 'text-xs text-gray-500 dark:text-gray-400' }, formatCost(windowProps.window.used_cost)),
      ]),
      h('div', { class: 'mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700' }, [
        h('div', {
          class: 'h-full rounded-full bg-emerald-500',
          style: { width: windowProgressWidth(windowProps.window) }
        })
      ]),
    ])
  }
})

const WindowBlock = defineComponent({
  props: {
    title: { type: String, required: true },
    tooltip: { type: String, default: '' },
    window: { type: Object as PropType<AccountPoolUsageSummaryWindow>, required: true },
  },
  setup(blockProps) {
    return () => h('div', { class: 'border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-gray-700 lg:border-b-0 lg:border-r lg:last:border-r-0', title: blockProps.tooltip || undefined }, [
      h('div', { class: 'flex items-center justify-between gap-3' }, [
        h('div', { class: 'text-sm font-semibold text-gray-900 dark:text-gray-100' }, blockProps.title),
        h('div', { class: 'text-sm font-semibold text-emerald-700 dark:text-emerald-300' }, formatWindowUsedPercent(blockProps.window)),
      ]),
      h('div', { class: 'mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700' }, [
        h('div', {
          class: 'h-full rounded-full bg-emerald-500',
          style: { width: windowProgressWidth(blockProps.window) }
        })
      ]),
      h('dl', { class: 'mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs' }, [
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.used')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(blockProps.window.used_cost)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.estimatedLimit')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(blockProps.window.estimated_limit_cost)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.usedPercentSum')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatPercent(blockProps.window.used_percent_sum)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.remainingPercentSum')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatPercent(blockProps.window.remaining_percent_sum)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.requests')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatNumber(blockProps.window.requests)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.snapshotCoverage')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, `${formatNumber(blockProps.window.accounts_with_snapshot)} / ${formatNumber(blockProps.window.accounts_with_limit_estimate)}`),
        ]),
      ]),
      blockProps.window.earliest_reset_at
        ? h('div', { class: 'mt-2 text-xs text-gray-500 dark:text-gray-400' }, `${t('admin.accounts.usageSummary.earliestReset')}: ${formatTime(blockProps.window.earliest_reset_at)}`)
        : null,
    ])
  }
})
</script>
