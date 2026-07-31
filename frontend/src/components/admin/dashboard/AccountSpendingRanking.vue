<template>
  <section class="card p-4" data-test="account-spending-ranking">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.accountSpendingRankingTitle') }}
      </h3>
      <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800" role="tablist">
        <button
          v-for="option in periodOptions"
          :key="option.value"
          type="button"
          role="tab"
          :aria-selected="period === option.value"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="period === option.value
            ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
          @click="setPeriod(option.value)"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex min-h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="error" class="flex min-h-64 flex-col items-center justify-center gap-3 text-sm text-gray-500 dark:text-gray-400">
      <span>{{ t('admin.dashboard.failedToLoad') }}</span>
      <button type="button" class="btn btn-secondary" @click="loadRanking">
        {{ t('common.refresh') }}
      </button>
    </div>
    <div v-else>
      <div class="mb-4 grid grid-cols-3 gap-3 border-b border-gray-100 pb-4 dark:border-gray-700">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.accountRankingSpend') }}</p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">${{ formatCost(totalAccountCost) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.requests') }}</p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatNumber(totalRequests) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.tokens') }}</p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatTokens(totalTokens) }}</p>
        </div>
      </div>

      <div v-if="items.length" class="overflow-x-auto">
        <table class="min-w-[640px] w-full text-xs">
          <thead>
            <tr class="text-gray-500 dark:text-gray-400">
              <th class="pb-2 text-left">#</th>
              <th class="pb-2 text-left">{{ t('admin.dashboard.accountRankingAccount') }}</th>
              <th class="pb-2 text-left">{{ t('admin.dashboard.accountRankingPlatform') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.accountRankingSpend') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(item, index) in items"
              :key="item.account_id"
              class="border-t border-gray-100 dark:border-gray-700"
            >
              <td class="py-2 font-semibold text-gray-500 dark:text-gray-400">#{{ index + 1 }}</td>
              <td class="max-w-[220px] truncate py-2 font-medium text-gray-900 dark:text-white" :title="accountLabel(item)">
                {{ accountLabel(item) }}
              </td>
              <td class="py-2 text-gray-600 dark:text-gray-400">{{ item.platform || '-' }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-gray-400">{{ formatNumber(item.requests) }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-gray-400">{{ formatTokens(item.tokens) }}</td>
              <td class="py-2 text-right font-medium text-green-600 dark:text-green-400">${{ formatCost(item.account_cost) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="flex min-h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import {
  getAccountSpendingRanking,
  type AccountSpendingRankingPeriod
} from '@/api/admin/dashboard'
import type { AccountSpendingRankingItem } from '@/types'

const { t } = useI18n()

const period = ref<AccountSpendingRankingPeriod>('today')
const items = ref<AccountSpendingRankingItem[]>([])
const totalAccountCost = ref(0)
const totalRequests = ref(0)
const totalTokens = ref(0)
const loading = ref(false)
const error = ref(false)
let loadSequence = 0

const periodOptions = computed(() => [
  { value: 'today' as const, label: t('admin.dashboard.accountRankingToday') },
  { value: '24h' as const, label: t('admin.dashboard.accountRanking24h') },
  { value: '7d' as const, label: t('admin.dashboard.accountRanking7d') }
])

// 加载当前时间范围的账号成本排行，并丢弃过期请求的响应。
const loadRanking = async () => {
  const sequence = ++loadSequence
  loading.value = true
  error.value = false
  try {
    const response = await getAccountSpendingRanking({ period: period.value, limit: 20 })
    if (sequence !== loadSequence) return
    items.value = response.ranking || []
    totalAccountCost.value = response.total_account_cost || 0
    totalRequests.value = response.total_requests || 0
    totalTokens.value = response.total_tokens || 0
  } catch (cause) {
    if (sequence !== loadSequence) return
    console.error('Failed to load account spending ranking:', cause)
    items.value = []
    totalAccountCost.value = 0
    totalRequests.value = 0
    totalTokens.value = 0
    error.value = true
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

const setPeriod = (nextPeriod: AccountSpendingRankingPeriod) => {
  if (period.value === nextPeriod) return
  period.value = nextPeriod
  void loadRanking()
}

const accountLabel = (item: AccountSpendingRankingItem) => {
  return item.account_name?.trim() || `#${item.account_id}`
}

const formatNumber = (value: number) => Number(value || 0).toLocaleString()

const formatTokens = (value: number) => {
  const amount = Number(value || 0)
  if (amount >= 1_000_000_000) return `${(amount / 1_000_000_000).toFixed(2)}B`
  if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(2)}M`
  if (amount >= 1_000) return `${(amount / 1_000).toFixed(2)}K`
  return amount.toLocaleString()
}

const formatCost = (value: number) => {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return '0.0000'
  if (amount >= 1000) return `${(amount / 1000).toFixed(2)}K`
  if (amount >= 1) return amount.toFixed(2)
  if (amount >= 0.01) return amount.toFixed(3)
  return amount.toFixed(4)
}

onMounted(() => {
  void loadRanking()
})
</script>
