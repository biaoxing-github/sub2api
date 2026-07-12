<template>
        <!-- Tab: Gateway -->
  <div class="space-y-6">
          <!-- Overload Cooldown (529) Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.overloadCooldown.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.overloadCooldown.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <div
                v-if="overloadCooldownLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <template v-else>
                <div class="flex items-center justify-between">
                  <div>
                    <label class="font-medium text-gray-900 dark:text-white">{{
                      t("admin.settings.overloadCooldown.enabled")
                    }}</label>
                    <p class="text-sm text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.overloadCooldown.enabledHint") }}
                    </p>
                  </div>
                  <Toggle v-model="overloadCooldownForm.enabled" />
                </div>

                <div
                  v-if="overloadCooldownForm.enabled"
                  class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{ t("admin.settings.overloadCooldown.cooldownMinutes") }}
                    </label>
                    <input
                      v-model.number="overloadCooldownForm.cooldown_minutes"
                      type="number"
                      min="1"
                      max="120"
                      class="input w-32"
                    />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t("admin.settings.overloadCooldown.cooldownMinutesHint")
                      }}
                    </p>
                  </div>
                </div>

                <SettingsSectionSaveButton
                  :saving="overloadCooldownSaving"
                  @click="saveOverloadCooldownSettings"
                />
              </template>
            </div>
          </div>

          <!-- Rate Limit Cooldown (429) Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.rateLimit429Cooldown.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.rateLimit429Cooldown.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <div
                v-if="rateLimit429CooldownLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <template v-else>
                <div class="flex items-center justify-between">
                  <div>
                    <label class="font-medium text-gray-900 dark:text-white">{{
                      t("admin.settings.rateLimit429Cooldown.enabled")
                    }}</label>
                    <p class="text-sm text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.rateLimit429Cooldown.enabledHint") }}
                    </p>
                  </div>
                  <Toggle v-model="rateLimit429CooldownForm.enabled" />
                </div>

                <div
                  v-if="rateLimit429CooldownForm.enabled"
                  class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.rateLimit429Cooldown.cooldownSeconds",
                        )
                      }}
                    </label>
                    <input
                      v-model.number="rateLimit429CooldownForm.cooldown_seconds"
                      type="number"
                      min="1"
                      max="7200"
                      class="input w-32"
                    />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t(
                          "admin.settings.rateLimit429Cooldown.cooldownSecondsHint",
                        )
                      }}
                    </p>
                  </div>
                </div>

                <SettingsSectionSaveButton
                  :saving="rateLimit429CooldownSaving"
                  @click="saveRateLimit429CooldownSettings"
                />
              </template>
            </div>
          </div>

          <!-- Stream Timeout Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.streamTimeout.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.streamTimeout.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Loading State -->
              <div
                v-if="streamTimeoutLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <template v-else>
                <!-- Enable Stream Timeout -->
                <div class="flex items-center justify-between">
                  <div>
                    <label class="font-medium text-gray-900 dark:text-white">{{
                      t("admin.settings.streamTimeout.enabled")
                    }}</label>
                    <p class="text-sm text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.streamTimeout.enabledHint") }}
                    </p>
                  </div>
                  <Toggle v-model="streamTimeoutForm.enabled" />
                </div>

                <!-- Settings - Only show when enabled -->
                <div
                  v-if="streamTimeoutForm.enabled"
                  class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <!-- Action -->
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{ t("admin.settings.streamTimeout.action") }}
                    </label>
                    <select
                      v-model="streamTimeoutForm.action"
                      class="input w-64"
                    >
                      <option value="temp_unsched">
                        {{
                          t("admin.settings.streamTimeout.actionTempUnsched")
                        }}
                      </option>
                      <option value="error">
                        {{ t("admin.settings.streamTimeout.actionError") }}
                      </option>
                      <option value="none">
                        {{ t("admin.settings.streamTimeout.actionNone") }}
                      </option>
                    </select>
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.streamTimeout.actionHint") }}
                    </p>
                  </div>

                  <!-- Temp Unsched Minutes (only show when action is temp_unsched) -->
                  <div v-if="streamTimeoutForm.action === 'temp_unsched'">
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{ t("admin.settings.streamTimeout.tempUnschedMinutes") }}
                    </label>
                    <input
                      v-model.number="streamTimeoutForm.temp_unsched_minutes"
                      type="number"
                      min="1"
                      max="60"
                      class="input w-32"
                    />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t("admin.settings.streamTimeout.tempUnschedMinutesHint")
                      }}
                    </p>
                  </div>

                  <!-- Threshold Count -->
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{ t("admin.settings.streamTimeout.thresholdCount") }}
                    </label>
                    <input
                      v-model.number="streamTimeoutForm.threshold_count"
                      type="number"
                      min="1"
                      max="10"
                      class="input w-32"
                    />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.streamTimeout.thresholdCountHint") }}
                    </p>
                  </div>

                  <!-- Threshold Window Minutes -->
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t("admin.settings.streamTimeout.thresholdWindowMinutes")
                      }}
                    </label>
                    <input
                      v-model.number="
                        streamTimeoutForm.threshold_window_minutes
                      "
                      type="number"
                      min="1"
                      max="60"
                      class="input w-32"
                    />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t(
                          "admin.settings.streamTimeout.thresholdWindowMinutesHint",
                        )
                      }}
                    </p>
                  </div>
                </div>

                <SettingsSectionSaveButton
                  :saving="streamTimeoutSaving"
                  @click="saveStreamTimeoutSettings"
                />
              </template>
            </div>
          </div>

          <!-- Request Rectifier Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.rectifier.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.rectifier.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Loading State -->
              <div
                v-if="rectifierLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <template v-else>
                <!-- Master Toggle -->
                <div class="flex items-center justify-between">
                  <div>
                    <label class="font-medium text-gray-900 dark:text-white">{{
                      t("admin.settings.rectifier.enabled")
                    }}</label>
                    <p class="text-sm text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.rectifier.enabledHint") }}
                    </p>
                  </div>
                  <Toggle v-model="rectifierForm.enabled" />
                </div>

                <!-- Sub-toggles (only show when master is enabled) -->
                <div
                  v-if="rectifierForm.enabled"
                  class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <!-- Thinking Signature Rectifier -->
                  <div class="flex items-center justify-between">
                    <div>
                      <label
                        class="text-sm font-medium text-gray-700 dark:text-gray-300"
                        >{{
                          t("admin.settings.rectifier.thinkingSignature")
                        }}</label
                      >
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{
                          t("admin.settings.rectifier.thinkingSignatureHint")
                        }}
                      </p>
                    </div>
                    <Toggle
                      v-model="rectifierForm.thinking_signature_enabled"
                    />
                  </div>

                  <!-- Thinking Budget Rectifier -->
                  <div class="flex items-center justify-between">
                    <div>
                      <label
                        class="text-sm font-medium text-gray-700 dark:text-gray-300"
                        >{{
                          t("admin.settings.rectifier.thinkingBudget")
                        }}</label
                      >
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t("admin.settings.rectifier.thinkingBudgetHint") }}
                      </p>
                    </div>
                    <Toggle v-model="rectifierForm.thinking_budget_enabled" />
                  </div>

                  <!-- API Key Signature Rectifier -->
                  <div class="flex items-center justify-between">
                    <div>
                      <label
                        class="text-sm font-medium text-gray-700 dark:text-gray-300"
                        >{{
                          t("admin.settings.rectifier.apikeySignature")
                        }}</label
                      >
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t("admin.settings.rectifier.apikeySignatureHint") }}
                      </p>
                    </div>
                    <Toggle v-model="rectifierForm.apikey_signature_enabled" />
                  </div>

                  <!-- Custom Patterns (only when apikey_signature_enabled) -->
                  <div
                    v-if="rectifierForm.apikey_signature_enabled"
                    class="ml-4 space-y-3 border-l-2 border-gray-200 pl-4 dark:border-dark-600"
                  >
                    <div>
                      <label
                        class="text-sm font-medium text-gray-700 dark:text-gray-300"
                        >{{
                          t("admin.settings.rectifier.apikeyPatterns")
                        }}</label
                      >
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t("admin.settings.rectifier.apikeyPatternsHint") }}
                      </p>
                    </div>
                    <div
                      v-for="(
                        _, index
                      ) in rectifierForm.apikey_signature_patterns"
                      :key="index"
                      class="flex items-center gap-2"
                    >
                      <input
                        v-model="rectifierForm.apikey_signature_patterns[index]"
                        type="text"
                        class="input input-sm flex-1"
                        :placeholder="
                          t('admin.settings.rectifier.apikeyPatternPlaceholder')
                        "
                      />
                      <button
                        type="button"
                        @click="
                          rectifierForm.apikey_signature_patterns.splice(
                            index,
                            1,
                          )
                        "
                        class="btn btn-ghost btn-xs text-red-500 hover:text-red-700"
                      >
                        <svg
                          class="h-4 w-4"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                      </button>
                    </div>
                    <button
                      type="button"
                      @click="rectifierForm.apikey_signature_patterns.push('')"
                      class="btn btn-ghost btn-xs text-primary-600 dark:text-primary-400"
                    >
                      + {{ t("admin.settings.rectifier.addPattern") }}
                    </button>
                  </div>
                </div>

                <SettingsSectionSaveButton
                  :saving="rectifierSaving"
                  @click="saveRectifierSettings"
                />
              </template>
            </div>
          </div>
          <!-- Beta Policy Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.betaPolicy.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.betaPolicy.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Loading State -->
              <div
                v-if="betaPolicyLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <template v-else>
                <!-- Rule Cards -->
                <div
                  v-for="rule in betaPolicyForm.rules"
                  :key="rule.beta_token"
                  class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
                >
                  <div class="mb-3 flex items-center gap-2">
                    <span
                      class="text-sm font-medium text-gray-900 dark:text-white"
                    >
                      {{ getBetaDisplayName(rule.beta_token) }}
                    </span>
                    <span
                      class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400"
                    >
                      {{ rule.beta_token }}
                    </span>
                  </div>

                  <div class="grid grid-cols-2 gap-4">
                    <!-- Action -->
                    <div>
                      <label
                        class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        {{ t("admin.settings.betaPolicy.action") }}
                      </label>
                      <Select
                        :modelValue="rule.action"
                        @update:modelValue="rule.action = $event as any"
                        :options="betaPolicyActionOptions"
                      />
                    </div>

                    <!-- Scope -->
                    <div>
                      <label
                        class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        {{ t("admin.settings.betaPolicy.scope") }}
                      </label>
                      <Select
                        :modelValue="rule.scope"
                        @update:modelValue="rule.scope = $event as any"
                        :options="betaPolicyScopeOptions"
                      />
                    </div>
                  </div>

                  <!-- Error Message (only when action=block) -->
                  <div v-if="rule.action === 'block'" class="mt-3">
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.betaPolicy.errorMessage") }}
                    </label>
                    <input
                      v-model="rule.error_message"
                      type="text"
                      class="input"
                      :placeholder="
                        t('admin.settings.betaPolicy.errorMessagePlaceholder')
                      "
                    />
                    <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                      {{ t("admin.settings.betaPolicy.errorMessageHint") }}
                    </p>
                  </div>

                  <!-- Quick Presets (only for tokens with presets) -->
                  <div v-if="betaPresets[rule.beta_token]?.length" class="mt-3">
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.betaPolicy.quickPresets") }}
                    </label>
                    <div class="flex flex-wrap gap-2">
                      <button
                        v-for="preset in betaPresets[rule.beta_token]"
                        :key="preset.label"
                        type="button"
                        class="inline-flex items-center gap-1 rounded-md border border-primary-200 bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 transition-colors hover:bg-primary-100 dark:border-primary-800 dark:bg-primary-900/30 dark:text-primary-300 dark:hover:bg-primary-900/50"
                        @click="applyBetaPreset(rule, preset)"
                        :title="preset.description"
                      >
                        {{ preset.label }}
                      </button>
                    </div>
                  </div>

                  <!-- Model Whitelist -->
                  <div class="mt-3">
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.betaPolicy.modelWhitelist") }}
                    </label>
                    <p class="mb-2 text-xs text-gray-400 dark:text-gray-500">
                      {{ t("admin.settings.betaPolicy.modelWhitelistHint") }}
                    </p>
                    <!-- Existing patterns -->
                    <div
                      v-for="(_, index) in rule.model_whitelist || []"
                      :key="index"
                      class="mb-1.5 flex items-center gap-2"
                    >
                      <input
                        v-model="rule.model_whitelist![index]"
                        type="text"
                        class="input input-sm flex-1"
                        :placeholder="
                          t('admin.settings.betaPolicy.modelPatternPlaceholder')
                        "
                      />
                      <button
                        type="button"
                        @click="rule.model_whitelist!.splice(index, 1)"
                        class="shrink-0 rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                      >
                        <svg
                          class="h-4 w-4"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="2"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                      </button>
                    </div>
                    <!-- Add pattern button -->
                    <button
                      type="button"
                      @click="
                        if (!rule.model_whitelist) rule.model_whitelist = [];
                        rule.model_whitelist.push('');
                      "
                      class="mb-2 inline-flex items-center gap-1 text-xs text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                    >
                      <svg
                        class="h-3.5 w-3.5"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M12 4v16m8-8H4"
                        />
                      </svg>
                      {{ t("admin.settings.betaPolicy.addModelPattern") }}
                    </button>
                    <!-- Common pattern chips -->
                    <div class="flex flex-wrap items-center gap-1.5">
                      <span class="text-xs text-gray-400 dark:text-gray-500"
                        >{{
                          t("admin.settings.betaPolicy.commonPatterns")
                        }}:</span
                      >
                      <button
                        v-for="pattern in commonModelPatterns"
                        :key="pattern"
                        type="button"
                        class="rounded border border-gray-200 px-2 py-0.5 text-xs text-gray-600 transition-colors hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 dark:border-dark-600 dark:text-gray-400 dark:hover:border-primary-700 dark:hover:bg-primary-900/30 dark:hover:text-primary-300"
                        @click="addQuickPattern(rule, pattern)"
                      >
                        {{ pattern }}
                      </button>
                    </div>
                  </div>

                  <!-- Fallback Action (only when model_whitelist is non-empty) -->
                  <div
                    v-if="
                      rule.model_whitelist && rule.model_whitelist.length > 0
                    "
                    class="mt-3"
                  >
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.betaPolicy.fallbackAction") }}
                    </label>
                    <Select
                      :modelValue="rule.fallback_action || 'pass'"
                      @update:modelValue="rule.fallback_action = $event as any"
                      :options="betaPolicyActionOptions"
                    />
                    <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                      {{ t("admin.settings.betaPolicy.fallbackActionHint") }}
                    </p>
                    <!-- Fallback Error Message (only when fallback_action=block) -->
                    <div v-if="rule.fallback_action === 'block'" class="mt-2">
                      <input
                        v-model="rule.fallback_error_message"
                        type="text"
                        class="input"
                        :placeholder="
                          t(
                            'admin.settings.betaPolicy.fallbackErrorMessagePlaceholder',
                          )
                        "
                      />
                      <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                        {{ t("admin.settings.betaPolicy.errorMessageHint") }}
                      </p>
                    </div>
                  </div>
                </div>

                <SettingsSectionSaveButton
                  :saving="betaPolicySaving"
                  @click="saveBetaPolicySettings"
                />
              </template>
            </div>
          </div>
          <!-- OpenAI Fast/Flex Policy Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.openaiFastPolicy.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.openaiFastPolicy.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <div class="rounded-lg border border-dashed border-primary-200 bg-primary-50/50 p-4 dark:border-primary-900/50 dark:bg-primary-900/10">
                <div class="mb-3">
                  <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ t("admin.settings.openaiFastPolicy.routeTemplateTitle") }}
                  </h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.openaiFastPolicy.routeTemplateDescription") }}
                  </p>
                </div>
                <div class="flex flex-wrap gap-2">
                  <button
                    v-for="preset in openaiRoutePolicyPresets"
                    :key="preset.key"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="applyOpenAIRoutePolicyPreset(preset.key)"
                  >
                    {{ preset.label }}
                  </button>
                </div>
              </div>

              <!-- Empty state -->
              <div
                v-if="openaiFastPolicyForm.rules.length === 0"
                class="rounded-lg border border-dashed border-gray-200 p-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
              >
                {{ t("admin.settings.openaiFastPolicy.empty") }}
              </div>

              <!-- Rule Cards -->
              <div
                v-for="(rule, ruleIndex) in openaiFastPolicyForm.rules"
                :key="ruleIndex"
                class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
              >
                <div class="mb-3 flex items-center justify-between">
                  <span
                    class="text-sm font-medium text-gray-900 dark:text-white"
                  >
                    {{
                      t("admin.settings.openaiFastPolicy.ruleHeader", {
                        index: ruleIndex + 1,
                      })
                    }}
                  </span>
                  <button
                    type="button"
                    @click="removeOpenAIFastPolicyRule(ruleIndex)"
                    class="rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                    :title="t('admin.settings.openaiFastPolicy.removeRule')"
                  >
                    <svg
                      class="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M6 18L18 6M6 6l12 12"
                      />
                    </svg>
                  </button>
                </div>

                <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                  <!-- Service Tier -->
                  <div>
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.openaiFastPolicy.serviceTier") }}
                    </label>
                    <Select
                      :modelValue="rule.service_tier"
                      @update:modelValue="
                        rule.service_tier = $event as
                          | 'all'
                          | 'priority'
                          | 'flex'
                      "
                      :options="openaiFastPolicyTierOptions"
                    />
                  </div>

                  <!-- Action -->
                  <div>
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.openaiFastPolicy.action") }}
                    </label>
                    <Select
                      :modelValue="rule.action"
                      @update:modelValue="
                        rule.action = $event as 'pass' | 'filter' | 'block'
                      "
                      :options="openaiFastPolicyActionOptions"
                    />
                  </div>

                  <!-- Scope -->
                  <div>
                    <label
                      class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      {{ t("admin.settings.openaiFastPolicy.scope") }}
                    </label>
                    <Select
                      :modelValue="rule.scope"
                      @update:modelValue="
                        rule.scope = $event as
                          | 'all'
                          | 'oauth'
                          | 'apikey'
                          | 'bedrock'
                      "
                      :options="openaiFastPolicyScopeOptions"
                    />
                  </div>
                </div>

                <div class="mt-3">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t("admin.settings.openaiFastPolicy.userIds") }}
                  </label>
                  <p class="mb-2 text-xs text-gray-400 dark:text-gray-500">
                    {{ t("admin.settings.openaiFastPolicy.userIdsHint") }}
                  </p>
                  <div v-for="(_, userIDIndex) in rule.user_ids || []" :key="userIDIndex" class="mb-1.5 flex items-center gap-2">
                    <input v-model.number="rule.user_ids![userIDIndex]" type="number" min="1" step="1" class="input input-sm flex-1" :placeholder="t('admin.settings.openaiFastPolicy.userIdPlaceholder')" />
                    <button type="button" @click="removeOpenAIFastPolicyUserID(rule, userIDIndex)" class="shrink-0 rounded p-1 text-red-400 hover:text-red-600" :title="t('admin.settings.openaiFastPolicy.removeUserId')">&times;</button>
                  </div>
                  <button type="button" @click="addOpenAIFastPolicyUserID(rule)" class="mb-2 text-xs text-primary-600 hover:text-primary-700">
                    {{ t("admin.settings.openaiFastPolicy.addUserId") }}
                  </button>
                </div>

                <!-- Error Message (only when action=block) -->
                <div v-if="rule.action === 'block'" class="mt-3">
                  <label
                    class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    {{ t("admin.settings.openaiFastPolicy.errorMessage") }}
                  </label>
                  <input
                    v-model="rule.error_message"
                    type="text"
                    class="input"
                    :placeholder="
                      t(
                        'admin.settings.openaiFastPolicy.errorMessagePlaceholder',
                      )
                    "
                  />
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t("admin.settings.openaiFastPolicy.errorMessageHint") }}
                  </p>
                </div>

                <!-- Model Whitelist -->
                <div class="mt-3">
                  <label
                    class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    {{ t("admin.settings.openaiFastPolicy.modelWhitelist") }}
                  </label>
                  <p class="mb-2 text-xs text-gray-400 dark:text-gray-500">
                    {{
                      t("admin.settings.openaiFastPolicy.modelWhitelistHint")
                    }}
                  </p>
                  <div
                    v-for="(_, patternIdx) in rule.model_whitelist || []"
                    :key="patternIdx"
                    class="mb-1.5 flex items-center gap-2"
                  >
                    <input
                      v-model="rule.model_whitelist![patternIdx]"
                      type="text"
                      class="input input-sm flex-1"
                      :placeholder="
                        t(
                          'admin.settings.openaiFastPolicy.modelPatternPlaceholder',
                        )
                      "
                    />
                    <button
                      type="button"
                      @click="
                        removeOpenAIFastPolicyModelPattern(rule, patternIdx)
                      "
                      class="shrink-0 rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                    >
                      <svg
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                  <button
                    type="button"
                    @click="addOpenAIFastPolicyModelPattern(rule)"
                    class="mb-2 inline-flex items-center gap-1 text-xs text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    <svg
                      class="h-3.5 w-3.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M12 4v16m8-8H4"
                      />
                    </svg>
                    {{ t("admin.settings.openaiFastPolicy.addModelPattern") }}
                  </button>
                </div>

                <!-- Fallback Action (only when model_whitelist is non-empty) -->
                <div
                  v-if="
                    rule.model_whitelist && rule.model_whitelist.length > 0
                  "
                  class="mt-3"
                >
                  <label
                    class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    {{ t("admin.settings.openaiFastPolicy.fallbackAction") }}
                  </label>
                  <Select
                    :modelValue="rule.fallback_action || 'pass'"
                    @update:modelValue="
                      rule.fallback_action = $event as
                        | 'pass'
                        | 'filter'
                        | 'block'
                    "
                    :options="openaiFastPolicyActionOptions"
                  />
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{
                      t("admin.settings.openaiFastPolicy.fallbackActionHint")
                    }}
                  </p>
                  <div v-if="rule.fallback_action === 'block'" class="mt-2">
                    <input
                      v-model="rule.fallback_error_message"
                      type="text"
                      class="input"
                      :placeholder="
                        t(
                          'admin.settings.openaiFastPolicy.fallbackErrorMessagePlaceholder',
                        )
                      "
                    />
                  </div>
                </div>
              </div>

              <!-- Add Rule Button -->
              <div>
                <button
                  type="button"
                  @click="addOpenAIFastPolicyRule"
                  class="btn btn-secondary btn-sm inline-flex items-center gap-1"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 4v16m8-8H4"
                    />
                  </svg>
                  {{ t("admin.settings.openaiFastPolicy.addRule") }}
                </button>
                <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
                  {{ t("admin.settings.openaiFastPolicy.saveHint") }}
                </p>
              </div>
            </div>
          </div>
        </div>
        <!-- /Tab: Gateway -->

        <!-- Tab: Gateway — Claude Code, Scheduling -->
  <div class="space-y-6">
          <!-- Claude Code Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.claudeCode.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.claudeCode.description") }}
              </p>
            </div>
            <div class="p-6">
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{ t("admin.settings.claudeCode.minVersion") }}
                </label>
                <input
                  v-model="form.min_claude_code_version"
                  type="text"
                  class="input max-w-xs font-mono text-sm"
                  :placeholder="
                    t('admin.settings.claudeCode.minVersionPlaceholder')
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.claudeCode.minVersionHint") }}
                </p>
              </div>
              <div class="mt-4">
                <label
                  class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{ t("admin.settings.claudeCode.maxVersion") }}
                </label>
                <input
                  v-model="form.max_claude_code_version"
                  type="text"
                  class="input max-w-xs font-mono text-sm"
                  :placeholder="
                    t('admin.settings.claudeCode.maxVersionPlaceholder')
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.claudeCode.maxVersionHint") }}
                </p>
              </div>
            </div>
          </div>

          <!-- Gateway Scheduling Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.scheduling.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.scheduling.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{ t("admin.settings.scheduling.allowUngroupedKey") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.scheduling.allowUngroupedKeyHint") }}
                  </p>
                </div>
                <Toggle v-model="form.allow_ungrouped_key_scheduling" />
              </div>

              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{ t("admin.settings.openaiExperimentalScheduler.title") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t("admin.settings.openaiExperimentalScheduler.description")
                    }}
                  </p>
                </div>
                <Toggle v-model="form.openai_advanced_scheduler_enabled" />
              </div>

              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.openaiSchedulerExhaustionProbeInfiniteWait.title",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.openaiSchedulerExhaustionProbeInfiniteWait.description",
                      )
                    }}
                  </p>
                </div>
                <Toggle
                  v-model="
                    form.openai_scheduler_exhaustion_probe_infinite_wait_enabled
                  "
                />
              </div>

              <div
                v-if="
                  form.openai_scheduler_exhaustion_probe_infinite_wait_enabled
                "
                class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700"
              >
                <div class="flex items-center justify-between">
                  <div>
                    <label
                      class="text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.title",
                        )
                      }}
                    </label>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.description",
                        )
                      }}
                    </p>
                  </div>
                  <Toggle
                    v-model="
                      form.openai_scheduler_exhaustion_probe_notify_enabled
                    "
                  />
                </div>

                <div
                  v-if="form.openai_scheduler_exhaustion_probe_notify_enabled"
                  class="grid gap-4 md:grid-cols-2"
                >
                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.afterSeconds",
                        )
                      }}
                    </label>
                    <input
                      v-model.number="
                        form.openai_scheduler_exhaustion_probe_notify_after_seconds
                      "
                      type="number"
                      min="1"
                      class="input"
                    />
                  </div>

                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.repeatSeconds",
                        )
                      }}
                    </label>
                    <input
                      v-model.number="
                        form.openai_scheduler_exhaustion_probe_notify_repeat_seconds
                      "
                      type="number"
                      min="1"
                      class="input"
                    />
                  </div>

                  <div class="md:col-span-2">
                    <label
                      class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.channel",
                        )
                      }}
                    </label>
                    <select
                      v-model="
                        form.openai_scheduler_exhaustion_probe_notify_channel
                      "
                      class="input"
                    >
                      <option value="feishu_webhook">
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.channelWebhook",
                          )
                        }}
                      </option>
                      <option value="feishu_app">
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.channelFeishuApp",
                          )
                        }}
                      </option>
                    </select>
                  </div>

                  <div
                    v-if="
                      form.openai_scheduler_exhaustion_probe_notify_channel !==
                      'feishu_app'
                    "
                    class="md:col-span-2"
                  >
                    <label
                      class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuWebhook",
                        )
                      }}
                    </label>
                    <input
                      v-model="
                        form.openai_scheduler_exhaustion_probe_notify_feishu_webhook_url
                      "
                      type="url"
                      class="input"
                      :placeholder="
                        t(
                          'admin.settings.openaiSchedulerExhaustionProbeNotify.feishuWebhookPlaceholder',
                        )
                      "
                    />
                  </div>

                  <template
                    v-if="
                      form.openai_scheduler_exhaustion_probe_notify_channel ===
                      'feishu_app'
                    "
                  >
                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuAppID",
                          )
                        }}
                      </label>
                      <input
                        v-model="
                          form.openai_scheduler_exhaustion_probe_notify_feishu_app_id
                        "
                        type="text"
                        class="input"
                        :placeholder="
                          t(
                            'admin.settings.openaiSchedulerExhaustionProbeNotify.feishuAppIDPlaceholder',
                          )
                        "
                      />
                    </div>

                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuAppSecret",
                          )
                        }}
                      </label>
                      <input
                        v-model="
                          form.openai_scheduler_exhaustion_probe_notify_feishu_app_secret
                        "
                        type="password"
                        autocomplete="new-password"
                        class="input"
                        :placeholder="
                          t(
                            'admin.settings.openaiSchedulerExhaustionProbeNotify.feishuAppSecretPlaceholder',
                          )
                        "
                      />
                    </div>

                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuDomain",
                          )
                        }}
                      </label>
                      <input
                        v-model="
                          form.openai_scheduler_exhaustion_probe_notify_feishu_domain
                        "
                        type="text"
                        class="input"
                        :placeholder="
                          t(
                            'admin.settings.openaiSchedulerExhaustionProbeNotify.feishuDomainPlaceholder',
                          )
                        "
                      />
                    </div>

                    <div>
                      <label
                        class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuReceiveIDType",
                          )
                        }}
                      </label>
                      <select
                        v-model="
                          form.openai_scheduler_exhaustion_probe_notify_feishu_receive_id_type
                        "
                        class="input"
                      >
                        <option value="open_id">open_id</option>
                        <option value="user_id">user_id</option>
                        <option value="union_id">union_id</option>
                        <option value="email">email</option>
                        <option value="chat_id">chat_id</option>
                      </select>
                    </div>

                    <div class="md:col-span-2">
                      <label
                        class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.feishuReceiveID",
                          )
                        }}
                      </label>
                      <input
                        v-model="
                          form.openai_scheduler_exhaustion_probe_notify_feishu_receive_id
                        "
                        type="text"
                        class="input"
                        :placeholder="
                          t(
                            'admin.settings.openaiSchedulerExhaustionProbeNotify.feishuReceiveIDPlaceholder',
                          )
                        "
                      />
                    </div>
                  </template>

                  <div class="flex items-center justify-between md:col-span-2">
                    <div>
                      <label
                        class="text-sm font-medium text-gray-700 dark:text-gray-300"
                      >
                        {{
                          t(
                            "admin.settings.openaiSchedulerExhaustionProbeNotify.recovered",
                          )
                        }}
                      </label>
                    </div>
                    <Toggle
                      v-model="
                        form.openai_scheduler_exhaustion_probe_notify_recovered_enabled
                      "
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Gateway Forwarding Behavior -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.gatewayForwarding.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.gatewayForwarding.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Fingerprint Unification -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.fingerprintUnification",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.fingerprintUnificationHint",
                      )
                    }}
                  </p>
                </div>
                <Toggle v-model="form.enable_fingerprint_unification" />
              </div>

              <!-- Metadata Passthrough -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t("admin.settings.gatewayForwarding.metadataPassthrough")
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.metadataPassthroughHint",
                      )
                    }}
                  </p>
                </div>
                <Toggle v-model="form.enable_metadata_passthrough" />
              </div>

              <!-- CCH Signing -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{ t("admin.settings.gatewayForwarding.cchSigning") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.gatewayForwarding.cchSigningHint") }}
                  </p>
                </div>
                <Toggle v-model="form.enable_cch_signing" />
              </div>

              <!-- Anthropic Cache TTL 1h Injection -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.anthropicCacheTTL1hInjection",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.anthropicCacheTTL1hInjectionHint",
                      )
                    }}
                  </p>
                </div>
                <Toggle
                  v-model="form.enable_anthropic_cache_ttl_1h_injection"
                />
              </div>

              <!-- messages cache_control 改写 -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.rewriteMessageCacheControl",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.rewriteMessageCacheControlHint",
                      )
                    }}
                  </p>
                </div>
                <Toggle v-model="form.rewrite_message_cache_control" />
              </div>

              <!-- Antigravity UA 版本 -->
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{
                    t(
                      "admin.settings.gatewayForwarding.antigravityUserAgentVersion",
                    )
                  }}
                </label>
                <input
                  v-model="form.antigravity_user_agent_version"
                  type="text"
                  class="input max-w-xs font-mono text-sm"
                  :placeholder="
                    t(
                      'admin.settings.gatewayForwarding.antigravityUserAgentVersionPlaceholder',
                    )
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{
                    t(
                      "admin.settings.gatewayForwarding.antigravityUserAgentVersionHint",
                    )
                  }}
                </p>
              </div>

              <!-- OpenAI Codex UA -->
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{
                    t(
                      "admin.settings.gatewayForwarding.openaiCodexUserAgent",
                    )
                  }}
                </label>
                <input
                  v-model="form.openai_codex_user_agent"
                  type="text"
                  class="input w-full font-mono text-sm"
                  :placeholder="
                    t(
                      'admin.settings.gatewayForwarding.openaiCodexUserAgentPlaceholder',
                    )
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{
                    t(
                      "admin.settings.gatewayForwarding.openaiCodexUserAgentHint",
                    )
                  }}
                </p>
              </div>

              <div class="flex items-center justify-between gap-4">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.openaiAllowClaudeCodeCodexPlugin",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.openaiAllowClaudeCodeCodexPluginDesc",
                      )
                    }}
                  </p>
                </div>
                <Toggle
                  v-model="form.openai_allow_claude_code_codex_plugin"
                />
              </div>

              <div
                class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-4">
                  <div>
                    <label
                      class="text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{
                        t(
                          "admin.settings.gatewayForwarding.openaiCodexDirectForceWS",
                        )
                      }}
                    </label>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t(
                          "admin.settings.gatewayForwarding.openaiCodexDirectForceWSHint",
                        )
                      }}
                    </p>
                  </div>
                  <Toggle v-model="form.openai_codex_direct_force_ws" />
                </div>

                <div>
                  <label
                    class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.openaiCodexDirectTLSFingerprintProfile",
                      )
                    }}
                  </label>
                  <select
                    v-model.number="form.openai_codex_direct_tls_fingerprint_profile_id"
                    class="input w-full"
                  >
                    <option :value="0">
                      {{
                        t(
                          "admin.settings.gatewayForwarding.openaiCodexDirectTLSFingerprintBuiltIn",
                        )
                      }}
                    </option>
                    <option
                      v-if="codexDirectTLSFingerprintProfiles.length > 0"
                      :value="-1"
                    >
                      {{
                        t(
                          "admin.settings.gatewayForwarding.openaiCodexDirectTLSFingerprintRandom",
                        )
                      }}
                    </option>
                    <option
                      v-for="profile in codexDirectTLSFingerprintProfiles"
                      :key="profile.id"
                      :value="profile.id"
                    >
                      {{ profile.name }}
                    </option>
                  </select>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.openaiCodexDirectTLSFingerprintProfileHint",
                      )
                    }}
                  </p>
                </div>
              </div>

              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{
                      t(
                        "admin.settings.gatewayForwarding.clientRequestDebug",
                      )
                    }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t(
                        "admin.settings.gatewayForwarding.clientRequestDebugHint",
                      )
                    }}
                  </p>
                </div>
                <Toggle v-model="form.client_request_debug_log_enabled" />
              </div>

              <!-- Codex Stability Mode -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div>
                    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexStability.title") }}
                    </label>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.codexStability.description") }}
                    </p>
                  </div>
                  <select
                    v-model="form.codex_stability_mode"
                    class="select max-w-xs"
                  >
                    <option value="off">
                      {{ t("admin.settings.gatewayForwarding.codexStability.modeOff") }}
                    </option>
                    <option value="codex">
                      {{ t("admin.settings.gatewayForwarding.codexStability.modeCodex") }}
                    </option>
                    <option value="all_openai_responses">
                      {{ t("admin.settings.gatewayForwarding.codexStability.modeAllResponses") }}
                    </option>
                  </select>
                </div>
                <div class="mt-4 grid gap-3 md:grid-cols-2">
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexStability.dynamicHeaderTimeout") }}
                    </span>
                    <Toggle v-model="form.codex_stability_dynamic_header_timeout_enabled" />
                  </label>
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexStability.requestPhaseFailover") }}
                    </span>
                    <Toggle v-model="form.codex_stability_request_phase_failover_enabled" />
                  </label>
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexStability.suppressTimeoutHeaders") }}
                    </span>
                    <Toggle v-model="form.codex_stability_suppress_client_timeout_headers" />
                  </label>
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexStability.streamKeepalive") }}
                    </span>
                    <Toggle v-model="form.codex_stability_stream_keepalive_enabled" />
                  </label>
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div>
                  <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t("admin.settings.gatewayForwarding.codexAutopilot.title") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.gatewayForwarding.codexAutopilot.description") }}
                  </p>
                </div>
                <div class="mt-4 grid gap-3 md:grid-cols-2">
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexAutopilot.enabled") }}
                    </span>
                    <Toggle v-model="form.codex_autopilot_enabled" />
                  </label>
                  <label class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="text-sm text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.codexAutopilot.observeOnly") }}
                    </span>
                    <Toggle v-model="form.codex_autopilot_observe_only" />
                  </label>
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.openaiPathHealth.title") }}
                    </label>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.openaiPathHealth.description") }}
                    </p>
                  </div>
                  <Toggle v-model="form.openai_path_health_circuit_breaker_enabled" />
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ t("admin.settings.gatewayForwarding.headerRace.title") }}
                    </h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.headerRace.description") }}
                    </p>
                  </div>
                  <Toggle v-model="form.openai_header_race_enabled" />
                </div>
                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.headerRace.delay") }}
                    </label>
                    <input
                      v-model.number="form.openai_header_race_delay_ms"
                      type="number"
                      min="0"
                      class="input max-w-xs"
                    />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.headerRace.dailyBudget") }}
                    </label>
                    <input
                      v-model.number="form.openai_header_race_daily_budget"
                      type="number"
                      min="0"
                      class="input max-w-xs"
                    />
                  </div>
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ t("admin.settings.gatewayForwarding.requestSnapshot.title") }}
                    </h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.requestSnapshot.description") }}
                    </p>
                  </div>
                  <Toggle v-model="form.openai_request_snapshot_enabled" />
                </div>
                <div class="mt-4">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t("admin.settings.gatewayForwarding.requestSnapshot.retention") }}
                  </label>
                  <input
                    v-model.number="form.openai_request_snapshot_retention_hours"
                    type="number"
                    min="1"
                    class="input max-w-xs"
                  />
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.openaiFastLane.title") }}
                    </label>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t("admin.settings.gatewayForwarding.openaiFastLane.description") }}
                    </p>
                  </div>
                  <Toggle v-model="form.openai_fast_lane_enabled" />
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div>
                  <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t("admin.settings.gatewayForwarding.realtimeBalanceConfirm.title") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.gatewayForwarding.realtimeBalanceConfirm.description") }}
                  </p>
                </div>
                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.realtimeBalanceConfirm.topN") }}
                    </label>
                    <input
                      v-model.number="form.realtime_balance_confirm_top_n"
                      type="number"
                      min="0"
                      class="input max-w-xs"
                    />
                  </div>
                  <div>
                    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.realtimeBalanceConfirm.timeoutMs") }}
                    </label>
                    <input
                      v-model.number="form.realtime_balance_confirm_timeout_ms"
                      type="number"
                      min="0"
                      class="input max-w-xs"
                    />
                  </div>
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div>
                  <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t("admin.settings.gatewayForwarding.contextJournal.title") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.gatewayForwarding.contextJournal.description") }}
                  </p>
                </div>
                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.contextJournal.backend") }}
                    </label>
                    <select
                      v-model="form.context_journal_backend"
                      class="select max-w-xs"
                    >
                      <option value="memory">
                        {{ t("admin.settings.gatewayForwarding.contextJournal.backendMemory") }}
                      </option>
                      <option value="redis">
                        {{ t("admin.settings.gatewayForwarding.contextJournal.backendRedis") }}
                      </option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t("admin.settings.gatewayForwarding.contextJournal.ttlHours") }}
                    </label>
                    <input
                      v-model.number="form.context_journal_ttl_hours"
                      type="number"
                      min="0"
                      class="input max-w-xs"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
          <!-- Web Search Emulation -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.webSearchEmulation.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.webSearchEmulation.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Global Toggle -->
              <div class="flex items-center justify-between">
                <div>
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{ t("admin.settings.webSearchEmulation.enabled") }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.webSearchEmulation.enabledHint") }}
                  </p>
                </div>
                <Toggle v-model="webSearchConfig.enabled" />
              </div>

              <!-- Providers -->
              <div v-if="webSearchConfig.enabled" class="space-y-4">
                <div class="flex items-center justify-between">
                  <label
                    class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >
                    {{ t("admin.settings.webSearchEmulation.providers") }}
                  </label>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="addWebSearchProvider"
                  >
                    {{ t("admin.settings.webSearchEmulation.addProvider") }}
                  </button>
                </div>

                <div
                  v-if="webSearchConfig.providers.length === 0"
                  class="rounded-lg border border-dashed border-gray-300 p-4 text-center text-sm text-gray-400 dark:border-dark-600"
                >
                  {{ t("admin.settings.webSearchEmulation.noProviders") }}
                </div>

                <div
                  v-for="(provider, pIdx) in webSearchConfig.providers"
                  :key="pIdx"
                  class="rounded-lg border border-gray-200 dark:border-dark-600"
                >
                  <!-- Collapsible header -->
                  <div
                    class="flex cursor-pointer items-center justify-between px-4 py-3"
                    @click="toggleProviderExpand(pIdx)"
                  >
                    <div class="flex items-center gap-3">
                      <svg
                        class="h-4 w-4 text-gray-400 transition-transform"
                        :class="{ 'rotate-90': expandedProviders[pIdx] }"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M9 5l7 7-7 7"
                        />
                      </svg>
                      <Select
                        v-model="provider.type"
                        :options="[
                          { value: 'brave', label: 'Brave Search' },
                          { value: 'tavily', label: 'Tavily' },
                        ]"
                        class="w-36"
                        @click.stop
                      />
                      <!-- Quota summary (always visible) -->
                      <span class="text-xs text-gray-400">
                        {{ provider.quota_used ?? 0 }} /
                        {{
                          provider.quota_limit != null &&
                          provider.quota_limit > 0
                            ? provider.quota_limit
                            : "∞"
                        }}
                      </span>
                      <span
                        v-if="
                          !expandedProviders[pIdx] &&
                          provider.api_key_configured
                        "
                        class="text-xs text-green-500"
                      >
                        {{
                          t(
                            "admin.settings.webSearchEmulation.apiKeyConfigured",
                          )
                        }}
                      </span>
                    </div>
                    <button
                      type="button"
                      class="text-red-500 hover:text-red-700 text-xs"
                      @click.stop="removeWebSearchProvider(pIdx)"
                    >
                      {{
                        t("admin.settings.webSearchEmulation.removeProvider")
                      }}
                    </button>
                  </div>

                  <!-- Expanded content -->
                  <div
                    v-if="expandedProviders[pIdx]"
                    class="space-y-3 border-t border-gray-100 px-4 pb-4 pt-3 dark:border-dark-700"
                  >
                    <!-- API Key with inline show/copy -->
                    <div>
                      <label class="text-xs text-gray-500">{{
                        t("admin.settings.webSearchEmulation.apiKey")
                      }}</label>
                      <div class="relative">
                        <input
                          v-model="provider.api_key"
                          :type="apiKeyVisible[pIdx] ? 'text' : 'password'"
                          class="input w-full text-sm"
                          :class="
                            provider.api_key || provider.api_key_configured
                              ? 'pr-16'
                              : ''
                          "
                          :placeholder="
                            provider.api_key_configured
                              ? '••••••••'
                              : t(
                                  'admin.settings.webSearchEmulation.apiKeyPlaceholder',
                                )
                          "
                        />
                        <div
                          v-if="provider.api_key || provider.api_key_configured"
                          class="absolute inset-y-0 right-0 flex items-center pr-1.5"
                        >
                          <button
                            type="button"
                            class="rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                            :title="
                              apiKeyVisible[pIdx]
                                ? t(
                                    'admin.settings.webSearchEmulation.hideApiKey',
                                  )
                                : t(
                                    'admin.settings.webSearchEmulation.showApiKey',
                                  )
                            "
                            @click="apiKeyVisible[pIdx] = !apiKeyVisible[pIdx]"
                          >
                            <svg
                              v-if="!apiKeyVisible[pIdx]"
                              class="h-4 w-4"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                              />
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                              />
                            </svg>
                            <svg
                              v-else
                              class="h-4 w-4"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
                              />
                            </svg>
                          </button>
                          <button
                            type="button"
                            class="rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                            :class="{
                              'opacity-30 cursor-not-allowed':
                                !provider.api_key,
                            }"
                            :title="
                              t('admin.settings.webSearchEmulation.copyApiKey')
                            "
                            :disabled="!provider.api_key"
                            @click="copyApiKey(pIdx)"
                          >
                            <svg
                              class="h-4 w-4"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                              />
                            </svg>
                          </button>
                        </div>
                      </div>
                    </div>

                    <!-- Quota + Subscription in compact row -->
                    <div class="grid grid-cols-2 gap-3">
                      <div>
                        <label class="text-xs text-gray-500">{{
                          t("admin.settings.webSearchEmulation.quotaLimit")
                        }}</label>
                        <input
                          v-model="provider.quota_limit"
                          type="number"
                          min="1"
                          class="input text-sm"
                          :placeholder="'∞'"
                        />
                        <p class="mt-0.5 text-xs text-gray-400">
                          {{
                            t(
                              "admin.settings.webSearchEmulation.quotaLimitHint",
                            )
                          }}
                        </p>
                      </div>
                      <div>
                        <label class="text-xs text-gray-500">{{
                          t("admin.settings.webSearchEmulation.subscribedAt")
                        }}</label>
                        <input
                          :value="formatSubscribedAt(provider.subscribed_at)"
                          type="date"
                          class="input text-sm"
                          @input="
                            provider.subscribed_at = parseSubscribedAt(
                              ($event.target as HTMLInputElement).value,
                            )
                          "
                        />
                        <p class="mt-0.5 text-xs text-gray-400">
                          {{
                            t(
                              "admin.settings.webSearchEmulation.subscribedAtHint",
                            )
                          }}
                        </p>
                      </div>
                    </div>

                    <!-- Usage display -->
                    <div class="flex items-center gap-2">
                      <span class="text-xs text-gray-500"
                        >{{
                          t("admin.settings.webSearchEmulation.quotaUsage")
                        }}:</span
                      >
                      <div
                        v-if="
                          provider.quota_limit != null &&
                          provider.quota_limit > 0
                        "
                        class="flex-1 rounded-full bg-gray-200 dark:bg-dark-600"
                        style="height: 6px"
                      >
                        <div
                          class="h-full rounded-full transition-all"
                          :class="
                            quotaPercentage(provider) > 90
                              ? 'bg-red-500'
                              : quotaPercentage(provider) > 70
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                          "
                          :style="{
                            width:
                              Math.min(quotaPercentage(provider), 100) + '%',
                          }"
                        />
                      </div>
                      <div v-else class="flex-1" />
                      <span class="text-xs text-gray-500"
                        >{{ provider.quota_used ?? 0 }} /
                        {{
                          provider.quota_limit != null &&
                          provider.quota_limit > 0
                            ? provider.quota_limit
                            : "∞"
                        }}</span
                      >
                      <button
                        v-if="(provider.quota_used ?? 0) > 0"
                        type="button"
                        class="text-xs text-primary-600 hover:text-primary-700"
                        @click="resetWebSearchUsage(pIdx)"
                      >
                        {{ t("admin.settings.webSearchEmulation.resetUsage") }}
                      </button>
                    </div>

                    <!-- Proxy + Test on same row -->
                    <div class="flex items-end gap-3">
                      <div class="flex-1">
                        <label class="text-xs text-gray-500">{{
                          t("admin.settings.webSearchEmulation.proxy")
                        }}</label>
                        <ProxySelector
                          v-model="provider.proxy_id"
                          :proxies="webSearchProxies"
                        />
                      </div>
                      <button
                        type="button"
                        class="btn btn-secondary btn-sm whitespace-nowrap"
                        @click="openTestDialog()"
                      >
                        {{ t("admin.settings.webSearchEmulation.test") }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Web Search Test Dialog -->
          <div
            v-if="wsTestDialogOpen"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
            @click.self="wsTestDialogOpen = false"
          >
            <div
              class="mx-4 w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800"
            >
              <h3
                class="mb-4 text-lg font-semibold text-gray-900 dark:text-white"
              >
                {{ t("admin.settings.webSearchEmulation.testResultTitle") }}
              </h3>
              <div class="flex items-center gap-2">
                <input
                  v-model="wsTestQuery"
                  type="text"
                  class="input flex-1 text-sm"
                  :placeholder="
                    t('admin.settings.webSearchEmulation.testDefaultQuery')
                  "
                  @keyup.enter="testWebSearchProvider()"
                />
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="wsTestLoading"
                  @click="testWebSearchProvider()"
                >
                  {{
                    wsTestLoading
                      ? t("admin.settings.webSearchEmulation.testing")
                      : t("admin.settings.webSearchEmulation.test")
                  }}
                </button>
              </div>
              <!-- Test results -->
              <div
                v-if="wsTestResult"
                class="mt-4 max-h-80 overflow-y-auto rounded-lg bg-gray-50 p-4 dark:bg-dark-700"
              >
                <p
                  class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{
                    t("admin.settings.webSearchEmulation.testResultProvider")
                  }}: {{ wsTestResult.provider }}
                </p>
                <div
                  v-if="wsTestResult.results.length === 0"
                  class="text-sm text-gray-400"
                >
                  {{ t("admin.settings.webSearchEmulation.testNoResults") }}
                </div>
                <div
                  v-for="(r, rIdx) in wsTestResult.results"
                  :key="rIdx"
                  class="mt-2 border-t border-gray-200 pt-2 first:mt-0 first:border-0 first:pt-0 dark:border-dark-600"
                >
                  <a
                    :href="r.url"
                    target="_blank"
                    class="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
                    >{{ r.title }}</a
                  >
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ r.snippet }}
                  </p>
                </div>
              </div>
              <div class="mt-4 flex justify-end">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="wsTestDialogOpen = false"
                >
                  {{ t("common.close") }}
                </button>
              </div>
            </div>
          </div>
        </div>
        <!-- /Tab: Gateway — Claude Code, Scheduling -->
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type {
  BetaPolicyRule,
  BetaPolicySettings,
  OpenAIFastPolicyRule,
  OpenAIFastPolicySettings,
  OverloadCooldownSettings,
  RateLimit429CooldownSettings,
  RectifierSettings,
  StreamTimeoutSettings,
  SystemSettings,
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
  WebSearchTestResult,
} from "@/api/admin/settings";
import type { TLSFingerprintProfile } from "@/api/admin/tlsFingerprintProfile";
import type { Proxy } from "@/types";
import Select from "@/components/common/Select.vue";
import Toggle from "@/components/common/Toggle.vue";
import ProxySelector from "@/components/common/ProxySelector.vue";
import SettingsSectionSaveButton from "@/components/admin/settings/SettingsSectionSaveButton.vue";

