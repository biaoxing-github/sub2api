<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.probe.title')"
    width="wide"
    :z-index="60"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div
        v-if="account"
        class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-700"
      >
        <div>
          <div class="font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ account.platform }} / {{ account.type }} · {{ modelId || t('admin.accounts.selectTestModel') }}
          </div>
        </div>
        <span :class="probeStatusBadgeClass(account.status || 'unknown')">
          {{ account.status || '-' }}
        </span>
      </div>

      <div
        v-if="!supportsAccountProbe"
        class="rounded-lg bg-yellow-50 px-3 py-2 text-sm text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-300"
      >
        {{ t('admin.accounts.probe.unsupported') }}
      </div>

      <div class="grid gap-3 sm:grid-cols-3">
        <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.requestCount') }}</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ probeRequestCount }}</div>
        </div>
        <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.estimatedTokens') }}</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ probeEstimatedTokens }}</div>
        </div>
        <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.modeLabel') }}</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t(`admin.accounts.probe.modes.${probeMode}`) }}
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          :class="probeModeButtonClass(probeMode === 'quick')"
          :disabled="probeSubmitting"
          @click="probeMode = 'quick'"
        >
          {{ t('admin.accounts.probe.modes.quick') }}
        </button>
        <button
          type="button"
          :class="probeModeButtonClass(probeMode === 'standard')"
          :disabled="probeSubmitting"
          @click="probeMode = 'standard'"
        >
          {{ t('admin.accounts.probe.modes.standard') }}
        </button>
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <input
            v-model="probeCodexStability"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :disabled="probeSubmitting"
          />
          {{ t('admin.accounts.probe.codexStability') }}
        </label>
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <input
            v-model="probeLongContext"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :disabled="probeSubmitting"
          />
          {{ t('admin.accounts.probe.longContext') }}
        </label>
        <button
          type="button"
          class="ml-auto inline-flex items-center gap-2 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:bg-primary-300"
          :disabled="probeSubmitting || !supportsAccountProbe || !modelId"
          @click="startProbe"
        >
          <Icon name="beaker" size="sm" :class="probeSubmitting ? 'animate-pulse' : ''" :stroke-width="2" />
          {{ probeSubmitting ? t('admin.accounts.probe.running') : t('admin.accounts.probe.run') }}
        </button>
      </div>

      <div
        v-if="errorMessage"
        class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300"
      >
        {{ errorMessage }}
      </div>
      <div
        v-if="infoMessage"
        class="rounded-lg bg-blue-50 px-3 py-2 text-sm text-blue-700 dark:bg-blue-500/10 dark:text-blue-300"
      >
        {{ infoMessage }}
      </div>

      <div v-if="latestProbeRun" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span :class="probeStatusBadgeClass(latestProbeRun.status)">
            {{ formatProbeStatus(latestProbeRun.status) }}
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatTime(latestProbeRun.created_at) }}</span>
        </div>
        <div class="mt-3 grid gap-3 text-sm sm:grid-cols-4">
          <div>
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.avgLatency') }}</div>
            <div class="font-medium text-gray-900 dark:text-white">{{ formatMs(latestProbeRun.avg_latency_ms) }}</div>
          </div>
          <div>
            <div class="text-gray-500 dark:text-gray-400">P95</div>
            <div class="font-medium text-gray-900 dark:text-white">{{ formatMs(latestProbeRun.latency?.p95_ms) }}</div>
          </div>
          <div>
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.successRate') }}</div>
            <div class="font-medium text-gray-900 dark:text-white">{{ formatProbeSuccessRate(latestProbeRun) }}</div>
          </div>
          <div>
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.tokensUsed') }}</div>
            <div class="font-medium text-gray-900 dark:text-white">{{ latestProbeRun.total_tokens ?? '-' }}</div>
          </div>
        </div>
        <p v-if="latestProbeRun.summary || latestProbeRun.error_message" class="mt-2 text-sm text-gray-600 dark:text-gray-300">
          {{ latestProbeRun.summary || latestProbeRun.error_message }}
        </p>
        <div v-if="latestProbeRun.samples?.length" class="mt-3 max-h-56 space-y-2 overflow-y-auto pr-1">
          <div
            v-for="sample in latestProbeRun.samples"
            :key="sample.id || sample.request_index"
            class="grid gap-2 rounded border border-gray-200 bg-white px-3 py-2 text-xs dark:border-dark-600 dark:bg-dark-800 sm:grid-cols-[40px_1fr_90px_90px]"
          >
            <span class="font-medium text-gray-500 dark:text-gray-400">#{{ sample.request_index }}</span>
            <span class="truncate text-gray-700 dark:text-gray-200">
              {{ sample.label || sample.type || '-' }}
              <span v-if="sample.api_key_masked" class="text-gray-400"> · {{ sample.api_key_masked }}</span>
            </span>
            <span :class="sample.status === 'success' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
              {{ formatProbeStatus(sample.status) }}
            </span>
            <span class="text-gray-600 dark:text-gray-300">{{ formatMs(sample.latency_ms) }}</span>
            <span v-if="sample.error" class="sm:col-span-4 text-red-500 dark:text-red-300">{{ sample.error }}</span>
          </div>
        </div>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.history') }}</div>
          <button
            type="button"
            class="text-xs font-medium text-primary-600 hover:text-primary-700 disabled:cursor-not-allowed disabled:opacity-60 dark:text-primary-400"
            :disabled="probeHistoryLoading"
            @click="loadProbeHistory"
          >
            {{ probeHistoryLoading ? t('common.loading') : t('common.refresh') }}
          </button>
        </div>
        <div v-if="probeHistoryLoading" class="py-2 text-center text-xs text-gray-500 dark:text-gray-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="probeHistory.length === 0" class="rounded-lg border border-dashed border-gray-200 py-4 text-center text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
          {{ t('admin.accounts.probe.noHistory') }}
        </div>
        <div v-else class="max-h-44 space-y-2 overflow-y-auto pr-1">
          <button
            v-for="run in probeHistory"
            :key="run.id"
            type="button"
            class="flex w-full items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 text-left text-xs transition-colors hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-800"
            @click="openProbeRun(run.id)"
          >
            <span class="flex items-center gap-2">
              <span :class="probeStatusBadgeClass(run.status)">{{ formatProbeStatus(run.status) }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ formatProbeSuccessRate(run) }} · {{ formatMs(run.avg_latency_ms) }}</span>
            </span>
            <span class="shrink-0 text-gray-400">{{ formatTime(run.created_at) }}</span>
          </button>
        </div>
      </div>
    </div>

    <template #footer>
      <button
        type="button"
        class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
        @click="emit('close')"
      >
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account, AccountProbeMode, AccountProbeRun } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  account: Account | null
  modelId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const probeMode = ref<AccountProbeMode>('quick')
