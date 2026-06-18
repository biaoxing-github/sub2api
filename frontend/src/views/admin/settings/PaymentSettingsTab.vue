<template>
  <div class="space-y-6">
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.payment.title") }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.payment.description") }}
          <a
            :href="paymentGuideHref"
            target="_blank"
            rel="noopener noreferrer"
            class="ml-2 inline-flex items-center text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
          >
            <svg class="mr-0.5 h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
              />
            </svg>
            {{ t("admin.settings.payment.configGuide") }}
          </a>
        </p>
      </div>
      <div class="space-y-4 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.payment.enabled") }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.payment.enabledHint") }}
            </p>
          </div>
          <Toggle v-model="form.payment_enabled" />
        </div>

        <template v-if="form.payment_enabled">
          <div class="grid grid-cols-3 gap-3">
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.productNamePrefix") }}
              </label>
              <input
                v-model="form.payment_product_name_prefix"
                type="text"
                class="input"
                placeholder="Sub2API"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.productNameSuffix") }}
              </label>
              <input
                v-model="form.payment_product_name_suffix"
                type="text"
                class="input"
                placeholder="CNY"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.preview") }}
              </label>
              <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">
                {{ (form.payment_product_name_prefix || "Sub2API") + " 100 " + (form.payment_product_name_suffix || "CNY") }}
              </div>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3 sm:grid-cols-5">
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.minAmount") }}
              </label>
              <input
                :value="form.payment_min_amount || ''"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
                @input="form.payment_min_amount = parseFloat(($event.target as HTMLInputElement).value) || 0"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.maxAmount") }}
              </label>
              <input
                :value="form.payment_max_amount || ''"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
                @input="form.payment_max_amount = parseFloat(($event.target as HTMLInputElement).value) || 0"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.dailyLimit") }}
              </label>
              <input
                :value="form.payment_daily_limit || ''"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
                @input="form.payment_daily_limit = parseFloat(($event.target as HTMLInputElement).value) || 0"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.balanceRechargeMultiplier") }}
              </label>
              <input
                :value="form.payment_balance_recharge_multiplier || ''"
                type="number"
                step="0.01"
                min="0.01"
                class="input"
                @input="form.payment_balance_recharge_multiplier = parseFloat(($event.target as HTMLInputElement).value) || 1"
              />
              <p class="mt-0.5 text-xs text-gray-400">
                {{ t("admin.settings.payment.balanceRechargeMultiplierHint") }}
              </p>
              <p class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-400">
                {{
                  t("admin.settings.payment.balanceRechargePreview", {
                    usd: (Number(form.payment_balance_recharge_multiplier) || 1).toFixed(2),
                  })
                }}
              </p>
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.rechargeFeeRate") }}
              </label>
              <div class="relative">
                <input
                  :value="form.payment_recharge_fee_rate ?? ''"
                  type="number"
                  step="0.01"
                  min="0"
                  max="100"
                  class="input pr-8"
                  @input="form.payment_recharge_fee_rate = clampPercentage(($event.target as HTMLInputElement).value)"
                />
                <span class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400">%</span>
              </div>
              <p class="mt-0.5 text-xs text-gray-400">
                {{ t("admin.settings.payment.rechargeFeeRateHint") }}
              </p>
              <p
                v-if="(Number(form.payment_recharge_fee_rate) || 0) > 0"
                class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-400"
              >
                {{
                  t("admin.settings.payment.rechargeFeePreview", {
                    fee: (Number(form.payment_recharge_fee_rate) || 0).toFixed(2),
                  })
                }}
              </p>
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.orderTimeout") }}
                <span class="text-red-500">*</span>
              </label>
              <input
                v-model.number="form.payment_order_timeout_minutes"
                type="number"
                min="1"
                class="input"
                required
              />
              <p class="mt-0.5 text-xs text-gray-400">
                {{ t("admin.settings.payment.orderTimeoutHint") }}
              </p>
            </div>
          </div>

          <div class="flex flex-wrap items-end gap-4">
            <div class="w-28">
              <label class="input-label">
                {{ t("admin.settings.payment.maxPendingOrders") }}
              </label>
              <input
                v-model.number="form.payment_max_pending_orders"
                type="number"
                min="1"
                class="input"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.loadBalanceStrategy") }}
              </label>
              <Select
                v-model="form.payment_load_balance_strategy"
                :options="loadBalanceOptions"
                class="w-40"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.cancelRateLimit") }}
              </label>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
                    form.payment_cancel_rate_limit_enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
                  ]"
                  @click="form.payment_cancel_rate_limit_enabled = !form.payment_cancel_rate_limit_enabled"
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                      form.payment_cancel_rate_limit_enabled ? 'translate-x-5' : 'translate-x-0',
                    ]"
                  />
                </button>
                <Select
                  v-model="form.payment_cancel_rate_limit_window_mode"
                  :options="cancelRateLimitModeOptions"
                  class="w-24"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span :class="rateLimitLabelClass">
                  {{ t("admin.settings.payment.cancelRateLimitEvery") }}
                </span>
                <input
                  v-model.number="form.payment_cancel_rate_limit_window"
                  type="number"
                  min="1"
                  required
                  class="input w-14 text-center"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <Select
                  v-model="form.payment_cancel_rate_limit_unit"
                  :options="cancelRateLimitUnitOptions"
                  class="w-28"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span :class="rateLimitLabelClass">
                  {{ t("admin.settings.payment.cancelRateLimitAllowMax") }}
                </span>
                <input
                  v-model.number="form.payment_cancel_rate_limit_max"
                  type="number"
                  min="1"
                  required
                  class="input w-14 text-center"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span :class="rateLimitLabelClass">
                  {{ t("admin.settings.payment.cancelRateLimitTimes") }}
                </span>
              </div>
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.alipayForceQRCode") }}
              </label>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
                    form.payment_alipay_force_qrcode ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
                  ]"
                  @click="form.payment_alipay_force_qrcode = !form.payment_alipay_force_qrcode"
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                      form.payment_alipay_force_qrcode ? 'translate-x-5' : 'translate-x-0',
                    ]"
                  />
                </button>
                <span class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.payment.alipayForceQRCodeHint") }}
                </span>
              </div>
            </div>
          </div>

          <div>
            <label class="input-label">
              {{ t("admin.settings.payment.enabledPaymentTypes") }}
            </label>
            <div class="mt-1.5 flex flex-wrap gap-2">
              <button
                v-for="pt in allPaymentTypes"
                :key="pt.value"
                type="button"
                :data-test="`payment-type-${pt.value}`"
                :class="[
                  'rounded-lg border px-3 py-1.5 text-sm font-medium transition-all',
                  isPaymentTypeEnabled(pt.value)
                    ? 'border-primary-500 bg-primary-500 text-white shadow-sm'
                    : 'border-gray-300 bg-white text-gray-600 hover:border-gray-400 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:border-dark-500',
                ]"
                @click="togglePaymentType(pt.value)"
              >
                {{ pt.label }}
              </button>
            </div>
            <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
              {{ t("admin.settings.payment.enabledPaymentTypesHint") }}
              <a
                :href="paymentMethodsHref"
                target="_blank"
                rel="noopener noreferrer"
                class="ml-1 text-primary-500 hover:text-primary-600 dark:text-primary-400 dark:hover:text-primary-300"
              >
                {{ t("admin.settings.payment.findProvider") }}
                <svg class="mb-0.5 ml-0.5 inline h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                  />
                </svg>
              </a>
            </p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.helpImage") }}
              </label>
              <ImageUpload
                v-model="form.payment_help_image_url"
                :upload-label="t('admin.settings.site.uploadImage')"
                :remove-label="t('admin.settings.site.remove')"
                :placeholder="t('admin.settings.payment.helpImagePlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">
                {{ t("admin.settings.payment.helpText") }}
              </label>
              <textarea
                v-model="form.payment_help_text"
                rows="3"
                class="input"
                :placeholder="t('admin.settings.payment.helpTextPlaceholder')"
              ></textarea>
            </div>
          </div>
        </template>
      </div>
    </div>

    <PaymentProviderList
      v-if="form.payment_enabled"
      :providers="providers"
      :loading="providersLoading"
      :can-create="hasAnyPaymentTypeEnabled"
      :enabled-payment-types="form.payment_enabled_types"
      :all-payment-types="allPaymentTypes"
      :redirect-label="t('admin.settings.payment.easypayRedirect')"
      @refresh="loadProviders"
      @create="openCreateProvider"
      @edit="openEditProvider"
      @delete="confirmDeleteProvider"
      @toggle-field="handleToggleField"
      @toggle-type="handleToggleType"
      @reorder="handleReorderProviders"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import ImageUpload from "@/components/common/ImageUpload.vue";
