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
        <DataTable
          :columns="schedulingPoolColumns"
          :data="items"
          :loading="loading && items.length === 0"
          :row-key="resolveSchedulingPoolRowKey"
          :estimate-row-height="132"
          :overscan="8"
          fit-width
        >
          <template #empty>
            <div class="py-8 text-center text-gray-500 dark:text-gray-400">
              {{ t('admin.accountSchedulingPool.empty') }}
            </div>
          </template>

          <template #cell-account="{ row: item }">
            <div class="font-medium text-gray-900 dark:text-gray-100">{{ item.account.name }}</div>
            <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              #{{ item.account.id }} · {{ formatAccountPlatform(item.account.platform) }} · {{ formatAccountType(item.account.type) }}
            </div>
            <div v-if="formatGroups(item.account)" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ formatGroups(item.account) }}
            </div>
          </template>

          <template #cell-pool_status="{ row: item }">
            <span :class="poolStatusClass(item.pool_status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
              {{ formatPoolStatus(item.pool_status) }}
            </span>
            <div v-if="item.runtime_block?.reason" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accountSchedulingPool.runtimeBlock') }}: {{ item.runtime_block.reason }}
            </div>
            <div v-if="item.next_scheduled_at" class="mt-1 text-xs text-gray-600 dark:text-gray-300">
              <div>{{ t('admin.accountSchedulingPool.nextScheduledAt') }}: {{ formatDateTime(item.next_scheduled_at) }}</div>
              <div v-if="item.next_scheduled_reason" class="mt-0.5 text-gray-500 dark:text-gray-400">
                {{ formatNextScheduledReason(item.next_scheduled_reason) }}
              </div>
            </div>
          </template>

          <template #cell-health="{ row: item }">
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
          </template>

          <template #cell-capacity="{ row: item }">
            <div class="flex items-center gap-1.5">
              <label :for="`scheduling-priority-${item.account.id}`" class="sr-only">
                {{ t('admin.accountSchedulingPool.priority') }}
              </label>
              <input
                :id="`scheduling-priority-${item.account.id}`"
                :value="priorityDrafts[item.account.id]"
                type="number"
                min="1"
                step="1"
                class="input h-8 w-20 px-2 py-1 text-sm"
                :aria-label="t('admin.accountSchedulingPool.priorityFor', { name: item.account.name })"
                :disabled="isSavingPriority(item.account.id)"
                data-test="priority-input"
                @input="setPriorityDraft(item.account.id, $event)"
                @keydown.enter.prevent="savePriority(item)"
              />
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :disabled="!canSavePriority(item)"
                :title="t('admin.accountSchedulingPool.savePriority')"
                data-test="save-priority"
                @click="savePriority(item)"
              >
                <Icon name="check" size="sm" :class="isSavingPriority(item.account.id) ? 'animate-pulse' : ''" />
              </button>
            </div>
            <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accountSchedulingPool.priorityHint') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accountSchedulingPool.concurrency') }} {{ item.account.concurrency }}
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accountSchedulingPool.loadFactor') }} {{ item.effective_load_factor }}
            </div>
          </template>

          <template #cell-reasons="{ row: item }">
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
          </template>

          <template #cell-actions="{ row: item }">
            <div class="space-y-2">
              <div v-if="disabledAPIKeyItems(item).length" class="space-y-1">
                <div
                  v-for="apiKey in disabledAPIKeyItems(item)"
                  :key="apiKey.fingerprint"
                  class="flex min-w-0 items-center gap-2 text-xs text-rose-700 dark:text-rose-200"
                >
                  <span class="min-w-0 flex-1 truncate font-mono" :title="apiKey.last_error || apiKey.reason || apiKey.masked">
                    {{ apiKey.masked }}
                  </span>
                  <button
                    type="button"
                    data-test="restore-api-key"
                    class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-rose-600 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-40 dark:text-rose-300 dark:hover:bg-rose-950/40"
                    :disabled="isRestoringAPIKey(item.account.id, apiKey.fingerprint)"
                    :title="t('admin.accountSchedulingPool.restoreApiKey')"
                    :aria-label="t('admin.accountSchedulingPool.restoreApiKey')"
                    @click="restoreAPIKey(item, apiKey.fingerprint)"
                  >
                    <Icon name="refresh" size="sm" :class="isRestoringAPIKey(item.account.id, apiKey.fingerprint) ? 'animate-spin' : ''" />
                  </button>
                </div>
              </div>
              <div class="flex flex-wrap items-center justify-end gap-2">
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
                  class="btn btn-primary px-2 py-1 text-sm"
                  :disabled="isProbing(item.account.id)"
                  :title="t('admin.accountSchedulingPool.manualProbe')"
                  @click="manualProbe(item)"
                >
                  <Icon name="refresh" size="sm" :class="isProbing(item.account.id) ? 'animate-spin' : ''" />
                  <span class="ml-1">{{ t('admin.accountSchedulingPool.manualProbe') }}</span>
                </button>
              </div>
            </div>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountAvailabilityRadarBadge from '@/components/account/AccountAvailabilityRadarBadge.vue'
import { listSchedulingPool, setSchedulable, manualProbeAccount, getAvailableModels, restoreAccountAPIKeyState, update as updateAccount } from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
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

const snapshot = shallowRef<OpenAIAccountSchedulingPoolResponse | null>(null)
const groups = ref<AdminGroup[]>([])
const loading = ref(false)
const error = ref('')
const message = ref('')
const disablingIds = ref<Set<number>>(new Set())
const probingIds = ref<Set<number>>(new Set())
const savingPriorityIds = ref<Set<number>>(new Set())
const restoringAPIKeyIds = ref<Set<string>>(new Set())
const priorityDrafts = reactive<Record<number, number | null>>({})

