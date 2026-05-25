<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.testAccountConnection')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <!-- Account Info Card -->
      <div
        v-if="account"
        class="flex items-center justify-between rounded-xl border border-gray-200 bg-gradient-to-r from-gray-50 to-gray-100 p-3 dark:border-dark-500 dark:from-dark-700 dark:to-dark-600"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-primary-500 to-primary-600"
          >
            <Icon name="play" size="md" class="text-white" :stroke-width="2" />
          </div>
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              <span
                class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] font-medium uppercase dark:bg-dark-500"
              >
                {{ account.type }}
              </span>
              <span>{{ t('admin.accounts.account') }}</span>
            </div>
          </div>
        </div>
        <span
          :class="[
            'rounded-full px-2.5 py-1 text-xs font-semibold',
            account.status === 'active'
              ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
              : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
          ]"
        >
          {{ account.status }}
        </span>
      </div>

      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.selectTestModel') }}
        </label>
        <Select
          v-model="selectedModelId"
          :options="availableModels"
          :disabled="loadingModels || status === 'connecting'"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? t('common.loading') + '...' : t('admin.accounts.selectTestModel')"
        />
      </div>

      <div v-if="supportsImageTest" class="space-y-1.5">
        <TextArea
          v-model="testPrompt"
          :label="t('admin.accounts.imagePromptLabel')"
          :placeholder="t('admin.accounts.imagePromptPlaceholder')"
          :hint="t('admin.accounts.imageTestHint')"
          :disabled="status === 'connecting'"
          rows="3"
        />
      </div>

      <!-- Terminal Output -->
      <div class="group relative">
        <div
          ref="terminalRef"
          class="max-h-[240px] min-h-[120px] overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
        >
          <!-- Status Line -->
          <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
            <Icon name="play" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.readyToTest') }}</span>
          </div>
          <div v-else-if="status === 'connecting'" class="flex items-center gap-2 text-yellow-400">
            <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <span>{{ t('admin.accounts.connectingToApi') }}</span>
          </div>

          <!-- Output Lines -->
          <div v-for="(line, index) in outputLines" :key="index" :class="line.class">
            {{ line.text }}
          </div>

          <!-- Streaming Content -->
          <div v-if="streamingContent" class="text-green-400">
            {{ streamingContent }}<span class="animate-pulse">_</span>
          </div>

          <!-- Result Status -->
          <div
            v-if="status === 'success'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
          >
            <Icon name="check" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.testCompleted') }}</span>
          </div>
          <div
            v-else-if="status === 'error'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-red-400"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
            <span>{{ errorMessage }}</span>
          </div>
        </div>

        <!-- Copy Button -->
        <button
          v-if="outputLines.length > 0"
          @click="copyOutput"
          class="absolute right-2 top-2 rounded-lg bg-gray-800/80 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white group-hover:opacity-100"
          :title="t('admin.accounts.copyOutput')"
        >
          <Icon name="link" size="sm" :stroke-width="2" />
        </button>
      </div>

      <div v-if="generatedImages.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">
          {{ t('admin.accounts.imagePreview') }}
        </div>
        <div class="flex flex-wrap justify-center gap-3">
          <div
            v-for="(image, index) in generatedImages"
            :key="`${image.url}-${index}`"
            class="group/img relative cursor-pointer overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm transition hover:border-primary-300 hover:shadow-md dark:border-dark-500 dark:bg-dark-700"
            @click="previewImageUrl = image.url"
          >
            <img :src="image.url" :alt="`test-image-${index + 1}`" class="max-h-[360px] w-full object-contain" />
            <div class="absolute inset-0 flex items-center justify-center bg-black/0 transition-colors group-hover/img:bg-black/20">
              <Icon name="eye" size="lg" class="text-white opacity-0 drop-shadow-lg transition-opacity group-hover/img:opacity-100" :stroke-width="2" />
            </div>
            <div class="border-t border-gray-100 px-3 py-1.5 text-xs text-gray-500 dark:border-dark-500 dark:text-gray-300">
              {{ image.mimeType || 'image/*' }}
            </div>
          </div>
        </div>
      </div>

      <!-- Image Lightbox -->
      <Teleport to="body">
        <Transition name="fade">
          <div
            v-if="previewImageUrl"
            class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4"
            @click.self="previewImageUrl = ''"
          >
            <button
              class="absolute right-4 top-4 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
              @click="previewImageUrl = ''"
            >
              <Icon name="x" size="lg" :stroke-width="2" />
            </button>
            <img
              :src="previewImageUrl"
              alt="preview"
              class="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
            />
          </div>
        </Transition>
      </Teleport>

      <div class="rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
              <Icon name="chart" size="sm" :stroke-width="2" />
              <span>{{ t('admin.accounts.probe.title') }}</span>
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.probe.description') }}
            </div>
          </div>
          <button
            type="button"
            class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-200 dark:hover:bg-dark-500"
            @click="toggleProbePanel"
          >
            {{ probeExpanded ? t('admin.accounts.probe.hide') : t('admin.accounts.probe.show') }}
          </button>
        </div>

        <div v-if="probeExpanded" class="mt-3 space-y-3 border-t border-gray-100 pt-3 dark:border-dark-600">
          <div
            v-if="!supportsAccountProbe"
            class="rounded-lg bg-yellow-50 px-3 py-2 text-xs text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-300"
          >
            {{ t('admin.accounts.probe.unsupported') }}
          </div>

          <div class="grid gap-3 sm:grid-cols-3">
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.requestCount') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ probeRequestCount }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.probe.estimatedTokens') }}</div>
              <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ probeEstimatedTokens }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
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
            <label class="ml-0 flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300 sm:ml-2">
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
              :disabled="probeSubmitting || !supportsAccountProbe || !selectedModelId"
              @click="startProbe"
            >
              <Icon name="beaker" size="sm" :class="probeSubmitting ? 'animate-pulse' : ''" :stroke-width="2" />
              {{ probeSubmitting ? t('admin.accounts.probe.running') : t('admin.accounts.probe.run') }}
            </button>
          </div>

          <div v-if="latestProbeRun" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
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
            <div v-if="latestProbeRun.samples?.length" class="mt-3 max-h-40 space-y-2 overflow-y-auto pr-1">
              <div
                v-for="sample in latestProbeRun.samples"
                :key="sample.id || sample.request_index"
                class="grid gap-2 rounded border border-gray-200 bg-white px-3 py-2 text-xs dark:border-dark-600 dark:bg-dark-700 sm:grid-cols-[40px_1fr_90px_90px]"
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
                class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
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
            <div v-else class="max-h-32 space-y-2 overflow-y-auto pr-1">
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
      </div>

      <!-- Test Info -->
      <div class="flex items-center justify-between px-1 text-xs text-gray-500 dark:text-gray-400">
        <div class="flex items-center gap-3">
          <span class="flex items-center gap-1">
            <Icon name="grid" size="sm" :stroke-width="2" />
            {{ t('admin.accounts.testModel') }}
          </span>
        </div>
        <span class="flex items-center gap-1">
          <Icon name="chat" size="sm" :stroke-width="2" />
          {{
            supportsImageTest
              ? t('admin.accounts.imageTestMode')
              : t('admin.accounts.testPrompt')
          }}
        </span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          @click="handleClose"
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
        >
          {{ t('common.close') }}
        </button>
        <button
          @click="startTest"
          :disabled="status === 'connecting' || !selectedModelId"
          :class="[
            'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all',
            status === 'connecting' || !selectedModelId
              ? 'cursor-not-allowed bg-primary-400 text-white'
              : status === 'success'
                ? 'bg-green-500 text-white hover:bg-green-600'
                : status === 'error'
                  ? 'bg-orange-500 text-white hover:bg-orange-600'
                  : 'bg-primary-500 text-white hover:bg-primary-600'
          ]"
        >
          <Icon
            v-if="status === 'connecting'"
            name="refresh"
            size="sm"
            class="animate-spin"
            :stroke-width="2"
          />
          <Icon v-else-if="status === 'idle'" name="play" size="sm" :stroke-width="2" />
          <Icon v-else name="refresh" size="sm" :stroke-width="2" />
          <span>
            {{
              status === 'connecting'
                ? t('admin.accounts.testing')
                : status === 'idle'
                  ? t('admin.accounts.startTest')
                  : t('admin.accounts.retry')
            }}
          </span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { Icon } from '@/components/icons'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { Account, AccountProbeMode, AccountProbeRun, ClaudeModel } from '@/types'

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

