<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountModelProbes.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.description') }}</p>
          </div>
          <button type="button" data-test="open-batch-model-probe-dialog" class="btn btn-secondary px-3" @click="openBatchDialog">
            <Icon name="sparkles" size="sm" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.batchModelProbe') }}</span>
          </button>
        </div>
        <form data-test="run-model-probe" class="grid gap-3 md:grid-cols-[160px_minmax(220px,1fr)_180px_auto]" @submit.prevent="submitProbe">
          <input
            v-model.number="form.account_id"
            data-test="model-probe-account-id"
            type="number"
            min="1"
            class="input"
            :placeholder="t('admin.accountModelProbes.accountId')"
          />
          <input
            v-model="form.model"
            data-test="model-probe-model"
            type="text"
            class="input"
            :placeholder="t('admin.accountModelProbes.modelPlaceholder')"
          />
          <select
            v-model="form.request_mode"
            data-test="model-probe-request-mode"
            class="input"
          >
            <option value="non_stream">{{ t('admin.accountModelProbes.requestModes.non_stream') }}</option>
            <option value="stream">{{ t('admin.accountModelProbes.requestModes.stream') }}</option>
          </select>
          <button type="submit" class="btn btn-primary justify-center" :disabled="submitting">
            <Icon name="beaker" size="sm" :class="submitting ? 'animate-pulse' : ''" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.run') }}</span>
          </button>
        </form>
        <div v-if="message" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200">
          {{ message }}
        </div>
        <div v-if="error" class="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
          {{ error }}
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountModelProbes.recentRuns') }}</div>
          <button type="button" class="btn btn-ghost px-2 py-1 text-sm" :disabled="loading" @click="loadRuns">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[760px]">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.account') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.model') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.requestMode') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.status') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.time') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && runs.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="runs.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.empty') }}</td>
              </tr>
              <tr v-for="run in runs" :key="run.id" class="border-t border-gray-100 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ run.account_name || `#${run.account_id}` }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">#{{ run.account_id }}</div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ run.model || '-' }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatRequestMode(run.request_mode) }}</td>
                <td class="px-4 py-3">
                  <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatNumber(run.score) }}</div>
                  <div v-if="run.grade_label" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ run.grade_label }}</div>
                </td>
                <td class="px-4 py-3">
                  <span :class="statusClass(run.status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                    {{ formatStatus(run.status) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(run.created_at) }}</td>
                <td class="px-4 py-3">
                  <button type="button" class="btn btn-ghost px-2 py-1 text-sm" :data-test="`model-probe-detail-${run.id}`" @click="loadDetail(run.id)">
                    <Icon name="eye" size="sm" />
                    <span class="ml-1">{{ t('common.view') }}</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <BaseDialog
        :show="detailDialogOpen"
        :title="t('admin.accountModelProbes.detailTitle')"
        width="extra-wide"
        @close="clearDetail"
      >
        <div v-if="detailLoading" class="text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
        <div v-else-if="detailError" class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">{{ detailError }}</div>
        <div v-else-if="detailRun" class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-3">
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.status') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatStatus(detailRun.status) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatNumber(detailRun.score) }}</div>
              <div v-if="detailRun.grade_label" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ detailRun.grade_label }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.time') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatDateTime(detailRun.created_at) }}</div>
            </div>
          </div>

          <div v-for="sample in detailSamples" :key="sample.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ sample.label || sample.type || `#${sample.request_index}` }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ sample.model || detailRun.model || '-' }}</div>
              </div>
              <span :class="statusClass(sample.status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                {{ formatStatus(sample.status) }}
              </span>
            </div>
            <pre v-if="sample.output_text" class="mt-3 max-h-40 overflow-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100">{{ sample.output_text }}</pre>
            <div v-if="sample.validation_evidence?.length" class="mt-3 overflow-x-auto">
              <table class="w-full min-w-[640px]">
                <thead>
                  <tr>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.evidence') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.expected') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.observed') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="evidence in sample.validation_evidence" :key="evidence.key" class="border-t border-gray-100 dark:border-dark-700">
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">
                      <span :class="evidence.passed ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'" class="font-medium">{{ evidence.label }}</span>
                    </td>
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">{{ evidence.expected || '-' }}</td>
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">{{ evidence.observed || '-' }}</td>
                    <td class="px-2 py-2 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatEvidenceScore(evidence) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <template #footer>
          <button type="button" class="btn btn-secondary" @click="clearDetail">{{ t('common.close') }}</button>
        </template>
      </BaseDialog>

      <BaseDialog
        :show="batchDialogOpen"
        :title="t('admin.accountModelProbes.batchModelDialogTitle')"
        width="extra-wide"
        @close="closeBatchDialog"
      >
        <div class="space-y-4">
          <div class="grid gap-3 md:grid-cols-[minmax(220px,1fr)_180px]">
            <input
              v-model="batchForm.model"
              data-test="batch-model-probe-model"
              type="text"
              class="input"
              :placeholder="t('admin.accountModelProbes.modelPlaceholder')"
            />
            <select
              v-model="batchForm.request_mode"
              data-test="batch-model-probe-request-mode"
              class="input"
            >
              <option value="non_stream">{{ t('admin.accountModelProbes.requestModes.non_stream') }}</option>
              <option value="stream">{{ t('admin.accountModelProbes.requestModes.stream') }}</option>
            </select>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <input
              v-model="batchAccountSearch"
              data-test="batch-model-account-search"
              type="search"
              class="input max-w-xs"
              :placeholder="t('admin.accountModelProbes.batchAccountSearchPlaceholder')"
              @input="onBatchAccountSearch"
            />
            <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
              <button type="button" class="btn btn-ghost px-2 py-1 text-sm" @click="selectAllBatchAccounts">
                {{ t('admin.accountModelProbes.selectAll') }}
              </button>
              <button type="button" class="btn btn-ghost px-2 py-1 text-sm" @click="clearBatchAccountSelection">
                {{ t('admin.accountModelProbes.clearSelection') }}
              </button>
              <span>{{ t('admin.accountModelProbes.selectedAccounts', { count: selectedAccountIds.length }) }}</span>
            </div>
          </div>

          <div v-if="batchError" class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
            {{ batchError }}
          </div>
          <div v-if="batchMessage" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200">
            {{ batchMessage }}
          </div>

          <div class="max-h-[420px] overflow-auto rounded-lg border border-gray-200 dark:border-dark-700">
            <div v-if="batchAccountsLoading" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
            <div v-else-if="batchAccounts.length === 0" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.batchAccountsEmpty') }}</div>
            <template v-else>
              <label
                v-for="account in batchAccounts"
                :key="account.id"
                class="flex items-start gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-dark-700"
              >
                <input
                  v-model="selectedAccountIds"
                  data-test="batch-account-select"
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600"
                  :value="account.id"
                />
                <span class="min-w-0 flex-1">
                  <span class="block font-medium text-gray-900 dark:text-gray-100">{{ account.name || `#${account.id}` }}</span>
                  <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">#{{ account.id }} · {{ account.platform }} · {{ account.type }}</span>
                </span>
              </label>
            </template>
          </div>
        </div>
        <template #footer>
          <button type="button" class="btn btn-secondary" @click="closeBatchDialog">{{ t('common.cancel') }}</button>
          <button
            type="button"
            data-test="batch-model-probe-submit"
            class="btn btn-primary"
            :disabled="selectedAccountIds.length === 0 || batchSubmitting"
            @click="submitBatchModelProbe"
          >
            <Icon name="sparkles" size="sm" :class="batchSubmitting ? 'animate-pulse' : ''" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.batchModelProbe') }}</span>
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { batchAccountModelProbeRuns, createAccountModelProbeRun, getAccountProbeRun, list as listAccounts, listAccountProbeRuns } from '@/api/admin/accounts'
import type { Account, AccountProbeRequestMode, AccountProbeRun, AccountProbeSample, AccountProbeValidationEvidence } from '@/types'

