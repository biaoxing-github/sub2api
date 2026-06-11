<template>
  <AppLayout>
    <TablePageLayout page-scroll>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-64">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.accountSchedulingPool.searchPlaceholder')"
              @input="handleSearchInput"
            />
          </div>
          <div class="w-full sm:w-44">
            <Select data-test="platform-filter" v-model="filters.platform" :options="platformOptions" @change="applyFilters" />
          </div>
          <div class="w-full sm:w-56">
            <Select data-test="group-filter" v-model="filters.group" :options="groupOptions" @change="applyFilters" />
          </div>
          <div v-if="isOpenAIPlatform" class="w-full sm:w-52">
            <Select v-model="filters.transport" :options="transportOptions" @change="applyFilters" />
          </div>
          <button type="button" class="btn btn-secondary px-3" :disabled="loading" @click="loadPool">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-1.5 hidden sm:inline">{{ t('common.refresh') }}</span>
          </button>
          <button type="button" class="btn btn-ghost px-3" @click="resetFilters">
            {{ t('common.reset') }}
          </button>
        </div>

        <div v-if="snapshot" class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <div v-for="metric in metrics" :key="metric.key" class="rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ metric.value }}</div>
          </div>
        </div>

        <div v-if="message" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200">
          {{ message }}
        </div>
        <div v-if="error" class="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
          {{ error }}
        </div>
      </template>

      <template #table>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>{{ t('admin.accountSchedulingPool.account') }}</th>
                <th>{{ t('admin.accountSchedulingPool.poolStatus') }}</th>
                <th>{{ t('admin.accountSchedulingPool.health') }}</th>
                <th>{{ t('admin.accountSchedulingPool.capacity') }}</th>
                <th>{{ t('admin.accountSchedulingPool.reasons') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && items.length === 0">
                <td colspan="6" class="py-12 text-center text-gray-500 dark:text-gray-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="!loading && items.length === 0">
                <td colspan="6" class="py-12 text-center text-gray-500 dark:text-gray-400">
                  {{ t('admin.accountSchedulingPool.empty') }}
                </td>
              </tr>
              <tr
                v-for="item in items"
                :key="item.account.id"
                :class="['hover:bg-gray-50 dark:hover:bg-dark-700/40', schedulingPoolRowClass(item)]"
              >
                <td>
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ item.account.name }}</div>
                  <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    #{{ item.account.id }} · {{ formatAccountPlatform(item.account.platform) }} · {{ formatAccountType(item.account.type) }}
                  </div>
                  <div v-if="formatGroups(item.account)" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ formatGroups(item.account) }}
                  </div>
                </td>
                <td>
                  <span :class="poolStatusClass(item.pool_status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                    {{ formatPoolStatus(item.pool_status) }}
                  </span>
                  <div v-if="item.runtime_block?.reason" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accountSchedulingPool.runtimeBlock') }}: {{ item.runtime_block.reason }}
                  </div>
                </td>
                <td>
                  <span :class="healthClass(item.derived_health?.state)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                    {{ item.derived_health?.label || formatPathHealth(item.path_health?.state) }}
                  </span>
                  <div v-if="item.account.load_factor_advice" class="mt-1">
                    <AccountAvailabilityRadarBadge :advice="item.account.load_factor_advice" />
                  </div>
                  <div v-if="item.path_health_available" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accountSchedulingPool.pathHealth') }}: {{ formatPathHealth(item.path_health?.state) }}
                  </div>
                  <div v-if="item.path_health_available && item.path_health?.last_failure_reason" class="mt-0.5 max-w-xs break-words text-xs text-gray-500 dark:text-gray-400">
                    {{ item.path_health.last_failure_reason }}
                  </div>
                </td>
                <td>
                  <div class="text-sm text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountSchedulingPool.priority') }} {{ item.account.priority }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accountSchedulingPool.concurrency') }} {{ item.account.concurrency }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accountSchedulingPool.loadFactor') }} {{ item.effective_load_factor }}
                  </div>
                </td>
                <td>
                  <div v-if="visiblePoolReasons(item).length" class="flex max-w-xl flex-wrap gap-1.5">
                    <span
                      v-for="reason in visiblePoolReasons(item)"
                      :key="reason"
                      class="rounded-md bg-gray-100 px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200"
                    >
                      {{ reason }}
                    </span>
                  </div>
                  <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
                </td>
                <td>
                  <button
                    type="button"
                    data-test="disable-scheduling"
                    class="btn btn-danger px-2 py-1 text-sm"
                    :disabled="isDisabling(item.account.id)"
                    :title="t('admin.accountSchedulingPool.disableScheduling')"
                    @click="disableScheduling(item)"
                  >
                    <Icon name="ban" size="sm" :class="isDisabling(item.account.id) ? 'animate-pulse' : ''" />
                    <span class="ml-1">{{ t('admin.accountSchedulingPool.disableScheduling') }}</span>
                  </button>
                  <button
                    v-if="shouldShowManualProbe(item.account)"
                    type="button"
                    data-test="manual-probe"
                    class="btn btn-primary px-2 py-1 text-sm ml-2"
                    :disabled="isProbing(item.account.id)"
                    :title="t('admin.accountSchedulingPool.manualProbe')"
                    @click="manualProbe(item)"
                  >
                    <Icon name="refresh" size="sm" :class="isProbing(item.account.id) ? 'animate-spin' : ''" />
                    <span class="ml-1">{{ t('admin.accountSchedulingPool.manualProbe') }}</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountAvailabilityRadarBadge from '@/components/account/AccountAvailabilityRadarBadge.vue'
