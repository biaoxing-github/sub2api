<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  opsAPI,
  type AccountEffectiveAvailability,
  type OpsAccountAvailabilityStatsResponse,
  type OpsConcurrencyStatsResponse,
  type OpsUserConcurrencyStatsResponse
} from '@/api/admin/ops'

interface Props {
  platformFilter?: string
  groupIdFilter?: number | null
  refreshToken: number
}

const props = withDefaults(defineProps<Props>(), {
  platformFilter: '',
  groupIdFilter: null
})

const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')
const concurrency = ref<OpsConcurrencyStatsResponse | null>(null)
const availability = ref<OpsAccountAvailabilityStatsResponse | null>(null)
const userConcurrency = ref<OpsUserConcurrencyStatsResponse | null>(null)

// 用户视图开关
const showByUser = ref(false)

const realtimeEnabled = computed(() => {
  return (concurrency.value?.enabled ?? true) && (availability.value?.enabled ?? true)
})

function safeNumber(n: unknown): number {
  return typeof n === 'number' && Number.isFinite(n) ? n : 0
}

// 计算显示维度
const displayDimension = computed<'platform' | 'group' | 'account' | 'user'>(() => {
  if (showByUser.value) {
    return 'user'
  }
  if (typeof props.groupIdFilter === 'number' && props.groupIdFilter > 0) {
    return 'account'
  }
  if (props.platformFilter) {
    return 'group'
  }
  return 'platform'
})

// 平台/分组汇总行数据
interface SummaryRow {
  key: string
  name: string
  platform?: string
  // 账号统计
  total_accounts: number
  available_accounts: number
  rate_limited_accounts: number
  error_accounts: number
  // 并发统计
  total_concurrency: number
  used_concurrency: number
  waiting_in_queue: number
  // 计算字段
  availability_percentage: number
  concurrency_percentage: number
}

// 账号详细行数据
interface AccountRow {
  key: string
  name: string
  platform: string
  group_name: string
  // 并发
  current_in_use: number
  max_capacity: number
  waiting_in_queue: number
  load_percentage: number
  // 状态
  status: string
  is_available: boolean
  is_rate_limited: boolean
  rate_limit_remaining_sec?: number
  is_overloaded: boolean
  overload_remaining_sec?: number
  has_error: boolean
  error_message?: string
  path_health_state?: string
  path_health_cooldown_until?: string
  path_health_last_failure_reason?: string
  path_health_consecutive_failures?: number
  path_health_window_failures?: number
  path_health_eof_count?: number
  path_health_header_timeout_count?: number
  path_health_ttft_ewma_ms?: number
  path_health_header_wait_ewma_ms?: number
  temp_unschedulable_until?: string
  effective_availability?: AccountEffectiveAvailability
}

// 用户行数据
interface UserRow {
  key: string
  user_id: number
  user_email: string
  username: string
  current_in_use: number
  max_capacity: number
  waiting_in_queue: number
  load_percentage: number
}

// 平台维度汇总
const platformRows = computed((): SummaryRow[] => {
  const concStats = concurrency.value?.platform || {}
  const availStats = availability.value?.platform || {}

  const platforms = new Set([...Object.keys(concStats), ...Object.keys(availStats)])

  return Array.from(platforms).map(platform => {
    const conc = concStats[platform] || {}
    const avail = availStats[platform] || {}

    const totalAccounts = safeNumber(avail.total_accounts)
    const availableAccounts = safeNumber(avail.available_count)
    const totalConcurrency = safeNumber(conc.max_capacity)
    const usedConcurrency = safeNumber(conc.current_in_use)

    return {
      key: platform,
      name: platform.toUpperCase(),
      total_accounts: totalAccounts,
      available_accounts: availableAccounts,
      rate_limited_accounts: safeNumber(avail.rate_limit_count),

      error_accounts: safeNumber(avail.error_count),
      total_concurrency: totalConcurrency,
      used_concurrency: usedConcurrency,
      waiting_in_queue: safeNumber(conc.waiting_in_queue),
      availability_percentage: totalAccounts > 0 ? Math.round((availableAccounts / totalAccounts) * 100) : 0,
      concurrency_percentage: totalConcurrency > 0 ? Math.round((usedConcurrency / totalConcurrency) * 100) : 0
    }
  }).sort((a, b) => b.concurrency_percentage - a.concurrency_percentage)
})

