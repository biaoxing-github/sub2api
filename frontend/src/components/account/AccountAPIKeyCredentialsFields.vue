<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AccountPlatform } from '@/types'

// 后端返回的多 API Key 摘要项，组件只负责展示和转发操作事件。
export interface AccountAPIKeyItem {
  fingerprint?: string
  masked: string
  status?: string
  disabled?: boolean
  reason?: string
  last_error?: string
  disabled_at?: string
  disabled_until?: string
  disabled_count?: number
}

const props = withDefaults(defineProps<{
  platform: AccountPlatform
  baseUrl: string
  requestBaseUrlsText: string
  balanceBaseUrl: string
  apiKey: string
  apiKeysText: string
  claudeCliVersion?: string
  openaiCodexCliUserAgent?: string
  baseUrlHint: string
  apiKeyHint?: string
  mode?: 'create' | 'edit'
  existingApiKeyItems?: AccountAPIKeyItem[]
  existingApiKeySummary?: string
  apiKeysEditMode?: 'append' | 'replace'
  apiKeysEditModeOptions?: SelectOption[]
  apiKeysEditModeHint?: string
  deletingApiKeyFingerprint?: string | null
  restoringApiKeyFingerprint?: string | null
  section?: 'all' | 'core' | 'advanced'
}>(), {
  apiKeyHint: '',
  mode: 'create',
  existingApiKeyItems: () => [],
  existingApiKeySummary: '',
  apiKeysEditMode: 'append',
  apiKeysEditModeOptions: () => [],
  apiKeysEditModeHint: '',
  deletingApiKeyFingerprint: '',
  restoringApiKeyFingerprint: '',
  section: 'all'
})

const emit = defineEmits<{
  'update:baseUrl': [value: string]
  'update:requestBaseUrlsText': [value: string]
  'update:balanceBaseUrl': [value: string]
  'update:apiKey': [value: string]
  'update:apiKeysText': [value: string]
  'update:claudeCliVersion': [value: string]
  'update:openaiCodexCliUserAgent': [value: string]
  'update:apiKeysEditMode': [value: 'append' | 'replace']
  deleteApiKey: [fingerprint: string]
  restoreApiKey: [fingerprint: string]
}>()

const { t } = useI18n()

const isEditMode = computed(() => props.mode === 'edit')
const showCoreFields = computed(() => props.section === 'all' || props.section === 'core')
const showAdvancedFields = computed(() => props.section === 'all' || props.section === 'advanced')
const supportsRequestBaseUrls = computed(
  () => props.platform === 'openai' || props.platform === 'anthropic'
)
const supportsBalanceBaseUrl = computed(() => props.platform === 'openai')
const supportsClaudeCliVersion = computed(
  () => props.platform === 'anthropic' || props.platform === 'antigravity'
)
const supportsOpenAICodexCliUserAgent = computed(() => props.platform === 'openai')

const baseUrlPlaceholder = computed(() => {
  if (props.platform === 'openai') return 'https://api.openai.com'
  if (props.platform === 'grok') return 'https://api.x.ai/v1'
  if (props.platform === 'gemini') return 'https://generativelanguage.googleapis.com'
  if (props.platform === 'antigravity') return 'https://cloudcode-pa.googleapis.com'
  return 'https://api.anthropic.com'
})

const apiKeyPlaceholder = computed(() => {
  if (props.platform === 'openai') return 'sk-proj-...'
  if (props.platform === 'grok') return 'xai-...'
  if (props.platform === 'gemini') return 'AIza...'
  if (props.platform === 'antigravity') return 'sk-...'
  return 'sk-ant-...'
})

const singleApiKeyLabel = computed(() =>
  isEditMode.value ? 'admin.accounts.apiKey' : 'admin.accounts.apiKeyRequired'
)

const singleApiKeyHint = computed(() =>
  isEditMode.value ? t('admin.accounts.leaveEmptyToKeep') : props.apiKeyHint
)

