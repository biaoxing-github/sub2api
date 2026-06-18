<template>
  <section class="mt-3 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 md:flex-row md:items-center md:justify-between">
      <div class="flex min-w-0 items-center gap-2">
        <span class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-gray-300">
          <Icon name="chart" size="sm" :stroke-width="2" />
        </span>
        <div class="min-w-0">
          <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
            {{ t('admin.accounts.usageSummary.title') }}
          </h2>
          <p class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.usageSummary.poolScope') }}
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

    <div v-if="summary" class="grid gap-0 md:grid-cols-4">
      <MetricCell :label="t('admin.accounts.usageSummary.accounts')" :value="formatNumber(summary.total_accounts)" />
      <MetricCell
        :label="t('admin.accounts.usageSummary.openaiBalance')"
        :value="formatBalancePair(summary.openai_upstream_balance)"
        :title="formatBalanceTitle(summary.openai_upstream_balance)"
        tone="slate"
      />
      <MetricCell
        :label="t('admin.accounts.usageSummary.anthropicBalance')"
        :value="formatBalancePair(summary.anthropic_upstream_balance)"
        :title="formatBalanceTitle(summary.anthropic_upstream_balance)"
        tone="slate"
      />
      <MetricCell
        :label="t('admin.accounts.usageSummary.missingUpstreamBalanceSnapshots')"
        :value="formatNumber(missingUpstreamBalanceSnapshots(summary))"
        tone="amber"
      />
    </div>

    <div v-if="expanded && !summary && loading" class="grid gap-3 p-4 md:grid-cols-4">
      <div v-for="index in 4" :key="index" class="h-16 animate-pulse rounded-md bg-gray-100 dark:bg-gray-700" />
    </div>

    <div v-else-if="expanded && summary" class="divide-y divide-gray-100 dark:divide-gray-700">
      <div class="grid gap-0 md:grid-cols-4">
        <MetricCell :label="t('admin.accounts.usageSummary.schedulable')" :value="formatNumber(summary.schedulable_accounts)" />
        <MetricCell :label="t('admin.accounts.usageSummary.rateLimited')" :value="formatNumber(summary.rate_limited_accounts)" tone="amber" />
        <MetricCell :label="t('admin.accounts.usageSummary.openaiKeys')" :value="formatUpstreamKeys(summary.openai_upstream_balance)" tone="slate" />
        <MetricCell :label="t('admin.accounts.usageSummary.anthropicKeys')" :value="formatUpstreamKeys(summary.anthropic_upstream_balance)" tone="slate" />
      </div>

      <div class="grid gap-0 md:grid-cols-2">
        <ProviderBalanceBlock
          :title="t('admin.accounts.usageSummary.openaiBalance')"
          :summary="summary.openai_upstream_balance"
        />
        <ProviderBalanceBlock
          :title="t('admin.accounts.usageSummary.anthropicBalance')"
          :summary="summary.anthropic_upstream_balance"
        />
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
  UpstreamBalanceSummary,
} from '@/types'

defineProps<{
  summary: AccountPoolUsageSummary | null
  loading?: boolean
  error?: string | null
}>()

const { t, locale } = useI18n()
const expanded = ref(false)

const formatNumber = (value: number | string) => {
  const numeric = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(numeric)) return '0'
  return numeric.toLocaleString()
}

const formatCost = (value?: number | null) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric <= 0) return '$0.0000'
  return `$${numeric.toFixed(4)}`
}

const formatTime = (value?: string | null) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale.value)
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

const formatBalancePair = (summary?: UpstreamBalanceSummary | null) => {
  return `${formatCost(upstreamUsableBalance(summary))} / ${formatCost(summary?.available)}`
}

const formatConvertedGroups = (groups?: Record<string, number> | null) => {
  if (!groups) return ''
  return Object.entries(groups)
    .filter(([, value]) => Number.isFinite(Number(value)))
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(0, 5)
    .map(([name, value]) => `${name} ${formatCost(value)}`)
    .join(' / ')
}