// 分组维度汇总
const groupRows = computed((): SummaryRow[] => {
  const concStats = concurrency.value?.group || {}
  const availStats = availability.value?.group || {}

  const groupIds = new Set([...Object.keys(concStats), ...Object.keys(availStats)])

  const rows = Array.from(groupIds)
    .map(gid => {
      const conc = concStats[gid] || {}
      const avail = availStats[gid] || {}

      // 只显示匹配的平台
      if (props.platformFilter && conc.platform !== props.platformFilter && avail.platform !== props.platformFilter) {
        return null
      }

      const totalAccounts = safeNumber(avail.total_accounts)
      const availableAccounts = safeNumber(avail.available_count)
      const totalConcurrency = safeNumber(conc.max_capacity)
      const usedConcurrency = safeNumber(conc.current_in_use)

      return {
        key: gid,
        name: String(conc.group_name || avail.group_name || `Group ${gid}`),
        platform: String(conc.platform || avail.platform || ''),
        total_accounts: totalAccounts,
        available_accounts: availableAccounts,
        rate_limited_accounts: safeNumber(avail.rate_limit_count),
  
        error_accounts: safeNumber(avail.error_count),
        total_concurrency: totalConcurrency,
        used_concurrency: usedConcurrency,
        waiting_in_queue: safeNumber(conc.waiting_in_queue),
        availability_percentage: totalAccounts > 0 ? Math.round((availableAccounts / totalAccounts) * 100) : 0,
        concurrency_percentage: totalConcurrency > 0 ? Math.round((usedConcurrency / totalConcurrency) * 100) : 0
      }
    })
    .filter((row): row is NonNullable<typeof row> => row !== null)

  return rows.sort((a, b) => b.concurrency_percentage - a.concurrency_percentage)
})

// 账号维度详细
const accountRows = computed((): AccountRow[] => {
  const concStats = concurrency.value?.account || {}
  const availStats = availability.value?.account || {}

  const accountIds = new Set([...Object.keys(concStats), ...Object.keys(availStats)])

  const rows = Array.from(accountIds)
    .map(aid => {
      const conc = concStats[aid] || {}
      const avail = availStats[aid] || {}

      // 只显示匹配的分组
      if (typeof props.groupIdFilter === 'number' && props.groupIdFilter > 0) {
        if (conc.group_id !== props.groupIdFilter && avail.group_id !== props.groupIdFilter) {
          return null
        }
      }

      return {
        key: aid,
        name: String(conc.account_name || avail.account_name || `Account ${aid}`),
        platform: String(conc.platform || avail.platform || ''),
        group_name: String(conc.group_name || avail.group_name || ''),
        current_in_use: safeNumber(conc.current_in_use),
        max_capacity: safeNumber(conc.max_capacity),
        waiting_in_queue: safeNumber(conc.waiting_in_queue),
        load_percentage: safeNumber(conc.load_percentage),
        status: String(avail.status || ''),
        is_available: avail.is_available || false,
        is_rate_limited: avail.is_rate_limited || false,
        rate_limit_remaining_sec: avail.rate_limit_remaining_sec,
        is_overloaded: avail.is_overloaded || false,
        overload_remaining_sec: avail.overload_remaining_sec,
        has_error: avail.has_error || false,
        error_message: avail.error_message || '',
        path_health_state: avail.path_health_state || '',
        path_health_cooldown_until: avail.path_health_cooldown_until,
        path_health_last_failure_reason: avail.path_health_last_failure_reason || '',
        path_health_consecutive_failures: avail.path_health_consecutive_failures,
        path_health_window_failures: avail.path_health_window_failures,
        path_health_eof_count: avail.path_health_eof_count,
        path_health_header_timeout_count: avail.path_health_header_timeout_count,
        path_health_ttft_ewma_ms: avail.path_health_ttft_ewma_ms,
        path_health_header_wait_ewma_ms: avail.path_health_header_wait_ewma_ms,
        temp_unschedulable_until: avail.temp_unschedulable_until,
        effective_availability: avail.effective_availability
      }
    })
    .filter((row): row is NonNullable<typeof row> => row !== null)

  return rows.sort((a, b) => {
    // 优先显示异常账号
    if (a.has_error !== b.has_error) return a.has_error ? -1 : 1
    if (a.is_rate_limited !== b.is_rate_limited) return a.is_rate_limited ? -1 : 1
    // 然后按负载排序
    return b.load_percentage - a.load_percentage
  })
})