import { listSchedulingPool, setSchedulable, manualProbeAccount, getAvailableModels } from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import type {
  Account,
  AdminGroup,
  ClaudeModel,
  OpenAIAccountSchedulingPoolFilters,
  OpenAIAccountSchedulingPoolItem,
  OpenAIAccountSchedulingPoolResponse,
  SelectOption,
} from '@/types'

const { t } = useI18n()

const filters = reactive<OpenAIAccountSchedulingPoolFilters>({
  group: '',
  platform: 'openai',
  transport: 'http_sse',
  search: '',
})

const snapshot = ref<OpenAIAccountSchedulingPoolResponse | null>(null)
const groups = ref<AdminGroup[]>([])
const loading = ref(false)
const error = ref('')
const message = ref('')
const disablingIds = ref<Set<number>>(new Set())
const probingIds = ref<Set<number>>(new Set())

let listAbortController: AbortController | null = null
let searchTimer: number | null = null

const items = computed(() => snapshot.value?.items || [])
const isOpenAIPlatform = computed(() => filters.platform === 'openai')
const metrics = computed(() => [
  { key: 'total', label: t('admin.accountSchedulingPool.metrics.total'), value: snapshot.value?.total ?? 0 },
  { key: 'schedulable', label: t('admin.accountSchedulingPool.metrics.schedulable'), value: snapshot.value?.schedulable_count ?? 0 },
  { key: 'degraded', label: t('admin.accountSchedulingPool.metrics.degraded'), value: snapshot.value?.degraded_count ?? 0 },
  { key: 'blocked', label: t('admin.accountSchedulingPool.metrics.blocked'), value: snapshot.value?.blocked_count ?? 0 },
  { key: 'filtered', label: t('admin.accountSchedulingPool.metrics.filtered'), value: snapshot.value?.filtered_count ?? 0 },
])

const platformOptions = computed<SelectOption[]>(() => [
  { value: 'openai', label: t('admin.accountSchedulingPool.platforms.openai') },
  { value: 'anthropic', label: t('admin.accountSchedulingPool.platforms.anthropic') },
])

const groupOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.accountSchedulingPool.ungrouped') },
  ...groups.value.map(group => ({
    value: String(group.id),
    label: formatGroupOption(group),
  })),
])

const transportOptions = computed<SelectOption[]>(() => [
  { value: 'http_sse', label: t('admin.accountSchedulingPool.transports.httpSse') },
  { value: '', label: t('admin.accountSchedulingPool.transports.any') },
  { value: 'responses_websockets', label: t('admin.accountSchedulingPool.transports.responsesWebsocket') },
  { value: 'responses_websockets_v2', label: t('admin.accountSchedulingPool.transports.responsesWebsocketV2') },
])

function buildFilters(): OpenAIAccountSchedulingPoolFilters {
  const built: OpenAIAccountSchedulingPoolFilters = {
    group: filters.group?.trim() || undefined,
    platform: filters.platform || 'openai',
    search: filters.search?.trim() || undefined,
  }
  if (filters.platform === 'openai') {
    built.transport = filters.transport || undefined
  }
  return built
}

async function loadGroups() {
  try {
    groups.value = await groupsAPI.getAll()
    if (!filters.group) {
      filters.group = defaultGroupValue()
    }
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || t('admin.accountSchedulingPool.failedToLoadGroups')
  }
}

