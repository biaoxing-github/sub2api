<template>
  <AppLayout>
    <TablePageLayout page-scroll>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-64">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.keyword"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.accountProbeReports.keywordPlaceholder')"
              @input="handleKeywordInput"
            />
          </div>
          <input
            v-model="filters.account_id"
            type="number"
            min="1"
            class="input w-full sm:w-32"
            :placeholder="t('admin.accountProbeReports.accountId')"
            @change="applyFilters"
          />
          <div class="w-full sm:w-36">
            <Select v-model="filters.status" :options="statusOptions" @change="applyFilters" />
          </div>
          <div class="w-full sm:w-36">
            <Select v-model="filters.mode" :options="modeOptions" @change="applyFilters" />
          </div>
          <div class="w-full sm:w-36">
            <Select v-model="filters.request_mode" :options="requestModeOptions" @change="applyFilters" />
          </div>
          <input
            v-model="filters.model"
            type="text"
            class="input w-full sm:w-40"
            :placeholder="t('admin.accountProbeReports.model')"
            @change="applyFilters"
            @keyup.enter="applyFilters"
          />
          <input
            v-model="filters.start_time"
            type="datetime-local"
            class="input w-full sm:w-52"
            :aria-label="t('admin.accountProbeReports.startTime')"
            @change="applyFilters"
          />
          <input
            v-model="filters.end_time"
            type="datetime-local"
            class="input w-full sm:w-52"
            :aria-label="t('admin.accountProbeReports.endTime')"
            @change="applyFilters"
          />
          <div class="w-full sm:w-44">
            <Select v-model="sortState.sort_by" :options="sortOptions" @change="handleSortOptionChange" />
          </div>
          <button type="button" class="btn btn-secondary px-3" :disabled="loading" @click="loadRuns">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-1.5 hidden sm:inline">{{ t('common.refresh') }}</span>
          </button>
          <button type="button" class="btn btn-ghost px-3" @click="resetFilters">
            {{ t('common.reset') }}
          </button>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-dark-700 dark:bg-dark-800">
          <span class="text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.accountProbeReports.selectedAccounts', { count: selectedAccountIds.length }) }}
          </span>
          <div class="w-full sm:w-36">
            <Select v-model="batchForm.mode" :options="batchModeOptions" />
          </div>
          <div class="w-full sm:w-36">
            <Select v-model="batchForm.request_mode" :options="batchRequestModeOptions" />
          </div>
          <input
            v-model="batchForm.model"
            data-test="batch-probe-model"
            type="text"
            class="input w-full sm:w-44"
            :placeholder="t('admin.accountProbeReports.batchModelPlaceholder')"
          />
          <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="batchForm.codex_stability" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            <span>{{ t('admin.accountProbeReports.codexStability') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="batchForm.long_context" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            <span>{{ t('admin.accountProbeReports.longContext') }}</span>
          </label>
          <button
            type="button"
            data-test="batch-probe-submit"
            class="btn btn-primary px-3"
            :disabled="selectedAccountIds.length === 0 || batchSubmitting"
            @click="submitBatchProbe"
          >
            <Icon name="refresh" size="sm" :class="batchSubmitting ? 'animate-spin' : ''" />
            <span class="ml-1.5">{{ t('admin.accountProbeReports.batchProbe') }}</span>
          </button>
          <span v-if="batchMessage" class="text-sm text-emerald-600 dark:text-emerald-300">{{ batchMessage }}</span>
          <span v-if="batchError" class="text-sm text-rose-600 dark:text-rose-300">{{ batchError }}</span>
        </div>
        <div
          v-if="error"
          class="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200"
        >
          {{ error }}
        </div>
      </template>

      <template #table>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th class="w-12">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :checked="allVisibleSelected"
                    :disabled="runs.length === 0"
                    :aria-label="t('admin.accountProbeReports.selectAll')"
                    @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
                  />
                </th>
                <th>{{ t('admin.accountProbeReports.score') }}</th>
                <th>{{ t('admin.accountProbeReports.grade') }}</th>
                <th>{{ t('admin.accountProbeReports.account') }}</th>
                <th>{{ t('admin.accountProbeReports.model') }}</th>
                <th>{{ t('admin.accountProbeReports.mode') }}</th>
                <th>{{ t('admin.accountProbeReports.successRate') }}</th>
                <th>{{ t('admin.accountProbeReports.avgLatency') }}</th>
                <th>{{ t('admin.accountProbeReports.p95') }}</th>
                <th>{{ t('admin.accountProbeReports.firstToken') }}</th>
                <th>{{ t('admin.accountProbeReports.tokens') }}</th>
                <th>{{ t('admin.accountProbeReports.time') }}</th>
                <th>{{ t('admin.accountProbeReports.status') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && runs.length === 0">
                <td colspan="14" class="py-12 text-center text-gray-500 dark:text-gray-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="!loading && runs.length === 0">
                <td colspan="14" class="py-12 text-center text-gray-500 dark:text-gray-400">
                  {{ t('admin.accountProbeReports.empty') }}
                </td>
              </tr>
              <tr v-for="run in runs" :key="run.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                <td>
                  <input
                    type="checkbox"
                    data-test="probe-row-select"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :checked="selectedAccountIdSet.has(run.account_id)"
                    :aria-label="t('admin.accountProbeReports.selectAccount', { id: run.account_id })"
                    @change="toggleAccount(run.account_id, ($event.target as HTMLInputElement).checked)"
                  />
                </td>
                <td>
                  <span class="font-semibold text-gray-900 dark:text-gray-100">{{ formatNumber(run.score) }}</span>
                </td>
                <td>
              <span :class="gradeClass(run.grade)" class="inline-flex min-w-8 justify-center rounded-full px-2 py-0.5 text-xs font-semibold">
                    {{ run.grade_label || formatGrade(run.grade) }}
                  </span>
                </td>
                <td>
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ run.account_name || `#${run.account_id}` }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">#{{ run.account_id }}</div>
                </td>
                <td>{{ run.model || '-' }}</td>
                <td>
                  <div>{{ formatMode(run.mode) }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ formatRequestMode(run.request_mode) }}</div>
                </td>
                <td>{{ formatPercent(run.success_rate) }}</td>
                <td>{{ formatMs(run.avg_latency_ms ?? run.latency?.avg_ms) }}</td>
                <td>{{ formatMs(run.p95_ms ?? run.latency?.p95_ms) }}</td>
                <td>{{ formatMs(run.first_token_ms) }}</td>
                <td>{{ formatInteger(run.total_tokens) }}</td>
                <td>{{ formatDateTime(run.created_at) }}</td>
                <td>
                  <span :class="statusClass(run.status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                    {{ formatStatus(run.status) }}
                  </span>
                </td>
                <td>
                  <button
                    type="button"
                    data-test="open-probe-detail"
                    class="btn btn-ghost px-2 py-1 text-sm"
                    @click="openDetail(run)"
                  >
                    <Icon name="eye" size="sm" />
                    <span class="ml-1">{{ t('common.view') }}</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>

  <Teleport to="body">
    <transition name="fade">
      <div v-if="detailOpen" class="fixed inset-0 z-50 bg-black/40" @click="closeDetail"></div>
    </transition>
    <transition name="slide-left">
      <aside
        v-if="detailOpen"
        class="fixed right-0 top-0 z-50 flex h-full w-full max-w-3xl flex-col border-l border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900 sm:w-[42rem]"
      >
        <div class="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {{ t('admin.accountProbeReports.detailTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ detailRun?.account_name || (detailRun ? `#${detailRun.account_id}` : '-') }}
            </p>
          </div>
          <button type="button" class="btn btn-ghost px-2" :aria-label="t('common.close')" @click="closeDetail">
            <Icon name="x" size="md" />
          </button>
        </div>

        <div v-if="detailLoading" class="flex flex-1 items-center justify-center text-gray-500 dark:text-gray-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="detailError" class="m-5 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
          {{ detailError }}
        </div>
        <div v-else class="flex-1 overflow-y-auto px-5 py-4">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.score') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ formatNumber(detailRun?.score) }}</div>
              <div v-if="detailRun?.confidence != null" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.confidence') }} {{ formatPercent(detailRun.confidence) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.successRate') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ formatPercent(detailRun?.success_rate) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.avgLatency') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ formatMs(detailRun?.avg_latency_ms ?? detailRun?.latency?.avg_ms) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.tokens') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ formatInteger(detailRun?.total_tokens) }}</div>
            </div>
          </div>

          <section class="mt-5">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountProbeReports.scoreItems') }}</h3>
            <div class="mt-2 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
              <div v-if="!scoreItems.length" class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
              <div v-for="(item, index) in scoreItems" :key="`score-${index}-${scoreItemLabel(item)}`" class="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                <span class="text-gray-700 dark:text-gray-300">{{ scoreItemLabel(item) }}</span>
                <span class="font-medium text-gray-900 dark:text-gray-100">{{ formatScoreItemValue(item) }}</span>
              </div>
            </div>
          </section>

          <section class="mt-5">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountProbeReports.penaltyItems') }}</h3>
            <div class="mt-2 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
              <div v-if="!penaltyItems.length" class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
              <div v-for="(item, index) in penaltyItems" :key="`penalty-${index}-${scoreItemLabel(item)}`" class="px-3 py-2 text-sm">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-gray-700 dark:text-gray-300">{{ scoreItemLabel(item) }}</span>
                  <span class="font-medium text-rose-600 dark:text-rose-300">{{ formatScoreItemValue(item) }}</span>
                </div>
                <p v-if="scoreItemDescription(item)" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ scoreItemDescription(item) }}</p>
              </div>
            </div>
          </section>

          <section class="mt-5">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountProbeReports.samples') }}</h3>
            <div class="mt-2 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="w-full min-w-[720px]">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">#</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.status') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.avgLatency') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.firstToken') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.tokens') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountProbeReports.error') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!detailRun?.samples?.length">
                    <td colspan="6" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</td>
                  </tr>
                  <tr v-for="sample in detailRun?.samples || []" :key="sample.id" class="border-t border-gray-100 dark:border-dark-700">
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ sample.request_index }}</td>
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ formatStatus(sample.status) }}</td>
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ formatMs(sample.latency_ms) }}</td>
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ formatMs(sample.first_token_ms) }}</td>
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ formatInteger(sample.tokens) }}</td>
                    <td class="px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{{ sample.error || sample.error_code || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </aside>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getConfiguredTablePageSizeOptions, normalizeTablePageSize } from '@/utils/tablePreferences'