let listAbortController: AbortController | null = null
let searchTimer: number | null = null

const items = computed(() => snapshot.value?.items || [])
const isOpenAIPlatform = computed(() => filters.platform === 'openai')
const schedulingPoolColumns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.accountSchedulingPool.account'), class: 'w-[18rem] align-top !whitespace-normal' },
  { key: 'pool_status', label: t('admin.accountSchedulingPool.poolStatus'), class: 'w-[12rem] align-top !whitespace-normal' },
  { key: 'health', label: t('admin.accountSchedulingPool.health'), class: 'w-[16rem] align-top !whitespace-normal' },
  { key: 'capacity', label: t('admin.accountSchedulingPool.capacity'), class: 'w-[10rem] align-top !whitespace-normal' },
  { key: 'reasons', label: t('admin.accountSchedulingPool.reasons'), class: 'min-w-[16rem] align-top !whitespace-normal' },
  { key: 'actions', label: t('common.actions'), class: 'w-[18rem] align-top !whitespace-normal' },
])
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
    const nextSnapshot = await listSchedulingPool(buildFilters(), { signal: controller.signal })
    snapshot.value = nextSnapshot
    for (const item of nextSnapshot.items) {
      priorityDrafts[item.account.id] = item.account.priority
    }
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

function resolveSchedulingPoolRowKey(item: OpenAIAccountSchedulingPoolItem): number {
  return item.account.id
}

function setPriorityDraft(accountId: number, event: Event) {
  const value = (event.target as HTMLInputElement).value
  priorityDrafts[accountId] = value === '' ? null : Number(value)
}

function normalizedPriority(accountId: number): number | null {
  const value = priorityDrafts[accountId]
  return typeof value === 'number' && Number.isInteger(value) && value >= 1 ? value : null
}

function canSavePriority(item: OpenAIAccountSchedulingPoolItem): boolean {
  const priority = normalizedPriority(item.account.id)
  return priority !== null && priority !== item.account.priority && !isSavingPriority(item.account.id)
}

function isSavingPriority(accountId: number): boolean {
  return savingPriorityIds.value.has(accountId)
}

async function savePriority(item: OpenAIAccountSchedulingPoolItem) {
  if (!canSavePriority(item)) return
  const priority = normalizedPriority(item.account.id)
  if (priority === null) return

  const accountId = item.account.id
  const nextSavingIds = new Set(savingPriorityIds.value)
  nextSavingIds.add(accountId)
  savingPriorityIds.value = nextSavingIds
  message.value = ''
  error.value = ''
  try {
    await updateAccount(accountId, { priority })
    message.value = t('admin.accountSchedulingPool.priorityUpdated', {
      name: item.account.name,
      priority,
    })
    await loadPool()
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || t('admin.accountSchedulingPool.priorityUpdateFailed')
  } finally {
    const remainingSavingIds = new Set(savingPriorityIds.value)
    remainingSavingIds.delete(accountId)
    savingPriorityIds.value = remainingSavingIds
  }
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

function disabledAPIKeyItems(item: OpenAIAccountSchedulingPoolItem): NonNullable<Account['api_key_items']> {
  return (item.account.api_key_items || []).filter(apiKey => apiKey.disabled && !!apiKey.fingerprint)
}

function restoringAPIKeyID(accountId: number, fingerprint: string): string {
  return `${accountId}:${fingerprint}`
}

function isRestoringAPIKey(accountId: number, fingerprint: string): boolean {
  return restoringAPIKeyIds.value.has(restoringAPIKeyID(accountId, fingerprint))
}

async function restoreAPIKey(item: OpenAIAccountSchedulingPoolItem, fingerprint: string) {
  const accountId = item.account.id
  const restoringId = restoringAPIKeyID(accountId, fingerprint)
  if (restoringAPIKeyIds.value.has(restoringId)) return

  const nextRestoringIds = new Set(restoringAPIKeyIds.value)
  nextRestoringIds.add(restoringId)
  restoringAPIKeyIds.value = nextRestoringIds
  message.value = ''
  error.value = ''
  try {
    await restoreAccountAPIKeyState(accountId, fingerprint)
    message.value = t('admin.accountSchedulingPool.apiKeyRestored', { name: item.account.name })
    await loadPool()
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || t('admin.accountSchedulingPool.apiKeyRestoreFailed')
  } finally {
    const remainingRestoringIds = new Set(restoringAPIKeyIds.value)
    remainingRestoringIds.delete(restoringId)
    restoringAPIKeyIds.value = remainingRestoringIds
  }
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
    let nextMessage = ''
    let nextError = ''
    if (result.success && probeResult?.success) {
      nextMessage = t('admin.accountSchedulingPool.probeSuccess', {
        name: item.account.name,
        firstToken: probeResult.first_token_ms ?? '-',
        latency: probeResult.latency_ms
      })
    } else {
      nextError = probeResult?.message || probeResult?.error || t('admin.accountSchedulingPool.probeFailed')
    }
    await loadPool()
    message.value = nextMessage
    error.value = nextError
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
    return availableModels.find(model => model.id === 'gpt-5.6-terra')?.id || availableModels[0]?.id || ''
  }
  if (account.platform === 'gemini') {
    return sortAccountTestModels(availableModels)[0]?.id || ''
  }
  // Anthropic 默认人工测试指定 Opus 4.8，缺失时保持可用模型列表兜底。
  const opusModel = availableModels.find(model => model.id === 'claude-opus-4-8')
  return opusModel?.id || availableModels[0]?.id || ''
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

function formatNextScheduledReason(reason: string): string {
  const key = `admin.accountSchedulingPool.nextReasons.${reason}`
  const translated = t(key)
  return translated === key ? reason : translated
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
