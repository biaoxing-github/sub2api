<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

type RPMStrategy = 'tiered' | 'sticky_exempt'

interface TLSFingerprintProfileOption {
  id: number
  name: string
}

const props = withDefaults(defineProps<{
  tlsFingerprintProfiles?: TLSFingerprintProfileOption[]
}>(), {
  tlsFingerprintProfiles: () => []
})

const windowCostEnabled = defineModel<boolean>('windowCostEnabled', { required: true })
const windowCostLimit = defineModel<number | null>('windowCostLimit', { required: true })
const windowCostStickyReserve = defineModel<number | null>('windowCostStickyReserve', { required: true })
const sessionLimitEnabled = defineModel<boolean>('sessionLimitEnabled', { required: true })
const maxSessions = defineModel<number | null>('maxSessions', { required: true })
const sessionIdleTimeout = defineModel<number | null>('sessionIdleTimeout', { required: true })
const rpmLimitEnabled = defineModel<boolean>('rpmLimitEnabled', { required: true })
const baseRpm = defineModel<number | null>('baseRpm', { required: true })
const rpmStrategy = defineModel<RPMStrategy>('rpmStrategy', { required: true })
const rpmStickyBuffer = defineModel<number | null>('rpmStickyBuffer', { required: true })
const userMsgQueueMode = defineModel<string>('userMsgQueueMode', { required: true })
const tlsFingerprintEnabled = defineModel<boolean>('tlsFingerprintEnabled', { required: true })
const tlsFingerprintProfileId = defineModel<number | null>('tlsFingerprintProfileId', { required: true })
const sessionIdMaskingEnabled = defineModel<boolean>('sessionIdMaskingEnabled', { required: true })
const cacheTTLOverrideEnabled = defineModel<boolean>('cacheTtlOverrideEnabled', { required: true })
const cacheTTLOverrideTarget = defineModel<string>('cacheTtlOverrideTarget', { required: true })
const customBaseUrlEnabled = defineModel<boolean>('customBaseUrlEnabled', { required: true })
const customBaseUrl = defineModel<string>('customBaseUrl', { required: true })

const { t } = useI18n()

const umqModeOptions = computed(() => [
  { value: '', label: t('admin.accounts.quotaControl.rpmLimit.umqModeOff') },
  { value: 'throttle', label: t('admin.accounts.quotaControl.rpmLimit.umqModeThrottle') },
  { value: 'serialize', label: t('admin.accounts.quotaControl.rpmLimit.umqModeSerialize') },
])
</script>