interface OutputLine {
  text: string
  class: string
}

interface PreviewImage {
  url: string
  mimeType?: string
}

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const outputLines = ref<OutputLine[]>([])
const streamingContent = ref('')
const errorMessage = ref('')
const availableModels = ref<ClaudeModel[]>([])
const selectedModelId = ref('')
const testPrompt = ref('')
const loadingModels = ref(false)
let abortController: AbortController | null = null
const generatedImages = ref<PreviewImage[]>([])
const previewImageUrl = ref('')
const probeExpanded = ref(false)
const probeMode = ref<AccountProbeMode>('quick')
const probeCodexStability = ref(false)
const probeLongContext = ref(false)
const probeSubmitting = ref(false)
const probeHistoryLoading = ref(false)
const probeHistory = ref<AccountProbeRun[]>([])
const latestProbeRun = ref<AccountProbeRun | null>(null)
const prioritizedGeminiModels = ['gemini-3.1-flash-image', 'gemini-2.5-flash-image', 'gemini-3.5-flash', 'gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-3-flash-preview', 'gemini-3-pro-preview', 'gemini-2.0-flash']
const supportsGeminiImageTest = computed(() => {
  const modelID = selectedModelId.value.toLowerCase()
  if (!modelID.startsWith('gemini-') || !modelID.includes('-image')) return false

  return props.account?.platform === 'gemini' || (props.account?.platform === 'antigravity' && props.account?.type === 'apikey')
})

