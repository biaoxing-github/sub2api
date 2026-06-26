<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.editAccount')"
    width="wide"
    @close="handleClose"
  >
    <form
      v-if="account"
      id="edit-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <AccountBasicInfoFields
        v-model:name="form.name"
        v-model:notes="form.notes"
        name-label-key="common.name"
        name-tour="edit-account-form-name"
      />

      <!-- API Key fields (only for apikey type) -->
      <div v-if="account.type === 'apikey'" class="space-y-4">
        <AccountAPIKeyCredentialsFields
          v-model:base-url="editBaseUrl"
          v-model:request-base-urls-text="editRequestBaseUrlsText"
          v-model:balance-base-url="editBalanceBaseUrl"
          v-model:api-key="editApiKey"
          v-model:api-keys-text="editApiKeysText"
          v-model:claude-cli-version="editClaudeCliVersion"
          v-model:openai-codex-cli-user-agent="editOpenAICodexCliUserAgent"
          v-model:api-keys-edit-mode="apiKeysEditMode"
          :platform="account.platform"
          :base-url-hint="baseUrlHint"
          :existing-api-key-items="existingApiKeyItems"
          :existing-api-key-summary="existingApiKeySummary"
          :api-keys-edit-mode-options="apiKeysEditModeOptions"
          :api-keys-edit-mode-hint="apiKeysEditModeHint"
          :deleting-api-key-fingerprint="deletingApiKeyFingerprint"
          :restoring-api-key-fingerprint="restoringApiKeyFingerprint"
          mode="edit"
          @delete-api-key="handleDeleteAPIKey"
          @restore-api-key="handleRestoreAPIKeyState"
        />
        <AccountUpstreamBalanceFields
          v-if="supportsAPIKeyUpstreamBalanceConfig"
          v-model:auth-username="upstreamAuthUsername"
          v-model:auth-password="upstreamAuthPassword"
          v-model:common-rate-multiplier="upstreamCommonRateMultiplier"
          v-model:common-rate-group-name="upstreamCommonRateGroupName"
          v-model:balance-endpoint-paths-text="upstreamBalanceEndpointPathsText"
          v-model:manual-balance-total="editQuotaLimit"
          :has-existing-auth-password="hasUpstreamAuthPassword"
          show-manual-balance
        />

        <AccountModelRestrictionSection
          v-if="account.platform !== 'antigravity'"
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          :platform="account?.platform || 'anthropic'"
          :account-id="account?.id"
          :preset-mappings="presetMappings"
          :disabled="isOpenAIModelRestrictionDisabled"
          disabled-hint-key="admin.accounts.openai.modelRestrictionDisabledByPassthrough"
          supports-all-requires-empty-mappings
        />

        <AccountPoolModeSection
          v-model:enabled="poolModeEnabled"
          v-model:retry-count="poolModeRetryCount"
          :default-retry-count="DEFAULT_POOL_MODE_RETRY_COUNT"
          :max-retry-count="MAX_POOL_MODE_RETRY_COUNT"
        />

        <AccountErrorHandlingCard
          v-model:rules="accountErrorHandlingRules"
          :account-type="account?.type || ''"
          :base-url="editBaseUrl"
          :provider-code="account?.platform || ''"
        />

      </div>

      <!-- OpenAI response text error section -->
      <div
        v-if="account.platform === 'openai'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="mb-3 flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.responseTextError') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.responseTextErrorHint') }}
            </p>
          </div>
          <button
            type="button"
            @click="openAIResponseTextErrorEnabled = !openAIResponseTextErrorEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              openAIResponseTextErrorEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                openAIResponseTextErrorEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div v-if="openAIResponseTextErrorEnabled" class="space-y-2">
          <textarea
            v-model="openAIResponseTextErrorKeywordsText"
            rows="3"
            class="input"
            :placeholder="t('admin.accounts.openai.responseTextErrorPlaceholder')"
          ></textarea>
          <p class="input-hint">{{ t('admin.accounts.openai.responseTextErrorKeywordsHint') }}</p>
        </div>
      </div>

      <AccountModelRestrictionSection
        v-if="account.platform === 'openai' && account.type === 'oauth'"
        v-model:mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        :platform="account?.platform || 'anthropic'"
        :account-id="account?.id"
        :preset-mappings="presetMappings"
        :disabled="isOpenAIModelRestrictionDisabled"
        disabled-hint-key="admin.accounts.openai.modelRestrictionDisabledByPassthrough"
        supports-all-requires-empty-mappings
      />

      <!-- Upstream fields (only for upstream type) -->
      <div v-if="account.type === 'upstream'" class="space-y-4">
        <AccountUpstreamCredentialsFields
          v-model:base-url="editBaseUrl"
          v-model:api-key="editApiKey"
          api-key-hint-key="admin.accounts.leaveEmptyToKeep"
        />
        <AccountUpstreamBalanceFields
          v-model:auth-username="upstreamAuthUsername"
          v-model:auth-password="upstreamAuthPassword"
          v-model:common-rate-multiplier="upstreamCommonRateMultiplier"
          v-model:common-rate-group-name="upstreamCommonRateGroupName"
          v-model:balance-endpoint-paths-text="upstreamBalanceEndpointPathsText"
          :has-existing-auth-password="hasUpstreamAuthPassword"
        />
      </div>

      <!-- Vertex Service Account -->
      <div v-if="(account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account'" class="space-y-4">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">Project ID</label>
            <input
              v-model="editVertexProjectId"
              type="text"
              class="input font-mono"
              readonly
              :placeholder="t('admin.accounts.vertexProjectIdPlaceholder')"
            />
            <p class="input-hint">{{ t('admin.accounts.vertexSaJsonEditHint') }}</p>
          </div>
          <div>
            <label class="input-label">Location</label>
            <select
              v-model="editVertexLocation"
              required
              class="input font-mono"
            >
              <optgroup
                v-for="group in VERTEX_LOCATION_OPTIONS"
                :key="group.label"
                :label="group.label"
              >
                <option
                  v-for="option in group.options"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </optgroup>
            </select>
            <p class="input-hint">{{ t('admin.accounts.vertexLocationHint') }}</p>
          </div>
        </div>

        <AccountModelRestrictionSection
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          :platform="account?.platform || 'anthropic'"
          :account-id="account?.id"
          :preset-mappings="presetMappings"
          supports-all-requires-empty-mappings
        />
      </div>

      <!-- Bedrock fields (for bedrock type, both SigV4 and API Key modes) -->
      <div v-if="account.type === 'bedrock'" class="space-y-4">
        <!-- SigV4 fields -->
        <template v-if="!isBedrockAPIKeyMode">
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
            <input
              v-model="editBedrockAccessKeyId"
              type="text"
              class="input font-mono"
              placeholder="AKIA..."
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
            <input
              v-model="editBedrockSecretAccessKey"
              type="password"
              class="input font-mono"
              :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockSecretKeyLeaveEmpty') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSessionToken') }}</label>
            <input
              v-model="editBedrockSessionToken"
              type="password"
              class="input font-mono"
              :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockSessionTokenHint') }}</p>
          </div>
        </template>

        <!-- API Key field -->
        <div v-if="isBedrockAPIKeyMode">
          <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
          <input
            v-model="editBedrockApiKeyValue"
            type="password"
            class="input font-mono"
            :placeholder="t('admin.accounts.bedrockApiKeyLeaveEmpty')"
          />
          <p class="input-hint">{{ t('admin.accounts.bedrockApiKeyLeaveEmpty') }}</p>
        </div>

        <!-- Shared: Region -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
          <input
            v-model="editBedrockRegion"
            type="text"
            class="input"
            placeholder="us-east-1"
          />
          <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
        </div>

        <!-- Shared: Force Global -->
        <div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="editBedrockForceGlobal"
              type="checkbox"
              class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500"
            />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
          </label>
          <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
        </div>

        <AccountModelRestrictionSection
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          platform="anthropic"
          :preset-mappings="bedrockPresets"
          :allow-duplicate-presets="true"
          from-placeholder-key="admin.accounts.fromModel"
          to-placeholder-key="admin.accounts.toModel"
          supports-all-requires-empty-mappings
        />

        <AccountPoolModeSection
          v-model:enabled="poolModeEnabled"
          v-model:retry-count="poolModeRetryCount"
          :default-retry-count="DEFAULT_POOL_MODE_RETRY_COUNT"
          :max-retry-count="MAX_POOL_MODE_RETRY_COUNT"
        />
      </div>

      <AccountAntigravityModelMappingSection
        v-if="account.platform === 'antigravity'"
        v-model:model-mappings="antigravityModelMappings"
        :preset-mappings="antigravityPresetMappings"
        :sync-loading="isSyncingAntigravityUpstream"
        :sync-disabled="!account?.id"
        show-sync-button
        @sync="syncAntigravityUpstreamModels"
      />

      <!-- Intercept Warmup Requests (Anthropic/Antigravity) -->
      <div
        v-if="account?.platform === 'anthropic' || account?.platform === 'antigravity'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{
              t('admin.accounts.interceptWarmupRequests')
            }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.interceptWarmupRequestsDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="interceptWarmupRequests = !interceptWarmupRequests"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              interceptWarmupRequests ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                interceptWarmupRequests ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <div>
        <div class="mb-1 flex items-center gap-2">
          <label class="input-label mb-0">{{ t('admin.accounts.proxy') }}</label>
          <ProxyAdBanner />
        </div>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" min="1" class="input"
            @input="form.concurrency = Math.max(1, form.concurrency || 1)" />
        </div>
        <div>
          <div class="mb-1 flex items-center justify-between gap-2">
            <label class="input-label mb-0">{{ t('admin.accounts.loadFactor') }}</label>
            <button
              v-if="loadFactorSuggestion != null"
              type="button"
              class="rounded-md border border-primary-200 bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 transition-colors hover:bg-primary-100 dark:border-primary-800 dark:bg-primary-900/20 dark:text-primary-300 dark:hover:bg-primary-900/30"
              :title="loadFactorSuggestionTitle"
              @click="applyLoadFactorSuggestion"
            >
              {{ t('admin.accounts.applyLoadFactorSuggestion', { value: loadFactorSuggestion }) }}
            </button>
          </div>
          <input v-model.number="form.load_factor" type="number" min="1"
            class="input" :placeholder="String(form.concurrency || 1)"
            data-testid="load-factor-input"
            @input="form.load_factor = (form.load_factor &amp;&amp; form.load_factor >= 1) ? form.load_factor : null" />
          <p class="input-hint">
            {{ loadFactorSuggestionHint || t('admin.accounts.loadFactorHint') }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.priority') }}</label>
          <input
            v-model.number="form.priority"
            type="number"
            min="1"
            class="input"
            data-tour="account-form-priority"
          />
          <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.billingRateMultiplier') }}</label>
          <input v-model.number="form.rate_multiplier" type="number" min="0" step="0.001" class="input" />
          <p class="input-hint">{{ t('admin.accounts.billingRateMultiplierHint') }}</p>
        </div>
      </div>
      <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" />
        <p class="input-hint">{{ t('admin.accounts.expiresAtHint') }}</p>
      </div>

      <!-- OpenAI 自动透传开关（OAuth/API Key） -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'apikey')"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.oauthPassthrough') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.oauthPassthroughDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="openaiPassthroughEnabled = !openaiPassthroughEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              openaiPassthroughEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                openaiPassthroughEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
        <div v-if="account?.type === 'oauth'" class="mt-3">
          <label class="input-label">{{ t('admin.accounts.openai.codexCLIUserAgent') }}</label>
          <input
            v-model="editOpenAICodexCliUserAgent"
            type="text"
            class="input font-mono text-xs"
            :placeholder="t('admin.accounts.openai.codexCLIUserAgentPlaceholder')"
            data-testid="edit-openai-codex-cli-user-agent-input"
          />
          <p class="input-hint">{{ t('admin.accounts.openai.codexCLIUserAgentHint') }}</p>
        </div>
      </div>

      <!-- OpenAI Codex 图片生成桥接账号级覆盖 -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'apikey')"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 shadow-sm dark:border-dark-700 dark:bg-dark-800/70">
          <div class="flex items-start gap-3 px-4 py-3">
            <div class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-white text-slate-600 shadow-sm ring-1 ring-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:ring-dark-600">
              <Icon name="sparkles" size="sm" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <label class="input-label mb-0">{{ t('admin.accounts.openai.codexImageGenerationBridge') }}</label>
                <span
                  class="rounded-full px-2 py-0.5 text-[11px] font-medium"
                  :class="codexImageGenerationBridgeBadgeClass"
                >
                  {{ codexImageGenerationBridgeBadgeLabel }}
                </span>
              </div>
              <p class="mt-1 text-xs leading-5 text-slate-600 dark:text-slate-300">
                {{ t('admin.accounts.openai.codexImageGenerationBridgeDesc') }}
              </p>
            </div>
          </div>
          <div class="border-t border-gray-200 bg-white/70 p-2 dark:border-dark-700 dark:bg-dark-800/70">
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
              <button
                v-for="option in codexImageGenerationBridgeOptions"
                :key="option.value"
                type="button"
                :data-testid="`codex-image-bridge-${option.value}`"
                @click="codexImageGenerationBridgeMode = option.value"
                :class="[
                  'group flex min-h-[68px] items-start gap-2 rounded-md border px-3 py-2 text-left transition-all',
                  codexImageGenerationBridgeMode === option.value
                    ? 'border-slate-400 bg-slate-50 text-slate-900 shadow-sm ring-1 ring-slate-200 dark:border-dark-400 dark:bg-dark-700 dark:text-gray-100 dark:ring-dark-500'
                    : 'border-transparent bg-transparent text-slate-600 hover:border-gray-200 hover:bg-gray-50 dark:text-slate-300 dark:hover:border-dark-500 dark:hover:bg-dark-700'
                ]"
              >
                <span
                  :class="[
                    'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors',
                    codexImageGenerationBridgeMode === option.value
                      ? 'border-slate-700 bg-slate-700 text-white dark:border-slate-200 dark:bg-slate-200 dark:text-dark-900'
                      : 'border-gray-300 text-transparent group-hover:border-gray-400 dark:border-dark-500'
                  ]"
                >
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
                <span class="min-w-0">
                  <span class="block text-sm font-medium">{{ option.label }}</span>
                  <span class="mt-0.5 block text-xs leading-4 text-slate-500 dark:text-slate-400">{{ option.description }}</span>
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- OpenAI WS Mode 三态（off/ctx_pool/passthrough） -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'apikey')"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.wsMode') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.wsModeDesc') }}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t(openAIWSModeConcurrencyHintKey) }}
            </p>
          </div>
          <div class="w-52">
            <Select v-model="openaiResponsesWebSocketV2Mode" :options="openAIWSModeOptions" />
          </div>
        </div>
      </div>

      <!-- OpenAI APIKey Responses API support mode -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'apikey'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600 space-y-3"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.responsesMode') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.responsesModeDesc') }}
            </p>
          </div>
          <div class="w-56">
            <Select
              v-model="openAIResponsesMode"
              :options="openAIResponsesModeOptions"
              data-testid="openai-responses-mode-select"
            />
          </div>
        </div>
        <div class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
          <span class="font-medium">{{ t(openAIResponsesStatusKey) }}</span>
        </div>
      </div>

      <!-- Anthropic API Key 自动透传开关 -->
      <div
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyPassthrough') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.anthropic.apiKeyPassthroughDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="anthropicPassthroughEnabled = !anthropicPassthroughEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              anthropicPassthroughEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                anthropicPassthroughEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- Anthropic API Key 1M 上下文开关 -->
      <div
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.context1M') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.anthropic.context1MDesc') }}
            </p>
          </div>
          <button
            type="button"
            :class="[
              'inline-flex min-w-24 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
              anthropicContext1MEnabled
                ? 'bg-emerald-100 text-emerald-700 ring-1 ring-emerald-500 dark:bg-emerald-900/30 dark:text-emerald-300'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500'
            ]"
            :aria-pressed="anthropicContext1MEnabled"
            @click="anthropicContext1MEnabled = !anthropicContext1MEnabled"
          >
            <Icon :name="anthropicContext1MEnabled ? 'checkCircle' : 'xCircle'" size="sm" :stroke-width="2" />
            {{ anthropicContext1MEnabled ? t('admin.accounts.anthropic.context1MEnabled') : t('admin.accounts.anthropic.context1MDisabled') }}
          </button>
        </div>
      </div>

      <!-- Anthropic API Key: Web Search Emulation (hidden when global disabled) -->
      <div
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey' && webSearchGlobalEnabled"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.webSearchEmulation') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.anthropic.webSearchEmulationDesc') }}
            </p>
          </div>
          <select v-model="webSearchEmulationMode" class="input w-24 text-sm">
            <option value="default">{{ t('admin.accounts.anthropic.webSearchDefault') }}</option>
            <option value="enabled">{{ t('admin.accounts.anthropic.webSearchEnabled') }}</option>
            <option value="disabled">{{ t('admin.accounts.anthropic.webSearchDisabled') }}</option>
          </select>
        </div>
      </div>

      <AccountQuotaControlSection
        v-if="account?.platform === 'anthropic' && (account?.type === 'apikey' || account?.type === 'bedrock')"
        hint-key="admin.accounts.quotaControl.hint"
        v-model:total-limit="editQuotaLimit"
        v-model:daily-limit="editQuotaDailyLimit"
        v-model:weekly-limit="editQuotaWeeklyLimit"
        v-model:daily-reset-mode="editDailyResetMode"
        v-model:daily-reset-hour="editDailyResetHour"
        v-model:weekly-reset-mode="editWeeklyResetMode"
        v-model:weekly-reset-day="editWeeklyResetDay"
        v-model:weekly-reset-hour="editWeeklyResetHour"
        v-model:reset-timezone="editResetTimezone"
        v-model:quota-notify-daily-enabled="quotaNotifyState.daily.enabled"
        v-model:quota-notify-daily-threshold="quotaNotifyState.daily.threshold"
        v-model:quota-notify-daily-threshold-type="quotaNotifyState.daily.thresholdType"
        v-model:quota-notify-weekly-enabled="quotaNotifyState.weekly.enabled"
        v-model:quota-notify-weekly-threshold="quotaNotifyState.weekly.threshold"
        v-model:quota-notify-weekly-threshold-type="quotaNotifyState.weekly.thresholdType"
        v-model:quota-notify-total-enabled="quotaNotifyState.total.enabled"
        v-model:quota-notify-total-threshold="quotaNotifyState.total.threshold"
        v-model:quota-notify-total-threshold-type="quotaNotifyState.total.thresholdType"
        :quota-notify-global-enabled="quotaNotifyGlobalEnabled"
      />
      <AccountQuotaControlSection
        v-else-if="account?.type === 'apikey' || account?.type === 'bedrock'"
        hint-key="admin.accounts.quotaLimitHint"
        v-model:total-limit="editQuotaLimit"
        v-model:daily-limit="editQuotaDailyLimit"
        v-model:weekly-limit="editQuotaWeeklyLimit"
        v-model:daily-reset-mode="editDailyResetMode"
        v-model:daily-reset-hour="editDailyResetHour"
        v-model:weekly-reset-mode="editWeeklyResetMode"
        v-model:weekly-reset-day="editWeeklyResetDay"
        v-model:weekly-reset-hour="editWeeklyResetHour"
        v-model:reset-timezone="editResetTimezone"
        v-model:quota-notify-daily-enabled="quotaNotifyState.daily.enabled"
        v-model:quota-notify-daily-threshold="quotaNotifyState.daily.threshold"
        v-model:quota-notify-daily-threshold-type="quotaNotifyState.daily.thresholdType"
        v-model:quota-notify-weekly-enabled="quotaNotifyState.weekly.enabled"
        v-model:quota-notify-weekly-threshold="quotaNotifyState.weekly.threshold"
        v-model:quota-notify-weekly-threshold-type="quotaNotifyState.weekly.thresholdType"
        v-model:quota-notify-total-enabled="quotaNotifyState.total.enabled"
        v-model:quota-notify-total-threshold="quotaNotifyState.total.threshold"
        v-model:quota-notify-total-threshold-type="quotaNotifyState.total.thresholdType"
        :quota-notify-global-enabled="quotaNotifyGlobalEnabled"
      />

      <!-- OpenAI OAuth Codex 官方客户端限制开关 -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth'"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="codexCLIOnlyEnabled = !codexCLIOnlyEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              codexCLIOnlyEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                codexCLIOnlyEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- OpenAI 上游 Codex CLI 模拟开关 -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'apikey')"
        class="border-t border-gray-200 pt-4 dark:border-dark-600"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLISimulation') }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.openai.codexCLISimulationDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="openAICodexCLISimulationEnabled = !openAICodexCLISimulationEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              openAICodexCLISimulationEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                openAICodexCLISimulationEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <AccountOpenAICompactModeSection
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'apikey')"
        v-model:compact-mode="openAICompactMode"
        v-model:compact-model-mappings="openAICompactModelMappings"
        :compact-mode-options="openAICompactModeOptions"
        :status-key="openAICompactStatusKey"
        :last-checked-text="
          account?.extra?.openai_compact_checked_at
            ? formatDateTime(new Date(String(account.extra.openai_compact_checked_at)))
            : undefined
        "
      />

      <div>
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{
              t('admin.accounts.autoPauseOnExpired')
            }}</label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.autoPauseOnExpiredDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="autoPauseOnExpired = !autoPauseOnExpired"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              autoPauseOnExpired ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                autoPauseOnExpired ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <AccountAvailabilityScheduleEditor
        v-model="accountAvailabilitySchedule"
        :summary="availabilityScheduleSummary"
        :validation-error="availabilityScheduleValidationError"
      />

      <AccountAnthropicQuotaControlSection
        v-if="account?.platform === 'anthropic' && (account?.type === 'oauth' || account?.type === 'setup-token')"
        v-model:window-cost-enabled="windowCostEnabled"
        v-model:window-cost-limit="windowCostLimit"
        v-model:window-cost-sticky-reserve="windowCostStickyReserve"
        v-model:session-limit-enabled="sessionLimitEnabled"
        v-model:max-sessions="maxSessions"
        v-model:session-idle-timeout="sessionIdleTimeout"
        v-model:rpm-limit-enabled="rpmLimitEnabled"
        v-model:base-rpm="baseRpm"
        v-model:rpm-strategy="rpmStrategy"
        v-model:rpm-sticky-buffer="rpmStickyBuffer"
        v-model:user-msg-queue-mode="userMsgQueueMode"
        v-model:tls-fingerprint-enabled="tlsFingerprintEnabled"
        v-model:tls-fingerprint-profile-id="tlsFingerprintProfileId"
        v-model:session-id-masking-enabled="sessionIdMaskingEnabled"
        v-model:cache-ttl-override-enabled="cacheTTLOverrideEnabled"
        v-model:cache-ttl-override-target="cacheTTLOverrideTarget"
        v-model:custom-base-url-enabled="customBaseUrlEnabled"
        v-model:custom-base-url="customBaseUrl"
        :tls-fingerprint-profiles="tlsFingerprintProfiles"
      />

      <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
        <div>
          <label class="input-label">{{ t('common.status') }}</label>
          <Select v-model="form.status" :options="statusOptions" />
        </div>

        <!-- Mixed Scheduling (only for antigravity accounts, read-only in edit mode) -->
        <div v-if="account?.platform === 'antigravity'" class="flex items-center gap-2">
          <label class="flex cursor-not-allowed items-center gap-2 opacity-60">
            <input
              type="checkbox"
              v-model="mixedScheduling"
              disabled
              class="h-4 w-4 cursor-not-allowed rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
            />
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.mixedScheduling') }}
            </span>
          </label>
          <div class="group relative">
            <span
              class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-200 text-xs text-gray-500 hover:bg-gray-300 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500"
            >
              ?
            </span>
            <!-- Tooltip（向下显示避免被弹窗裁剪） -->
            <div
              class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-gray-900 px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100 dark:bg-gray-700"
            >
              {{ t('admin.accounts.mixedSchedulingTooltip') }}
              <div
                class="absolute bottom-full left-3 border-4 border-transparent border-b-gray-900 dark:border-b-gray-700"
              ></div>
            </div>
          </div>
        </div>
        <div v-if="account?.platform === 'antigravity'" class="mt-3 flex items-center gap-2">
          <label class="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              v-model="allowOverages"
              class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
            />
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.allowOverages') }}
            </span>
          </label>
          <div class="group relative">
            <span
              class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-200 text-xs text-gray-500 hover:bg-gray-300 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500"
            >
              ?
            </span>
            <div
              class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-gray-900 px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100 dark:bg-gray-700"
            >
              {{ t('admin.accounts.allowOveragesTooltip') }}
              <div
                class="absolute bottom-full left-3 border-4 border-transparent border-b-gray-900 dark:border-b-gray-700"
              ></div>
            </div>
          </div>
        </div>
      </div>

      <!-- Group Selection - 仅标准模式显示 -->
      <GroupSelector
        v-if="!authStore.isSimpleMode"
        v-model="form.group_ids"
        :groups="groups"
        :platform="account?.platform"
        :mixed-scheduling="mixedScheduling"
        data-tour="account-form-groups"
      />

    </form>

    <template #footer>
      <div v-if="account" class="flex justify-end gap-3">
        <button @click="handleClose" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
          data-tour="account-form-submit"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ submitting ? t('admin.accounts.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- Mixed Channel Warning Dialog -->
  <ConfirmDialog
    :show="showMixedChannelWarning"
    :title="t('admin.accounts.mixedChannelWarningTitle')"
    :message="mixedChannelWarningMessageText"
    :confirm-text="t('common.confirm')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="handleMixedChannelConfirm"
    @cancel="handleMixedChannelCancel"
  />
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import { useQuotaNotifyState } from '@/composables/useQuotaNotifyState'
import type { Account, Proxy, AdminGroup, CheckMixedChannelResponse, OpenAICompactMode, OpenAIResponsesMode } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountErrorHandlingCard from '@/components/account/AccountErrorHandlingCard.vue'
import AccountAPIKeyCredentialsFields from '@/components/account/AccountAPIKeyCredentialsFields.vue'
import AccountBasicInfoFields from '@/components/account/AccountBasicInfoFields.vue'
import AccountPoolModeSection from '@/components/account/AccountPoolModeSection.vue'
import AccountUpstreamCredentialsFields from '@/components/account/AccountUpstreamCredentialsFields.vue'
import AccountUpstreamBalanceFields from '@/components/account/AccountUpstreamBalanceFields.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'
import { loadAccountErrorHandlingRules } from '@/components/account/errorHandlingRules'
import { writeAccountErrorHandlingToCredentials } from '@/components/account/accountErrorHandlingPayload'
import type { AccountErrorHandlingRuleForm } from '@/components/account/accountErrorHandlingTypes'
import AccountAvailabilityScheduleEditor from '@/components/account/AccountAvailabilityScheduleEditor.vue'
import {
  accountScheduleSummary,
  buildAccountAvailabilitySchedulePayload,
  createAccountAvailabilityScheduleForm,
  readAccountAvailabilityScheduleFromExtra,
  validateAccountAvailabilityScheduleForm,
  writeAccountAvailabilityScheduleToExtra,
  type AccountAvailabilityScheduleForm
} from '@/components/account/accountAvailabilitySchedule'
import GroupSelector from '@/components/common/GroupSelector.vue'
import AccountModelRestrictionSection from '@/components/account/AccountModelRestrictionSection.vue'
import AccountQuotaControlSection from '@/components/account/AccountQuotaControlSection.vue'
import AccountAnthropicQuotaControlSection from '@/components/account/AccountAnthropicQuotaControlSection.vue'
import AccountAntigravityModelMappingSection from '@/components/account/AccountAntigravityModelMappingSection.vue'
import AccountOpenAICompactModeSection from '@/components/account/AccountOpenAICompactModeSection.vue'
import { applyInterceptWarmup } from '@/components/account/credentialsBuilder'
import { formatDateTime, formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'
import {
  OPENAI_WS_MODE_CTX_POOL,
  OPENAI_WS_MODE_OFF,
  OPENAI_WS_MODE_PASSTHROUGH,
  isOpenAIWSModeEnabled,
  resolveOpenAIWSModeConcurrencyHintKey,
  type OpenAIWSMode,
  resolveOpenAIWSModeFromExtra
} from '@/utils/openaiWsMode'
import {
  getPresetMappingsByPlatform,
  buildModelMappingObject,
  splitModelMappingObject
} from '@/composables/useModelWhitelist'

interface Props {
  show: boolean
  account: Account | null
  proxies: Proxy[]
  groups: AdminGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

// Platform-specific hint for Base URL
const baseUrlHint = computed(() => {
  if (!props.account) return t('admin.accounts.baseUrlHint')
  if (props.account.platform === 'openai') return t('admin.accounts.openai.baseUrlHint')
  if (props.account.platform === 'gemini') return t('admin.accounts.gemini.baseUrlHint')
  return t('admin.accounts.baseUrlHint')
})

function parseAPIKeysText(value: string): string[] {
  const seen = new Set<string>()
  const keys: string[] = []
  value
    .split(/\r?\n|,/)
    .map(item => item.trim())
    .filter(Boolean)
    .forEach(key => {
      if (seen.has(key)) return
      seen.add(key)
      keys.push(key)
    })
  return keys
}

function parseBaseURLsText(value: string): string[] {
  const seen = new Set<string>()
  const urls: string[] = []
  value
    .split(/\r?\n|,/)
    .map(item => item.trim().replace(/\/+$/, ''))
    .filter(Boolean)
    .forEach(url => {
      if (seen.has(url)) return
      seen.add(url)
      urls.push(url)
    })
  return urls
}

function parseEndpointPathsText(value: string): string[] {
  const seen = new Set<string>()
  const paths: string[] = []
  value
    .split(/\r?\n|,/)
    .map(item => item.trim())
    .filter(Boolean)
    .forEach(path => {
      if (!path.startsWith('http://') && !path.startsWith('https://') && !path.startsWith('/')) {
        path = `/${path}`
      }
      if (seen.has(path)) return
      seen.add(path)
      paths.push(path)
    })
  return paths
}

function parseOpenAIResponseTextErrorKeywordsText(value: string): string[] {
  const seen = new Set<string>()
  const keywords: string[] = []
  value
    .split(/\r?\n|,/)
    .map(item => item.trim())
    .filter(Boolean)
    .forEach(keyword => {
      const key = keyword.toLowerCase()
      if (seen.has(key)) return
      seen.add(key)
      keywords.push(keyword)
    })
  return keywords
}

function openAIResponseTextErrorKeywordsToText(raw: unknown): string {
  if (Array.isArray(raw)) {
    return raw.map(item => String(item).trim()).filter(Boolean).join('\n')
  }
  return typeof raw === 'string' ? raw : ''
}

function endpointPathsToText(raw: unknown): string {
  if (Array.isArray(raw)) {
    const values = raw.map(item => String(item).trim()).filter(Boolean)
    return (values.length > 0 ? values : DEFAULT_UPSTREAM_BALANCE_ENDPOINT_PATHS).join('\n')
  }
  if (typeof raw === 'string' && raw.trim()) {
    return raw
  }
  return DEFAULT_UPSTREAM_BALANCE_ENDPOINT_PATHS.join('\n')
}

function upstreamManualRateMultiplierFrom(
  credentials?: Record<string, unknown>,
  extra?: Record<string, unknown>
): number | null {
  const extraManual = extra?.upstream_manual_rate_multiplier
  if (typeof extraManual === 'number' && extraManual > 0) {
    return extraManual
  }
  const credentialManual = credentials?.upstream_manual_rate_multiplier
  if (typeof credentialManual === 'number' && credentialManual > 0) {
    return credentialManual
  }
  const legacyCredentialManual = credentials?.upstream_common_rate_multiplier
  if (typeof legacyCredentialManual === 'number' && legacyCredentialManual > 0) {
    return legacyCredentialManual
  }
  return null
}

function upstreamManualRateGroupNameFrom(
  credentials?: Record<string, unknown>,
  extra?: Record<string, unknown>
): string {
  const extraManual = extra?.upstream_manual_rate_group_name
  if (typeof extraManual === 'string' && extraManual.trim()) {
    return extraManual
  }
  const credentialManual = credentials?.upstream_manual_rate_group_name
  if (typeof credentialManual === 'string' && credentialManual.trim()) {
    return credentialManual
  }
  const legacyCredentialManual = credentials?.upstream_common_rate_group_name
  return typeof legacyCredentialManual === 'string' ? legacyCredentialManual : ''
}

function applyOpenAICodexCliUserAgentCredentials(credentials: Record<string, unknown>) {
  const normalizedUserAgent = editOpenAICodexCliUserAgent.value.trim()
  if (normalizedUserAgent) {
    credentials.openai_codex_cli_user_agent = normalizedUserAgent
  } else {
    delete credentials.openai_codex_cli_user_agent
  }
}

const antigravityPresetMappings = computed(() => getPresetMappingsByPlatform('antigravity'))
const bedrockPresets = computed(() => getPresetMappingsByPlatform('bedrock'))

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State
const submitting = ref(false)
const editBaseUrl = ref('https://api.anthropic.com')
const editRequestBaseUrlsText = ref('')
const editBalanceBaseUrl = ref('')
const editApiKey = ref('')
const editApiKeysText = ref('')
const editClaudeCliVersion = ref('')
const editOpenAICodexCliUserAgent = ref('')
const apiKeysEditMode = ref<'append' | 'replace'>('append')
const deletingApiKeyFingerprint = ref<string | null>(null)
const restoringApiKeyFingerprint = ref<string | null>(null)
const upstreamAuthUsername = ref('')
const upstreamAuthPassword = ref('')
const upstreamCommonRateMultiplier = ref<number | null>(null)
const upstreamCommonRateGroupName = ref('')
const DEFAULT_UPSTREAM_BALANCE_ENDPOINT_PATHS = [
  '/v1/usage',
  '/v1/dashboard/billing/subscription',
  '/dashboard/billing/subscription',
  '/api/usage/token/',
  '/dashboard/billing/credit_grants',
  '/v1/dashboard/billing/credit_grants',
  '/api/user/self',
  '/api/user/self/stat',
  '/api/user/self/usage',
  '/api/user/balance',
  '/user/balance',
  '/balance',
  '/api/v1/usage'
]
const upstreamBalanceEndpointPathsText = ref(DEFAULT_UPSTREAM_BALANCE_ENDPOINT_PATHS.join('\n'))
const hasUpstreamAuthPassword = computed(() =>
  Boolean(props.account?.credentials_status?.has_upstream_auth_password)
)
const supportsAPIKeyUpstreamBalanceConfig = computed(() =>
  props.account?.type === 'apikey' &&
  (props.account.platform === 'openai' || props.account.platform === 'anthropic')
)
// Bedrock credentials
const editBedrockAccessKeyId = ref('')
const editBedrockSecretAccessKey = ref('')
const editBedrockSessionToken = ref('')
const editBedrockRegion = ref('')
const editBedrockForceGlobal = ref(false)
const editBedrockApiKeyValue = ref('')
const editVertexProjectId = ref('')
const editVertexClientEmail = ref('')
const editVertexLocation = ref('us-central1')
const isBedrockAPIKeyMode = computed(() =>
  props.account?.type === 'bedrock' &&
  (props.account?.credentials as Record<string, unknown>)?.auth_mode === 'apikey'
)
const modelMappings = ref<ModelMapping[]>([])
const openAICompactModelMappings = ref<ModelMapping[]>([])
const modelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const allowedModels = ref<string[]>([])
const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
const poolModeEnabled = ref(false)
const poolModeRetryCount = ref(DEFAULT_POOL_MODE_RETRY_COUNT)
const accountErrorHandlingRules = ref<AccountErrorHandlingRuleForm[]>([])
const openAIResponseTextErrorEnabled = ref(false)
const openAIResponseTextErrorKeywordsText = ref('')
const interceptWarmupRequests = ref(false)
const autoPauseOnExpired = ref(false)
const accountAvailabilitySchedule = ref<AccountAvailabilityScheduleForm>(createAccountAvailabilityScheduleForm())
const mixedScheduling = ref(false) // For antigravity accounts: enable mixed scheduling
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityModelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const antigravityWhitelistModels = ref<string[]>([])
const antigravityModelMappings = ref<ModelMapping[]>([])
const isSyncingAntigravityUpstream = ref(false)
const existingApiKeyItems = computed(() => props.account?.api_key_items || [])
const existingApiKeySummary = computed(() => {
  const total = existingApiKeyItems.value.length
  const disabled = existingApiKeyItems.value.filter(item => item.disabled).length
  return disabled > 0
    ? t('admin.accounts.apiKeysSummaryWithDisabled', { total, disabled })
    : t('admin.accounts.apiKeysSummary', { total })
})
const apiKeysEditModeOptions = computed(() => [
  { value: 'append', label: t('admin.accounts.apiKeysAppendMode') },
  { value: 'replace', label: t('admin.accounts.apiKeysReplaceMode') }
])
const apiKeysEditModeHint = computed(() =>
  apiKeysEditMode.value === 'append'
    ? t('admin.accounts.apiKeysAppendHint')
    : t('admin.accounts.apiKeysReplaceHint')
)

const showMixedChannelWarning = ref(false)
const mixedChannelWarningDetails = ref<{ groupName: string; currentPlatform: string; otherPlatform: string } | null>(
  null
)
const mixedChannelWarningRawMessage = ref('')
const mixedChannelWarningAction = ref<(() => Promise<void>) | null>(null)
const antigravityMixedChannelConfirmed = ref(false)

// Quota control state (Anthropic OAuth/SetupToken only)
const windowCostEnabled = ref(false)
const windowCostLimit = ref<number | null>(null)
const windowCostStickyReserve = ref<number | null>(null)
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const sessionIdleTimeout = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)
const rpmStrategy = ref<'tiered' | 'sticky_exempt'>('tiered')
const rpmStickyBuffer = ref<number | null>(null)
const userMsgQueueMode = ref('')
const tlsFingerprintEnabled = ref(false)
const tlsFingerprintProfileId = ref<number | null>(null)
const tlsFingerprintProfiles = ref<{ id: number; name: string }[]>([])
const sessionIdMaskingEnabled = ref(false)
const cacheTTLOverrideEnabled = ref(false)
const cacheTTLOverrideTarget = ref<string>('5m')
const customBaseUrlEnabled = ref(false)
const customBaseUrl = ref('')

// OpenAI 自动透传开关（OAuth/API Key）
const openaiPassthroughEnabled = ref(false)
const openAICompactMode = ref<OpenAICompactMode>('auto')
const openAIResponsesMode = ref<OpenAIResponsesMode>('auto')
const openaiOAuthResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const openaiAPIKeyResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const codexCLIOnlyEnabled = ref(false)
const openAICodexCLISimulationEnabled = ref(false)
type CodexImageGenerationBridgeMode = 'inherit' | 'enabled' | 'disabled'
const codexImageGenerationBridgeMode = ref<CodexImageGenerationBridgeMode>('inherit')
const anthropicPassthroughEnabled = ref(false)
const anthropicContext1MEnabled = ref(false)
const webSearchEmulationMode = ref('default')
const webSearchGlobalEnabled = ref(false)
const {
  globalEnabled: quotaNotifyGlobalEnabled,
  state: quotaNotifyState,
  loadGlobalState: loadQuotaNotifyGlobal,
  loadFromExtra: loadQuotaNotifyFromExtra,
  writeToExtra: writeQuotaNotifyToExtra,
  reset: resetQuotaNotify,
} = useQuotaNotifyState()

// Load global feature states once
adminAPI.settings.getWebSearchEmulationConfig().then(cfg => {
  webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
}).catch(() => { webSearchGlobalEnabled.value = false })

loadQuotaNotifyGlobal()
const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
const editDailyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editDailyResetHour = ref<number | null>(null)
const editWeeklyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editWeeklyResetDay = ref<number | null>(null)
const editWeeklyResetHour = ref<number | null>(null)
const editResetTimezone = ref<string | null>(null)
const openAIWSModeOptions = computed(() => [
  { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
  { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
  { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') }
])
const openaiResponsesWebSocketV2Mode = computed({
  get: () => {
    if (props.account?.type === 'apikey') {
      return openaiAPIKeyResponsesWebSocketV2Mode.value
    }
    return openaiOAuthResponsesWebSocketV2Mode.value
  },
  set: (mode: OpenAIWSMode) => {
    if (props.account?.type === 'apikey') {
      openaiAPIKeyResponsesWebSocketV2Mode.value = mode
      return
    }
    openaiOAuthResponsesWebSocketV2Mode.value = mode
  }
})
const openAIWSModeConcurrencyHintKey = computed(() =>
  resolveOpenAIWSModeConcurrencyHintKey(openaiResponsesWebSocketV2Mode.value)
)
const codexImageGenerationBridgeOptions = computed<Array<{
  value: CodexImageGenerationBridgeMode
  label: string
  description: string
}>>(() => [
  {
    value: 'inherit',
    label: t('admin.accounts.openai.codexImageGenerationBridgeInherit'),
    description: t('admin.accounts.openai.codexImageGenerationBridgeInheritDesc')
  },
  {
    value: 'enabled',
    label: t('admin.accounts.openai.codexImageGenerationBridgeEnabled'),
    description: t('admin.accounts.openai.codexImageGenerationBridgeEnabledDesc')
  },
  {
    value: 'disabled',
    label: t('admin.accounts.openai.codexImageGenerationBridgeDisabled'),
    description: t('admin.accounts.openai.codexImageGenerationBridgeDisabledDesc')
  }
])
const codexImageGenerationBridgeBadgeLabel = computed(() => {
  switch (codexImageGenerationBridgeMode.value) {
    case 'enabled':
      return t('admin.accounts.openai.codexImageGenerationBridgeBadgeEnabled')
    case 'disabled':
      return t('admin.accounts.openai.codexImageGenerationBridgeBadgeDisabled')
    default:
      return t('admin.accounts.openai.codexImageGenerationBridgeBadgeInherit')
  }
})
const codexImageGenerationBridgeBadgeClass = computed(() => {
  switch (codexImageGenerationBridgeMode.value) {
    case 'enabled':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'disabled':
      return 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-dark-600 dark:text-slate-300'
  }
})
const openAICompactModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
  { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
  { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
])
const openAIResponsesModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.responsesModeAuto') },
  { value: 'force_responses', label: t('admin.accounts.openai.responsesModeForceResponses') },
  { value: 'force_chat_completions', label: t('admin.accounts.openai.responsesModeForceChatCompletions') }
])
const normalizeOpenAIResponsesMode = (mode: unknown): OpenAIResponsesMode => {
  if (mode === 'force_responses' || mode === 'force_chat_completions') {
    return mode
  }
  return 'auto'
}
const isOpenAIModelRestrictionDisabled = computed(() =>
  props.account?.platform === 'openai' && openaiPassthroughEnabled.value
)
const openAIResponsesStatusKey = computed(() => {
  if (openAIResponsesMode.value === 'force_responses') {
    return 'admin.accounts.openai.responsesStatusForcedResponses'
  }
  if (openAIResponsesMode.value === 'force_chat_completions') {
    return 'admin.accounts.openai.responsesStatusForcedChatCompletions'
  }
  const extra = props.account?.extra as Record<string, unknown> | undefined
  if (extra?.openai_responses_supported === true) {
    return 'admin.accounts.openai.responsesStatusAutoSupported'
  }
  if (extra?.openai_responses_supported === false) {
    return 'admin.accounts.openai.responsesStatusAutoUnsupported'
  }
  return 'admin.accounts.openai.responsesStatusAutoUnknown'
})
const openAICompactStatusKey = computed(() => {
  const extra = props.account?.extra as Record<string, unknown> | undefined
  if (!props.account || props.account.platform !== 'openai') return ''
  const mode = typeof extra?.openai_compact_mode === 'string' ? extra.openai_compact_mode : 'auto'
  if (mode === 'force_on') return 'admin.accounts.openai.compactSupported'
  if (mode === 'force_off') return 'admin.accounts.openai.compactUnsupported'
  if (typeof extra?.openai_compact_supported === 'boolean') {
    return extra.openai_compact_supported
      ? 'admin.accounts.openai.compactSupported'
      : 'admin.accounts.openai.compactUnsupported'
  }
  return 'admin.accounts.openai.compactAuto'
})
const availabilityScheduleValidationError = computed(() =>
  validateAccountAvailabilityScheduleForm(accountAvailabilitySchedule.value)
)
const availabilityScheduleSummary = computed(() => {
  if (availabilityScheduleValidationError.value) return ''
  return accountScheduleSummary(buildAccountAvailabilitySchedulePayload(accountAvailabilitySchedule.value))
})
const loadFactorSuggestion = computed(() => props.account?.load_factor_advice?.suggested_load_factor ?? null)
const loadFactorSuggestionReasons = computed(() => props.account?.load_factor_advice?.reasons ?? [])
const loadFactorSuggestionTitle = computed(() => loadFactorSuggestionReasons.value.join('\n'))
const loadFactorSuggestionHint = computed(() => {
  if (loadFactorSuggestion.value != null) {
    return t('admin.accounts.loadFactorSuggestionHint', { value: loadFactorSuggestion.value })
  }
  const radar = props.account?.load_factor_advice?.availability_radar
  if (radar?.status === 'needs_probe') {
    return radar.label || t('admin.accounts.loadFactorInsufficientSamples')
  }
  return ''
})
const applyLoadFactorSuggestion = () => {
  if (loadFactorSuggestion.value == null) return
  form.load_factor = loadFactorSuggestion.value
}

// Computed: current preset mappings based on platform
const presetMappings = computed(() => getPresetMappingsByPlatform(props.account?.platform || 'anthropic'))

// Computed: default base URL based on platform
const defaultBaseUrl = computed(() => {
  if (props.account?.platform === 'openai') return 'https://api.openai.com'
  if (props.account?.platform === 'gemini') return 'https://generativelanguage.googleapis.com'
  return 'https://api.anthropic.com'
})

const mixedChannelWarningMessageText = computed(() => {
  if (mixedChannelWarningDetails.value) {
    return t('admin.accounts.mixedChannelWarning', mixedChannelWarningDetails.value)
  }
  return mixedChannelWarningRawMessage.value
})

const form = reactive({
  name: '',
  notes: '',
  proxy_id: null as number | null,
  concurrency: 1,
  load_factor: null as number | null,
  priority: 1,
  rate_multiplier: 1,
  status: 'active' as 'active' | 'inactive' | 'error',
  group_ids: [] as number[],
  expires_at: null as number | null
})

const statusOptions = computed(() => {
  const options = [
    { value: 'active', label: t('common.active') },
    { value: 'inactive', label: t('common.inactive') }
  ]
  if (form.status === 'error') {
    options.push({ value: 'error', label: t('admin.accounts.status.error') })
  }
  return options
})

const expiresAtInput = computed({
  get: () => formatDateTimeLocal(form.expires_at),
  set: (value: string) => {
    form.expires_at = parseDateTimeLocal(value)
  }
})

// Watchers
const normalizePoolModeRetryCount = (value: number) => {
  if (!Number.isFinite(value)) {
    return DEFAULT_POOL_MODE_RETRY_COUNT
  }
  const normalized = Math.trunc(value)
  if (normalized < 0) {
    return 0
  }
  if (normalized > MAX_POOL_MODE_RETRY_COUNT) {
    return MAX_POOL_MODE_RETRY_COUNT
  }
  return normalized
}

const loadModelRestrictionFromMapping = (rawMapping?: Record<string, unknown>) => {
  const parsed = splitModelMappingObject(rawMapping)
  allowedModels.value = parsed.allowedModels
  modelMappings.value = parsed.modelMappings
  modelRestrictionMode.value =
    parsed.modelMappings.length > 0 && parsed.allowedModels.length === 0
      ? 'mapping'
      : 'whitelist'
}

const buildModelRestrictionMapping = () =>
  buildModelMappingObject('combined', allowedModels.value, modelMappings.value)

const syncFormFromAccount = (newAccount: Account | null) => {
  if (!newAccount) {
    return
  }
  antigravityMixedChannelConfirmed.value = false
  showMixedChannelWarning.value = false
  mixedChannelWarningDetails.value = null
  mixedChannelWarningRawMessage.value = ''
  mixedChannelWarningAction.value = null
  form.name = newAccount.name
  form.notes = newAccount.notes || ''
  form.proxy_id = newAccount.proxy_id
  form.concurrency = newAccount.concurrency
  form.load_factor = newAccount.load_factor ?? null
  form.priority = newAccount.priority
  form.rate_multiplier = newAccount.rate_multiplier ?? 1
  form.status = (newAccount.status === 'active' || newAccount.status === 'inactive' || newAccount.status === 'error')
    ? newAccount.status
    : 'active'
  form.group_ids = newAccount.group_ids || []
  form.expires_at = newAccount.expires_at ?? null

  // Load intercept warmup requests setting (applies to all account types)
  const credentials = newAccount.credentials as Record<string, unknown> | undefined
  interceptWarmupRequests.value = credentials?.intercept_warmup_requests === true
  accountErrorHandlingRules.value = loadAccountErrorHandlingRules(credentials)
  editOpenAICodexCliUserAgent.value =
    newAccount.platform === 'openai' && typeof credentials?.openai_codex_cli_user_agent === 'string'
      ? credentials.openai_codex_cli_user_agent
      : ''
  openAIResponseTextErrorEnabled.value =
    newAccount.platform === 'openai' && credentials?.openai_response_text_error_enabled === true
  openAIResponseTextErrorKeywordsText.value =
    newAccount.platform === 'openai'
      ? openAIResponseTextErrorKeywordsToText(credentials?.openai_response_text_error_keywords)
      : ''
  autoPauseOnExpired.value = newAccount.auto_pause_on_expired === true
  editVertexProjectId.value = ''
  editVertexClientEmail.value = ''
  editVertexLocation.value = 'us-central1'

  // Load mixed scheduling setting (only for antigravity accounts)
  mixedScheduling.value = false
  allowOverages.value = false
  const extra = newAccount.extra as Record<string, unknown> | undefined
  accountAvailabilitySchedule.value = createAccountAvailabilityScheduleForm(
    readAccountAvailabilityScheduleFromExtra(extra)
  )
  mixedScheduling.value = extra?.mixed_scheduling === true
  allowOverages.value = extra?.allow_overages === true

  // Load OpenAI passthrough toggle (OpenAI OAuth/API Key)
  openaiPassthroughEnabled.value = false
  openAICompactMode.value = 'auto'
  openAIResponsesMode.value = 'auto'
  openAICompactModelMappings.value = []
  openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  codexCLIOnlyEnabled.value = false
  openAICodexCLISimulationEnabled.value = false
  codexImageGenerationBridgeMode.value = 'inherit'
  anthropicPassthroughEnabled.value = false
  anthropicContext1MEnabled.value = false
  webSearchEmulationMode.value = 'default'
  if (newAccount.platform === 'openai' && (newAccount.type === 'oauth' || newAccount.type === 'apikey')) {
    openaiPassthroughEnabled.value = extra?.openai_passthrough === true || extra?.openai_oauth_passthrough === true
    openAICompactMode.value = (extra?.openai_compact_mode as OpenAICompactMode) || 'auto'
    if (newAccount.type === 'apikey') {
      openAIResponsesMode.value = normalizeOpenAIResponsesMode(extra?.openai_responses_mode)
    }
    const codexImageGenerationBridgeValue = typeof extra?.codex_image_generation_bridge === 'boolean'
      ? extra.codex_image_generation_bridge
      : extra?.codex_image_generation_bridge_enabled
    if (codexImageGenerationBridgeValue === true) {
      codexImageGenerationBridgeMode.value = 'enabled'
    } else if (codexImageGenerationBridgeValue === false) {
      codexImageGenerationBridgeMode.value = 'disabled'
    }
    openaiOAuthResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
      modeKey: 'openai_oauth_responses_websockets_v2_mode',
      enabledKey: 'openai_oauth_responses_websockets_v2_enabled',
      fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
      defaultMode: OPENAI_WS_MODE_OFF
    })
    openaiAPIKeyResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
      modeKey: 'openai_apikey_responses_websockets_v2_mode',
      enabledKey: 'openai_apikey_responses_websockets_v2_enabled',
      fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
      defaultMode: OPENAI_WS_MODE_OFF
    })
    if (newAccount.type === 'oauth') {
      codexCLIOnlyEnabled.value = extra?.codex_cli_only === true
    }
    openAICodexCLISimulationEnabled.value = extra?.openai_codex_cli_simulation_enabled === true
    const credentials = newAccount.credentials as Record<string, unknown> | undefined
    if (newAccount.type === 'apikey') {
      upstreamAuthUsername.value = (credentials?.upstream_auth_username as string) || ''
      upstreamCommonRateMultiplier.value = upstreamManualRateMultiplierFrom(credentials, extra)
      upstreamCommonRateGroupName.value = upstreamManualRateGroupNameFrom(credentials, extra)
      upstreamBalanceEndpointPathsText.value = endpointPathsToText(
        credentials?.upstream_balance_endpoint_paths ?? extra?.upstream_balance_endpoint_paths
      )
    }
    const compactMappings = credentials?.compact_model_mapping as Record<string, string> | undefined
    if (compactMappings && typeof compactMappings === 'object') {
      openAICompactModelMappings.value = Object.entries(compactMappings).map(([from, to]) => ({ from, to }))
    }
  }
  if (newAccount.platform === 'anthropic' && newAccount.type === 'apikey') {
    anthropicPassthroughEnabled.value = extra?.anthropic_passthrough === true
    anthropicContext1MEnabled.value = extra?.anthropic_context_1m_enabled === true
    // 三态：string "default"/"enabled"/"disabled"，向后兼容旧 bool
    const wsVal = extra?.web_search_emulation
    if (wsVal === 'enabled' || wsVal === 'disabled') {
      webSearchEmulationMode.value = wsVal
    } else if (wsVal === true) {
      webSearchEmulationMode.value = 'enabled'
    } else {
      webSearchEmulationMode.value = 'default'
    }
  }
  if ((newAccount.platform === 'openai' || newAccount.platform === 'anthropic') && newAccount.type === 'apikey') {
    const credentials = newAccount.credentials as Record<string, unknown> | undefined
    upstreamAuthUsername.value = (credentials?.upstream_auth_username as string) || ''
    upstreamCommonRateMultiplier.value = upstreamManualRateMultiplierFrom(credentials, extra)
    upstreamCommonRateGroupName.value = upstreamManualRateGroupNameFrom(credentials, extra)
    upstreamBalanceEndpointPathsText.value = endpointPathsToText(
      credentials?.upstream_balance_endpoint_paths ?? extra?.upstream_balance_endpoint_paths
    )
  }

  // Load quota limit for apikey/bedrock accounts (bedrock quota is also loaded in its own branch above)
  if (newAccount.type === 'apikey' || newAccount.type === 'bedrock') {
    const quotaVal = extra?.quota_limit as number | undefined
    editQuotaLimit.value = (quotaVal && quotaVal > 0) ? quotaVal : null
    const dailyVal = extra?.quota_daily_limit as number | undefined
    editQuotaDailyLimit.value = (dailyVal && dailyVal > 0) ? dailyVal : null
    const weeklyVal = extra?.quota_weekly_limit as number | undefined
    editQuotaWeeklyLimit.value = (weeklyVal && weeklyVal > 0) ? weeklyVal : null
    // Load quota reset mode config
    editDailyResetMode.value = (extra?.quota_daily_reset_mode as 'rolling' | 'fixed') || null
    editDailyResetHour.value = (extra?.quota_daily_reset_hour as number) ?? null
    editWeeklyResetMode.value = (extra?.quota_weekly_reset_mode as 'rolling' | 'fixed') || null
    editWeeklyResetDay.value = (extra?.quota_weekly_reset_day as number) ?? null
    editWeeklyResetHour.value = (extra?.quota_weekly_reset_hour as number) ?? null
    editResetTimezone.value = (extra?.quota_reset_timezone as string) || null
    // Load quota notify config
    loadQuotaNotifyFromExtra(extra)
  } else {
    editQuotaLimit.value = null
    editQuotaDailyLimit.value = null
    editQuotaWeeklyLimit.value = null
    editDailyResetMode.value = null
    editDailyResetHour.value = null
    editWeeklyResetMode.value = null
    editWeeklyResetDay.value = null
    editWeeklyResetHour.value = null
    editResetTimezone.value = null
    resetQuotaNotify()
  }

  // Load antigravity model mapping (Antigravity 只支持映射模式)
  if (newAccount.platform === 'antigravity') {
    const credentials = newAccount.credentials as Record<string, unknown> | undefined

    // Antigravity 始终使用映射模式
    antigravityModelRestrictionMode.value = 'mapping'
    antigravityWhitelistModels.value = []

    // 从 model_mapping 读取映射配置
    const rawAgMapping = credentials?.model_mapping as Record<string, string> | undefined
    if (rawAgMapping && typeof rawAgMapping === 'object') {
      const entries = Object.entries(rawAgMapping)
      // 无论是白名单样式(key===value)还是真正的映射，都统一转换为映射列表
      antigravityModelMappings.value = entries.map(([from, to]) => ({ from, to }))
    } else {
      // 兼容旧数据：从 model_whitelist 读取，转换为映射格式
      const rawWhitelist = credentials?.model_whitelist
      if (Array.isArray(rawWhitelist) && rawWhitelist.length > 0) {
        antigravityModelMappings.value = rawWhitelist
          .map((v) => String(v).trim())
          .filter((v) => v.length > 0)
          .map((m) => ({ from: m, to: m }))
      } else {
        antigravityModelMappings.value = []
      }
    }
  } else {
    antigravityModelRestrictionMode.value = 'mapping'
    antigravityWhitelistModels.value = []
    antigravityModelMappings.value = []
  }

  // Load quota control settings (Anthropic OAuth/SetupToken only)
  loadQuotaControlSettings(newAccount)

  loadAccountErrorHandlingConfig(credentials)

  // Initialize API Key fields for apikey type
  if (newAccount.type === 'apikey' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    const platformDefaultUrl =
      newAccount.platform === 'openai'
        ? 'https://api.openai.com'
        : newAccount.platform === 'gemini'
          ? 'https://generativelanguage.googleapis.com'
          : 'https://api.anthropic.com'
    editBaseUrl.value = (credentials.base_url as string) || platformDefaultUrl
    const requestBaseURLs = parseBaseURLsText(
      Array.isArray(credentials.request_base_urls)
        ? (credentials.request_base_urls as unknown[]).join('\n')
        : typeof credentials.request_base_urls === 'string'
          ? credentials.request_base_urls
          : ''
    )
    editRequestBaseUrlsText.value = requestBaseURLs.length > 0 ? requestBaseURLs.join('\n') : editBaseUrl.value
    editBalanceBaseUrl.value = (credentials.balance_base_url as string) || ''
    editClaudeCliVersion.value = typeof credentials.claude_cli_version === 'string'
      ? credentials.claude_cli_version
      : ''

    // Load model mappings and detect mode
    loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)

    // Load pool mode
    poolModeEnabled.value = credentials.pool_mode === true
    poolModeRetryCount.value = normalizePoolModeRetryCount(
      Number(credentials.pool_mode_retry_count ?? DEFAULT_POOL_MODE_RETRY_COUNT)
    )

  } else if (newAccount.type === 'bedrock' && newAccount.credentials) {
    const bedrockCreds = newAccount.credentials as Record<string, unknown>
    const authMode = (bedrockCreds.auth_mode as string) || 'sigv4'
    editBedrockRegion.value = (bedrockCreds.aws_region as string) || ''
    editBedrockForceGlobal.value = (bedrockCreds.aws_force_global as string) === 'true'

    if (authMode === 'apikey') {
      editBedrockApiKeyValue.value = ''
    } else {
      editBedrockAccessKeyId.value = (bedrockCreds.aws_access_key_id as string) || ''
      editBedrockSecretAccessKey.value = ''
      editBedrockSessionToken.value = ''
    }

    // Load pool mode for bedrock
    poolModeEnabled.value = bedrockCreds.pool_mode === true
    const retryCount = bedrockCreds.pool_mode_retry_count
    poolModeRetryCount.value = (typeof retryCount === 'number' && retryCount >= 0) ? retryCount : DEFAULT_POOL_MODE_RETRY_COUNT

    // Load quota limits for bedrock
    const bedrockExtra = (newAccount.extra as Record<string, unknown>) || {}
    editQuotaLimit.value = typeof bedrockExtra.quota_limit === 'number' ? bedrockExtra.quota_limit : null
    editQuotaDailyLimit.value = typeof bedrockExtra.quota_daily_limit === 'number' ? bedrockExtra.quota_daily_limit : null
    editQuotaWeeklyLimit.value = typeof bedrockExtra.quota_weekly_limit === 'number' ? bedrockExtra.quota_weekly_limit : null
    // Load quota notify for bedrock
    loadQuotaNotifyFromExtra(bedrockExtra)

    // Load model mappings for bedrock
    loadModelRestrictionFromMapping(bedrockCreds.model_mapping as Record<string, unknown> | undefined)
  } else if (newAccount.type === 'upstream' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    const extra = (newAccount.extra as Record<string, unknown>) || {}
    editBaseUrl.value = (credentials.base_url as string) || ''
    upstreamAuthUsername.value = (credentials.upstream_auth_username as string) || ''
    upstreamCommonRateMultiplier.value = upstreamManualRateMultiplierFrom(credentials, extra)
    upstreamCommonRateGroupName.value = upstreamManualRateGroupNameFrom(credentials, extra)
    upstreamBalanceEndpointPathsText.value = endpointPathsToText(
      credentials.upstream_balance_endpoint_paths ?? extra.upstream_balance_endpoint_paths
    )
  } else if ((newAccount.platform === 'gemini' || newAccount.platform === 'anthropic') && newAccount.type === 'service_account' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    editVertexProjectId.value = (credentials.project_id as string) || ''
    editVertexClientEmail.value = (credentials.client_email as string) || ''
    editVertexLocation.value = (credentials.location as string) || (credentials.vertex_location as string) || 'us-central1'

    // Load model mappings for service_account
    loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)
  } else {
    const platformDefaultUrl =
      newAccount.platform === 'openai'
        ? 'https://api.openai.com'
        : newAccount.platform === 'gemini'
          ? 'https://generativelanguage.googleapis.com'
          : 'https://api.anthropic.com'
    editBaseUrl.value = platformDefaultUrl

    // Load model mappings for OpenAI OAuth accounts
    if (newAccount.platform === 'openai' && newAccount.credentials) {
      const oauthCredentials = newAccount.credentials as Record<string, unknown>
      loadModelRestrictionFromMapping(oauthCredentials.model_mapping as Record<string, unknown> | undefined)
    } else {
      modelRestrictionMode.value = 'whitelist'
      modelMappings.value = []
      allowedModels.value = []
    }
    poolModeEnabled.value = false
    poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
  }
  editApiKey.value = ''
  editApiKeysText.value = ''
  if (!(newAccount.type === 'apikey' && (newAccount.platform === 'anthropic' || newAccount.platform === 'antigravity'))) {
    editClaudeCliVersion.value = ''
  }
  apiKeysEditMode.value = 'append'
  upstreamAuthPassword.value = ''
}

