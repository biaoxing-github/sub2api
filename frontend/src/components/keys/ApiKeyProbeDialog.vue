<template>
  <BaseDialog
    :show="show"
    :title="t('keys.probe.title')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ apiKey?.name || t('keys.apiKey') }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ t('keys.probe.description') }}
          </div>
        </div>
        <span
          v-if="apiKey?.group"
          class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300"
        >
          {{ apiKey.group.name }}
        </span>
      </div>

      <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
        <button
          type="button"
          :class="tabClass(activeTab === 'run')"
          @click="activeTab = 'run'"
        >
          {{ t('keys.probe.currentTab') }}
        </button>
        <button
          type="button"
          :class="tabClass(activeTab === 'history')"
          @click="activeTab = 'history'"
        >
          {{ t('keys.probe.historyTab') }}
        </button>
      </div>

      <div v-if="activeTab === 'run'" class="space-y-4">
        <div class="grid gap-3 md:grid-cols-3">
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('keys.probe.requestCount') }}</div>
            <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ requestCount }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('keys.probe.estimatedTokens') }}</div>
            <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ estimatedTokens }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('keys.probe.estimatedTokensHint') }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('keys.probe.modeLabel') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t(`keys.probe.modes.${mode}`) }}
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex items-start justify-between gap-4">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('keys.probe.standardTitle') }}
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('keys.probe.standardDescription') }}
              </p>
            </div>
            <button
              type="button"
              class="btn btn-secondary shrink-0"
              :disabled="submitting"
              @click="runProbe('quick')"
            >
              <Icon name="bolt" size="sm" class="mr-2" />
              {{ t('keys.probe.quickRun') }}
            </button>
          </div>

          <div class="mt-4 space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-dark-400">
              {{ t('keys.probe.advanced') }}
            </div>
            <label class="flex cursor-pointer items-start gap-3 rounded-lg p-2 transition-colors hover:bg-gray-50 dark:hover:bg-dark-800">
              <input v-model="codexStability" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('keys.probe.codexStability') }}
                </span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">
                  {{ t('keys.probe.codexStabilityHint') }}
                </span>
              </span>
            </label>
            <label class="flex cursor-pointer items-start gap-3 rounded-lg p-2 transition-colors hover:bg-gray-50 dark:hover:bg-dark-800">
              <input v-model="longContext" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('keys.probe.longContext') }}
                </span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">
                  {{ t('keys.probe.longContextHint') }}
                </span>
              </span>
            </label>
          </div>
        </div>

        <div v-if="latestRun" class="rounded-lg bg-gray-50 p-4 dark:bg-dark-800">
          <div class="mb-3 flex items-center justify-between gap-3">
            <span :class="statusBadgeClass(latestRun.status)">
              {{ formatStatus(latestRun.status) }}
            </span>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ formatTime(latestRun.created_at) }}</span>
          </div>
          <div class="grid gap-3 text-sm sm:grid-cols-3">
            <div>
              <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.avgLatency') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatMs(latestRun.avg_latency_ms) }}</div>
            </div>
            <div>
              <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.successRate') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatSuccessRate(latestRun) }}</div>
            </div>
            <div>
              <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.tokensUsed') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ latestRun.total_tokens ?? '-' }}</div>
            </div>
          </div>
          <p v-if="latestRun.summary || latestRun.error_message" class="mt-3 text-sm text-gray-600 dark:text-dark-300">
            {{ latestRun.summary || latestRun.error_message }}
          </p>
        </div>
      </div>

      <div v-else class="space-y-3">
        <div class="flex items-center justify-between">
          <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.probe.historyTab') }}</div>
          <button type="button" class="btn btn-secondary" :disabled="historyLoading" @click="loadHistory">
            <Icon name="refresh" size="sm" class="mr-2" :class="historyLoading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="historyLoading" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="history.length === 0" class="rounded-lg border border-dashed border-gray-300 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
          {{ t('keys.probe.noHistory') }}
        </div>
        <div v-else class="max-h-[420px] space-y-3 overflow-y-auto pr-1">
          <div
            v-for="run in history"
            :key="run.id"
            class="rounded-lg border border-gray-200 p-4 dark:border-dark-700"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="flex items-center gap-2">
                <span :class="statusBadgeClass(run.status)">{{ formatStatus(run.status) }}</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t(`keys.probe.modes.${run.mode === 'quick' ? 'quick' : 'standard'}`) }}
                </span>
              </div>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ formatTime(run.created_at) }}</span>
            </div>
            <div class="mt-3 grid gap-3 text-sm sm:grid-cols-4">
              <div>
                <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.requestCount') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ run.request_count ?? '-' }}</div>
              </div>
              <div>
                <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.avgLatency') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ formatMs(run.avg_latency_ms) }}</div>
              </div>
              <div>
                <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.successRate') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ formatSuccessRate(run) }}</div>
              </div>
              <div>
                <div class="text-gray-500 dark:text-dark-400">{{ t('keys.probe.tokensUsed') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ run.total_tokens ?? '-' }}</div>
              </div>
            </div>
            <p v-if="run.summary || run.error_message" class="mt-3 text-sm text-gray-600 dark:text-dark-300">
              {{ run.summary || run.error_message }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="handleClose">
        {{ t('common.close') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="submitting || !apiKey"
        @click="runProbe('standard')"
      >
        <Icon name="beaker" size="sm" class="mr-2" :class="submitting ? 'animate-pulse' : ''" />
        {{ submitting ? t('keys.probe.running') : t('keys.probe.standardRun') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { keysAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey, ApiKeyProbeMode, ApiKeyProbeRun } from '@/types'

const props = defineProps<{
  show: boolean
  apiKey: ApiKey | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const activeTab = ref<'run' | 'history'>('run')
const mode = ref<ApiKeyProbeMode>('standard')
const codexStability = ref(false)
const longContext = ref(false)
const submitting = ref(false)
const historyLoading = ref(false)
const history = ref<ApiKeyProbeRun[]>([])
const latestRun = ref<ApiKeyProbeRun | null>(null)

const requestCount = computed(() => {
  const base = mode.value === 'quick' ? 1 : 3
  return base + (codexStability.value ? 5 : 0) + (longContext.value ? 1 : 0)
})

const estimatedTokens = computed(() => {
  const baseMin = mode.value === 'quick' ? 50 : 200
  const baseMax = mode.value === 'quick' ? 150 : 600
  const min = baseMin + (codexStability.value ? 500 : 0) + (longContext.value ? 1000 : 0)
  const max = baseMax + (codexStability.value ? 1200 : 0) + (longContext.value ? 2000 : 0)
  return `${min}-${max}`
})

watch(
  () => props.show,
  (show) => {
    if (!show) return
    activeTab.value = 'run'
    mode.value = 'standard'
    latestRun.value = null
    loadHistory()
  }
)

const tabClass = (active: boolean) => [
  'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
  active
    ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
    : 'text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'
]

const runProbe = async (selectedMode: ApiKeyProbeMode) => {
  if (!props.apiKey) return
  mode.value = selectedMode
  submitting.value = true
  try {
    latestRun.value = await keysAPI.createProbeRun(props.apiKey.id, {
      mode: selectedMode,
      codex_stability: codexStability.value,
      long_context: longContext.value
    })
    appStore.showSuccess(t('keys.probe.runSuccess'))
    await loadHistory()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || error.message || t('keys.probe.runFailed')
    appStore.showError(errorMsg)
  } finally {
    submitting.value = false
  }
}

const loadHistory = async () => {
  if (!props.apiKey) return
  historyLoading.value = true
  try {
    history.value = await keysAPI.listProbeRuns(props.apiKey.id)
  } catch (error) {
    appStore.showError(t('keys.probe.historyFailed'))
  } finally {
    historyLoading.value = false
  }
}

const handleClose = () => {
  emit('close')
}

const formatMs = (value?: number | null) => {
  if (value === null || value === undefined) return '-'
  return `${Math.round(value)} ms`
}

const formatTime = (value?: string | null) => {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

const formatSuccessRate = (run: ApiKeyProbeRun) => {
  const success = run.success_count ?? null
  const failure = run.failure_count ?? null
  if (success === null || failure === null) return '-'
  const total = success + failure
  if (total <= 0) return '-'
  return `${Math.round((success / total) * 100)}%`
}

const formatStatus = (status: string) => {
  const key = `keys.probe.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

const statusBadgeClass = (status: string) => [
  'badge',
  status === 'success'
    ? 'badge-success'
    : status === 'failed'
      ? 'badge-danger'
      : status === 'partial'
        ? 'badge-warning'
        : 'badge-gray'
]
</script>