const supportsOpenAIImageTest = computed(() => {
  const modelID = selectedModelId.value.toLowerCase()
  if (!modelID.startsWith('gpt-image-')) return false
  return props.account?.platform === 'openai'
})

const supportsImageTest = computed(() => supportsGeminiImageTest.value || supportsOpenAIImageTest.value)
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

const sortTestModels = (models: ClaudeModel[]) => {
  const priorityMap = new Map(prioritizedGeminiModels.map((id, index) => [id, index]))

  return [...models].sort((a, b) => {
    const aPriority = priorityMap.get(a.id) ?? Number.MAX_SAFE_INTEGER
    const bPriority = priorityMap.get(b.id) ?? Number.MAX_SAFE_INTEGER
    if (aPriority !== bPriority) return aPriority - bPriority
    return 0
  })
}

// Load available models when modal opens
watch(
  () => props.show,
  async (newVal) => {
    if (newVal && props.account) {
      testPrompt.value = ''
      resetState()
      probeExpanded.value = false
      latestProbeRun.value = null
      probeHistory.value = []
      await loadAvailableModels()
    } else {
      abortStream()
    }
  }
)

watch(selectedModelId, () => {
  if (supportsImageTest.value && !testPrompt.value.trim()) {
    testPrompt.value = t('admin.accounts.imagePromptDefault')
  }
})

const loadAvailableModels = async () => {
  if (!props.account) return

  loadingModels.value = true
  selectedModelId.value = '' // Reset selection before loading
  try {
    const models = await adminAPI.accounts.getAvailableModels(props.account.id)
    availableModels.value = props.account.platform === 'gemini' || props.account.platform === 'antigravity'
      ? sortTestModels(models)
      : models
    // Default selection by platform
    if (availableModels.value.length > 0) {
      if (props.account.platform === 'gemini') {
        selectedModelId.value = availableModels.value[0].id
      } else if (props.account.platform === 'openai') {
        const gpt54Model = availableModels.value.find((m) => m.id === 'gpt-5.4')
        selectedModelId.value = gpt54Model?.id || availableModels.value[0].id
      } else {
        // Try to select Sonnet as default, otherwise use first model
        const sonnetModel = availableModels.value.find((m) => m.id.includes('sonnet'))
        selectedModelId.value = sonnetModel?.id || availableModels.value[0].id
      }
    }
  } catch (error) {
    console.error('Failed to load available models:', error)
    // Fallback to empty list
    availableModels.value = []
    selectedModelId.value = ''
  } finally {
    loadingModels.value = false
  }
}

const resetState = () => {
  status.value = 'idle'
  outputLines.value = []
  streamingContent.value = ''
  errorMessage.value = ''
  generatedImages.value = []
  previewImageUrl.value = ''
}

const handleClose = () => {
  abortStream()
  emit('close')
}

const toggleProbePanel = async () => {
  probeExpanded.value = !probeExpanded.value
  if (probeExpanded.value && props.account) {
    await loadProbeHistory()
  }
}

const abortStream = () => {
  if (abortController) {
    abortController.abort()
    abortController = null
  }
}