const probeCodexStability = ref(false)
const probeLongContext = ref(false)
const probeSubmitting = ref(false)
const probeHistoryLoading = ref(false)
const probeHistory = ref<AccountProbeRun[]>([])
const latestProbeRun = ref<AccountProbeRun | null>(null)
const errorMessage = ref('')
const infoMessage = ref('')

const supportsAccountProbe = computed(() => props.account?.platform === 'openai' && props.account?.type === 'apikey')

const probeRequestCount = computed(() => {
  const base = probeMode.value === 'quick' ? 1 : 3
  return base + (probeCodexStability.value ? 5 : 0) + (probeLongContext.value ? 1 : 0)
})

const probeEstimatedTokens = computed(() => {
  const baseMin = probeMode.value === 'quick' ? 50 : 200
  const baseMax = probeMode.value === 'quick' ? 150 : 600
  const min = baseMin + (probeCodexStability.value ? 500 : 0) + (probeLongContext.value ? 1000 : 0)
  const max = baseMax + (probeCodexStability.value ? 1200 : 0) + (probeLongContext.value ? 2000 : 0)
  return `${min}-${max}`
})

watch(
  () => props.show,
  async (visible) => {
    if (visible) {
      errorMessage.value = ''
      infoMessage.value = ''
      latestProbeRun.value = null
      await loadProbeHistory()
    }
  }
)

const startProbe = async () => {
  if (!props.account || !props.modelId) return
  probeSubmitting.value = true
  errorMessage.value = ''
  infoMessage.value = ''
  try {
    latestProbeRun.value = await adminAPI.accounts.createProbeRun(props.account.id, {
      mode: probeMode.value,
      model: props.modelId,
      codex_stability: probeCodexStability.value,
      long_context: probeLongContext.value
    })
    infoMessage.value = t('admin.accounts.probe.started')
    await loadProbeHistory()
  } catch (error: any) {
    errorMessage.value = error?.message || t('admin.accounts.probe.runFailed')
    latestProbeRun.value = null
  } finally {
    probeSubmitting.value = false
  }
}

const loadProbeHistory = async () => {
  if (!props.account || !supportsAccountProbe.value) {
    probeHistory.value = []
    return
  }
  probeHistoryLoading.value = true
  try {
    const history = await adminAPI.accounts.listProbeRuns(props.account.id)
    probeHistory.value = Array.isArray(history) ? history : []
  } catch {
    probeHistory.value = []
  } finally {
    probeHistoryLoading.value = false
  }
}

const openProbeRun = async (runId: number) => {
  if (!props.account) return
  errorMessage.value = ''
  infoMessage.value = ''
  try {
    latestProbeRun.value = await adminAPI.accounts.getProbeRun(props.account.id, runId)
  } catch (error: any) {
    errorMessage.value = error?.message || t('admin.accounts.probe.historyFailed')
  }
}

const formatMs = (value?: number | null) => {
  if (value === null || value === undefined) return '-'
  return `${Math.round(value)} ms`
}

const formatTime = (value?: string | null) => {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

const formatProbeSuccessRate = (run: AccountProbeRun) => {
  const success = run.success_count ?? null
  const failure = run.failure_count ?? null
  if (success === null || failure === null) return '-'
  const total = success + failure
  if (total <= 0) return '-'
  return `${Math.round((success / total) * 100)}%`
}

const formatProbeStatus = (status: string) => {
  const key = `admin.accounts.probe.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

const probeStatusBadgeClass = (status: string) => [
  'rounded-full px-2 py-0.5 text-[11px] font-semibold',
  status === 'success' || status === 'active'
    ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300'
    : status === 'failed' || status === 'error'
      ? 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300'
      : status === 'partial'
        ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300'
        : 'bg-gray-100 text-gray-600 dark:bg-gray-600 dark:text-gray-200'
]

const probeModeButtonClass = (active: boolean) => [
  'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
  active
    ? 'bg-primary-500 text-white'
    : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-200 dark:hover:bg-dark-500'
]
</script>