import { batchAccountProbeRuns, listAccountProbeRuns, getAccountProbeRun } from '@/api/admin/accounts'
import type { AccountProbeRun, AccountProbeRunListFilters, AccountProbeRunSortBy, AccountProbeScoreBreakdownItem, SelectOption } from '@/types'

const { t } = useI18n()

const runs = ref<AccountProbeRun[]>([])
const loading = ref(false)
const error = ref('')
const detailOpen = ref(false)
const detailLoading = ref(false)
const detailError = ref('')
const detailRun = ref<AccountProbeRun | null>(null)
const selectedAccountIds = ref<number[]>([])
const batchSubmitting = ref(false)
const batchMessage = ref('')
const batchError = ref('')

const filters = reactive({
  account_id: '',
  status: '',
  mode: '',
  request_mode: '',
  model: '',
  keyword: '',
  start_time: '',
  end_time: '',
})

const sortState = reactive({
  sort_by: 'created_at' as AccountProbeRunSortBy,
  sort_order: 'desc' as 'asc' | 'desc',
})

const batchForm = reactive({
  mode: 'standard',
  request_mode: 'stream',
  model: '',
  codex_stability: false,
  long_context: false,
})

const pagination = reactive({
  page: 1,
  page_size: normalizeTablePageSize(getConfiguredTablePageSizeOptions()[0] ?? 20),
  total: 0,
})

