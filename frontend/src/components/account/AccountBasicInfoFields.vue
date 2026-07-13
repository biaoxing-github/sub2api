<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  name: string
  notes: string
  nameLabelKey?: string
  namePlaceholderKey?: string
  nameTour?: string
  showName?: boolean
  showNotes?: boolean
}>(), {
  nameLabelKey: 'common.name',
  namePlaceholderKey: '',
  nameTour: '',
  showName: true,
  showNotes: true
})

const emit = defineEmits<{
  'update:name': [value: string]
  'update:notes': [value: string]
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="props.showName" data-testid="account-name-field">
    <label class="input-label">{{ t(props.nameLabelKey) }}</label>
    <input
      :value="props.name"
      type="text"
      required
      class="input"
      :placeholder="props.namePlaceholderKey ? t(props.namePlaceholderKey) : undefined"
      :data-tour="props.nameTour || undefined"
      @input="emit('update:name', ($event.target as HTMLInputElement).value)"
    />
  </div>
  <div v-if="props.showNotes" data-testid="account-notes-field">
    <label class="input-label">{{ t('admin.accounts.notes') }}</label>
    <textarea
      :value="props.notes"
      rows="3"
      class="input"
      :placeholder="t('admin.accounts.notesPlaceholder')"
      @input="emit('update:notes', ($event.target as HTMLTextAreaElement).value)"
    ></textarea>
    <p class="input-hint">{{ t('admin.accounts.notesHint') }}</p>
  </div>
</template>