const formatUpstreamKeys = (summary?: UpstreamBalanceSummary | null) => {
  if (!summary || summary.key_count <= 0) return t('admin.accounts.usageSummary.notApplicable')
  return `${formatNumber(summary.ok_key_count)} / ${formatNumber(summary.key_count)}`
}

const formatBalanceTitle = (summary?: UpstreamBalanceSummary | null) => {
  if (!summary) return ''
  return [
    `${t('admin.accounts.usageSummary.upstreamUsableBalance')}: ${formatCost(upstreamUsableBalance(summary))}`,
    `${t('admin.accounts.usageSummary.upstreamActualBalance')}: ${formatCost(summary.available)}`,
    `${t('admin.accounts.usageSummary.upstreamUsed')}: ${formatCost(summary.used)}`,
    `${t('admin.accounts.usageSummary.upstreamTotal')}: ${formatCost(summary.total)}`,
    `${t('admin.accounts.usageSummary.upstreamKeys')}: ${formatUpstreamKeys(summary)}`,
    formatConvertedGroups(summary.converted_available_by_group),
  ].filter(Boolean).join('\n')
}

const missingUpstreamBalanceSnapshots = (summary: AccountPoolUsageSummary) => {
  return (summary.openai_upstream_balance?.missing_accounts ?? 0) +
    (summary.anthropic_upstream_balance?.missing_accounts ?? 0)
}

const MetricCell = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    title: { type: String, default: '' },
    tone: { type: String as PropType<'emerald' | 'amber' | 'slate'>, default: 'slate' },
  },
  setup(cellProps) {
    const toneClass = computed(() => ({
      emerald: 'text-emerald-700 dark:text-emerald-300',
      amber: 'text-amber-700 dark:text-amber-300',
      slate: 'text-gray-700 dark:text-gray-200',
    }[cellProps.tone]))

    return () => h('div', { class: 'border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-gray-700 md:border-b-0 md:border-r md:last:border-r-0', title: cellProps.title || undefined }, [
      h('div', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, cellProps.label),
      h('div', { class: ['mt-1 text-xl font-semibold', toneClass.value] }, cellProps.value),
    ])
  }
})

const ProviderBalanceBlock = defineComponent({
  props: {
    title: { type: String, required: true },
    summary: { type: Object as PropType<UpstreamBalanceSummary | null | undefined>, default: null },
  },
  setup(blockProps) {
    return () => h('div', { class: 'px-4 py-3' }, [
      h('div', { class: 'text-sm font-semibold text-gray-900 dark:text-gray-100' }, blockProps.title),
      h('dl', { class: 'mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs' }, [
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.upstreamUsableBalance')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(upstreamUsableBalance(blockProps.summary))),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.upstreamActualBalance')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(blockProps.summary?.available)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.upstreamUsed')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(blockProps.summary?.used)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.upstreamTotal')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatCost(blockProps.summary?.total)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.upstreamKeys')),
          h('dd', { class: 'mt-0.5 font-medium text-gray-900 dark:text-gray-100' }, formatUpstreamKeys(blockProps.summary)),
        ]),
        h('div', [
          h('dt', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.accounts.usageSummary.missingUpstreamBalanceSnapshots')),
          h('dd', { class: 'mt-0.5 font-medium text-amber-700 dark:text-amber-300' }, formatNumber(blockProps.summary?.missing_accounts ?? 0)),
        ]),
      ]),
      formatConvertedGroups(blockProps.summary?.converted_available_by_group)
        ? h('div', { class: 'mt-2 text-xs text-gray-600 dark:text-gray-300' }, `${t('admin.accounts.usageSummary.convertedUpstreamBalance')}: ${formatConvertedGroups(blockProps.summary?.converted_available_by_group)}`)
        : null,
    ])
  }
})
</script>