const scoreItems = computed<AccountProbeScoreBreakdownItem[]>(() => detailRun.value?.score_items || [])
const penaltyItems = computed<AccountProbeScoreBreakdownItem[]>(() => detailRun.value?.penalty_items || [])
const selectedAccountIdSet = computed(() => new Set(selectedAccountIds.value))
const visibleAccountIds = computed(() => Array.from(new Set(runs.value.map(run => run.account_id))))
const allVisibleSelected = computed(() => visibleAccountIds.value.length > 0 && visibleAccountIds.value.every(id => selectedAccountIdSet.value.has(id)))

let listAbortController: AbortController | null = null
let detailAbortController: AbortController | null = null
let batchAbortController: AbortController | null = null
let keywordTimer: number | null = null

const statusOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.accountProbeReports.allStatuses') },
  { value: 'success', label: t('admin.accountProbeReports.statuses.success') },
  { value: 'partial', label: t('admin.accountProbeReports.statuses.partial') },
  { value: 'failed', label: t('admin.accountProbeReports.statuses.failed') },
  { value: 'running', label: t('admin.accountProbeReports.statuses.running') },
  { value: 'pending', label: t('admin.accountProbeReports.statuses.pending') },
])

const modeOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.accountProbeReports.allModes') },
  { value: 'quick', label: t('admin.accountProbeReports.modes.quick') },
  { value: 'standard', label: t('admin.accountProbeReports.modes.standard') },
])

const requestModeOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.accountProbeReports.allRequestModes') },
  { value: 'non_stream', label: t('admin.accountProbeReports.requestModes.non_stream') },
  { value: 'stream', label: t('admin.accountProbeReports.requestModes.stream') },
])

const batchModeOptions = computed<SelectOption[]>(() => [
  { value: 'quick', label: t('admin.accountProbeReports.modes.quick') },
  { value: 'standard', label: t('admin.accountProbeReports.modes.standard') },
])

const batchRequestModeOptions = computed<SelectOption[]>(() => [
  { value: 'non_stream', label: t('admin.accountProbeReports.requestModes.non_stream') },
  { value: 'stream', label: t('admin.accountProbeReports.requestModes.stream') },
])

const sortOptions = computed<SelectOption[]>(() => [
  { value: 'created_at', label: t('admin.accountProbeReports.sort.created_at') },
  { value: 'score', label: t('admin.accountProbeReports.sort.score') },
  { value: 'success_rate', label: t('admin.accountProbeReports.sort.success_rate') },
  { value: 'avg_latency_ms', label: t('admin.accountProbeReports.sort.avg_latency_ms') },
  { value: 'p95_ms', label: t('admin.accountProbeReports.sort.p95_ms') },
  { value: 'first_token_ms', label: t('admin.accountProbeReports.sort.first_token_ms') },
  { value: 'total_tokens', label: t('admin.accountProbeReports.sort.total_tokens') },
])

const buildFilters = (): AccountProbeRunListFilters => ({
  account_id: filters.account_id || undefined,
  status: filters.status || undefined,
  mode: filters.mode || undefined,
  request_mode: filters.request_mode || undefined,
  model: filters.model.trim() || undefined,
  keyword: filters.keyword.trim() || undefined,
  start_time: toIsoString(filters.start_time),
  end_time: toIsoString(filters.end_time),
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order,
})