type RequiredGatewaySettingsFields = {
  allow_ungrouped_key_scheduling: boolean;
  openai_advanced_scheduler_enabled: boolean;
  openai_scheduler_exhaustion_probe_infinite_wait_enabled: boolean;
  openai_scheduler_exhaustion_probe_notify_enabled: boolean;
  openai_scheduler_exhaustion_probe_notify_recovered_enabled: boolean;
  enable_fingerprint_unification: boolean;
  enable_metadata_passthrough: boolean;
  enable_cch_signing: boolean;
  enable_anthropic_cache_ttl_1h_injection: boolean;
  rewrite_message_cache_control: boolean;
  openai_allow_claude_code_codex_plugin: boolean;
  openai_codex_direct_force_ws: boolean;
  client_request_debug_log_enabled: boolean;
  codex_stability_dynamic_header_timeout_enabled: boolean;
  codex_stability_request_phase_failover_enabled: boolean;
  codex_stability_suppress_client_timeout_headers: boolean;
  codex_stability_stream_keepalive_enabled: boolean;
  codex_autopilot_enabled: boolean;
  codex_autopilot_observe_only: boolean;
  openai_path_health_circuit_breaker_enabled: boolean;
  openai_header_race_enabled: boolean;
  openai_request_snapshot_enabled: boolean;
  openai_fast_lane_enabled: boolean;
};
type GatewaySettingsForm = SystemSettings &
  RequiredGatewaySettingsFields &
  Record<string, any>;