const { t } = useI18n()

const runs = ref<AccountProbeRun[]>([])
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const message = ref('')
const detailRun = ref<AccountProbeRun | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailDialogOpen = ref(false)
const batchDialogOpen = ref(false)
const batchSubmitting = ref(false)
const batchAccountsLoading = ref(false)
const batchError = ref('')
const batchMessage = ref('')
const batchAccounts = ref<Account[]>([])
const selectedAccountIds = ref<number[]>([])
const batchAccountSearch = ref('')
const defaultOpenAIAccountTestModelID = 'gpt-5.5'

const form = reactive({
  account_id: undefined as number | undefined,
  model: defaultOpenAIAccountTestModelID,
  request_mode: 'non_stream' as AccountProbeRequestMode,
})

const batchForm = reactive({
  model: defaultOpenAIAccountTestModelID,
  request_mode: 'non_stream' as AccountProbeRequestMode,
})

const detailSamples = computed<AccountProbeSample[]>(() => detailRun.value?.samples || [])

let listAbortController: AbortController | null = null
let submitAbortController: AbortController | null = null
let detailAbortController: AbortController | null = null
let batchAccountsAbortController: AbortController | null = null
let batchSubmitAbortController: AbortController | null = null
let batchSearchTimer: ReturnType<typeof setTimeout> | null = null

