<template>
  <div class="relative">
    <button
      type="button"
      class="btn btn-secondary px-2 md:px-3"
      :title="t('admin.accounts.autoRefresh')"
      @click="$emit('update:open', !open)"
    >
      <Icon name="refresh" size="sm" :class="[enabled ? 'animate-spin' : '']" />
      <span class="hidden md:inline">
        {{
          enabled
            ? t("admin.accounts.autoRefreshCountdown", { seconds: countdown })
            : t("admin.accounts.autoRefresh")
        }}
      </span>
    </button>
    <div
      v-if="open"
      class="absolute right-0 z-50 mt-2 w-56 origin-top-right rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
    >
      <div class="p-2">
        <button
          type="button"
          class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
          @click="$emit('set-enabled', !enabled)"
        >
          <span>{{ t("admin.accounts.enableAutoRefresh") }}</span>
          <Icon v-if="enabled" name="check" size="sm" class="text-primary-500" />
        </button>
        <div class="my-1 border-t border-gray-100 dark:border-gray-700"></div>
        <button
          v-for="sec in intervals"
          :key="sec"
          type="button"
          class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
          @click="emitSetInterval(sec)"
        >
          <span>{{ intervalLabel(sec) }}</span>
          <Icon v-if="intervalSeconds === sec" name="check" size="sm" class="text-primary-500" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import Icon from "@/components/icons/Icon.vue";

const { t } = useI18n();

const props = defineProps<{
  open: boolean;
  enabled: boolean;
  countdown: number;
  intervals: readonly number[];
  intervalSeconds: number;
  intervalLabel: (seconds: number) => string;
}>();

const emit = defineEmits<{
  (event: "update:open", open: boolean): void;
  (event: "set-enabled", enabled: boolean): void;
  (event: "set-interval", seconds: 5 | 10 | 15 | 30): void;
}>();

function emitSetInterval(seconds: number): void {
  if (props.intervals.includes(seconds) && [5, 10, 15, 30].includes(seconds)) {
    emit("set-interval", seconds as 5 | 10 | 15 | 30);
  }
}
</script>
