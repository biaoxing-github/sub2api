<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  baseUrl: string
  apiKey: string
  apiKeyHintKey: string
  required?: boolean
}>(), {
  required: false
})

const emit = defineEmits<{
  'update:baseUrl': [value: string]
  'update:apiKey': [value: string]
}>()

const { t } = useI18n()
</script>

<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.upstream.baseUrl') }}</label>
    <input
      :value="props.baseUrl"
      type="text"
      :required="props.required"
      class="input"
      placeholder="https://cloudcode-pa.googleapis.com"
      @input="emit('update:baseUrl', ($event.target as HTMLInputElement).value)"
    />
    <p class="input-hint">{{ t('admin.accounts.upstream.baseUrlHint') }}</p>
  </div>
  <div>
    <label class="input-label">{{ t('admin.accounts.upstream.apiKey') }}</label>
    <input
      :value="props.apiKey"
      type="password"
      :required="props.required"
      class="input font-mono"
      placeholder="sk-..."
      @input="emit('update:apiKey', ($event.target as HTMLInputElement).value)"
    />
    <p class="input-hint">{{ t(props.apiKeyHintKey) }}</p>
  </div>
</template>