async function loadTLSProfiles() {
  try {
    const profiles = await adminAPI.tlsFingerprintProfiles.list()
    tlsFingerprintProfiles.value = profiles.map(p => ({ id: p.id, name: p.name }))
  } catch {
    tlsFingerprintProfiles.value = []
  }
}

watch(
  [() => props.show, () => props.account],
  ([show, newAccount], [wasShow, previousAccount]) => {
    if (!show || !newAccount) {
      return
    }
    if (!wasShow || newAccount !== previousAccount) {
      syncFormFromAccount(newAccount)
      loadTLSProfiles()
    }
  },
  { immediate: true }
)

const syncAntigravityUpstreamModels = async () => {
  if (!props.account?.id || isSyncingAntigravityUpstream.value) return

  isSyncingAntigravityUpstream.value = true
  try {
    const result = await adminAPI.accounts.syncUpstreamModels(props.account.id)
    const upstreamModels = result.models.map((model) => model.trim()).filter(Boolean)
    if (upstreamModels.length === 0) {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsEmpty'))
      return
    }

    let addedCount = 0
    for (const model of upstreamModels) {
      const exists = antigravityModelMappings.value.some((mapping) => mapping.from === model)
      if (!exists) {
        antigravityModelMappings.value.push({ from: model, to: model })
        addedCount += 1
      }
    }

    if (addedCount > 0) {
      appStore.showSuccess(t('admin.accounts.syncUpstreamModelsSuccess', { count: addedCount, total: upstreamModels.length }))
    } else {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsNoChanges', { count: upstreamModels.length }))
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : t('admin.accounts.syncUpstreamModelsFailed')
    appStore.showError(t('admin.accounts.syncUpstreamModelsError', { message }))
  } finally {
    isSyncingAntigravityUpstream.value = false
  }
}

const parseStatusCodesText = (value: string) => {
  const seen = new Set<number>()
  const codes: number[] = []
  value
    .split(/[,;，；\s]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .forEach((item) => {
      if (!/^\d+$/.test(item)) return
      const code = Number(item)
      if (!Number.isInteger(code) || code < 100 || code > 599 || (code >= 200 && code <= 299) || seen.has(code)) return
      seen.add(code)
      codes.push(code)
    })
  return codes
}

const writeLegacyErrorHandlingCompat = (credentials: Record<string, unknown>, rules: AccountErrorHandlingRuleForm[]) => {
  const customErrorCodes = Array.from(new Set(
    rules
      .filter((rule) => rule.enabled !== false && rule.action === 'error_disabled')
      .flatMap((rule) => parseStatusCodesText(rule.status_codes))
  ))
  if (customErrorCodes.length > 0) {
    credentials.custom_error_codes_enabled = true
    credentials.custom_error_codes = customErrorCodes
  } else {
    delete credentials.custom_error_codes_enabled
    delete credentials.custom_error_codes
  }

  const tempRules = rules
    .filter((rule) => rule.enabled !== false && rule.action === 'temp_unschedulable')
    .map((rule) => {
      const errorCode = parseStatusCodesText(rule.status_codes)[0] ?? null
      const duration = Number(rule.durationMinutes)
      const keywords = splitTempUnschedKeywords(rule.keywords)
      if (errorCode === null || !Number.isFinite(duration) || duration <= 0 || keywords.length === 0) {
        return null
      }
      return {
        error_code: errorCode,
        keywords,
        duration_minutes: Math.trunc(duration),
        description: rule.description.trim()
      }
    })
    .filter((rule): rule is { error_code: number; keywords: string[]; duration_minutes: number; description: string } => rule !== null)

  if (tempRules.length > 0) {
    credentials.temp_unschedulable_enabled = true
    credentials.temp_unschedulable_rules = tempRules
  } else {
    delete credentials.temp_unschedulable_enabled
    delete credentials.temp_unschedulable_rules
  }
}

const applyAccountErrorHandlingConfig = (credentials: Record<string, unknown>) => {
  try {
    writeAccountErrorHandlingToCredentials(credentials, accountErrorHandlingRules.value)
    writeLegacyErrorHandlingCompat(credentials, accountErrorHandlingRules.value)
  } catch (error) {
    const message = error instanceof Error ? error.message : '错误处理策略规则无效'
    appStore.showError(message)
    return false
  }
  return true
}

const applyOpenAIResponseTextErrorConfig = (credentials: Record<string, unknown>) => {
  const keywords = parseOpenAIResponseTextErrorKeywordsText(openAIResponseTextErrorKeywordsText.value)
  if (openAIResponseTextErrorEnabled.value && keywords.length > 0) {
    credentials.openai_response_text_error_enabled = true
    credentials.openai_response_text_error_keywords = keywords
  } else {
    delete credentials.openai_response_text_error_enabled
    delete credentials.openai_response_text_error_keywords
  }
}

const applyUpstreamAuthCredentials = (credentials: Record<string, unknown>) => {
  if (upstreamAuthUsername.value.trim()) {
    credentials.upstream_auth_username = upstreamAuthUsername.value.trim()
  } else {
    delete credentials.upstream_auth_username
  }
  if (upstreamAuthPassword.value.trim()) {
    credentials.upstream_auth_password = upstreamAuthPassword.value.trim()
  }
  if (upstreamCommonRateMultiplier.value != null && upstreamCommonRateMultiplier.value > 0) {
    credentials.upstream_manual_rate_multiplier = upstreamCommonRateMultiplier.value
  } else {
    delete credentials.upstream_manual_rate_multiplier
  }
  if (upstreamCommonRateGroupName.value.trim()) {
    credentials.upstream_manual_rate_group_name = upstreamCommonRateGroupName.value.trim()
  } else {
    delete credentials.upstream_manual_rate_group_name
  }
  delete credentials.upstream_common_rate_multiplier
  delete credentials.upstream_common_rate_group_name
  credentials.upstream_balance_endpoint_paths = parseEndpointPathsText(upstreamBalanceEndpointPathsText.value)
}

function loadAccountErrorHandlingConfig(credentials?: Record<string, unknown>) {
  accountErrorHandlingRules.value = loadAccountErrorHandlingRules(credentials)
}

// Load quota control settings from account (Anthropic OAuth/SetupToken only)
function loadQuotaControlSettings(account: Account) {
  // Reset all quota control state first
  windowCostEnabled.value = false
  windowCostLimit.value = null
  windowCostStickyReserve.value = null
  sessionLimitEnabled.value = false
  maxSessions.value = null
  sessionIdleTimeout.value = null
  rpmLimitEnabled.value = false
  baseRpm.value = null
  rpmStrategy.value = 'tiered'
  rpmStickyBuffer.value = null
  userMsgQueueMode.value = ''
  tlsFingerprintEnabled.value = false
  tlsFingerprintProfileId.value = null
  sessionIdMaskingEnabled.value = false
  cacheTTLOverrideEnabled.value = false
  cacheTTLOverrideTarget.value = '5m'
  customBaseUrlEnabled.value = false
  customBaseUrl.value = ''

  // Remaining quota control settings only apply to Anthropic accounts
  if (account.platform !== 'anthropic') {
    return
  }

  // Window cost / session limit only apply to Anthropic OAuth/SetupToken accounts
  if (account.type !== 'oauth' && account.type !== 'setup-token') {
    return
  }

  // Load from extra field (via backend DTO fields)
  if (account.window_cost_limit != null && account.window_cost_limit > 0) {
    windowCostEnabled.value = true
    windowCostLimit.value = account.window_cost_limit
    windowCostStickyReserve.value = account.window_cost_sticky_reserve ?? 10
  }

  if (account.max_sessions != null && account.max_sessions > 0) {
    sessionLimitEnabled.value = true
    maxSessions.value = account.max_sessions
    sessionIdleTimeout.value = account.session_idle_timeout_minutes ?? 5
  }

  // RPM limit
  if (account.base_rpm != null && account.base_rpm > 0) {
    rpmLimitEnabled.value = true
    baseRpm.value = account.base_rpm
    rpmStrategy.value = (account.rpm_strategy as 'tiered' | 'sticky_exempt') || 'tiered'
    rpmStickyBuffer.value = account.rpm_sticky_buffer ?? null
  }

  // UMQ mode（独立于 RPM 加载，防止编辑无 RPM 账号时丢失已有配置）
  userMsgQueueMode.value = account.user_msg_queue_mode ?? ''

  // Load TLS fingerprint setting
  if (account.enable_tls_fingerprint === true) {
    tlsFingerprintEnabled.value = true
  }
  tlsFingerprintProfileId.value = account.tls_fingerprint_profile_id ?? null

  // Load session ID masking setting
  if (account.session_id_masking_enabled === true) {
    sessionIdMaskingEnabled.value = true
  }

  // Load cache TTL override setting
  if (account.cache_ttl_override_enabled === true) {
    cacheTTLOverrideEnabled.value = true
    cacheTTLOverrideTarget.value = account.cache_ttl_override_target || '5m'
  }

  // Load custom base URL setting
  if (account.custom_base_url_enabled === true) {
    customBaseUrlEnabled.value = true
    customBaseUrl.value = account.custom_base_url || ''
  }
}

const splitTempUnschedKeywords = (value: string) => {
  return value
    .split(/[,;]/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

const needsMixedChannelCheck = () => props.account?.platform === 'antigravity' || props.account?.platform === 'anthropic'

const buildMixedChannelDetails = (resp?: CheckMixedChannelResponse) => {
  const details = resp?.details
  if (!details) {
    return null
  }
  return {
    groupName: details.group_name || 'Unknown',
    currentPlatform: details.current_platform || 'Unknown',
    otherPlatform: details.other_platform || 'Unknown'
  }
}

const clearMixedChannelDialog = () => {
  showMixedChannelWarning.value = false
  mixedChannelWarningDetails.value = null
  mixedChannelWarningRawMessage.value = ''
  mixedChannelWarningAction.value = null
}

const openMixedChannelDialog = (opts: {
  response?: CheckMixedChannelResponse
  message?: string
  onConfirm: () => Promise<void>
}) => {
  mixedChannelWarningDetails.value = buildMixedChannelDetails(opts.response)
  mixedChannelWarningRawMessage.value =
    opts.message || opts.response?.message || t('admin.accounts.failedToUpdate')
  mixedChannelWarningAction.value = opts.onConfirm
  showMixedChannelWarning.value = true
}

const withAntigravityConfirmFlag = (payload: Record<string, unknown>) => {
  if (needsMixedChannelCheck() && antigravityMixedChannelConfirmed.value) {
    return {
      ...payload,
      confirm_mixed_channel_risk: true
    }
  }
  const cloned = { ...payload }
  delete cloned.confirm_mixed_channel_risk
  return cloned
}

const ensureAntigravityMixedChannelConfirmed = async (onConfirm: () => Promise<void>): Promise<boolean> => {
  if (!needsMixedChannelCheck()) {
    return true
  }
  if (antigravityMixedChannelConfirmed.value) {
    return true
  }
  if (!props.account) {
    return false
  }

  try {
    const result = await adminAPI.accounts.checkMixedChannelRisk({
      platform: props.account.platform,
      group_ids: form.group_ids,
      account_id: props.account.id
    })
    if (!result.has_risk) {
      return true
    }
    openMixedChannelDialog({
      response: result,
      onConfirm: async () => {
        antigravityMixedChannelConfirmed.value = true
        await onConfirm()
      }
    })
    return false
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
    return false
  }
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// Methods
const handleClose = () => {
  antigravityMixedChannelConfirmed.value = false
  clearMixedChannelDialog()
  emit('close')
}

// 按后端返回的非敏感指纹删除单个已保存 Key，并把更新后的账号状态交给父组件刷新。
const handleDeleteAPIKey = async (fingerprint: string) => {
  if (!props.account || !fingerprint || deletingApiKeyFingerprint.value || restoringApiKeyFingerprint.value) {
    return
  }
  deletingApiKeyFingerprint.value = fingerprint
  try {
    const updatedAccount = await adminAPI.accounts.deleteAccountAPIKey(props.account.id, fingerprint)
    appStore.showSuccess(t('admin.accounts.accountUpdated'))
    emit('updated', updatedAccount)
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  } finally {
    deletingApiKeyFingerprint.value = null
  }
}

// 按后端返回的非敏感指纹恢复单个已停用 Key，并把更新后的账号状态交给父组件刷新。
const handleRestoreAPIKeyState = async (fingerprint: string) => {
  if (!props.account || !fingerprint || restoringApiKeyFingerprint.value || deletingApiKeyFingerprint.value) {
    return
  }
  restoringApiKeyFingerprint.value = fingerprint
  try {
    const updatedAccount = await adminAPI.accounts.restoreAccountAPIKeyState(props.account.id, fingerprint)
    appStore.showSuccess(t('admin.accounts.accountUpdated'))
    emit('updated', updatedAccount)
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  } finally {
    restoringApiKeyFingerprint.value = null
  }
}

const submitUpdateAccount = async (accountID: number, updatePayload: Record<string, unknown>) => {
  submitting.value = true
  try {
    const updatedAccount = await adminAPI.accounts.update(accountID, withAntigravityConfirmFlag(updatePayload))
    appStore.showSuccess(t('admin.accounts.accountUpdated'))
    emit('updated', updatedAccount)
    handleClose()
  } catch (error: any) {
    if (error.status === 409 && error.error === 'mixed_channel_warning' && needsMixedChannelCheck()) {
      openMixedChannelDialog({
        message: error.message,
        onConfirm: async () => {
          antigravityMixedChannelConfirmed.value = true
          await submitUpdateAccount(accountID, updatePayload)
        }
      })
      return
    }
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  } finally {
    submitting.value = false
  }
}

const handleSubmit = async () => {
  if (!props.account) return
  const accountID = props.account.id

  if (form.status !== 'active' && form.status !== 'inactive' && form.status !== 'error') {
    appStore.showError(t('admin.accounts.pleaseSelectStatus'))
    return
  }

  const updatePayload: Record<string, unknown> = { ...form }
  try {
    // 后端期望 proxy_id: 0 表示清除代理，而不是 null
    if (updatePayload.proxy_id === null) {
      updatePayload.proxy_id = 0
    }
    if (form.expires_at === null) {
      updatePayload.expires_at = 0
    }
    // load_factor: 空值/NaN/0/负数 时发送 0（后端约定 <= 0 = 清除）
    const lf = form.load_factor
    if (lf == null || Number.isNaN(lf) || lf <= 0) {
      updatePayload.load_factor = 0
    }
    updatePayload.auto_pause_on_expired = autoPauseOnExpired.value

    // For apikey type, handle credentials update
    if (props.account.type === 'apikey') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const requestBaseUrls = props.account.platform === 'openai' || props.account.platform === 'anthropic'
        ? parseBaseURLsText([editBaseUrl.value, editRequestBaseUrlsText.value].filter(Boolean).join('\n'))
        : []
      const newBaseUrl = requestBaseUrls[0] || editBaseUrl.value.trim() || defaultBaseUrl.value
      const shouldApplyModelMapping = !(props.account.platform === 'openai' && openaiPassthroughEnabled.value)

      // Always update credentials for apikey type to handle model mapping changes
      const newCredentials: Record<string, unknown> = {
        ...currentCredentials,
        base_url: newBaseUrl
      }
      if (props.account.platform === 'openai' || props.account.platform === 'anthropic') {
        newCredentials.request_base_urls = requestBaseUrls.length > 0 ? requestBaseUrls : [newBaseUrl]
      }
      if (props.account.platform === 'anthropic' || props.account.platform === 'antigravity') {
        const normalizedClaudeCliVersion = editClaudeCliVersion.value.trim()
        if (normalizedClaudeCliVersion) {
          newCredentials.claude_cli_version = normalizedClaudeCliVersion
        } else {
          delete newCredentials.claude_cli_version
        }
      }
      if (props.account.platform === 'openai') {
        const normalizedBalanceBaseURL = parseBaseURLsText(editBalanceBaseUrl.value)[0]
        if (normalizedBalanceBaseURL) {
          newCredentials.balance_base_url = normalizedBalanceBaseURL
        } else {
          delete newCredentials.balance_base_url
        }
      }

      // Handle API key. 后端响应已脱敏，追加模式通过 api_keys_append 让服务端用已保存明文合并。
      const apiKeys = parseAPIKeysText(editApiKeysText.value)
      const hasExistingApiKey =
        Boolean(props.account.credentials_status?.has_api_key) ||
        Boolean(props.account.credentials_status?.has_api_keys) ||
        existingApiKeyItems.value.length > 0 ||
        Boolean(currentCredentials.api_key) ||
        Array.isArray(currentCredentials.api_keys)
      if (apiKeys.length > 0) {
        if (apiKeysEditMode.value === 'append' && hasExistingApiKey) {
          newCredentials.api_keys_append = apiKeys
        } else {
          newCredentials.api_keys = apiKeys
          delete newCredentials.api_key
        }
      } else if (editApiKey.value.trim()) {
        newCredentials.api_key = editApiKey.value.trim()
        delete newCredentials.api_keys
        delete newCredentials.api_keys_append
      } else if (!hasExistingApiKey) {
        appStore.showError(t('admin.accounts.apiKeyIsRequired'))
        return
      }

      // Add model mapping if configured（OpenAI 开启自动透传时保留现有映射，不再编辑）
      if (shouldApplyModelMapping) {
        const modelMapping = buildModelRestrictionMapping()
        if (modelMapping) {
          newCredentials.model_mapping = modelMapping
        } else {
          delete newCredentials.model_mapping
        }
      } else if (currentCredentials.model_mapping) {
        newCredentials.model_mapping = currentCredentials.model_mapping
      }
      if (props.account.platform === 'openai') {
        const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
        if (compactModelMapping) {
          newCredentials.compact_model_mapping = compactModelMapping
        } else {
          delete newCredentials.compact_model_mapping
        }
      }
      if (props.account.platform === 'openai' || props.account.platform === 'anthropic') {
        applyUpstreamAuthCredentials(newCredentials)
      }

      // Add pool mode if enabled
      if (poolModeEnabled.value) {
        newCredentials.pool_mode = true
        newCredentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
      } else {
        delete newCredentials.pool_mode
        delete newCredentials.pool_mode_retry_count
      }

      // Add intercept warmup requests setting
      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
      if (!applyAccountErrorHandlingConfig(newCredentials)) {
        return
      }

      updatePayload.credentials = newCredentials
    } else if (props.account.type === 'upstream') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      newCredentials.base_url = editBaseUrl.value.trim()

      if (editApiKey.value.trim()) {
        newCredentials.api_key = editApiKey.value.trim()
      }
      applyUpstreamAuthCredentials(newCredentials)

      // Add intercept warmup requests setting
      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      if (!applyAccountErrorHandlingConfig(newCredentials)) {
        return
      }

      updatePayload.credentials = newCredentials
    } else if ((props.account.platform === 'gemini' || props.account.platform === 'anthropic') && props.account.type === 'service_account') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      if (!editVertexProjectId.value.trim()) {
        appStore.showError(t('admin.accounts.vertexSaJsonMissingProjectId'))
        return
      }
      if (!editVertexClientEmail.value.trim()) {
        appStore.showError(t('admin.accounts.vertexSaJsonMissingClientEmail'))
        return
      }
      if (!editVertexLocation.value.trim()) {
        appStore.showError(t('admin.accounts.vertexLocationRequired'))
        return
      }

      // SA JSON 已脱敏不再随 credentials 返回，存在性优先读 credentials_status。
      // 若后端尚未升级（无 credentials_status），回退读旧结构 service_account_json / service_account。
      const credentialsStatus = props.account.credentials_status
      const hasExistingServiceAccountJson = credentialsStatus
        ? Boolean(
            credentialsStatus.has_service_account_json || credentialsStatus.has_service_account
          )
        : Boolean(currentCredentials.service_account_json || currentCredentials.service_account)
      if (!hasExistingServiceAccountJson) {
        appStore.showError(t('admin.accounts.vertexSaJsonRequired'))
        return
      }
      newCredentials.project_id = editVertexProjectId.value.trim()
      newCredentials.client_email = editVertexClientEmail.value.trim()
      newCredentials.location = editVertexLocation.value.trim()
      newCredentials.tier_id = 'vertex'

      // Add model mapping if configured
      const modelMapping = buildModelRestrictionMapping()
      if (modelMapping) {
        newCredentials.model_mapping = modelMapping
      } else {
        delete newCredentials.model_mapping
      }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
      if (!applyAccountErrorHandlingConfig(newCredentials)) {
        return
      }

      updatePayload.credentials = newCredentials
    } else if (props.account.type === 'bedrock') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      newCredentials.aws_region = editBedrockRegion.value.trim()
      if (editBedrockForceGlobal.value) {
        newCredentials.aws_force_global = 'true'
      } else {
        delete newCredentials.aws_force_global
      }

      if (isBedrockAPIKeyMode.value) {
        // API Key mode: only update api_key if user provided new value
        if (editBedrockApiKeyValue.value.trim()) {
          newCredentials.api_key = editBedrockApiKeyValue.value.trim()
        }
      } else {
        // SigV4 mode
        newCredentials.aws_access_key_id = editBedrockAccessKeyId.value.trim()
        if (editBedrockSecretAccessKey.value.trim()) {
          newCredentials.aws_secret_access_key = editBedrockSecretAccessKey.value.trim()
        }
        if (editBedrockSessionToken.value.trim()) {
          newCredentials.aws_session_token = editBedrockSessionToken.value.trim()
        }
      }

      // Pool mode
      if (poolModeEnabled.value) {
        newCredentials.pool_mode = true
        newCredentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
      } else {
        delete newCredentials.pool_mode
        delete newCredentials.pool_mode_retry_count
      }

      // Model mapping
      const modelMapping = buildModelRestrictionMapping()
      if (modelMapping) {
        newCredentials.model_mapping = modelMapping
      } else {
        delete newCredentials.model_mapping
      }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
      if (!applyAccountErrorHandlingConfig(newCredentials)) {
        return
      }

      updatePayload.credentials = newCredentials
    } else {
      // For oauth/setup-token types, only update intercept_warmup_requests if changed
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
      if (!applyAccountErrorHandlingConfig(newCredentials)) {
        return
      }

      updatePayload.credentials = newCredentials
    }

    // OpenAI OAuth: persist model mapping to credentials
    if (props.account.platform === 'openai' && props.account.type === 'oauth') {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      const shouldApplyModelMapping = !openaiPassthroughEnabled.value

      if (shouldApplyModelMapping) {
        const modelMapping = buildModelRestrictionMapping()
        if (modelMapping) {
          newCredentials.model_mapping = modelMapping
        } else {
          delete newCredentials.model_mapping
        }
      } else if (currentCredentials.model_mapping) {
        // 透传模式保留现有映射
        newCredentials.model_mapping = currentCredentials.model_mapping
      }
      const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
      if (compactModelMapping) {
        newCredentials.compact_model_mapping = compactModelMapping
      } else {
        delete newCredentials.compact_model_mapping
      }

      updatePayload.credentials = newCredentials
    }

    // Antigravity: persist model mapping to credentials (applies to all antigravity types)
    // Antigravity 只支持映射模式
    if (props.account.platform === 'antigravity') {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      // 移除旧字段
      delete newCredentials.model_whitelist
      delete newCredentials.model_mapping

      // 只使用映射模式
      const antigravityModelMapping = buildModelMappingObject(
        'mapping',
        [],
        antigravityModelMappings.value
      )
      if (antigravityModelMapping) {
        newCredentials.model_mapping = antigravityModelMapping
      }

      updatePayload.credentials = newCredentials
    }

    // For antigravity accounts, handle mixed_scheduling and allow_overages in extra
    if (props.account.platform === 'antigravity') {
      const currentExtra = (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (mixedScheduling.value) {
        newExtra.mixed_scheduling = true
      } else {
        delete newExtra.mixed_scheduling
      }
      if (allowOverages.value) {
        newExtra.allow_overages = true
      } else {
        delete newExtra.allow_overages
      }
      updatePayload.extra = newExtra
    }

    // For Anthropic OAuth/SetupToken accounts, handle quota control settings in extra
    if (props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }

      // Window cost limit settings
      if (windowCostEnabled.value && windowCostLimit.value != null && windowCostLimit.value > 0) {
        newExtra.window_cost_limit = windowCostLimit.value
        newExtra.window_cost_sticky_reserve = windowCostStickyReserve.value ?? 10
      } else {
        delete newExtra.window_cost_limit
        delete newExtra.window_cost_sticky_reserve
      }

      // Session limit settings
      if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
        newExtra.max_sessions = maxSessions.value
        newExtra.session_idle_timeout_minutes = sessionIdleTimeout.value ?? 5
      } else {
        delete newExtra.max_sessions
        delete newExtra.session_idle_timeout_minutes
      }

      // RPM limit settings
      if (rpmLimitEnabled.value) {
        const DEFAULT_BASE_RPM = 15
        newExtra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
          ? baseRpm.value
          : DEFAULT_BASE_RPM
        newExtra.rpm_strategy = rpmStrategy.value
        if (rpmStickyBuffer.value != null && rpmStickyBuffer.value > 0) {
          newExtra.rpm_sticky_buffer = rpmStickyBuffer.value
        } else {
          delete newExtra.rpm_sticky_buffer
        }
      } else {
        delete newExtra.base_rpm
        delete newExtra.rpm_strategy
        delete newExtra.rpm_sticky_buffer
      }

      // UMQ mode（独立于 RPM 保存）
      if (userMsgQueueMode.value) {
        newExtra.user_msg_queue_mode = userMsgQueueMode.value
      } else {
        delete newExtra.user_msg_queue_mode
      }
      delete newExtra.user_msg_queue_enabled  // 清理旧字段

      // TLS fingerprint setting
      if (tlsFingerprintEnabled.value) {
        newExtra.enable_tls_fingerprint = true
        if (tlsFingerprintProfileId.value) {
          newExtra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
        } else {
          delete newExtra.tls_fingerprint_profile_id
        }
      } else {
        delete newExtra.enable_tls_fingerprint
        delete newExtra.tls_fingerprint_profile_id
      }

      // Session ID masking setting
      if (sessionIdMaskingEnabled.value) {
        newExtra.session_id_masking_enabled = true
      } else {
        delete newExtra.session_id_masking_enabled
      }

      // Cache TTL override setting
      if (cacheTTLOverrideEnabled.value) {
        newExtra.cache_ttl_override_enabled = true
        newExtra.cache_ttl_override_target = cacheTTLOverrideTarget.value
      } else {
        delete newExtra.cache_ttl_override_enabled
        delete newExtra.cache_ttl_override_target
      }

      // Custom base URL relay setting
      if (customBaseUrlEnabled.value && customBaseUrl.value.trim()) {
        newExtra.custom_base_url_enabled = true
        newExtra.custom_base_url = customBaseUrl.value.trim()
      } else {
        delete newExtra.custom_base_url_enabled
        delete newExtra.custom_base_url
      }

      updatePayload.extra = newExtra
    }

    // For Anthropic API Key accounts, handle passthrough mode + web search emulation in extra
    if (props.account.platform === 'anthropic' && props.account.type === 'apikey') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (anthropicPassthroughEnabled.value) {
        newExtra.anthropic_passthrough = true
      } else {
        delete newExtra.anthropic_passthrough
      }
      if (anthropicContext1MEnabled.value) {
        newExtra.anthropic_context_1m_enabled = true
      } else {
        delete newExtra.anthropic_context_1m_enabled
      }
      if (webSearchEmulationMode.value === 'default') {
        delete newExtra.web_search_emulation
      } else {
        newExtra.web_search_emulation = webSearchEmulationMode.value
      }
      updatePayload.extra = newExtra
    }

    // For OpenAI OAuth/API Key accounts, handle passthrough mode in extra
    if (props.account.platform === 'openai' && (props.account.type === 'oauth' || props.account.type === 'apikey')) {
      const currentExtra = (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      const hadCodexCLIOnlyEnabled = currentExtra.codex_cli_only === true
      const hadCodexCLISimulationEnabled = currentExtra.openai_codex_cli_simulation_enabled === true
      if (props.account.type === 'oauth') {
        newExtra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
        newExtra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiOAuthResponsesWebSocketV2Mode.value)
      } else if (props.account.type === 'apikey') {
        newExtra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
        newExtra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiAPIKeyResponsesWebSocketV2Mode.value)
      }
      delete newExtra.responses_websockets_v2_enabled
      delete newExtra.openai_ws_enabled
      if (openaiPassthroughEnabled.value) {
        newExtra.openai_passthrough = true
      } else {
        delete newExtra.openai_passthrough
        delete newExtra.openai_oauth_passthrough
      }
      if (openAICompactMode.value === 'auto') {
        delete newExtra.openai_compact_mode
      } else {
        newExtra.openai_compact_mode = openAICompactMode.value
      }
      if (props.account.type === 'apikey') {
        if (openAIResponsesMode.value === 'auto') {
          delete newExtra.openai_responses_mode
        } else {
          newExtra.openai_responses_mode = openAIResponsesMode.value
        }
      }

      delete newExtra.codex_image_generation_bridge_enabled
      if (codexImageGenerationBridgeMode.value === 'inherit') {
        delete newExtra.codex_image_generation_bridge
      } else {
        newExtra.codex_image_generation_bridge = codexImageGenerationBridgeMode.value === 'enabled'
      }

      if (props.account.type === 'oauth') {
        if (codexCLIOnlyEnabled.value) {
          newExtra.codex_cli_only = true
        } else if (hadCodexCLIOnlyEnabled) {
          // 关闭时显式写 false，避免 extra 为空被后端忽略导致旧值无法清除
          newExtra.codex_cli_only = false
        } else {
          delete newExtra.codex_cli_only
        }
      }

      if (openAICodexCLISimulationEnabled.value) {
        newExtra.openai_codex_cli_simulation_enabled = true
      } else if (hadCodexCLISimulationEnabled) {
        newExtra.openai_codex_cli_simulation_enabled = false
      } else {
        delete newExtra.openai_codex_cli_simulation_enabled
      }

      updatePayload.extra = newExtra
    }

    if (props.account.platform === 'openai') {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      applyOpenAIResponseTextErrorConfig(newCredentials)
      applyOpenAICodexCliUserAgentCredentials(newCredentials)
      updatePayload.credentials = newCredentials
    }

    // For apikey/bedrock accounts, handle quota_limit in extra
    if (props.account.type === 'apikey' || props.account.type === 'bedrock') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
        (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      // Total quota
      if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
        newExtra.quota_limit = editQuotaLimit.value
      } else {
        delete newExtra.quota_limit
      }
      // Daily quota
      if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
        newExtra.quota_daily_limit = editQuotaDailyLimit.value
      } else {
        delete newExtra.quota_daily_limit
        delete newExtra.quota_daily_used
        delete newExtra.quota_daily_start
      }
      // Weekly quota
      if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
        newExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
      } else {
        delete newExtra.quota_weekly_limit
        delete newExtra.quota_weekly_used
        delete newExtra.quota_weekly_start
      }
      // Quota reset mode config
      if (editDailyResetMode.value === 'fixed') {
        newExtra.quota_daily_reset_mode = 'fixed'
        newExtra.quota_daily_reset_hour = editDailyResetHour.value ?? 0
      } else {
        delete newExtra.quota_daily_reset_mode
        delete newExtra.quota_daily_reset_hour
      }
      if (editWeeklyResetMode.value === 'fixed') {
        newExtra.quota_weekly_reset_mode = 'fixed'
        newExtra.quota_weekly_reset_day = editWeeklyResetDay.value ?? 1
        newExtra.quota_weekly_reset_hour = editWeeklyResetHour.value ?? 0
      } else {
        delete newExtra.quota_weekly_reset_mode
        delete newExtra.quota_weekly_reset_day
        delete newExtra.quota_weekly_reset_hour
      }
      if (editDailyResetMode.value === 'fixed' || editWeeklyResetMode.value === 'fixed') {
        newExtra.quota_reset_timezone = editResetTimezone.value || 'UTC'
      } else {
        delete newExtra.quota_reset_timezone
      }
      // Quota notify config
      writeQuotaNotifyToExtra(newExtra, 'update')
      updatePayload.extra = newExtra
    }

    const availabilityScheduleError = validateAccountAvailabilityScheduleForm(accountAvailabilitySchedule.value)
    if (availabilityScheduleError) {
      appStore.showError(availabilityScheduleError)
      return
    }
    const extraWithAvailability: Record<string, unknown> = {
      ...(((updatePayload.extra as Record<string, unknown>) ||
        (props.account.extra as Record<string, unknown>) ||
        {}))
    }
    writeAccountAvailabilityScheduleToExtra(extraWithAvailability, accountAvailabilitySchedule.value)
    if (Object.keys(extraWithAvailability).length > 0 || props.account.extra) {
      updatePayload.extra = extraWithAvailability
    } else {
      delete updatePayload.extra
    }

    const canContinue = await ensureAntigravityMixedChannelConfirmed(async () => {
      await submitUpdateAccount(accountID, updatePayload)
    })
    if (!canContinue) {
      return
    }

    await submitUpdateAccount(accountID, updatePayload)
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  }
}

// Handle mixed channel warning confirmation
const handleMixedChannelConfirm = async () => {
  const action = mixedChannelWarningAction.value
  if (!action) {
    clearMixedChannelDialog()
    return
  }
  clearMixedChannelDialog()
  submitting.value = true
  try {
    await action()
  } finally {
    submitting.value = false
  }
}

const handleMixedChannelCancel = () => {
  clearMixedChannelDialog()
}
</script>