import Select, { type SelectOption } from "@/components/common/Select.vue";
import Toggle from "@/components/common/Toggle.vue";
import PaymentProviderList from "@/components/payment/PaymentProviderList.vue";
import type { ProviderInstance } from "@/types/payment";

type PaymentTypeOption = {
  value: string;
  label: string;
};

type PaymentSettingsForm = {
  payment_enabled: boolean;
  payment_product_name_prefix: string;
  payment_product_name_suffix: string;
  payment_min_amount: number;
  payment_max_amount: number;
  payment_daily_limit: number;
  payment_balance_recharge_multiplier: number;
  payment_recharge_fee_rate: number;
  payment_order_timeout_minutes: number;
  payment_max_pending_orders: number;
  payment_load_balance_strategy: string;
  payment_cancel_rate_limit_enabled: boolean;
  payment_cancel_rate_limit_window_mode: string;
  payment_cancel_rate_limit_window: number;
  payment_cancel_rate_limit_unit: string;
  payment_cancel_rate_limit_max: number;
  payment_alipay_force_qrcode?: boolean;
  payment_enabled_types: string[];
  payment_help_image_url: string;
  payment_help_text: string;
};

const props = defineProps<{
  form: PaymentSettingsForm;
  paymentGuideHref: string;
  paymentMethodsHref: string;
  providers: ProviderInstance[];
  providersLoading: boolean;
  hasAnyPaymentTypeEnabled: boolean;
  allPaymentTypes: PaymentTypeOption[];
  loadBalanceOptions: SelectOption[];
  cancelRateLimitModeOptions: SelectOption[];
  cancelRateLimitUnitOptions: SelectOption[];
  togglePaymentType: (type: string) => void;
  isPaymentTypeEnabled: (type: string) => boolean;
  loadProviders: () => void | Promise<void>;
  openCreateProvider: () => void;
  openEditProvider: (provider: ProviderInstance) => void;
  confirmDeleteProvider: (provider: ProviderInstance) => void;
  handleToggleField: (
    provider: ProviderInstance,
    field: "enabled" | "refund_enabled" | "allow_user_refund",
  ) => void | Promise<void>;
  handleToggleType: (provider: ProviderInstance, type: string) => void | Promise<void>;
  handleReorderProviders: (
    providers: { id: number; sort_order: number }[],
  ) => void | Promise<void>;
}>();

const { t } = useI18n();

const rateLimitLabelClass = computed(() => [
  "whitespace-nowrap text-sm",
  props.form.payment_cancel_rate_limit_enabled
    ? "text-gray-700 dark:text-gray-300"
    : "text-gray-400 dark:text-gray-600",
]);

function clampPercentage(value: string): number {
  return Math.min(
    100,
    Math.max(0, Math.round(parseFloat(value || "0") * 100) / 100),
  );
}
</script>