async function loadRuns() {
  listAbortController?.abort()
  const controller = new AbortController()
  listAbortController = controller
  loading.value = true
  error.value = ''
  try {
    const response = await listAccountProbeRuns(1, 20, {
      mode: 'model_validation',
      sort_by: 'created_at',
      sort_order: 'desc',
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    runs.value = response.items || []
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoad')
    runs.value = []
  } finally {
    if (listAbortController === controller) {
      loading.value = false
      listAbortController = null
    }
  }
}

async function submitProbe() {
  if (submitting.value) return
  if (!form.account_id || form.account_id <= 0) {
    error.value = t('admin.accountModelProbes.accountRequired')
    return
  }
  submitAbortController?.abort()
  const controller = new AbortController()
  submitAbortController = controller
  submitting.value = true
  error.value = ''
  message.value = ''
  try {
    await createAccountModelProbeRun({
      account_id: form.account_id,
      model: form.model.trim() || undefined,
      request_mode: form.request_mode,
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    message.value = t('admin.accountModelProbes.started')
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToSubmit')
  } finally {
    if (submitAbortController === controller) {
      submitting.value = false
      submitAbortController = null
    }
  }
}

async function loadBatchAccounts() {
  batchAccountsAbortController?.abort()
  const controller = new AbortController()
  batchAccountsAbortController = controller
  batchAccountsLoading.value = true
  batchError.value = ''
  try {
    const response = await listAccounts(1, 100, {
      platform: 'openai',
      type: 'apikey',
      search: batchAccountSearch.value.trim() || undefined,
      sort_by: 'name',
      sort_order: 'asc',
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    batchAccounts.value = response.items || []
    selectedAccountIds.value = batchAccounts.value.map(account => account.id)
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    batchError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoadAccounts')
    batchAccounts.value = []
    selectedAccountIds.value = []
  } finally {
    if (batchAccountsAbortController === controller) {
      batchAccountsLoading.value = false
      batchAccountsAbortController = null
    }
  }
}

async function submitBatchModelProbe() {
  if (selectedAccountIds.value.length === 0 || batchSubmitting.value) return
  batchSubmitAbortController?.abort()
  const controller = new AbortController()
  batchSubmitAbortController = controller
  batchSubmitting.value = true
  batchError.value = ''
  batchMessage.value = ''
  try {
    const response = await batchAccountModelProbeRuns({
      account_ids: selectedAccountIds.value,
      model: batchForm.model.trim() || undefined,
      request_mode: batchForm.request_mode,
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    batchMessage.value = t('admin.accountModelProbes.batchModelAccepted', { count: response.accepted_count })
    batchDialogOpen.value = false
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    batchError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.batchModelFailed')
  } finally {
    if (batchSubmitAbortController === controller) {
      batchSubmitting.value = false
      batchSubmitAbortController = null
    }
  }
}

function openBatchDialog() {
  batchDialogOpen.value = true
  batchError.value = ''
  batchMessage.value = ''
  if (batchAccounts.value.length === 0) {
    loadBatchAccounts()
  } else {
    selectedAccountIds.value = batchAccounts.value.map(account => account.id)
  }
}

function closeBatchDialog() {
  batchDialogOpen.value = false
  batchAccountsAbortController?.abort()
  batchSubmitAbortController?.abort()
}

function selectAllBatchAccounts() {
  selectedAccountIds.value = batchAccounts.value.map(account => account.id)
}

function clearBatchAccountSelection() {
  selectedAccountIds.value = []
}

function onBatchAccountSearch() {
  if (batchSearchTimer) {
    clearTimeout(batchSearchTimer)
  }
  batchSearchTimer = setTimeout(() => {
    loadBatchAccounts()
  }, 250)
}

async function loadDetail(runId: number) {
  detailAbortController?.abort()
  const controller = new AbortController()
  detailAbortController = controller
  detailDialogOpen.value = true
  detailLoading.value = true
  detailError.value = ''
  try {
    detailRun.value = await getAccountProbeRun(runId, {
      signal: controller.signal,
    })
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    detailError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoadDetail')
  } finally {
    if (detailAbortController === controller) {
      detailLoading.value = false
      detailAbortController = null
    }
  }
}

function clearDetail() {
  detailAbortController?.abort()
  detailDialogOpen.value = false
  detailRun.value = null
  detailLoading.value = false
  detailError.value = ''
}

function formatNumber(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(value % 1 === 0 ? 0 : 1) : '-'
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatRequestMode(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountModelProbes.requestModes.${value}`)
}

function formatStatus(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountModelProbes.statuses.${value}`)
}

function formatEvidenceScore(evidence: AccountProbeValidationEvidence): string {
  return `${formatNumber(evidence.score)} / ${formatNumber(evidence.max_score)}`
}

function statusClass(status: string | undefined): string {
  if (status === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'partial' || status === 'running' || status === 'pending') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (status === 'failed') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

onMounted(() => {
  loadRuns()
})

onUnmounted(() => {
  listAbortController?.abort()
  submitAbortController?.abort()
  detailAbortController?.abort()
  batchAccountsAbortController?.abort()
  batchSubmitAbortController?.abort()
  if (batchSearchTimer) {
    clearTimeout(batchSearchTimer)
  }
})
</script>
