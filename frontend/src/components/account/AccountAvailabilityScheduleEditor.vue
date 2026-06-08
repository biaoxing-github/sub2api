<template>
  <div class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-600">
    <div class="flex items-start justify-between gap-4">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.availabilitySchedule.title') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.availabilitySchedule.description') }}
        </p>
        <p v-if="summary" class="mt-1 text-xs text-gray-600 dark:text-gray-300">{{ summary }}</p>
      </div>
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
          modelValue.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
        ]"
        :aria-pressed="modelValue.enabled"
        :aria-label="t('admin.accounts.availabilitySchedule.toggle')"
        @click="setEnabled(!modelValue.enabled)"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            modelValue.enabled ? 'translate-x-5' : 'translate-x-0'
          ]"
        />
      </button>
    </div>

    <div v-if="modelValue.enabled" class="space-y-4">
      <p
        v-if="validationError"
        class="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ validationError }}
      </p>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <label class="input-label">{{ t('admin.accounts.availabilitySchedule.timezone') }}</label>
          <input
            :value="modelValue.timezone"
            type="text"
            class="input"
            placeholder="Asia/Shanghai"
            @input="setTimezone(eventValue($event))"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.availabilitySchedule.startDate') }}</label>
          <input
            :value="modelValue.dateRange?.startDate || ''"
            type="date"
            class="input"
            @input="setDateRange('startDate', eventValue($event))"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.availabilitySchedule.endDate') }}</label>
          <input
            :value="modelValue.dateRange?.endDate || ''"
            type="date"
            class="input"
            @input="setDateRange('endDate', eventValue($event))"
          />
        </div>
      </div>

      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <label class="input-label mb-0">{{ t('admin.accounts.availabilitySchedule.windows') }}</label>
          <button type="button" class="btn-secondary btn-sm inline-flex items-center gap-1" @click="addWindow">
            <Icon name="plus" size="xs" />
            {{ t('admin.accounts.availabilitySchedule.addWindow') }}
          </button>
        </div>
        <div
          v-for="(window, index) in modelValue.windows"
          :key="window.key"
          class="space-y-3 rounded-md border border-gray-100 p-3 dark:border-dark-700"
        >
          <div class="flex flex-wrap gap-2">
            <label
              v-for="option in weekdayOptions"
              :key="option.value"
              class="inline-flex items-center gap-1 rounded-md bg-gray-50 px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-300"
            >
              <input
                type="checkbox"
                class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="window.daysOfWeek.includes(option.value)"
                @change="toggleWindowDay(index, option.value)"
              />
              {{ option.label }}
            </label>
          </div>
          <div class="grid grid-cols-[1fr_1fr_auto] gap-2">
            <input
              :value="window.start"
              type="time"
              class="input"
              @input="setWindowTime(index, 'start', eventValue($event))"
            />
            <input
              :value="window.end"
              type="time"
              class="input"
              @input="setWindowTime(index, 'end', eventValue($event))"
            />
            <button
              type="button"
              class="btn-secondary inline-flex items-center justify-center px-3"
              :disabled="modelValue.windows.length <= 1"
              :title="t('common.delete')"
              @click="removeWindow(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
      </div>

      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <label class="input-label mb-0">{{ t('admin.accounts.availabilitySchedule.exceptions') }}</label>
          <button type="button" class="btn-secondary btn-sm inline-flex items-center gap-1" @click="addException">
            <Icon name="plus" size="xs" />
            {{ t('admin.accounts.availabilitySchedule.addException') }}
          </button>
        </div>
        <p v-if="modelValue.exceptions.length === 0" class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.availabilitySchedule.noExceptions') }}
        </p>
        <div
          v-for="(exception, index) in modelValue.exceptions"
          :key="exception.key"
          class="space-y-3 rounded-md border border-gray-100 p-3 dark:border-dark-700"
        >
          <div class="grid grid-cols-[1fr_160px_auto] gap-2">
            <input
              :value="exception.date"
              type="date"
              class="input"
              @input="setExceptionDate(index, eventValue($event))"
            />
            <select
              :value="exception.action"
              class="input"
              @change="setExceptionAction(index, eventValue($event))"
            >
              <option value="deny">{{ t('admin.accounts.availabilitySchedule.deny') }}</option>
              <option value="allow">{{ t('admin.accounts.availabilitySchedule.allow') }}</option>
            </select>
            <button
              type="button"
              class="btn-secondary inline-flex items-center justify-center px-3"
              :title="t('common.delete')"
              @click="removeException(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
          <div v-if="exception.action === 'allow'" class="space-y-2">
            <div
              v-for="(window, windowIndex) in exception.windows"
              :key="window.key"
              class="grid grid-cols-[1fr_1fr_auto] gap-2"
            >
              <input
                :value="window.start"
                type="time"
                class="input"
                @input="setExceptionWindowTime(index, windowIndex, 'start', eventValue($event))"
              />
              <input
                :value="window.end"
                type="time"
                class="input"
                @input="setExceptionWindowTime(index, windowIndex, 'end', eventValue($event))"
              />
              <button
                type="button"
                class="btn-secondary inline-flex items-center justify-center px-3"
                :disabled="exception.windows.length <= 1"
                :title="t('common.delete')"
                @click="removeExceptionWindow(index, windowIndex)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
            <button
              type="button"
              class="btn-secondary btn-sm inline-flex items-center gap-1"
              @click="addExceptionWindow(index)"
            >
              <Icon name="plus" size="xs" />
              {{ t('admin.accounts.availabilitySchedule.addExceptionWindow') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {
  cloneAccountAvailabilityScheduleForm,
  createAccountScheduleExceptionFormRow,
  createAccountScheduleExceptionWindowFormRow,
  createAccountScheduleWindowFormRow,
  weekdayOptions,
  type AccountAvailabilityScheduleForm
} from './accountAvailabilitySchedule'

const props = defineProps<{
  modelValue: AccountAvailabilityScheduleForm
  summary?: string
  validationError?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: AccountAvailabilityScheduleForm]
}>()

const { t } = useI18n()

const updateSchedule = (mutate: (draft: AccountAvailabilityScheduleForm) => void) => {
  const draft = cloneAccountAvailabilityScheduleForm(props.modelValue)
  mutate(draft)
  emit('update:modelValue', draft)
}

const setEnabled = (enabled: boolean) => updateSchedule((draft) => {
  draft.enabled = enabled
})

const setTimezone = (timezone: string) => updateSchedule((draft) => {
  draft.timezone = timezone
})

const setDateRange = (field: 'startDate' | 'endDate', value: string) => updateSchedule((draft) => {
  const next = { ...(draft.dateRange || {}) }
  if (value) {
    next[field] = value
  } else {
    delete next[field]
  }
  draft.dateRange = next.startDate || next.endDate ? next : undefined
})

const addWindow = () => updateSchedule((draft) => {
  draft.windows.push(createAccountScheduleWindowFormRow())
})

const removeWindow = (index: number) => updateSchedule((draft) => {
  if (draft.windows.length > 1) {
    draft.windows.splice(index, 1)
  }
})

const toggleWindowDay = (index: number, day: number) => updateSchedule((draft) => {
  const window = draft.windows[index]
  if (!window) return
  if (window.daysOfWeek.includes(day)) {
    window.daysOfWeek = window.daysOfWeek.filter((item) => item !== day)
  } else {
    window.daysOfWeek = [...window.daysOfWeek, day].sort((left, right) => left - right)
  }
})

const setWindowTime = (index: number, field: 'start' | 'end', value: string) => updateSchedule((draft) => {
  const window = draft.windows[index]
  if (window) {
    window[field] = value
  }
})

const addException = () => updateSchedule((draft) => {
  draft.exceptions.push(createAccountScheduleExceptionFormRow())
})

const removeException = (index: number) => updateSchedule((draft) => {
  draft.exceptions.splice(index, 1)
})

const setExceptionDate = (index: number, value: string) => updateSchedule((draft) => {
  const exception = draft.exceptions[index]
  if (exception) {
    exception.date = value
  }
})

const setExceptionAction = (index: number, value: string) => updateSchedule((draft) => {
  const exception = draft.exceptions[index]
  if (!exception || (value !== 'allow' && value !== 'deny')) return
  exception.action = value
  exception.windows = value === 'allow' ? [createAccountScheduleExceptionWindowFormRow()] : []
})

const addExceptionWindow = (index: number) => updateSchedule((draft) => {
  draft.exceptions[index]?.windows.push(createAccountScheduleExceptionWindowFormRow())
})

const removeExceptionWindow = (index: number, windowIndex: number) => updateSchedule((draft) => {
  const windows = draft.exceptions[index]?.windows
  if (windows && windows.length > 1) {
    windows.splice(windowIndex, 1)
  }
})

const setExceptionWindowTime = (
  index: number,
  windowIndex: number,
  field: 'start' | 'end',
  value: string
) => updateSchedule((draft) => {
  const window = draft.exceptions[index]?.windows[windowIndex]
  if (window) {
    window[field] = value
  }
})

const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value
</script>
