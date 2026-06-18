<template>
  <div class="space-y-6">
    <div v-if="!form.email_verify_enabled" class="card">
      <div class="p-6">
        <div class="flex items-start gap-3">
          <Icon
            name="mail"
            size="md"
            class="mt-0.5 flex-shrink-0 text-gray-400 dark:text-gray-500"
          />
          <div>
            <h3 class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.emailTabDisabledTitle") }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.emailTabDisabledHint") }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="form.email_verify_enabled" class="card">
      <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t("admin.settings.smtp.title") }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.smtp.description") }}
          </p>
        </div>
        <button
          type="button"
          data-test="test-smtp"
          class="btn btn-secondary btn-sm"
          :disabled="testingSmtp || loadFailed"
          @click="testSmtpConnection"
        >
          <svg
            v-if="testingSmtp"
            class="h-4 w-4 animate-spin"
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
          {{
            testingSmtp
              ? t("admin.settings.smtp.testing")
              : t("admin.settings.smtp.testConnection")
          }}
        </button>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.host") }}
            </label>
            <input
              v-model="form.smtp_host"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.hostPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.port") }}
            </label>
            <input
              v-model.number="form.smtp_port"
              type="number"
              min="1"
              max="65535"
              class="input"
              :placeholder="t('admin.settings.smtp.portPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.username") }}
            </label>
            <input
              v-model="form.smtp_username"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.usernamePlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.password") }}
            </label>
            <input
              v-model="form.smtp_password"
              type="password"
              class="input"
              autocomplete="new-password"
              autocapitalize="off"
              spellcheck="false"
              :placeholder="
                form.smtp_password_configured
                  ? t('admin.settings.smtp.passwordConfiguredPlaceholder')
                  : t('admin.settings.smtp.passwordPlaceholder')
              "
              @keydown="markSmtpPasswordEdited"
              @paste="markSmtpPasswordEdited"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{
                form.smtp_password_configured
                  ? t("admin.settings.smtp.passwordConfiguredHint")
                  : t("admin.settings.smtp.passwordHint")
              }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.fromEmail") }}
            </label>
            <input
              v-model="form.smtp_from_email"
              type="email"
              class="input"
              :placeholder="t('admin.settings.smtp.fromEmailPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.smtp.fromName") }}
            </label>
            <input
              v-model="form.smtp_from_name"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.fromNamePlaceholder')"
            />
          </div>
        </div>

        <div class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.smtp.useTls") }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.smtp.useTlsHint") }}
            </p>
          </div>
          <Toggle v-model="form.smtp_use_tls" />
        </div>
      </div>
    </div>

    <div v-if="form.email_verify_enabled" class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.testEmail.title") }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.testEmail.description") }}
        </p>
      </div>
      <div class="p-6">
        <div class="flex items-end gap-4">
          <div class="flex-1">
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.testEmail.recipientEmail") }}
            </label>
            <input
              :value="testEmailAddress"
              data-test="test-email-address"
              type="email"
              class="input"
              :placeholder="t('admin.settings.testEmail.recipientEmailPlaceholder')"
              @input="emit('update:testEmailAddress', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <button
            type="button"
            data-test="send-test-email"
            class="btn btn-secondary"
            :disabled="sendingTestEmail || !testEmailAddress || loadFailed"
            @click="sendTestEmail"
          >
            <svg
              v-if="sendingTestEmail"
              class="h-4 w-4 animate-spin"
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
            {{
              sendingTestEmail
                ? t("admin.settings.testEmail.sending")
                : t("admin.settings.testEmail.sendTestEmail")
            }}
          </button>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h3 class="text-base font-medium text-gray-900 dark:text-white">
          {{ t("admin.settings.subscriptionExpiryNotify.title") }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.subscriptionExpiryNotify.description") }}
        </p>
      </div>
      <div class="px-6 py-6">
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.subscriptionExpiryNotify.enabled") }}
            </label>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.subscriptionExpiryNotify.enabledHint") }}
            </p>
          </div>
          <Toggle v-model="form.subscription_expiry_notify_enabled" />
        </div>
      </div>
    </div>

    <EmailTemplateEditor />

    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h3 class="text-base font-medium text-gray-900 dark:text-white">
          {{ t("admin.settings.balanceNotify.title") }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.balanceNotify.description") }}
        </p>
      </div>
      <div class="space-y-4 px-6 py-6">
        <div class="flex items-center justify-between">
          <label class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.balanceNotify.enabled") }}
          </label>
          <Toggle v-model="form.balance_low_notify_enabled" />
        </div>
        <div v-if="form.balance_low_notify_enabled">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.balanceNotify.threshold") }}
          </label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">$</span>
            <input
              v-model.number="form.balance_low_notify_threshold"
              type="number"
              min="0"
              step="0.01"
              class="input pl-7"
            />
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.balanceNotify.thresholdHint") }}
          </p>
        </div>
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.balanceNotify.rechargeUrl") }}
          </label>
          <input
            v-model="form.balance_low_notify_recharge_url"
            type="url"
            class="input"
            :placeholder="currentOrigin"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.balanceNotify.rechargeUrlHint") }}
          </p>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h3 class="text-base font-medium text-gray-900 dark:text-white">
          {{ t("admin.settings.quotaNotify.title") }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.quotaNotify.description") }}
        </p>
      </div>
      <div class="space-y-4 px-6 py-6">
        <div class="flex items-center justify-between">
          <label class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.quotaNotify.enabled") }}
          </label>
          <Toggle v-model="form.account_quota_notify_enabled" />
        </div>
        <div v-if="form.account_quota_notify_enabled">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.quotaNotify.emails") }}
          </label>
          <div class="space-y-2">
            <div
              v-for="(entry, index) in form.account_quota_notify_emails || []"
              :key="index"
              class="flex items-center gap-2"
            >
              <label class="relative inline-flex shrink-0 cursor-pointer items-center">
                <input
                  type="checkbox"
                  :checked="!entry.disabled"
                  class="peer sr-only"
                  @change="entry.disabled = !entry.disabled"
                />
                <div class="peer h-5 w-9 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none dark:bg-gray-600 dark:after:border-gray-500"></div>
              </label>
              <input
                v-model="entry.email"
                type="email"
                class="input flex-1"
                :placeholder="t('admin.settings.quotaNotify.emailPlaceholder')"
              />
              <button
                type="button"
                :data-test="`remove-quota-email-${index}`"
                class="btn btn-secondary px-2"
                @click="form.account_quota_notify_emails.splice(index, 1)"
              >
                <Icon name="x" size="xs" class="h-4 w-4" />
              </button>
            </div>
            <button
              type="button"
              data-test="add-quota-email"
              class="btn btn-secondary btn-sm"
              @click="addQuotaNotifyEmail"
            >
              + {{ t("admin.settings.quotaNotify.addEmail") }}
            </button>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.quotaNotify.emailsHint") }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

import Toggle from "@/components/common/Toggle.vue";
import Icon from "@/components/icons/Icon.vue";
import EmailTemplateEditor from "@/views/admin/settings/EmailTemplateEditor.vue";
import type { NotifyEmailEntry } from "@/types";

type EmailSettingsForm = {
  email_verify_enabled: boolean;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  smtp_password_configured: boolean;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
  subscription_expiry_notify_enabled: boolean;
  balance_low_notify_enabled: boolean;
  balance_low_notify_threshold: number;
  balance_low_notify_recharge_url: string;
  account_quota_notify_enabled: boolean;
  account_quota_notify_emails: NotifyEmailEntry[];
};

defineProps<{
  form: EmailSettingsForm;
  testEmailAddress: string;
  testingSmtp: boolean;
  sendingTestEmail: boolean;
  loadFailed: boolean;
  currentOrigin: string;
  testSmtpConnection: () => void | Promise<void>;
  sendTestEmail: () => void | Promise<void>;
  addQuotaNotifyEmail: () => void;
  markSmtpPasswordEdited: () => void;
}>();

const emit = defineEmits<{
  (event: "update:testEmailAddress", value: string): void;
}>();

const { t } = useI18n();
</script>