function defaultGroupValue(): string {
  const activeGroups = groups.value.filter(group => group.status === 'active')
  const selfUse = activeGroups.find(group => group.name?.trim() === '自用')
  const selected = selfUse || activeGroups[0]
  return selected ? String(selected.id) : ''
}

async function loadPool() {
  listAbortController?.abort()
  const controller = new AbortController()
  listAbortController = controller
  loading.value = true
  error.value = ''
  try {
    snapshot.value = await listSchedulingPool(buildFilters(), { signal: controller.signal })
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountSchedulingPool.failedToLoad')
    snapshot.value = null
  } finally {
    if (listAbortController === controller) {
      loading.value = false
      listAbortController = null
    }
  }
}

function applyFilters() {
  message.value = ''
  loadPool()
}

function handleSearchInput() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(applyFilters, 300)
}

function resetFilters() {
  Object.assign(filters, {
    group: defaultGroupValue(),
    platform: 'openai',
    transport: 'http_sse',
    search: '',
  })
  applyFilters()
}

async function disableScheduling(item: OpenAIAccountSchedulingPoolItem) {
  if (isDisabling(item.account.id)) return
  if (!window.confirm(t('admin.accountSchedulingPool.disableConfirm', { name: item.account.name }))) return
  setDisabling(item.account.id, true)
  message.value = ''
  error.value = ''
  try {
    await setSchedulable(item.account.id, false)
    message.value = t('admin.accountSchedulingPool.disabledSuccess', { name: item.account.name })
    await loadPool()
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || t('admin.accountSchedulingPool.disabledFailed')
  } finally {
    setDisabling(item.account.id, false)
  }
}

function isDisabling(accountId: number): boolean {
  return disablingIds.value.has(accountId)
}

function setDisabling(accountId: number, value: boolean) {
  const next = new Set(disablingIds.value)
  if (value) {
    next.add(accountId)
  } else {
    next.delete(accountId)
  }
  disablingIds.value = next
}

function isProbing(accountId: number): boolean {
  return probingIds.value.has(accountId)
}

function shouldShowManualProbe(account: Account): boolean {
  if (account.platform === 'openai') return true
  if (account.platform === 'anthropic') return true
  if (account.platform === 'antigravity') {
    return !!(account as any).mixed_scheduling_enabled
  }
  return false
}

async function manualProbe(item: OpenAIAccountSchedulingPoolItem) {
  const accountId = item.account.id
  probingIds.value.add(accountId)
  error.value = ''
  message.value = ''

  try {
    const result = await manualProbeAccount(accountId, await buildManualProbePayload(item.account))

    const probeResult = result.result
    if (result.success && probeResult?.success) {
      message.value = t('admin.accountSchedulingPool.probeSuccess', {
        name: item.account.name,
        latency: probeResult.latency_ms
      })
      await loadPool()
    } else {
      error.value = probeResult?.message || probeResult?.error || t('admin.accountSchedulingPool.probeFailed')
    }
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.response?.data?.message || err?.message || t('common.error')
  } finally {
    probingIds.value.delete(accountId)
  }
}

async function buildManualProbePayload(account: Account): Promise<{ model?: string }> {
  const models = await getAvailableModels(account.id)
  const model = selectAccountTestModel(account, models)
  return model ? { model } : {}
}

function selectAccountTestModel(account: Account, models: ClaudeModel[]): string {
  // 与账号测试弹窗保持一致：按平台选中实际会提交给 /test 的默认模型。
  const availableModels = account.platform === 'antigravity' ? sortAccountTestModels(models) : models
  if (availableModels.length === 0) return ''
  if (account.platform === 'openai') {
    return availableModels.find(model => model.id === 'gpt-5.5')?.id || availableModels[0]?.id || ''
  }
  if (account.platform === 'gemini') {
    return sortAccountTestModels(availableModels)[0]?.id || ''
  }
  const sonnetModel = availableModels.find(model => model.id.includes('sonnet'))
  return sonnetModel?.id || availableModels[0]?.id || ''
}