async function loadRuns() {
  listAbortController?.abort()
  const controller = new AbortController()
  listAbortController = controller
  loading.value = true
  error.value = ''
  try {
    const response = await listAccountProbeRuns(pagination.page, pagination.page_size, buildFilters(), {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    runs.value = response.items || []
    pagination.total = response.total || 0
    pagination.page = response.page || pagination.page
    pagination.page_size = response.page_size || pagination.page_size
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountProbeReports.failedToLoad')
    runs.value = []
    pagination.total = 0
  } finally {
    if (listAbortController === controller) {
      loading.value = false
      listAbortController = null
    }
  }
}

async function openDetail(run: AccountProbeRun) {
  detailOpen.value = true
  detailRun.value = run
  detailError.value = ''
  detailLoading.value = true
  detailAbortController?.abort()
  const controller = new AbortController()
  detailAbortController = controller
  try {
    detailRun.value = await getAccountProbeRun(run.id, { signal: controller.signal })
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    detailError.value = err?.response?.data?.error || err?.message || t('admin.accountProbeReports.failedToLoadDetail')
  } finally {
    if (detailAbortController === controller) {
      detailLoading.value = false
      detailAbortController = null
    }
  }
}

async function submitBatchProbe() {
  if (selectedAccountIds.value.length === 0 || batchSubmitting.value) return
  batchAbortController?.abort()
  const controller = new AbortController()
  batchAbortController = controller
  batchSubmitting.value = true
  batchError.value = ''
  batchMessage.value = ''
  try {
    const response = await batchAccountProbeRuns({
      account_ids: selectedAccountIds.value,
      mode: batchForm.mode,
      model: batchForm.model.trim() || undefined,
      request_mode: batchForm.request_mode,
      codex_stability: batchForm.codex_stability,
      long_context: batchForm.long_context,
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    batchMessage.value = t('admin.accountProbeReports.batchAccepted', { count: response.accepted_count })
    selectedAccountIds.value = []
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    batchError.value = err?.response?.data?.error || err?.message || t('admin.accountProbeReports.batchFailed')
  } finally {
    if (batchAbortController === controller) {
      batchSubmitting.value = false
      batchAbortController = null
    }
  }
}

function closeDetail() {
  detailAbortController?.abort()
  detailOpen.value = false
  detailLoading.value = false
}

function toggleAccount(accountId: number, checked: boolean) {
  batchMessage.value = ''
  batchError.value = ''
  const ids = new Set(selectedAccountIds.value)
  if (checked) {
    ids.add(accountId)
  } else {
    ids.delete(accountId)
  }
  selectedAccountIds.value = Array.from(ids)
}

function toggleAllVisible(checked: boolean) {
  batchMessage.value = ''
  batchError.value = ''
  const ids = new Set(selectedAccountIds.value)
  for (const accountId of visibleAccountIds.value) {
    if (checked) {
      ids.add(accountId)
    } else {
      ids.delete(accountId)
    }
  }
  selectedAccountIds.value = Array.from(ids)
}

function applyFilters() {
  pagination.page = 1
  loadRuns()
}

function handleKeywordInput() {
  if (keywordTimer) window.clearTimeout(keywordTimer)
  keywordTimer = window.setTimeout(applyFilters, 300)
}

function resetFilters() {
  Object.assign(filters, {
    account_id: '',
    status: '',
    mode: '',
    request_mode: '',
    model: '',
    keyword: '',
    start_time: '',
    end_time: '',
  })
  sortState.sort_by = 'created_at'
  sortState.sort_order = 'desc'
  applyFilters()
}

function handleSortOptionChange() {
  sortState.sort_order = 'desc'
  applyFilters()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRuns()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRuns()
}

function toIsoString(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function formatNumber(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(value % 1 === 0 ? 0 : 1) : '-'
}

function formatInteger(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString() : '-'
}

function formatPercent(value: number | null | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  const pct = value <= 1 ? value * 100 : value
  return `${pct.toFixed(1)}%`
}

function formatMs(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${Math.round(value).toLocaleString()}ms` : '-'
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatMode(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountProbeReports.modes.${value}`)
}

function formatRequestMode(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountProbeReports.requestModes.${value}`)
}

function formatStatus(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountProbeReports.statuses.${value}`)
}

function formatScoreItemValue(item: AccountProbeScoreBreakdownItem): string {
  if (typeof item === 'string') return ''
  const value = item.value ?? item.score ?? '-'
  return item.max == null ? String(value) : `${String(value)} / ${String(item.max)}`
}

function scoreItemLabel(item: AccountProbeScoreBreakdownItem): string {
  return typeof item === 'string' ? item : String(item.label || item.key || '-')
}

function scoreItemDescription(item: AccountProbeScoreBreakdownItem): string {
  return typeof item === 'string' ? '' : String(item.reason || item.description || '')
}

function formatGrade(value: string | null | undefined): string {
  if (!value) return '-'
  return t(`admin.accountProbeReports.grades.${value}`)
}

function gradeClass(grade: string | null | undefined): string {
  const normalized = (grade || '').toLowerCase()
  if (normalized === 'excellent' || normalized.startsWith('a')) return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (normalized === 'stable' || normalized.startsWith('b')) return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
  if (normalized === 'slow' || normalized.startsWith('c')) return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (normalized === 'unstable' || normalized === 'poor') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

function statusClass(status: string): string {
  if (status === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'partial' || status === 'running') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (status === 'failed') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

onMounted(loadRuns)

onUnmounted(() => {
  listAbortController?.abort()
  detailAbortController?.abort()
  batchAbortController?.abort()
  if (keywordTimer) window.clearTimeout(keywordTimer)
})
</script>

<style scoped>
.slide-left-enter-active,
.slide-left-leave-active {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.slide-left-enter-from,
.slide-left-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
