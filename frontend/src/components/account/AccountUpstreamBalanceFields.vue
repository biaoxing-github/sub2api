<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  authUsername: string
  authPassword: string
  commonRateMultiplier: number | null
  commonRateGroupName: string
  balanceEndpointPathsText: string
  manualBalanceTotal?: number | null
  hasExistingAuthPassword?: boolean
  showManualBalance?: boolean
}>(), {
  manualBalanceTotal: null,
  hasExistingAuthPassword: false,
  showManualBalance: false
})

const emit = defineEmits<{
  'update:authUsername': [value: string]
  'update:authPassword': [value: string]
  'update:commonRateMultiplier': [value: number | null]
  'update:commonRateGroupName': [value: string]
  'update:balanceEndpointPathsText': [value: string]
  'update:manualBalanceTotal': [value: number | null]
}>()

const { t } = useI18n()

const numberValue = (value: string) => {
  if (value.trim() === '') return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}
</script>

<template>
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.authUsername') }}</label>
      <input
        :value="props.authUsername"
        type="text"
        class="input"
        autocomplete="username"
        :placeholder="t('admin.accounts.upstream.authUsernamePlaceholder')"
        @input="emit('update:authUsername', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.authUsernameHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.authPassword') }}</label>
      <input
        :value="props.authPassword"
        type="password"
        class="input"
        autocomplete="new-password"
        data-1p-ignore
        data-lpignore="true"
        data-bwignore="true"
        :placeholder="props.hasExistingAuthPassword ? t('admin.accounts.leaveEmptyToKeep') : t('admin.accounts.upstream.authPasswordPlaceholder')"
        @input="emit('update:authPassword', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.authPasswordHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.commonRateMultiplier') }}</label>
      <input
        :value="props.commonRateMultiplier ?? ''"
        type="number"
        min="0"
        step="0.0001"
        class="input"
        :placeholder="t('admin.accounts.upstream.commonRateMultiplierPlaceholder')"
        @input="emit('update:commonRateMultiplier', numberValue(($event.target as HTMLInputElement).value))"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.commonRateMultiplierHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.commonRateGroupName') }}</label>
      <input
        :value="props.commonRateGroupName"
        type="text"
        class="input"
        :placeholder="t('admin.accounts.upstream.commonRateGroupNamePlaceholder')"
        @input="emit('update:commonRateGroupName', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.commonRateGroupNameHint') }}</p>
    </div>
    <div class="sm:col-span-2">
      <label class="input-label">{{ t('admin.accounts.upstream.balanceEndpointPaths') }}</label>
      <textarea
        :value="props.balanceEndpointPathsText"
        rows="5"
        class="input font-mono text-xs"
        :placeholder="t('admin.accounts.upstream.balanceEndpointPathsPlaceholder')"
        @input="emit('update:balanceEndpointPathsText', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
      <p class="input-hint">{{ t('admin.accounts.upstream.balanceEndpointPathsHint') }}</p>
    </div>
    <div v-if="props.showManualBalance" class="sm:col-span-2">
      <label class="input-label">{{ t('admin.accounts.upstream.manualBalanceTotal') }}</label>
      <input
        :value="props.manualBalanceTotal ?? ''"
        type="number"
        min="0"
        step="0.0001"
        class="input"
        :placeholder="t('admin.accounts.upstream.manualBalanceTotalPlaceholder')"
        @input="emit('update:manualBalanceTotal', numberValue(($event.target as HTMLInputElement).value))"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.manualBalanceTotalHint') }}</p>
    </div>
  </div>
</template>
