<template>
  <AppLayout>
    <TablePageLayout page-scroll>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="filters.q" type="text" class="input w-full sm:w-64" :placeholder="t('admin.openaiRouteTrace.query')" @keyup.enter="reload" />
          <input v-model="filters.request_id" type="text" class="input w-full sm:w-52" :placeholder="t('admin.openaiRouteTrace.requestId')" @keyup.enter="reload" />
          <input v-model.number="filters.account_id" type="number" min="1" class="input w-full sm:w-32" :placeholder="t('admin.openaiRouteTrace.accountId')" @change="reload" />
          <input v-model="filters.model" type="text" class="input w-full sm:w-40" :placeholder="t('admin.openaiRouteTrace.model')" @keyup.enter="reload" />
          <div class="w-full sm:w-36">
            <Select v-model="filters.kind" :options="kindOptions" @change="reload" />
          </div>
          <div class="w-full sm:w-36">
            <Select v-model="filters.time_range" :options="timeRangeOptions" @change="reload" />
          </div>
          <button type="button" class="btn btn-secondary px-3" :disabled="loading" @click="reload">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-1.5">{{ t('common.refresh') }}</span>
          </button>
          <button type="button" class="btn btn-ghost px-3" @click="resetFilters">
            {{ t('common.reset') }}
          </button>
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
                <th>{{ t('admin.openaiRouteTrace.time') }}</th>
                <th>{{ t('admin.openaiRouteTrace.status') }}</th>
                <th>{{ t('admin.openaiRouteTrace.request') }}</th>
                <th>{{ t('admin.openaiRouteTrace.account') }}</th>
                <th>{{ t('admin.openaiRouteTrace.route') }}</th>
                <th>{{ t('admin.openaiRouteTrace.latency') }}</th>
                <th>{{ t('admin.openaiRouteTrace.tokens') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && rows.length === 0">
                <td colspan="8" class="py-12 text-center text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="!loading && rows.length === 0">
                <td colspan="8" class="py-12 text-center text-gray-500 dark:text-gray-400">{{ t('admin.openaiRouteTrace.empty') }}</td>
              </tr>
              <tr v-for="row in rows" :key="`${row.kind}-${row.request_id || row.created_at}`" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                <td class="whitespace-nowrap">{{ formatDateTime(row.created_at) }}</td>
                <td>
                  <span :class="kindBadgeClass(row.kind)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold">
                    {{ formatKind(row.kind) }}
                  </span>
                  <div v-if="row.status_code" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ row.status_code }}</div>
                </td>
                <td>
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.requested_model || row.model || '-' }}</div>
                  <div class="max-w-[240px] truncate font-mono text-xs text-gray-500 dark:text-gray-400" :title="row.request_id">{{ row.request_id || '-' }}</div>
                </td>
                <td>
                  <div>{{ row.account_name || (row.account_id ? `#${row.account_id}` : '-') }}</div>
                  <div v-if="row.api_key_id || row.group_id" class="text-xs text-gray-500 dark:text-gray-400">
                    API #{{ row.api_key_id || '-' }} / G #{{ row.group_id || '-' }}
                  </div>
                </td>
                <td>
                  <div class="max-w-[260px] truncate text-sm" :title="routeLabel(row)">{{ routeLabel(row) }}</div>
                  <div v-if="failoverLabel(row)" class="mt-1 text-xs text-amber-600 dark:text-amber-300">{{ failoverLabel(row) }}</div>
                </td>
                <td>
                  <div>{{ t('admin.openaiRouteTrace.duration') }} {{ formatMs(row.duration_ms) }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.openaiRouteTrace.ttft') }} {{ formatMs(row.first_token_ms ?? row.time_to_first_token_ms) }}</div>
                </td>
                <td>
                  <div>{{ formatInteger(row.total_tokens) }}</div>
                  <div v-if="row.tokens_per_second != null" class="text-xs text-gray-500 dark:text-gray-400">{{ formatNumber(row.tokens_per_second) }}/s</div>
                </td>
                <td>
                  <button type="button" class="btn btn-ghost px-2 py-1 text-sm" :disabled="!row.request_id" @click="openTrace(row)">
                    <Icon name="eye" size="sm" />
                    <span class="ml-1">{{ t('admin.openaiRouteTrace.viewTrace') }}</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="total > 0"
          :page="page"
          :total="total"
          :page-size="pageSize"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>

  <BaseDialog :show="traceOpen" :title="t('admin.openaiRouteTrace.traceTitle')" width="wide" @close="closeTrace">
    <template #default>
      <div v-if="traceLoading" class="py-12 text-center text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
      <div v-else-if="selectedRow" class="space-y-4">
        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">{{ t('admin.openaiRouteTrace.request') }}</div>
          <div class="mt-1 break-all font-mono text-xs text-gray-800 dark:text-gray-100">{{ selectedRow.request_id }}</div>
          <div class="mt-2 grid gap-2 sm:grid-cols-3">
            <div>{{ t('admin.openaiRouteTrace.account') }}: {{ selectedRow.account_name || selectedRow.account_id || '-' }}</div>
            <div>{{ t('admin.openaiRouteTrace.model') }}: {{ selectedRow.requested_model || selectedRow.model || '-' }}</div>
            <div>{{ t('admin.openaiRouteTrace.status') }}: {{ selectedRow.status_code || selectedRow.kind }}</div>
          </div>
        </section>

        <section v-if="diagnosis" class="grid gap-3 md:grid-cols-2">
          <div v-for="section in diagnosisSections" :key="section.key" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <h3 class="mb-2 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ section.title }}</h3>
            <dl class="space-y-2">
              <div v-for="[key, value] in Object.entries(section.data || {})" :key="key">
                <dt class="text-xs text-gray-500 dark:text-gray-400">{{ key }}</dt>
                <dd class="mt-0.5 break-words font-mono text-xs text-gray-800 dark:text-gray-100">{{ formatValue(value) }}</dd>
              </div>
            </dl>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="border-b border-gray-200 px-3 py-2 text-sm font-semibold dark:border-dark-700">{{ t('admin.openaiRouteTrace.timeline') }}</div>
          <div class="max-h-80 overflow-auto">
            <table class="min-w-full">
              <tbody>
                <tr v-for="(event, index) in timeline?.events || []" :key="index" class="border-b border-gray-100 dark:border-dark-700">
                  <td class="whitespace-nowrap px-3 py-2 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(event.at) }}</td>
                  <td class="px-3 py-2 font-mono text-xs">{{ event.phase }} / {{ event.event_type }}</td>
                  <td class="px-3 py-2 text-xs">{{ event.reason || '-' }}</td>
                  <td class="px-3 py-2 font-mono text-xs">{{ formatValue(event.details) }}</td>
                </tr>
                <tr v-if="!(timeline?.events || []).length">
                  <td colspan="4" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.openaiRouteTrace.noTimeline') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { opsAPI, type OpsCodexDiagnosis, type OpsRequestDetail, type OpsRequestDetailsParams, type OpsRequestTimeline } from '@/api/admin/ops'
import { formatDateTime, parseTimeRangeMinutes } from './utils/opsFormatters'

const { t } = useI18n()
const appStore = useAppStore()

const filters = reactive({
  q: '',
  request_id: '',
  account_id: undefined as number | undefined,
  model: '',
  kind: 'all' as OpsRequestDetailsParams['kind'],
  time_range: '1h' as NonNullable<OpsRequestDetailsParams['time_range']>,
})

const rows = ref<OpsRequestDetail[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const error = ref('')
const traceOpen = ref(false)
const traceLoading = ref(false)
const selectedRow = ref<OpsRequestDetail | null>(null)
const timeline = ref<OpsRequestTimeline | null>(null)
const diagnosis = ref<OpsCodexDiagnosis | null>(null)

const kindOptions = computed(() => [
  { value: 'all', label: t('admin.openaiRouteTrace.kindAll') },
  { value: 'success', label: t('admin.openaiRouteTrace.kindSuccess') },
  { value: 'error', label: t('admin.openaiRouteTrace.kindError') },
])

const timeRangeOptions = computed(() => [
  { value: '5m', label: '5m' },
  { value: '30m', label: '30m' },
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
])

const diagnosisSections = computed(() => {
  if (!diagnosis.value) return []
  return [
    { key: 'latency', title: t('admin.openaiRouteTrace.latency'), data: diagnosis.value.latency },
    { key: 'path', title: t('admin.openaiRouteTrace.route'), data: diagnosis.value.path },
    { key: 'routing', title: t('admin.openaiRouteTrace.routing'), data: diagnosis.value.routing },
    { key: 'context', title: t('admin.openaiRouteTrace.context'), data: diagnosis.value.context },
  ].filter((section) => section.data && Object.keys(section.data).length > 0)
})

function buildTimeParams() {
  const minutes = parseTimeRangeMinutes(filters.time_range || '1h')
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return { start_time: start.toISOString(), end_time: end.toISOString() }
}

async function loadRows() {
  loading.value = true
  error.value = ''
  try {
    const params: OpsRequestDetailsParams = {
      ...buildTimeParams(),
      kind: filters.kind || 'all',
      platform: 'openai',
      page: page.value,
      page_size: pageSize.value,
      sort: 'created_at_desc',
    }
    if (filters.q.trim()) params.q = filters.q.trim()
    if (filters.request_id.trim()) params.request_id = filters.request_id.trim()
    if (filters.model.trim()) params.model = filters.model.trim()
    if (filters.account_id && filters.account_id > 0) params.account_id = filters.account_id
    const res = await opsAPI.listRequestDetails(params)
    rows.value = res.items || []
    total.value = res.total || 0
  } catch (e: any) {
    error.value = e?.message || t('admin.openaiRouteTrace.loadFailed')
    appStore.showError(error.value)
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  loadRows()
}

function resetFilters() {
  filters.q = ''
  filters.request_id = ''
  filters.account_id = undefined
  filters.model = ''
  filters.kind = 'all'
  filters.time_range = '1h'
  reload()
}

function handlePageChange(next: number) {
  page.value = next
  loadRows()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  loadRows()
}

async function openTrace(row: OpsRequestDetail) {
  if (!row.request_id) return
  selectedRow.value = row
  traceOpen.value = true
  traceLoading.value = true
  timeline.value = null
  diagnosis.value = null
  try {
    const [nextTimeline, nextDiagnosis] = await Promise.all([
      opsAPI.getRequestTimeline(row.request_id),
      opsAPI.getCodexDiagnosis(row.request_id),
    ])
    timeline.value = nextTimeline
    diagnosis.value = nextDiagnosis
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.openaiRouteTrace.traceLoadFailed'))
  } finally {
    traceLoading.value = false
  }
}

function closeTrace() {
  traceOpen.value = false
}

function formatKind(kind: string) {
  return kind === 'error' ? t('admin.openaiRouteTrace.kindError') : t('admin.openaiRouteTrace.kindSuccess')
}

function kindBadgeClass(kind: string) {
  if (kind === 'error') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
}

function routeLabel(row: OpsRequestDetail) {
  return row.upstream_endpoint || latestUpstreamValue(row, ['request_base_url', 'selected_request_base_url', 'selected_base_url']) || '-'
}

function failoverLabel(row: OpsRequestDetail) {
  const events = row.upstream_errors || []
  const count = events.filter((event) => event.kind === 'base_url_failover' || event.kind === 'header_race_winner').length
  return count > 0 ? t('admin.openaiRouteTrace.failoverCount', { count }) : ''
}

function latestUpstreamValue(row: OpsRequestDetail, keys: string[]) {
  const events = row.upstream_errors || []
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const detail = events[i]?.detail || ''
    if (detail && /^https?:\/\//i.test(detail)) return detail
  }
  for (const key of keys) {
    const value = (row as unknown as Record<string, unknown>)[key]
    if (typeof value === 'string' && value.trim()) return value
  }
  return ''
}

function formatMs(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${Math.round(value)} ms`
}

function formatInteger(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return Math.round(value).toLocaleString()
}

function formatNumber(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return (Math.round(value * 100) / 100).toLocaleString()
}

function formatValue(value: unknown): string {
  if (value == null) return '-'
  if (typeof value === 'string') return value
  if (typeof value === 'number') return formatNumber(value)
  if (typeof value === 'boolean') return value ? t('common.yes') : t('common.no')
  return JSON.stringify(value, null, 2)
}

onMounted(loadRows)
</script>