// 用户维度详细
const userRows = computed((): UserRow[] => {
  const userStats = userConcurrency.value?.user || {}

  return Object.keys(userStats)
    .map(uid => {
      const u = userStats[uid] || {}
      return {
        key: uid,
        user_id: safeNumber(u.user_id),
        user_email: u.user_email || `User ${uid}`,
        username: u.username || '',
        current_in_use: safeNumber(u.current_in_use),
        max_capacity: safeNumber(u.max_capacity),
        waiting_in_queue: safeNumber(u.waiting_in_queue),
        load_percentage: safeNumber(u.load_percentage)
      }
    })
    .sort((a, b) => b.current_in_use - a.current_in_use || b.load_percentage - a.load_percentage)
})

// 根据维度选择数据
const displayRows = computed(() => {
  if (displayDimension.value === 'user') return userRows.value
  if (displayDimension.value === 'account') return accountRows.value
  if (displayDimension.value === 'group') return groupRows.value
  return platformRows.value
})

const displayTitle = computed(() => {
  if (displayDimension.value === 'user') return t('admin.ops.concurrency.byUser')
  if (displayDimension.value === 'account') return t('admin.ops.concurrency.byAccount')
  if (displayDimension.value === 'group') return t('admin.ops.concurrency.byGroup')
  return t('admin.ops.concurrency.byPlatform')
})

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    if (showByUser.value) {
      // 用户视图模式只加载用户并发数据
      const userData = await opsAPI.getUserConcurrencyStats()
      userConcurrency.value = userData
    } else {
      // 常规模式加载账号/平台/分组数据
      const [concData, availData] = await Promise.all([
        opsAPI.getConcurrencyStats(props.platformFilter, props.groupIdFilter),
        opsAPI.getAccountAvailabilityStats(props.platformFilter, props.groupIdFilter)
      ])
      concurrency.value = concData
      availability.value = availData
    }
  } catch (err: any) {
    console.error('[OpsConcurrencyCard] Failed to load data', err)
    errorMessage.value = err?.response?.data?.detail || t('admin.ops.concurrency.loadFailed')
  } finally {
    loading.value = false
  }
}

// 刷新节奏由父组件统一控制（OpsDashboard Header 的刷新状态/倒计时）
watch(
  () => props.refreshToken,
  () => {
    if (!realtimeEnabled.value) return
    loadData()
  }
)

// 切换用户视图时重新加载数据
watch(
  () => showByUser.value,
  () => {
    loadData()
  }
)

function getLoadBarClass(loadPct: number): string {
  if (loadPct >= 90) return 'bg-red-500 dark:bg-red-600'
  if (loadPct >= 70) return 'bg-orange-500 dark:bg-orange-600'
  if (loadPct >= 50) return 'bg-yellow-500 dark:bg-yellow-600'
  return 'bg-green-500 dark:bg-green-600'
}

function getLoadBarStyle(loadPct: number): string {
  return `width: ${Math.min(100, Math.max(0, loadPct))}%`
}

function getLoadTextClass(loadPct: number): string {
  if (loadPct >= 90) return 'text-red-600 dark:text-red-400'
  if (loadPct >= 70) return 'text-orange-600 dark:text-orange-400'
  if (loadPct >= 50) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-green-600 dark:text-green-400'
}