<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600 space-y-4">
    <div class="mb-3">
      <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.quotaControl.hint') }}
      </p>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.windowCost.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.windowCost.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            windowCostEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="windowCostEnabled = !windowCostEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              windowCostEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>

      <div v-if="windowCostEnabled" class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.limit') }}</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500 dark:text-gray-400">$</span>
            <input
              v-model.number="windowCostLimit"
              type="number"
              min="0"
              step="1"
              class="input pl-7"
              :placeholder="t('admin.accounts.quotaControl.windowCost.limitPlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.limitHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.stickyReserve') }}</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500 dark:text-gray-400">$</span>
            <input
              v-model.number="windowCostStickyReserve"
              type="number"
              min="0"
              step="1"
              class="input pl-7"
              :placeholder="t('admin.accounts.quotaControl.windowCost.stickyReservePlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.stickyReserveHint') }}</p>
        </div>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionLimit.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.sessionLimit.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            sessionLimitEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="sessionLimitEnabled = !sessionLimitEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              sessionLimitEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>

      <div v-if="sessionLimitEnabled" class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessions') }}</label>
          <input
            v-model.number="maxSessions"
            type="number"
            min="1"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.sessionLimit.maxSessionsPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessionsHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeout') }}</label>
          <div class="relative">
            <input
              v-model.number="sessionIdleTimeout"
              type="number"
              min="1"
              step="1"
              class="input pr-12"
              :placeholder="t('admin.accounts.quotaControl.sessionLimit.idleTimeoutPlaceholder')"
            />
            <span class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 dark:text-gray-400">{{ t('common.minutes') }}</span>
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeoutHint') }}</p>
        </div>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.rpmLimit.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.rpmLimit.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            rpmLimitEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="rpmLimitEnabled = !rpmLimitEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              rpmLimitEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>

      <div v-if="rpmLimitEnabled" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
          <input
            v-model.number="baseRpm"
            type="number"
            min="1"
            max="1000"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.strategy') }}</label>
          <div class="flex gap-2">
            <button
              type="button"
              :class="[
                'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
                rpmStrategy === 'tiered'
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500'
              ]"
              @click="rpmStrategy = 'tiered'"
            >
              <div class="text-center">
                <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyTiered') }}</div>
                <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyTieredHint') }}</div>
              </div>
            </button>
            <button
              type="button"
              :class="[
                'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
                rpmStrategy === 'sticky_exempt'
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500'
              ]"
              @click="rpmStrategy = 'sticky_exempt'"
            >
              <div class="text-center">
                <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExempt') }}</div>
                <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExemptHint') }}</div>
              </div>
            </button>
          </div>
        </div>

        <div v-if="rpmStrategy === 'tiered'">
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBuffer') }}</label>
          <input
            v-model.number="rpmStickyBuffer"
            type="number"
            min="1"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.stickyBufferPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBufferHint') }}</p>
        </div>
      </div>

      <div class="mt-4">
        <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueue') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400 mb-2">
          {{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueueHint') }}
        </p>
        <div class="flex space-x-2">
          <button
            v-for="opt in umqModeOptions"
            :key="opt.value"
            type="button"
            :class="[
              'px-3 py-1.5 text-sm rounded-md border transition-colors',
              userMsgQueueMode === opt.value
                ? 'bg-primary-600 text-white border-primary-600'
                : 'bg-white dark:bg-dark-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-dark-500 hover:bg-gray-50 dark:hover:bg-dark-600'
            ]"
            @click="userMsgQueueMode = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.tlsFingerprint.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.tlsFingerprint.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            tlsFingerprintEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="tlsFingerprintEnabled = !tlsFingerprintEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              tlsFingerprintEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
      <div v-if="tlsFingerprintEnabled" class="mt-3">
        <select v-model="tlsFingerprintProfileId" class="input">
          <option :value="null">{{ t('admin.accounts.quotaControl.tlsFingerprint.defaultProfile') }}</option>
          <option v-if="props.tlsFingerprintProfiles.length > 0" :value="-1">{{ t('admin.accounts.quotaControl.tlsFingerprint.randomProfile') }}</option>
          <option v-for="profile in props.tlsFingerprintProfiles" :key="profile.id" :value="profile.id">{{ profile.name }}</option>
        </select>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionIdMasking.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.sessionIdMasking.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            sessionIdMaskingEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="sessionIdMaskingEnabled = !sessionIdMaskingEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              sessionIdMaskingEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.cacheTTLOverride.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.cacheTTLOverride.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            cacheTTLOverrideEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="cacheTTLOverrideEnabled = !cacheTTLOverrideEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              cacheTTLOverrideEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
      <div v-if="cacheTTLOverrideEnabled" class="mt-3">
        <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.cacheTTLOverride.target') }}</label>
        <select
          v-model="cacheTTLOverrideTarget"
          class="mt-1 block w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-700 dark:text-white"
        >
          <option value="5m">5m</option>
          <option value="1h">1h</option>
        </select>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.quotaControl.cacheTTLOverride.targetHint') }}
        </p>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.customBaseUrl.label') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quotaControl.customBaseUrl.hint') }}
          </p>
        </div>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            customBaseUrlEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
          ]"
          @click="customBaseUrlEnabled = !customBaseUrlEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              customBaseUrlEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
      <div v-if="customBaseUrlEnabled" class="mt-3">
        <input
          v-model="customBaseUrl"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.quotaControl.customBaseUrl.urlHint')"
        />
      </div>
    </div>
  </div>
</template>