const addLine = (text: string, className: string = 'text-gray-300') => {
  outputLines.value.push({ text, class: className })
  scrollToBottom()
}

const scrollToBottom = async () => {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

const startTest = async () => {
  if (!props.account || !selectedModelId.value) return

  resetState()
  status.value = 'connecting'
  addLine(t('admin.accounts.startingTestForAccount', { name: props.account.name }), 'text-blue-400')
  addLine(t('admin.accounts.testAccountTypeLabel', { type: props.account.type }), 'text-gray-400')
  addLine('', 'text-gray-300')

  abortStream()

  abortController = new AbortController()

  try {
    // Create EventSource for SSE
    const url = `/api/v1/admin/accounts/${props.account.id}/test`

    // Use fetch with streaming for SSE since EventSource doesn't support POST
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
              model_id: selectedModelId.value,
              prompt: supportsImageTest.value ? testPrompt.value.trim() : ''
            }),
      signal: abortController.signal
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error('No response body')
    }

    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const jsonStr = line.slice(6).trim()
          if (jsonStr) {
            try {
              const event = JSON.parse(jsonStr)
              handleEvent(event)
            } catch (e) {
              console.error('Failed to parse SSE event:', e)
            }
          }
        }
      }
    }
  } catch (error: unknown) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      status.value = 'idle'
      return
    }
    status.value = 'error'
    const msg = error instanceof Error ? error.message : 'Unknown error'
    errorMessage.value = msg
    addLine(`Error: ${msg}`, 'text-red-400')
  }
}

const handleEvent = (event: {
  type: string
  text?: string
  model?: string
  success?: boolean
  error?: string
  image_url?: string
  mime_type?: string
}) => {
  switch (event.type) {
    case 'test_start':
      addLine(t('admin.accounts.connectedToApi'), 'text-green-400')
      if (event.model) {
        addLine(t('admin.accounts.usingModel', { model: event.model }), 'text-cyan-400')
      }
      addLine(
        supportsImageTest.value
            ? t('admin.accounts.sendingImageRequest')
            : t('admin.accounts.sendingTestMessage'),
        'text-gray-400'
      )
      addLine('', 'text-gray-300')
      addLine(t('admin.accounts.response'), 'text-yellow-400')
      break

    case 'content':
      if (event.text) {
        streamingContent.value += event.text
        scrollToBottom()
      }
      break

    case 'image':
      if (event.image_url) {
        generatedImages.value.push({
          url: event.image_url,
          mimeType: event.mime_type
        })
        addLine(t('admin.accounts.imageReceived', { count: generatedImages.value.length }), 'text-purple-300')
      }
      break

    case 'test_complete':
      // Move streaming content to output lines
      if (streamingContent.value) {
        addLine(streamingContent.value, 'text-green-300')
        streamingContent.value = ''
      }
      if (event.success) {
        status.value = 'success'
      } else {
        status.value = 'error'
        errorMessage.value = event.error || 'Test failed'
      }
      break

    case 'error':
      status.value = 'error'
      errorMessage.value = event.error || 'Unknown error'
      if (streamingContent.value) {
        addLine(streamingContent.value, 'text-green-300')
        streamingContent.value = ''
      }
      break
  }
}

const copyOutput = () => {
  const text = outputLines.value.map((l) => l.text).join('\n')
  copyToClipboard(text, t('admin.accounts.outputCopied'))
}

const startProbe = async () => {
  if (!props.account || !selectedModelId.value) return
  probeSubmitting.value = true
  try {
    latestProbeRun.value = await adminAPI.accounts.createProbeRun(props.account.id, {
      mode: probeMode.value,
      model: selectedModelId.value,
      codex_stability: probeCodexStability.value,
      long_context: probeLongContext.value
    })
    await loadProbeHistory()
  } catch (error: any) {
    errorMessage.value = error?.message || t('admin.accounts.probe.runFailed')
    latestProbeRun.value = null
  } finally {
    probeSubmitting.value = false
  }
}

const loadProbeHistory = async () => {
  if (!props.account || !supportsAccountProbe.value) return
  probeHistoryLoading.value = true
  try {
    probeHistory.value = await adminAPI.accounts.listProbeRuns(props.account.id)
  } catch {
    probeHistory.value = []
  } finally {
    probeHistoryLoading.value = false
  }
}

const openProbeRun = async (runId: number) => {
  if (!props.account) return
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
  status === 'success'
    ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300'
    : status === 'failed'
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

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