type SelectOption = { value: string | number; label: string };
type BetaPolicyPreset = {
  label: string;
  description: string;
  action: BetaPolicyRule["action"];
  model_whitelist: string[];
  fallback_action: NonNullable<BetaPolicyRule["fallback_action"]>;
};
type OpenAIRoutePolicyPresetKey =
  | "codex_stable"
  | "first_token_fast"
  | "quota_saver"
  | "long_context";
type OpenAIRoutePolicyPreset = { key: OpenAIRoutePolicyPresetKey; label: string };

const wsTestQuery = defineModel<string>("wsTestQuery", { required: true });
const wsTestDialogOpen = defineModel<boolean>("wsTestDialogOpen", { required: true });

defineProps<{
  form: GatewaySettingsForm;
  overloadCooldownLoading: boolean;
  overloadCooldownSaving: boolean;
  overloadCooldownForm: OverloadCooldownSettings;
  saveOverloadCooldownSettings: () => void | Promise<void>;
  rateLimit429CooldownLoading: boolean;
  rateLimit429CooldownSaving: boolean;
  rateLimit429CooldownForm: RateLimit429CooldownSettings;
  saveRateLimit429CooldownSettings: () => void | Promise<void>;
  streamTimeoutLoading: boolean;
  streamTimeoutSaving: boolean;
  streamTimeoutForm: StreamTimeoutSettings;
  saveStreamTimeoutSettings: () => void | Promise<void>;
  rectifierLoading: boolean;
  rectifierSaving: boolean;
  rectifierForm: RectifierSettings;
  saveRectifierSettings: () => void | Promise<void>;
  betaPolicyLoading: boolean;
  betaPolicySaving: boolean;
  betaPolicyForm: BetaPolicySettings;
  betaPolicyActionOptions: SelectOption[];
  betaPolicyScopeOptions: SelectOption[];
  betaPresets: Record<string, BetaPolicyPreset[]>;
  commonModelPatterns: string[];
  getBetaDisplayName: (token: string) => string;
  applyBetaPreset: (rule: BetaPolicyRule, preset: BetaPolicyPreset) => void;
  addQuickPattern: (rule: BetaPolicyRule, pattern: string) => void;
  saveBetaPolicySettings: () => void | Promise<void>;
  openaiFastPolicyForm: OpenAIFastPolicySettings;
  openaiRoutePolicyPresets: OpenAIRoutePolicyPreset[];
  applyOpenAIRoutePolicyPreset: (key: OpenAIRoutePolicyPresetKey) => void;
  openaiFastPolicyTierOptions: SelectOption[];
  openaiFastPolicyActionOptions: SelectOption[];
  openaiFastPolicyScopeOptions: SelectOption[];
  addOpenAIFastPolicyRule: () => void;
  removeOpenAIFastPolicyRule: (index: number) => void;
  addOpenAIFastPolicyUserID: (rule: OpenAIFastPolicyRule) => void;
  removeOpenAIFastPolicyUserID: (rule: OpenAIFastPolicyRule, index: number) => void;
  addOpenAIFastPolicyModelPattern: (rule: OpenAIFastPolicyRule) => void;
  removeOpenAIFastPolicyModelPattern: (rule: OpenAIFastPolicyRule, index: number) => void;
  codexDirectTLSFingerprintProfiles: TLSFingerprintProfile[];
  webSearchConfig: WebSearchEmulationConfig;
  expandedProviders: Record<number, boolean>;
  apiKeyVisible: Record<number, boolean>;
  webSearchProxies: Proxy[];
  wsTestLoading: boolean;
  wsTestResult: WebSearchTestResult | null;
  openTestDialog: () => void;
  toggleProviderExpand: (index: number) => void;
  removeWebSearchProvider: (index: number) => void;
  addWebSearchProvider: () => void;
  formatSubscribedAt: (timestamp: number | null) => string;
  parseSubscribedAt: (date: string) => number | null;
  quotaPercentage: (provider: WebSearchProviderConfig) => number;
  resetWebSearchUsage: (index: number) => void | Promise<void>;
  copyApiKey: (index: number) => void | Promise<void>;
  testWebSearchProvider: () => void | Promise<void>;
}>();

const { t } = useI18n();
</script>