function sortAccountTestModels(models: ClaudeModel[]): ClaudeModel[] {
  const prioritizedGeminiModels = [
    'gemini-3.1-flash-image',
    'gemini-2.5-flash-image',
    'gemini-3.5-flash',
    'gemini-2.5-flash',
    'gemini-2.5-pro',
    'gemini-3-flash-preview',
    'gemini-3-pro-preview',
    'gemini-2.0-flash',
  ]
  const priorityMap = new Map(prioritizedGeminiModels.map((id, index) => [id, index]))
  return [...models].sort((a, b) => {
    const aPriority = priorityMap.get(a.id) ?? Number.MAX_SAFE_INTEGER
    const bPriority = priorityMap.get(b.id) ?? Number.MAX_SAFE_INTEGER
    if (aPriority !== bPriority) return aPriority - bPriority
    return 0
  })
}

function formatPoolStatus(status: string): string {
  const key = `admin.accountSchedulingPool.statuses.${status}`
  const translated = t(key)
  return translated === key ? status || '-' : translated
}

function formatPathHealth(state?: string): string {
  if (!state) return '-'
  const key = `admin.accountSchedulingPool.pathHealthStates.${state}`
  const translated = t(key)
  return translated === key ? state : translated
}

function formatAccountType(type: string): string {
  const key = `admin.accounts.types.${type}`
  const translated = t(key)
  return translated === key ? type : translated
}

function formatAccountPlatform(platform: string): string {
  const key = `admin.accountSchedulingPool.platforms.${platform}`
  const translated = t(key)
  return translated === key ? platform : translated
}

function formatGroupOption(group: AdminGroup): string {
  const platform = formatAccountPlatform(group.platform)
  return `${group.name} · ${platform}`
}

function formatGroups(account: Account): string {
  const groupNames = (account.groups || [])
    .map(group => group?.name || (group?.id != null ? `#${group.id}` : ''))
    .filter(Boolean)
  if (groupNames.length > 0) return groupNames.join(', ')
  const groupIds = (account.group_ids || []).map(id => `#${id}`)
  return groupIds.join(', ')
}

function poolStatusClass(status: string): string {
  if (status === 'schedulable') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'degraded') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (status === 'blocked') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  if (status === 'filtered') return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

function healthClass(state?: string): string {
  if (state === 'normal') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (state === 'light_abnormal') return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
  if (state === 'moderate_abnormal' || state === 'rate_limited_cooldown' || state === 'pending_retest') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (state === 'line_degraded' || state === 'temp_unschedulable') return 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
  if (state === 'quota_low' || state === 'quota_exhausted' || state === 'disabled' || state === 'unauthorized_invalid' || state === 'upstream_abnormal') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

function visiblePoolReasons(item: OpenAIAccountSchedulingPoolItem): string[] {
  const advice = item.account.load_factor_advice
  return uniqueVisibleReasons([
    ...(item.pool_reasons || []),
    item.derived_health?.reason || '',
    item.derived_health?.last_failure_reason || '',
    ...(advice?.availability_radar?.reasons || []),
    ...(advice?.reasons || []),
  ])
}

function schedulingPoolRowClass(item: OpenAIAccountSchedulingPoolItem): string {
  if (item.pool_status === 'blocked') return 'bg-rose-50/60 dark:bg-rose-950/20'
  if (item.pool_status === 'filtered') return 'bg-sky-50/50 dark:bg-sky-950/20'
  if (item.pool_status === 'degraded') return 'bg-amber-50/60 dark:bg-amber-950/20'

  const state = item.derived_health?.state
  if (state === 'quota_low' || state === 'quota_exhausted' || state === 'disabled' || state === 'unauthorized_invalid' || state === 'upstream_abnormal') {
    return 'bg-rose-50/60 dark:bg-rose-950/20'
  }
  if (state === 'line_degraded' || state === 'temp_unschedulable' || state === 'moderate_abnormal' || state === 'rate_limited_cooldown') {
    return 'bg-amber-50/60 dark:bg-amber-950/20'
  }
  if (state === 'light_abnormal' || state === 'pending_retest') {
    return 'bg-sky-50/50 dark:bg-sky-950/20'
  }

  switch (item.account.load_factor_advice?.availability_radar?.status) {
    case 'unstable':
    case 'cooldown':
      return 'bg-rose-50/60 dark:bg-rose-950/20'
    case 'balance_risk':
    case 'needs_probe':
      return 'bg-amber-50/60 dark:bg-amber-950/20'
    case 'slow_usable':
      return 'bg-sky-50/50 dark:bg-sky-950/20'
    default:
      return ''
  }
}

function uniqueVisibleReasons(values: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const raw of values) {
    const value = String(raw || '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    result.push(value)
  }
  return result
}

onMounted(async () => {
  await loadGroups()
  await loadPool()
})

onUnmounted(() => {
  listAbortController?.abort()
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>
