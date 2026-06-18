<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  saving: boolean
}>()

const emit = defineEmits<{
  click: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
    <button
      type="button"
      data-testid="settings-section-save-button"
      :disabled="props.saving"
      class="btn btn-primary btn-sm"
      @click="emit('click')"
    >
      <svg
        v-if="props.saving"
        data-testid="settings-section-save-spinner"
        class="mr-1 h-4 w-4 animate-spin"
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
      {{ props.saving ? t('common.saving') : t('common.save') }}
    </button>
  </div>
</template>