const multiApiKeysPlaceholder = computed(() =>
  isEditMode.value
    ? t('admin.accounts.apiKeysPlaceholderKeep')
    : t('admin.accounts.apiKeysPlaceholder')
)

const updateEditMode = (value: string | number | boolean | null) => {
  emit('update:apiKeysEditMode', value === 'replace' ? 'replace' : 'append')
}

const keyStateLabel = (item: AccountAPIKeyItem) => {
  if (item.disabled || item.status === 'cooling') return t('admin.accounts.apiKeyStatusCooling')
  return t('admin.accounts.apiKeyStatusActive')
}

const keyStateTestId = (item: AccountAPIKeyItem) =>
  `api-key-state-${item.fingerprint || item.masked.replace(/[^a-zA-Z0-9_-]/g, '-')}`
</script>

<template>
  <div data-testid="account-api-key-credentials-fields" class="space-y-4">
    <div v-if="showCoreFields">
      <label class="input-label">{{ t('admin.accounts.baseUrl') }}</label>
      <input
        :value="props.baseUrl"
        type="text"
        class="input"
        :placeholder="baseUrlPlaceholder"
        @input="emit('update:baseUrl', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ props.baseUrlHint }}</p>
    </div>

    <div
      v-if="showAdvancedFields && supportsRequestBaseUrls"
      class="grid grid-cols-1 gap-4 sm:grid-cols-2"
    >
      <div>
        <label class="input-label">{{ t('admin.accounts.openai.requestBaseUrls') }}</label>
        <textarea
          :value="props.requestBaseUrlsText"
          rows="3"
          class="input font-mono text-xs"
          :placeholder="t('admin.accounts.openai.requestBaseUrlsPlaceholder')"
          @input="emit('update:requestBaseUrlsText', ($event.target as HTMLTextAreaElement).value)"
        ></textarea>
        <p class="input-hint">{{ t('admin.accounts.openai.requestBaseUrlsHint') }}</p>
      </div>
      <div v-if="supportsBalanceBaseUrl">
        <label class="input-label">{{ t('admin.accounts.openai.balanceBaseUrl') }}</label>
        <input
          :value="props.balanceBaseUrl"
          type="text"
          class="input font-mono text-xs"
          placeholder="https://api.openai.com"
          @input="emit('update:balanceBaseUrl', ($event.target as HTMLInputElement).value)"
        />
        <p class="input-hint">{{ t('admin.accounts.openai.balanceBaseUrlHint') }}</p>
      </div>
    </div>

    <div v-if="showCoreFields">
      <label class="input-label">{{ t(singleApiKeyLabel) }}</label>
      <input
        :value="props.apiKey"
        type="password"
        class="input font-mono"
        autocomplete="new-password"
        data-1p-ignore
        data-lpignore="true"
        data-bwignore="true"
        :placeholder="apiKeyPlaceholder"
        @input="emit('update:apiKey', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ singleApiKeyHint }}</p>
    </div>

    <div v-if="showAdvancedFields && supportsClaudeCliVersion">
      <label class="input-label">{{ t('admin.accounts.anthropic.claudeCliVersion') }}</label>
      <input
        :value="props.claudeCliVersion || ''"
        type="text"
        class="input font-mono"
        data-testid="claude-cli-version-input"
        placeholder="2.1.126"
        @input="emit('update:claudeCliVersion', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.accounts.anthropic.claudeCliVersionHint') }}</p>
    </div>

    <div v-if="showAdvancedFields && supportsOpenAICodexCliUserAgent">
      <label class="input-label">{{ t('admin.accounts.openai.codexCLIUserAgent') }}</label>
      <input
        :value="props.openaiCodexCliUserAgent || ''"
        type="text"
        class="input font-mono text-xs"
        data-testid="openai-codex-cli-user-agent-input"
        :placeholder="t('admin.accounts.openai.codexCLIUserAgentPlaceholder')"
        @input="emit('update:openaiCodexCliUserAgent', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.accounts.openai.codexCLIUserAgentHint') }}</p>
    </div>

    <div v-if="showCoreFields">
      <label class="input-label">{{ t('admin.accounts.apiKeys') }}</label>
      <div
        v-if="isEditMode && props.existingApiKeyItems.length > 0"
        class="mb-2 space-y-1 rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="flex items-center justify-between gap-2 text-xs text-gray-600 dark:text-gray-300">
          <span>{{ t('admin.accounts.existingApiKeys') }}</span>
          <span>{{ props.existingApiKeySummary }}</span>
        </div>
        <div class="space-y-1.5">
          <span
            v-for="item in props.existingApiKeyItems"
            :key="item.fingerprint || item.masked"
            :data-testid="keyStateTestId(item)"
            :class="[
              'flex items-center gap-2 rounded-md px-2 py-1.5 text-xs',
              item.disabled
                ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-200'
                : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-200'
            ]"
            :title="item.disabled ? (item.reason || t('admin.accounts.apiKeyDisabled')) : t('common.active')"
          >
            <span class="font-mono text-[11px]">{{ item.masked }}</span>
            <span class="rounded-full bg-white/70 px-1.5 py-0.5 font-sans text-[11px] dark:bg-black/20">
              {{ keyStateLabel(item) }}
            </span>
            <span v-if="item.reason" class="truncate font-sans text-[11px]">{{ item.reason }}</span>
            <span v-if="item.last_error" class="truncate font-sans text-[11px]">{{ item.last_error }}</span>
            <span v-if="item.disabled_until" class="font-sans text-[11px]">
              {{ t('admin.accounts.apiKeyDisabledUntil') }} {{ item.disabled_until }}
            </span>
            <span v-if="item.disabled_count" class="font-sans text-[11px]">
              {{ t('admin.accounts.apiKeyDisabledCount') }} {{ item.disabled_count }}
            </span>
            <button
              v-if="item.fingerprint && item.disabled"
              type="button"
              class="ml-auto inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full transition-colors hover:bg-black/10 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-white/10"
              :title="t('admin.accounts.restoreApiKey')"
              :aria-label="t('admin.accounts.restoreApiKey')"
              :disabled="
                props.restoringApiKeyFingerprint === item.fingerprint ||
                props.deletingApiKeyFingerprint === item.fingerprint
              "
              @click="emit('restoreApiKey', item.fingerprint)"
            >
              <Icon
                name="refresh"
                size="xs"
                :class="{ 'animate-spin': props.restoringApiKeyFingerprint === item.fingerprint }"
                :stroke-width="2"
              />
            </button>
            <button
              v-if="item.fingerprint"
              type="button"
              class="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full transition-colors hover:bg-black/10 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-white/10"
              :title="t('admin.accounts.deleteApiKey')"
              :aria-label="t('admin.accounts.deleteApiKey')"
              :disabled="
                props.deletingApiKeyFingerprint === item.fingerprint ||
                props.restoringApiKeyFingerprint === item.fingerprint
              "
              @click="emit('deleteApiKey', item.fingerprint)"
            >
              <Icon name="trash" size="xs" :stroke-width="2" />
            </button>
          </span>
        </div>
      </div>
      <div v-if="isEditMode" class="mb-2 grid gap-2 sm:grid-cols-[180px_1fr]">
        <Select
          :model-value="props.apiKeysEditMode"
          :options="props.apiKeysEditModeOptions"
          @update:model-value="updateEditMode"
        />
        <p class="input-hint m-0 flex items-center">{{ props.apiKeysEditModeHint }}</p>
      </div>
      <textarea
        :value="props.apiKeysText"
        rows="4"
        class="input font-mono"
        autocomplete="new-password"
        data-1p-ignore
        data-lpignore="true"
        data-bwignore="true"
        :placeholder="multiApiKeysPlaceholder"
        @input="emit('update:apiKeysText', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
      <p v-if="!isEditMode" class="input-hint">{{ t('admin.accounts.apiKeysHint') }}</p>
    </div>
  </div>
</template>