function formatDuration(seconds: number): string {
  if (seconds <= 0) return '0s'
  if (seconds < 60) return `${Math.round(seconds)}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  return `${hours}h`
}

function formatMs(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return '-'
  return `${Math.round(value)}ms`
}

function getPathHealthLabel(state?: string): string {
  const key = state && ['healthy', 'degraded', 'open_circuit', 'half_open'].includes(state) ? state : 'unknown'
  return t(`admin.ops.accountAvailability.pathHealth.${key}`)
}

function getPathHealthTitle(row: AccountRow): string {
  const parts = [
    `${t('admin.ops.accountAvailability.pathHealth.reason')}: ${row.path_health_last_failure_reason || '-'}`,
    `${t('admin.ops.accountAvailability.pathHealth.windowFailures')}: ${row.path_health_window_failures ?? 0}`,
    `EOF: ${row.path_health_eof_count ?? 0}`,
    `Header timeout: ${row.path_health_header_timeout_count ?? 0}`,
    `TTFT: ${formatMs(row.path_health_ttft_ewma_ms)}`,
    `Header wait: ${formatMs(row.path_health_header_wait_ewma_ms)}`
  ]
  return parts.join('\n')
}

type UnifiedAvailabilityTone = 'success' | 'warning' | 'danger' | 'neutral'

interface UnifiedAvailabilitySummary {
  tone: UnifiedAvailabilityTone
  label: string
  title?: string
}

function isFutureTime(value?: string): boolean {
  if (!value) return false
  const time = new Date(value).getTime()
  return Number.isFinite(time) && time > Date.now()
}

function formatUntil(value?: string): string {
  if (!value) return ''
  const time = new Date(value)
  return Number.isFinite(time.getTime()) ? time.toLocaleString() : ''
}

// 统一把后端 effective_availability 状态映射到管理端文案。
function effectiveAvailabilityLabelKey(state: string): string {
  return `admin.ops.accountAvailability.effective.${state}`
}

// 鼠标悬浮时展示后端归一原因和预计恢复时间。
function getEffectiveAvailabilityTitle(effective?: AccountEffectiveAvailability): string {
  if (!effective) return ''
  const parts = []
  if (effective.reason) {
    parts.push(`${t('admin.ops.accountAvailability.pathHealth.reason')}: ${effective.reason}`)
  }
  const until = formatUntil(effective.until)
  if (until) {
    parts.push(`${t('admin.ops.accountAvailability.until')}: ${until}`)
  }
  return parts.join('\n')
}

// 后端归一状态优先级高于旧字段，避免同一账号显示成多个互相矛盾的状态。
function getEffectiveAvailabilitySummary(row: AccountRow): UnifiedAvailabilitySummary | null {
  const state = row.effective_availability?.state
  if (!state || state === 'healthy') return null

  if (state === 'path_open_circuit') {
    return { tone: 'danger', label: getPathHealthLabel('open_circuit'), title: getPathHealthTitle(row) || getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  if (state === 'path_half_open') {
    return { tone: 'warning', label: getPathHealthLabel('half_open'), title: getPathHealthTitle(row) || getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  if (state === 'path_degraded') {
    return { tone: 'warning', label: getPathHealthLabel('degraded'), title: getPathHealthTitle(row) || getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  if (state === 'precheck_failed' || state === 'overloaded' || state === 'error') {
    return { tone: 'danger', label: t(effectiveAvailabilityLabelKey(state)), title: getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  if (state === 'precheck_pending' || state === 'local_suppressed' || state === 'temp_unschedulable' || state === 'rate_limited') {
    return { tone: 'warning', label: t(effectiveAvailabilityLabelKey(state)), title: getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  if (state === 'disabled' || state === 'unschedulable') {
    return { tone: 'neutral', label: t(effectiveAvailabilityLabelKey(state)), title: getEffectiveAvailabilityTitle(row.effective_availability) }
  }
  return { tone: 'neutral', label: t('admin.ops.accountAvailability.unavailable'), title: getEffectiveAvailabilityTitle(row.effective_availability) }
}

function getUnifiedAvailabilitySummary(row: AccountRow): UnifiedAvailabilitySummary {
  const pathHealthState = row.path_health_state
  const pathHealthUnavailable = (!!pathHealthState && pathHealthState !== 'healthy') || isFutureTime(row.path_health_cooldown_until)
  const tempUnschedulable = isFutureTime(row.temp_unschedulable_until)
  const effectiveSummary = getEffectiveAvailabilitySummary(row)

  if (effectiveSummary) {
    return effectiveSummary
  }

  if (row.has_error) {
    return {
      tone: 'danger',
      label: t('admin.ops.accountAvailability.accountError'),
      title: row.error_message || row.path_health_last_failure_reason || ''
    }
  }

  if (row.status && row.status !== 'active') {
    return {
      tone: 'neutral',
      label: t(`admin.accounts.status.${row.status}`),
      title: row.path_health_last_failure_reason || row.error_message || ''
    }
  }

  if (tempUnschedulable) {
    return {
      tone: 'warning',
      label: t('admin.accounts.status.tempUnschedulable'),
      title: formatUntil(row.temp_unschedulable_until)
    }
  }

  if (row.is_rate_limited) {
    return {
      tone: 'warning',
      label: t('admin.accounts.status.rateLimited'),
      title: row.rate_limit_remaining_sec ? formatDuration(row.rate_limit_remaining_sec) : row.path_health_last_failure_reason || ''
    }
  }

  if (row.is_overloaded) {
    return {
      tone: 'danger',
      label: t('admin.ops.accountAvailability.unavailable'),
      title: row.overload_remaining_sec ? formatDuration(row.overload_remaining_sec) : row.path_health_last_failure_reason || ''
    }
  }

  if (pathHealthUnavailable) {
    const pathLabel = pathHealthState && pathHealthState !== 'healthy'
      ? getPathHealthLabel(pathHealthState)
      : t('admin.ops.accountAvailability.pathHealth.open_circuit')
    return {
      tone: pathHealthState === 'open_circuit' ? 'danger' : pathHealthState === 'half_open' ? 'warning' : 'warning',
      label: pathLabel,
      title: getPathHealthTitle(row)
    }
  }

  if (row.is_available) {
    return {
      tone: 'success',
      label: t('admin.ops.accountAvailability.available'),
      title: row.group_name || row.platform || ''
    }
  }

  return {
    tone: 'neutral',
    label: t('admin.ops.accountAvailability.unavailable'),
    title: row.path_health_last_failure_reason || row.error_message || row.path_health_cooldown_until || ''
  }
}

function getUnifiedAvailabilityClass(tone: UnifiedAvailabilityTone): string {
  if (tone === 'success') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  if (tone === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
  if (tone === 'danger') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-400'
}


watch(
  () => realtimeEnabled.value,
  async (enabled) => {
    if (enabled) {
      await loadData()
    }
  },
  { immediate: true }
)
</script>

<template>
  <div class="flex h-full flex-col rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <!-- 头部 -->
    <div class="mb-4 flex shrink-0 items-center justify-between gap-3">
      <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
        <svg class="h-4 w-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
        {{ t('admin.ops.concurrency.title') }}
      </h3>
      <div class="flex items-center gap-2">
        <!-- 用户视图切换按钮 -->
        <button
          class="flex items-center justify-center rounded-lg px-2 py-1 transition-colors"
          :class="showByUser
            ? 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
            : 'bg-gray-100 text-gray-500 hover:bg-gray-200 hover:text-gray-700 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-gray-300'"
          :title="showByUser ? t('admin.ops.concurrency.switchToPlatform') : t('admin.ops.concurrency.switchToUser')"
          @click="showByUser = !showByUser"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
        </button>
        <!-- 刷新按钮 -->
        <button
          class="flex items-center gap-1 rounded-lg bg-gray-100 px-2 py-1 text-[11px] font-semibold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="loadData"
        >
          <svg class="h-3 w-3" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>
    </div>

    <!-- 错误提示 -->
    <div v-if="errorMessage" class="mb-3 shrink-0 rounded-xl bg-red-50 p-2.5 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ errorMessage }}
    </div>

    <!-- 禁用状态 -->
    <div
      v-if="!realtimeEnabled"
      class="flex flex-1 items-center justify-center rounded-xl border border-dashed border-gray-200 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400"
    >
      {{ t('admin.ops.concurrency.disabledHint') }}
    </div>

    <!-- 数据展示区域 -->
    <div v-else class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
      <!-- 维度标题栏 -->
      <div class="flex shrink-0 items-center justify-between border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
        <span class="text-[10px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ displayTitle }}
        </span>
        <span class="text-[10px] text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.totalRows', { count: displayRows.length }) }}
        </span>
      </div>

      <!-- 空状态 -->
      <div v-if="displayRows.length === 0" class="flex flex-1 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.ops.concurrency.empty') }}
      </div>

      <!-- 用户视图 -->
      <div v-else-if="displayDimension === 'user'" class="custom-scrollbar max-h-[360px] flex-1 space-y-2 overflow-y-auto p-3">
        <div v-for="row in (displayRows as UserRow[])" :key="row.key" class="rounded-lg bg-gray-50 p-2.5 dark:bg-dark-900">
          <!-- 用户信息和并发 -->
          <div class="mb-1.5 flex items-center justify-between gap-2">
            <div class="flex min-w-0 flex-1 items-center gap-1.5">
              <span class="truncate text-[11px] font-bold text-gray-900 dark:text-white" :title="row.username || row.user_email">
                {{ row.username || row.user_email }}
              </span>
              <span v-if="row.username" class="shrink-0 truncate text-[10px] text-gray-400 dark:text-gray-500" :title="row.user_email">
                {{ row.user_email }}
              </span>
            </div>
            <div class="flex shrink-0 items-center gap-2 text-[10px]">
              <span class="font-mono font-bold text-gray-900 dark:text-white"> {{ row.current_in_use }}/{{ row.max_capacity }} </span>
              <span :class="['font-bold', getLoadTextClass(row.load_percentage)]"> {{ Math.round(row.load_percentage) }}% </span>
            </div>
          </div>

          <!-- 进度条 -->
          <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
            <div class="h-full rounded-full transition-all duration-300" :class="getLoadBarClass(row.load_percentage)" :style="getLoadBarStyle(row.load_percentage)"></div>
          </div>

          <!-- 等待队列 -->
          <div v-if="row.waiting_in_queue > 0" class="mt-1.5 flex justify-end">
            <span class="rounded-full bg-purple-100 px-1.5 py-0.5 text-[10px] font-semibold text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
              {{ t('admin.ops.concurrency.queued', { count: row.waiting_in_queue }) }}
            </span>
          </div>
        </div>
      </div>

      <!-- 汇总视图（平台/分组） -->
      <div v-else-if="displayDimension === 'platform' || displayDimension === 'group'" class="custom-scrollbar max-h-[360px] flex-1 space-y-2 overflow-y-auto p-3">
        <div v-for="row in (displayRows as SummaryRow[])" :key="row.key" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
          <!-- 标题行 -->
          <div class="mb-2 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <div class="truncate text-[11px] font-bold text-gray-900 dark:text-white" :title="row.name">
                {{ row.name }}
              </div>
              <span v-if="displayDimension === 'group' && row.platform" class="text-[10px] text-gray-400 dark:text-gray-500">
                {{ row.platform.toUpperCase() }}
              </span>
            </div>
            <div class="flex shrink-0 items-center gap-2 text-[10px]">
              <span class="font-mono font-bold text-gray-900 dark:text-white"> {{ row.used_concurrency }}/{{ row.total_concurrency }} </span>
              <span :class="['font-bold', getLoadTextClass(row.concurrency_percentage)]"> {{ row.concurrency_percentage }}% </span>
            </div>
          </div>

          <!-- 进度条 -->
          <div class="mb-2 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
            <div
              class="h-full rounded-full transition-all duration-300"
              :class="getLoadBarClass(row.concurrency_percentage)"
              :style="getLoadBarStyle(row.concurrency_percentage)"
            ></div>
          </div>

          <!-- 统计信息 -->
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px]">
            <!-- 账号统计 -->
            <div class="flex items-center gap-1">
              <svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                />
              </svg>
              <span class="text-gray-600 dark:text-gray-300">
                <span class="font-bold text-green-600 dark:text-green-400">{{ row.available_accounts }}</span
                >/{{ row.total_accounts }}
              </span>
              <span class="text-gray-400 dark:text-gray-500">{{ row.availability_percentage }}%</span>
            </div>

            <!-- 限流账号 -->
            <span
              v-if="row.rate_limited_accounts > 0"
              class="rounded-full bg-amber-100 px-1.5 py-0.5 font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
            >
              {{ t('admin.ops.concurrency.rateLimited', { count: row.rate_limited_accounts }) }}
            </span>

            <!-- 异常账号 -->
            <span
              v-if="row.error_accounts > 0"
              class="rounded-full bg-red-100 px-1.5 py-0.5 font-semibold text-red-700 dark:bg-red-900/30 dark:text-red-400"
            >
              {{ t('admin.ops.concurrency.errorAccounts', { count: row.error_accounts }) }}
            </span>

            <!-- 等待队列 -->
            <span
              v-if="row.waiting_in_queue > 0"
              class="rounded-full bg-purple-100 px-1.5 py-0.5 font-semibold text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
            >
              {{ t('admin.ops.concurrency.queued', { count: row.waiting_in_queue }) }}
            </span>
          </div>
        </div>
      </div>

      <!-- 账号详细视图 -->
      <div v-else class="custom-scrollbar max-h-[360px] flex-1 space-y-2 overflow-y-auto p-3">
        <div v-for="row in (displayRows as AccountRow[])" :key="row.key" class="rounded-lg bg-gray-50 p-2.5 dark:bg-dark-900">
          <!-- 账号名称和并发 -->
          <div class="mb-1.5 flex items-center justify-between gap-2">
            <div class="min-w-0 flex-1">
              <div class="truncate text-[11px] font-bold text-gray-900 dark:text-white" :title="row.name">
                {{ row.name }}
              </div>
              <div class="mt-0.5 text-[9px] text-gray-400 dark:text-gray-500">
                {{ row.group_name }}
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <!-- 并发使用 -->
              <span class="font-mono text-[11px] font-bold text-gray-900 dark:text-white"> {{ row.current_in_use }}/{{ row.max_capacity }} </span>
              <!-- 状态徽章 -->
              <span
                class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium"
                :class="getUnifiedAvailabilityClass(getUnifiedAvailabilitySummary(row).tone)"
                :title="getUnifiedAvailabilitySummary(row).title || undefined"
              >
                <svg v-if="getUnifiedAvailabilitySummary(row).tone === 'success'" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <svg v-else-if="getUnifiedAvailabilitySummary(row).tone === 'warning'" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <svg v-else-if="getUnifiedAvailabilitySummary(row).tone === 'danger'" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
                {{ getUnifiedAvailabilitySummary(row).label }}
              </span>
            </div>
          </div>

          <!-- 进度条 -->
          <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
            <div class="h-full rounded-full transition-all duration-300" :class="getLoadBarClass(row.load_percentage)" :style="getLoadBarStyle(row.load_percentage)"></div>
          </div>

          <!-- 等待队列 -->
          <div v-if="row.waiting_in_queue > 0" class="mt-1.5 flex justify-end">
            <span class="rounded-full bg-purple-100 px-1.5 py-0.5 text-[10px] font-semibold text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
              {{ t('admin.ops.concurrency.queued', { count: row.waiting_in_queue }) }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.custom-scrollbar {
  scrollbar-width: thin;
  scrollbar-color: rgba(156, 163, 175, 0.3) transparent;
}

.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}

.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background-color: rgba(156, 163, 175, 0.3);
  border-radius: 3px;
}

.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background-color: rgba(156, 163, 175, 0.5);
}
</style>
