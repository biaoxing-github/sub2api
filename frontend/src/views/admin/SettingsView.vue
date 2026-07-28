<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"
        ></div>
      </div>

      <!-- Settings Form -->
      <form v-else @submit.prevent="saveSettings" class="space-y-6" novalidate>
        <SettingsTabNavigation
          :tabs="settingsTabs"
          :active-tab="activeTab"
          :ariaLabel="t('admin.settings.title')"
          @select="selectSettingsTab"
        />

        <SecuritySettingsTab
          v-if="activeTab === 'security'"
          :form="form"
          :admin-api-key-loading="adminApiKeyLoading"
          :admin-api-key-exists="adminApiKeyExists"
          :admin-api-key-masked="adminApiKeyMasked"
          :admin-api-key-operating="adminApiKeyOperating"
          :new-admin-api-key="newAdminApiKey"
          :registration-email-suffix-whitelist-tags="registrationEmailSuffixWhitelistTags"
          v-model:registration-email-suffix-whitelist-draft="registrationEmailSuffixWhitelistDraft"
          :linuxdo-redirect-url-suggestion="linuxdoRedirectUrlSuggestion"
          :githubOAuthRedirectUrlSuggestion="githubOAuthRedirectUrlSuggestion"
          :googleOAuthRedirectUrlSuggestion="googleOAuthRedirectUrlSuggestion"
          :wechat-redirect-url-suggestion="wechatRedirectUrlSuggestion"
          :oidc-redirect-url-suggestion="oidcRedirectUrlSuggestion"
          :local-text="localText"
          :create-admin-api-key="createAdminApiKey"
          :regenerate-admin-api-key="regenerateAdminApiKey"
          :delete-admin-api-key="deleteAdminApiKey"
          :copy-new-key="copyNewKey"
          :remove-registration-email-suffix-whitelist-tag="removeRegistrationEmailSuffixWhitelistTag"
          :handle-registration-email-suffix-whitelist-draft-input="handleRegistrationEmailSuffixWhitelistDraftInput"
          :handle-registration-email-suffix-whitelist-draft-keydown="handleRegistrationEmailSuffixWhitelistDraftKeydown"
          :commit-registration-email-suffix-whitelist-draft="commitRegistrationEmailSuffixWhitelistDraft"
          :handle-registration-email-suffix-whitelist-paste="handleRegistrationEmailSuffixWhitelistPaste"
          :set-and-copy-linuxdo-redirect-url="setAndCopyLinuxdoRedirectUrl"
          :setAndCopyEmailOAuthRedirectUrl="setAndCopyEmailOAuthRedirectUrl"
          :setAndCopyWeChatRedirectUrl="setAndCopyWeChatRedirectUrl"
          :setAndCopyOIDCRedirectUrl="setAndCopyOIDCRedirectUrl"
          :handleWeChatOpenEnabledChange="handleWeChatOpenEnabledChange"
          :handleWeChatMPEnabledChange="handleWeChatMPEnabledChange"
          :handleWeChatMobileEnabledChange="handleWeChatMobileEnabledChange"
        />

        <GatewaySettingsTab
          v-if="activeTab === 'gateway'"
          :form="form"
          :overload-cooldown-loading="overloadCooldownLoading"
          :overload-cooldown-saving="overloadCooldownSaving"
          :overload-cooldown-form="overloadCooldownForm"
          :save-overload-cooldown-settings="saveOverloadCooldownSettings"
          :rate-limit429-cooldown-loading="rateLimit429CooldownLoading"
          :rate-limit429-cooldown-saving="rateLimit429CooldownSaving"
          :rate-limit429-cooldown-form="rateLimit429CooldownForm"
          :save-rate-limit429-cooldown-settings="saveRateLimit429CooldownSettings"
          :stream-timeout-loading="streamTimeoutLoading"
          :stream-timeout-saving="streamTimeoutSaving"
          :stream-timeout-form="streamTimeoutForm"
          :save-stream-timeout-settings="saveStreamTimeoutSettings"
          :rectifier-loading="rectifierLoading"
          :rectifier-saving="rectifierSaving"
          :rectifier-form="rectifierForm"
          :save-rectifier-settings="saveRectifierSettings"
          :beta-policy-loading="betaPolicyLoading"
          :beta-policy-saving="betaPolicySaving"
          :beta-policy-form="betaPolicyForm"
          :beta-policy-action-options="betaPolicyActionOptions"
          :beta-policy-scope-options="betaPolicyScopeOptions"
          :beta-presets="betaPresets"
          :common-model-patterns="commonModelPatterns"
          :get-beta-display-name="getBetaDisplayName"
          :apply-beta-preset="applyBetaPreset"
          :add-quick-pattern="addQuickPattern"
          :save-beta-policy-settings="saveBetaPolicySettings"
          :openai-fast-policy-form="openaiFastPolicyForm"
          :openai-route-policy-presets="openaiRoutePolicyPresets"
          :applyOpenAIRoutePolicyPreset="applyOpenAIRoutePolicyPreset"
          :openai-fast-policy-tier-options="openaiFastPolicyTierOptions"
          :openai-fast-policy-action-options="openaiFastPolicyActionOptions"
          :openai-fast-policy-scope-options="openaiFastPolicyScopeOptions"
          :addOpenAIFastPolicyRule="addOpenAIFastPolicyRule"
          :removeOpenAIFastPolicyRule="removeOpenAIFastPolicyRule"
          :addOpenAIFastPolicyUserID="addOpenAIFastPolicyUserID"
          :removeOpenAIFastPolicyUserID="removeOpenAIFastPolicyUserID"
          :addOpenAIFastPolicyModelPattern="addOpenAIFastPolicyModelPattern"
          :removeOpenAIFastPolicyModelPattern="removeOpenAIFastPolicyModelPattern"
          :codexDirectTLSFingerprintProfiles="codexDirectTLSFingerprintProfiles"
          :web-search-config="webSearchConfig"
          :expanded-providers="expandedProviders"
          :api-key-visible="apiKeyVisible"
          :web-search-proxies="webSearchProxies"
          v-model:ws-test-query="wsTestQuery"
          v-model:ws-test-dialog-open="wsTestDialogOpen"
          :ws-test-loading="wsTestLoading"
          :ws-test-result="wsTestResult"
          :open-test-dialog="openTestDialog"
          :toggle-provider-expand="toggleProviderExpand"
          :remove-web-search-provider="removeWebSearchProvider"
          :add-web-search-provider="addWebSearchProvider"
          :format-subscribed-at="formatSubscribedAt"
          :parse-subscribed-at="parseSubscribedAt"
          :quota-percentage="quotaPercentage"
          :reset-web-search-usage="resetWebSearchUsage"
          :copy-api-key="copyApiKey"
          :test-web-search-provider="testWebSearchProvider"
        />



        <!-- Tab: Users -->
        <UsersSettingsTab
          v-if="activeTab === 'users'"
          :form="form"
          :subscription-groups="subscriptionGroups"
          :default-subscription-group-options="defaultSubscriptionGroupOptions"
          :auth-source-defaults="authSourceDefaults"
          :auth-source-defaults-meta="authSourceDefaultsMeta"
          :add-default-subscription="addDefaultSubscription"
          :remove-default-subscription="removeDefaultSubscription"
          :add-auth-source-default-subscription="addAuthSourceDefaultSubscription"
          :remove-auth-source-default-subscription="removeAuthSourceDefaultSubscription"
        />
        <!-- /Tab: Users -->


        <GeneralSettingsTab
          v-if="activeTab === 'general'"
          :form="form"
          v-model:table-page-size-options-input="tablePageSizeOptionsInput"
          @add-endpoint="addEndpoint"
          @remove-endpoint="removeEndpoint"
          @add-menu-item="addMenuItem"
          @remove-menu-item="removeMenuItem"
          @move-menu-item="moveMenuItem"
        />

        <AgreementSettingsTab
          v-if="activeTab === 'agreement'"
          :form="form"
          :local-text="localText"
          :login-agreement-route-path="loginAgreementRoutePath"
          @add-document="addLoginAgreementDocument"
          @remove-document="removeLoginAgreementDocument"
        />

        <FeaturesSettingsTab
          v-if="activeTab === 'features'"
          :form="form"
          :affiliate-state="affiliateState"
          :affiliate-modal="affiliateModal"
          :affiliate-batch-modal="affiliateBatchModal"
          :affiliate-modal-can-submit="affiliateModalCanSubmit"
          :open-affiliate-modal="openAffiliateModal"
          :on-affiliate-search-input="onAffiliateSearchInput"
          :open-affiliate-batch-modal="openAffiliateBatchModal"
          :toggle-affiliate-select-all="toggleAffiliateSelectAll"
          :toggle-affiliate-select="toggleAffiliateSelect"
          :change-affiliate-page="changeAffiliatePage"
          :ask-reset-affiliate-user="askResetAffiliateUser"
          :close-affiliate-modal="closeAffiliateModal"
          :clear-selected-affiliate-user="clearSelectedAffiliateUser"
          :on-affiliate-user-search-input="onAffiliateUserSearchInput"
          :select-affiliate-user="selectAffiliateUser"
          :submit-affiliate-modal="submitAffiliateModal"
          :submit-affiliate-batch-modal="submitAffiliateBatchModal"
        />

        <!-- Tab: Email -->
        <PaymentSettingsTab
          v-if="activeTab === 'payment'"
          :form="form"
          :payment-guide-href="paymentGuideHref"
          :payment-methods-href="paymentMethodsHref"
          :providers="providers"
          :providers-loading="providersLoading"
          :has-any-payment-type-enabled="hasAnyPaymentTypeEnabled"
          :all-payment-types="allPaymentTypes"
          :load-balance-options="loadBalanceOptions"
          :cancel-rate-limit-mode-options="cancelRateLimitModeOptions"
          :cancel-rate-limit-unit-options="cancelRateLimitUnitOptions"
          :toggle-payment-type="togglePaymentType"
          :is-payment-type-enabled="isPaymentTypeEnabled"
          :load-providers="loadProviders"
          :open-create-provider="openCreateProvider"
          :open-edit-provider="openEditProvider"
          :confirm-delete-provider="confirmDeleteProvider"
          :handle-toggle-field="handleToggleField"
          :handle-toggle-type="handleToggleType"
          :handle-reorder-providers="handleReorderProviders"
        />
        <EmailSettingsTab
          v-if="activeTab === 'email'"
          :form="form"
          v-model:test-email-address="testEmailAddress"
          :testing-smtp="testingSmtp"
          :sending-test-email="sendingTestEmail"
          :load-failed="loadFailed"
          :current-origin="currentOrigin"
          :test-smtp-connection="testSmtpConnection"
          :send-test-email="sendTestEmail"
          :add-quota-notify-email="addQuotaNotifyEmail"
          :mark-smtp-password-edited="() => (smtpPasswordManuallyEdited = true)"
        />

        <!-- Tab: Backup -->
        <div v-if="activeTab === 'backup'">
          <BackupSettings />
        </div>

        <SettingsSaveBar
          v-if="activeTab !== 'backup'"
          :saving="saving"
          :load-failed="loadFailed"
        />
      </form>

      <!-- Provider dialogs placed outside the settings form to prevent form submission bubbling -->
      <PaymentProviderDialog
        ref="providerDialogRef"
        :show="showProviderDialog"
        :saving="providerSaving"
        :editing="editingProvider"
        :all-key-options="providerKeyOptions"
        :enabled-key-options="enabledProviderKeyOptions"
        :all-payment-types="allPaymentTypes"
        :redirect-label="t('admin.settings.payment.easypayRedirect')"
        @close="showProviderDialog = false"
        @save="handleSaveProvider"
      />
      <ConfirmDialog
        :show="showDeleteProviderDialog"
        :title="t('admin.settings.payment.deleteProvider')"
        :message="t('admin.settings.payment.deleteProviderConfirm')"
        :confirm-text="t('common.delete')"
        danger
        @confirm="handleDeleteProvider"
        @cancel="showDeleteProviderDialog = false"
      />
      <ConfirmDialog
        :show="affiliateConfirmDialog.show"
        :title="affiliateConfirmDialog.title"
        :message="affiliateConfirmDialog.message"
        :confirm-text="affiliateConfirmDialog.confirmText"
        danger
        @confirm="handleAffiliateConfirm"
        @cancel="cancelAffiliateConfirm"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api";
import {
  appendAuthSourceDefaultsToUpdateRequest,
  buildAuthSourceDefaultsState,
  defaultWeChatConnectScopesForMode,
  deriveWeChatConnectStoredMode,
  normalizeDefaultSubscriptionSettings,
  resolveWeChatConnectModeCapabilities,
} from "@/api/admin/settings";
import type {
  AuthSourceDefaultsState,
  AuthSourceType,
  SystemSettings,
  UpdateSettingsRequest,
  DefaultSubscriptionSetting,
  OpenAIOAuthCompatMode,
  OpenAIFastPolicyRule,
  WeChatConnectMode,
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
  WebSearchTestResult,
} from "@/api/admin/settings";
import type { TLSFingerprintProfile } from "@/api/admin/tlsFingerprintProfile";
import type {
  AdminGroup,
  LoginAgreementDocument,
  NotifyEmailEntry,
  Proxy,
} from "@/types";
import type { ProviderInstance } from "@/types/payment";
import AppLayout from "@/components/layout/AppLayout.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import PaymentProviderDialog from "@/components/payment/PaymentProviderDialog.vue";
import BackupSettings from "@/views/admin/BackupView.vue";
import AgreementSettingsTab from "@/views/admin/settings/AgreementSettingsTab.vue";
import EmailSettingsTab from "@/views/admin/settings/EmailSettingsTab.vue";
import FeaturesSettingsTab from "@/views/admin/settings/FeaturesSettingsTab.vue";
import GatewaySettingsTab from "@/views/admin/settings/GatewaySettingsTab.vue";
import GeneralSettingsTab from "@/views/admin/settings/GeneralSettingsTab.vue";
import PaymentSettingsTab from "@/views/admin/settings/PaymentSettingsTab.vue";
import SecuritySettingsTab from "@/views/admin/settings/SecuritySettingsTab.vue";
import UsersSettingsTab from "@/views/admin/settings/UsersSettingsTab.vue";
import SettingsTabNavigation, {
  type SettingsTabNavigationItem,
  type SettingsTabNavigationKey,
} from "@/components/admin/settings/SettingsTabNavigation.vue";
import SettingsSaveBar from "@/components/admin/settings/SettingsSaveBar.vue";
import { useClipboard } from "@/composables/useClipboard";
import { affiliatesAPI, type AffiliateAdminEntry, type SimpleUser as AffiliateSimpleUser } from "@/api/admin/affiliates";
import { extractApiErrorMessage, extractI18nErrorMessage } from "@/utils/apiError";
import { useAppStore } from "@/stores";
import { useAdminSettingsStore } from "@/stores/adminSettings";
import { normalizeVisibleMethod } from "@/components/payment/paymentFlow";
import {
  isRegistrationEmailSuffixDomainValid,
  normalizeRegistrationEmailSuffixDomain,
  normalizeRegistrationEmailSuffixDomains,
  parseRegistrationEmailSuffixWhitelistInput,
} from "@/utils/registrationEmailPolicy";

const { t, locale } = useI18n();
const appStore = useAppStore();
const adminSettingsStore = useAdminSettingsStore();
const isZhLocale = computed(() => locale.value.startsWith("zh"));

function localText(zh: string, en: string): string {
  return isZhLocale.value ? zh : en;
}

const paymentGuideHref = computed(() =>
  locale.value.startsWith("zh")
    ? "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md"
    : "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT.md",
);

const paymentMethodsHref = computed(() =>
  locale.value.startsWith("zh")
    ? "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md#支持的支付方式"
    : "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT.md#supported-payment-methods",
);

type SettingsTab = SettingsTabNavigationKey;
const activeTab = ref<SettingsTab>("general");
const settingsTabs: SettingsTabNavigationItem[] = [
  { key: "general", icon: "home" },
  { key: "agreement", icon: "document" },
  { key: "features", icon: "bolt" },
  { key: "security", icon: "shield" },
  { key: "users", icon: "user" },
  { key: "gateway", icon: "server" },
  { key: "payment", icon: "creditCard" },
  { key: "email", icon: "mail" },
  { key: "backup", icon: "database" },
];

function selectSettingsTab(tab: SettingsTab): void {
  activeTab.value = tab;
}

const { copyToClipboard } = useClipboard();

const loading = ref(true);
const loadFailed = ref(false);
const saving = ref(false);
const testingSmtp = ref(false);
const sendingTestEmail = ref(false);
const smtpPasswordManuallyEdited = ref(false);
const testEmailAddress = ref("");
const registrationEmailSuffixWhitelistTags = ref<string[]>([]);
const registrationEmailSuffixWhitelistDraft = ref("");
const tablePageSizeOptionsInput = ref("10, 20, 50, 100");

// Admin API Key 状态
const adminApiKeyLoading = ref(true);
const adminApiKeyExists = ref(false);
const adminApiKeyMasked = ref("");
const adminApiKeyOperating = ref(false);
const newAdminApiKey = ref("");
const subscriptionGroups = ref<AdminGroup[]>([]);
const codexDirectTLSFingerprintProfiles = ref<TLSFingerprintProfile[]>([]);

// Overload Cooldown (529) 状态
const overloadCooldownLoading = ref(true);
const overloadCooldownSaving = ref(false);
const overloadCooldownForm = reactive({
  enabled: true,
  cooldown_minutes: 10,
});

// Rate Limit Cooldown (429) 状态
const rateLimit429CooldownLoading = ref(true);
const rateLimit429CooldownSaving = ref(false);
const rateLimit429CooldownForm = reactive({
  enabled: true,
  cooldown_seconds: 5,
});

// Stream Timeout 状态
const streamTimeoutLoading = ref(true);
const streamTimeoutSaving = ref(false);
const streamTimeoutForm = reactive({
  enabled: true,
  action: "temp_unsched" as "temp_unsched" | "error" | "none",
  temp_unsched_minutes: 5,
  threshold_count: 3,
  threshold_window_minutes: 10,
});

// Rectifier 状态
const rectifierLoading = ref(true);
const rectifierSaving = ref(false);
const rectifierForm = reactive({
  enabled: true,
  thinking_signature_enabled: true,
  thinking_budget_enabled: true,
  apikey_signature_enabled: false,
  apikey_signature_patterns: [] as string[],
});

// Beta Policy 状态
const betaPolicyLoading = ref(true);
const betaPolicySaving = ref(false);
const betaPolicyForm = reactive({
  rules: [] as Array<{
    beta_token: string;
    action: "pass" | "filter" | "block";
    scope: "all" | "oauth" | "apikey" | "bedrock";
    error_message?: string;
    model_whitelist?: string[];
    fallback_action?: "pass" | "filter" | "block";
    fallback_error_message?: string;
  }>,
});

// OpenAI Fast/Flex Policy 状态
const openaiFastPolicyForm = reactive({
  rules: [] as OpenAIFastPolicyRule[],
});
// 标记 openai_fast_policy_settings 是否已成功从后端加载，
// 避免后端 GET 出错或字段缺失时，保存把默认规则覆盖成空数组。
const openaiFastPolicyLoaded = ref(false);

const tablePageSizeMin = 5;
const tablePageSizeMax = 1000;
const tablePageSizeDefault = 20;

function defaultLoginAgreementDocuments(): LoginAgreementDocument[] {
  return [
    {
      id: "terms",
      title: "服务条款",
      content_md: "",
    },
    {
      id: "usage-policy",
      title: "使用政策",
      content_md: "",
    },
    {
      id: "supported-regions",
      title: "支持的国家和地区",
      content_md: "",
    },
    {
      id: "service-specific-terms",
      title: "服务特定条款",
      content_md: "",
    },
  ];
}

function normalizeLoginAgreementDocumentId(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/[-_]{2,}/g, "-")
    .replace(/^[-_]+|[-_]+$/g, "");
}

function loginAgreementRoutePath(
  doc: LoginAgreementDocument,
  index: number,
): string {
  const id =
    normalizeLoginAgreementDocumentId(doc.id || doc.title) || `doc-${index + 1}`;
  return `/legal/${id}`;
}

interface DefaultSubscriptionGroupOption {
  value: number;
  label: string;
  description: string | null;
  platform: AdminGroup["platform"];
  subscriptionType: AdminGroup["subscription_type"];
  rate: number;
  [key: string]: unknown;
}

type SettingsForm = Omit<
  SystemSettings,
  | "wechat_connect_open_enabled"
  | "wechat_connect_mp_enabled"
  | "wechat_connect_mobile_enabled"
> & {
  smtp_password: string;
  turnstile_secret_key: string;
  linuxdo_connect_client_secret: string;
  dingtalk_connect_client_secret: string;
  wechat_connect_app_secret: string;
  wechat_connect_open_app_secret: string;
  wechat_connect_mp_app_secret: string;
  wechat_connect_mobile_app_secret: string;
  wechat_connect_open_enabled: boolean;
  wechat_connect_mp_enabled: boolean;
  wechat_connect_mobile_enabled: boolean;
  oidc_connect_client_secret: string;
  github_oauth_client_secret: string;
  google_oauth_client_secret: string;
  force_email_on_third_party_signup: boolean;
  openai_advanced_scheduler_enabled: boolean;
  openai_scheduler_exhaustion_probe_infinite_wait_enabled: boolean;
  openai_scheduler_exhaustion_probe_notify_enabled: boolean;
  openai_scheduler_exhaustion_probe_notify_channel: string;
  openai_scheduler_exhaustion_probe_notify_after_seconds: number;
  openai_scheduler_exhaustion_probe_notify_repeat_seconds: number;
  openai_scheduler_exhaustion_probe_notify_feishu_webhook_url: string;
  openai_scheduler_exhaustion_probe_notify_feishu_app_id: string;
  openai_scheduler_exhaustion_probe_notify_feishu_app_secret: string;
  openai_scheduler_exhaustion_probe_notify_feishu_domain: string;
  openai_scheduler_exhaustion_probe_notify_feishu_receive_id_type: string;
  openai_scheduler_exhaustion_probe_notify_feishu_receive_id: string;
  openai_scheduler_exhaustion_probe_notify_recovered_enabled: boolean;
  // OpenAI OAuth 上游兼容模式：关闭或 Cockpit Tools；codex_direct 历史返回值会显示为关闭。
  openai_oauth_compat_mode: OpenAIOAuthCompatMode;
  openai_cockpit_tools_compat: boolean;
  // 账号级 Codex CLI 模拟开启时，允许把 Codex HTTP/SSE 入站强制转为上游 WSv2。
  openai_codex_direct_force_ws: boolean;
  // 账号级 Codex CLI 模拟上游 HTTP 请求使用的 TLS 指纹模板 ID，0 表示内置默认。
  openai_codex_direct_tls_fingerprint_profile_id: number;
  client_request_debug_log_enabled: boolean;
  codex_stability_mode: "off" | "codex" | "all_openai_responses" | string;
  codex_stability_dynamic_header_timeout_enabled: boolean;
  codex_stability_request_phase_failover_enabled: boolean;
  codex_stability_suppress_client_timeout_headers: boolean;
  codex_stability_stream_keepalive_enabled: boolean;
  codex_autopilot_enabled: boolean;
  codex_autopilot_observe_only: boolean;
  openai_path_health_circuit_breaker_enabled: boolean;
  openai_fast_lane_enabled: boolean;
  realtime_balance_confirm_top_n: number;
  realtime_balance_confirm_timeout_ms: number;
  openai_header_race_enabled: boolean;
  openai_header_race_delay_ms: number;
  openai_header_race_daily_budget: number;
  openai_request_snapshot_enabled: boolean;
  openai_request_snapshot_retention_hours: number;
  context_journal_backend: string;
  context_journal_ttl_hours: number;
};

const form = reactive<SettingsForm>({
  registration_enabled: true,
  email_verify_enabled: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  invitation_code_enabled: false,
  password_reset_enabled: false,
  totp_enabled: false,
  totp_encryption_key_configured: false,
  passkey_enabled: false,
  passkey_configured: false,
  passkey_rp_id: "",
  passkey_rp_origins: [],
  login_agreement_enabled: false,
  login_agreement_mode: "modal",
  login_agreement_updated_at: "2026-03-31",
  login_agreement_documents: defaultLoginAgreementDocuments(),
  default_balance: 0,
  affiliate_rebate_rate: 20,
  affiliate_rebate_freeze_hours: 0,
  affiliate_rebate_duration_days: 0,
  affiliate_rebate_per_invitee_cap: 0,
  default_concurrency: 1,
  default_subscriptions: [],
  force_email_on_third_party_signup: false,
  default_user_rpm_limit: 0,
  site_name: "Sub2API",
  site_logo: "",
  site_subtitle: "Subscription to API Conversion Platform",
  api_base_url: "",
  contact_info: "",
  doc_url: "",
  home_content: "",
  backend_mode_enabled: false,
  hide_ccs_import_button: false,
  payment_enabled: false,
  risk_control_enabled: false,
  payment_min_amount: 1,
  payment_max_amount: 10000,
  payment_daily_limit: 50000,
  payment_max_pending_orders: 3,
  payment_order_timeout_minutes: 30,
  payment_balance_disabled: false,
  payment_balance_recharge_multiplier: 1,
  payment_recharge_fee_rate: 0,
  payment_enabled_types: [],
  payment_help_image_url: "",
  payment_help_text: "",
  payment_product_name_prefix: "",
  payment_product_name_suffix: "",
  payment_load_balance_strategy: "round-robin",
  payment_cancel_rate_limit_enabled: false,
  payment_cancel_rate_limit_max: 10,
  payment_cancel_rate_limit_window: 1,
  payment_cancel_rate_limit_unit: "day",
  payment_cancel_rate_limit_window_mode: "rolling",
  payment_alipay_force_qrcode: false,
  table_default_page_size: tablePageSizeDefault,
  table_page_size_options: [10, 20, 50, 100],
  custom_menu_items: [] as Array<{
    id: string;
    label: string;
    icon_svg: string;
    url: string;
    visibility: "user" | "admin";
    sort_order: number;
  }>,
  custom_endpoints: [] as Array<{
    name: string;
    endpoint: string;
    description: string;
  }>,
  frontend_url: "",
  smtp_host: "",
  smtp_port: 587,
  smtp_username: "",
  smtp_password: "",
  smtp_password_configured: false,
  smtp_from_email: "",
  smtp_from_name: "",
  smtp_use_tls: true,
  // Cloudflare Turnstile
  turnstile_enabled: false,
  turnstile_site_key: "",
  turnstile_secret_key: "",
  turnstile_secret_key_configured: false,
  api_key_acl_trust_forwarded_ip: false,
  // LinuxDo Connect OAuth 登录
  linuxdo_connect_enabled: false,
  linuxdo_connect_client_id: "",
  linuxdo_connect_client_secret: "",
  linuxdo_connect_client_secret_configured: false,
  linuxdo_connect_redirect_url: "",
  // DingTalk Connect OAuth 登录
  dingtalk_connect_enabled: false,
  dingtalk_connect_client_id: "",
  dingtalk_connect_client_secret: "",
  dingtalk_connect_client_secret_configured: false,
  dingtalk_connect_redirect_url: "",
  dingtalk_connect_corp_restriction_policy: "none",
  dingtalk_connect_internal_corp_id: "",
  dingtalk_connect_bypass_registration: false,
  dingtalk_connect_sync_corp_email: false,
  dingtalk_connect_sync_display_name: false,
  dingtalk_connect_sync_dept: false,
  dingtalk_connect_sync_corp_email_attr_key: "dingtalk_email",
  dingtalk_connect_sync_display_name_attr_key: "dingtalk_name",
  dingtalk_connect_sync_dept_attr_key: "dingtalk_department",
  dingtalk_connect_sync_corp_email_attr_name: "钉钉企业邮箱",
  dingtalk_connect_sync_display_name_attr_name: "钉钉姓名",
  dingtalk_connect_sync_dept_attr_name: "钉钉部门",
  wechat_connect_enabled: false,
  wechat_connect_app_id: "",
  wechat_connect_app_secret: "",
  wechat_connect_app_secret_configured: false,
  wechat_connect_open_app_id: "",
  wechat_connect_open_app_secret: "",
  wechat_connect_open_app_secret_configured: false,
  wechat_connect_mp_app_id: "",
  wechat_connect_mp_app_secret: "",
  wechat_connect_mp_app_secret_configured: false,
  wechat_connect_mobile_app_id: "",
  wechat_connect_mobile_app_secret: "",
  wechat_connect_mobile_app_secret_configured: false,
  wechat_connect_open_enabled: false,
  wechat_connect_mp_enabled: false,
  wechat_connect_mobile_enabled: false,
  wechat_connect_mode: "open",
  wechat_connect_scopes: "snsapi_login",
  wechat_connect_redirect_url: "",
  wechat_connect_frontend_redirect_url: "/auth/wechat/callback",
  // Generic OIDC OAuth 登录
  oidc_connect_enabled: false,
  oidc_connect_provider_name: "OIDC",
  oidc_connect_client_id: "",
  oidc_connect_client_secret: "",
  oidc_connect_client_secret_configured: false,
  oidc_connect_issuer_url: "",
  oidc_connect_discovery_url: "",
  oidc_connect_authorize_url: "",
  oidc_connect_token_url: "",
  oidc_connect_userinfo_url: "",
  oidc_connect_jwks_url: "",
  oidc_connect_scopes: "openid email profile",
  oidc_connect_redirect_url: "",
  oidc_connect_frontend_redirect_url: "/auth/oidc/callback",
  oidc_connect_token_auth_method: "client_secret_post",
  oidc_connect_use_pkce: false,
  oidc_connect_validate_id_token: false,
  oidc_connect_allowed_signing_algs: "RS256,ES256,PS256",
  oidc_connect_clock_skew_seconds: 120,
  oidc_connect_require_email_verified: false,
  oidc_connect_userinfo_email_path: "",
  oidc_connect_userinfo_id_path: "",
  oidc_connect_userinfo_username_path: "",
  // GitHub / Google 邮箱快捷登录
  github_oauth_enabled: false,
  github_oauth_client_id: "",
  github_oauth_client_secret: "",
  github_oauth_client_secret_configured: false,
  github_oauth_redirect_url: "",
  github_oauth_frontend_redirect_url: "/auth/oauth/callback",
  google_oauth_enabled: false,
  google_oauth_client_id: "",
  google_oauth_client_secret: "",
  google_oauth_client_secret_configured: false,
  google_oauth_redirect_url: "",
  google_oauth_frontend_redirect_url: "/auth/oauth/callback",
  // Model fallback
  enable_model_fallback: false,
  fallback_model_anthropic: "claude-3-5-sonnet-20241022",
  fallback_model_openai: "gpt-4o",
  fallback_model_gemini: "gemini-2.5-pro",
  fallback_model_antigravity: "gemini-2.5-pro",
  // Identity patch (Claude -> Gemini)
  enable_identity_patch: true,
  identity_patch_prompt: "",
  // Ops monitoring (vNext)
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: true,
  ops_query_mode_default: "auto",
  ops_metrics_interval_seconds: 60,
  // Claude Code version check
  min_claude_code_version: "",
  max_claude_code_version: "",
  // 分组隔离
  allow_ungrouped_key_scheduling: false,
  openai_advanced_scheduler_enabled: false,
  openai_scheduler_exhaustion_probe_infinite_wait_enabled: false,
  openai_scheduler_exhaustion_probe_notify_enabled: false,
  openai_scheduler_exhaustion_probe_notify_channel: "feishu_webhook",
  openai_scheduler_exhaustion_probe_notify_after_seconds: 60,
  openai_scheduler_exhaustion_probe_notify_repeat_seconds: 300,
  openai_scheduler_exhaustion_probe_notify_feishu_webhook_url: "",
  openai_scheduler_exhaustion_probe_notify_feishu_app_id: "",
  openai_scheduler_exhaustion_probe_notify_feishu_app_secret: "",
  openai_scheduler_exhaustion_probe_notify_feishu_domain: "feishu",
  openai_scheduler_exhaustion_probe_notify_feishu_receive_id_type: "chat_id",
  openai_scheduler_exhaustion_probe_notify_feishu_receive_id: "",
  openai_scheduler_exhaustion_probe_notify_recovered_enabled: true,
  // Gateway forwarding behavior
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: false,
  enable_anthropic_cache_ttl_1h_injection: false,
  rewrite_message_cache_control: false,
  antigravity_user_agent_version: "",
  openai_codex_user_agent: "",
  openai_allow_claude_code_codex_plugin: false,
  openai_oauth_compat_mode: "off",
  openai_cockpit_tools_compat: false,
  openai_codex_direct_force_ws: false,
  openai_codex_direct_tls_fingerprint_profile_id: 0,
  client_request_debug_log_enabled: false,
  codex_stability_mode: "codex",
  codex_stability_dynamic_header_timeout_enabled: true,
  codex_stability_request_phase_failover_enabled: true,
  codex_stability_suppress_client_timeout_headers: true,
  codex_stability_stream_keepalive_enabled: true,
  codex_autopilot_enabled: true,
  codex_autopilot_observe_only: true,
  openai_path_health_circuit_breaker_enabled: true,
  openai_fast_lane_enabled: true,
  realtime_balance_confirm_top_n: 3,
  realtime_balance_confirm_timeout_ms: 1200,
  openai_header_race_enabled: false,
  openai_header_race_delay_ms: 3500,
  openai_header_race_daily_budget: 0,
  openai_request_snapshot_enabled: true,
  openai_request_snapshot_retention_hours: 72,
  context_journal_backend: "memory",
  context_journal_ttl_hours: 24,
  // 余额、订阅到期与账号限额通知
  balance_low_notify_enabled: false,
  balance_low_notify_threshold: 0,
  balance_low_notify_recharge_url: "",
  subscription_expiry_notify_enabled: true,
  account_quota_notify_enabled: false,
  account_quota_notify_emails: [] as NotifyEmailEntry[],
  // Channel Monitor feature switch
  channel_monitor_enabled: true,
  channel_monitor_default_interval_seconds: 60,
  // Available Channels feature switch
  available_channels_enabled: false,
  // 模型广场功能开关与页面说明。
  model_plaza_enabled: false,
  model_plaza_require_auth: false,
  model_plaza_description: "",
  // Affiliate (邀请返利) feature switch
  affiliate_enabled: false,
});

const authSourceDefaults = reactive<AuthSourceDefaultsState>(
  buildAuthSourceDefaultsState({}),
);

const authSourceDefaultsMeta = computed(() => [
  {
    source: "email" as AuthSourceType,
    title: t("admin.settings.authSourceDefaults.sources.email.title"),
    description: t("admin.settings.authSourceDefaults.sources.email.description"),
  },
  {
    source: "linuxdo" as AuthSourceType,
    title: t("admin.settings.authSourceDefaults.sources.linuxdo.title"),
    description: t("admin.settings.authSourceDefaults.sources.linuxdo.description"),
  },
  {
    source: "oidc" as AuthSourceType,
    title: t("admin.settings.authSourceDefaults.sources.oidc.title"),
    description: t("admin.settings.authSourceDefaults.sources.oidc.description"),
  },
  {
    source: "wechat" as AuthSourceType,
    title: t("admin.settings.authSourceDefaults.sources.wechat.title"),
    description: t("admin.settings.authSourceDefaults.sources.wechat.description"),
  },
  {
    source: "github" as AuthSourceType,
    title: "GitHub",
    description: localText(
      "通过 GitHub 已验证邮箱首次注册或首次绑定时应用。",
      "Applied on first signup or first bind through a verified GitHub email.",
    ),
  },
  {
    source: "google" as AuthSourceType,
    title: "Google",
    description: localText(
      "通过 Google 已验证邮箱首次注册或首次绑定时应用。",
      "Applied on first signup or first bind through a verified Google email.",
    ),
  },
  {
    source: "dingtalk" as AuthSourceType,
    title: "钉钉",
    description: localText(
      "通过钉钉首次注册或首次绑定时应用。",
      "Applied on first signup or first bind through DingTalk.",
    ),
  },
]);

// Proxies for web search emulation ProxySelector
const webSearchProxies = ref<Proxy[]>([]);

// Web Search Emulation config (loaded/saved separately)
const DEFAULT_WEB_SEARCH_QUOTA_LIMIT = 1000;

const webSearchConfig = reactive<WebSearchEmulationConfig>({
  enabled: false,
  providers: [],
});

const expandedProviders = reactive<Record<number, boolean>>({});
const apiKeyVisible = reactive<Record<number, boolean>>({});
const wsTestQuery = ref("");
const wsTestLoading = ref(false);
const wsTestResult = ref<WebSearchTestResult | null>(null);
const wsTestDialogOpen = ref(false);

function openTestDialog() {
  wsTestResult.value = null;
  wsTestDialogOpen.value = true;
}

function toggleProviderExpand(idx: number) {
  expandedProviders[idx] = !expandedProviders[idx];
}

function removeWebSearchProvider(idx: number) {
  webSearchConfig.providers.splice(idx, 1);
  // Re-index expandedProviders and apiKeyVisible after removal
  const newExpanded: Record<number, boolean> = {};
  const newVisible: Record<number, boolean> = {};
  for (let i = 0; i < webSearchConfig.providers.length; i++) {
    const oldIdx = i >= idx ? i + 1 : i;
    newExpanded[i] = expandedProviders[oldIdx] ?? false;
    newVisible[i] = apiKeyVisible[oldIdx] ?? false;
  }
  Object.keys(expandedProviders).forEach(
    (k) => delete expandedProviders[Number(k)],
  );
  Object.keys(apiKeyVisible).forEach((k) => delete apiKeyVisible[Number(k)]);
  Object.assign(expandedProviders, newExpanded);
  Object.assign(apiKeyVisible, newVisible);
}

function addWebSearchProvider() {
  const idx = webSearchConfig.providers.length;
  webSearchConfig.providers.push({
    type: "brave",
    api_key: "",
    api_key_configured: false,
    quota_limit: DEFAULT_WEB_SEARCH_QUOTA_LIMIT,
    subscribed_at: null,
    proxy_id: null,
    expires_at: null,
  } as WebSearchProviderConfig);
  expandedProviders[idx] = true;
}

function formatSubscribedAt(ts: number | null): string {
  if (!ts) return "";
  // Use UTC to avoid timezone drift on repeated edits
  const d = new Date(ts * 1000);
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function parseSubscribedAt(dateStr: string): number | null {
  if (!dateStr) return null;
  // Parse as UTC to match formatSubscribedAt
  return Math.floor(new Date(dateStr + "T00:00:00Z").getTime() / 1000);
}

function quotaPercentage(provider: WebSearchProviderConfig): number {
  if (!provider.quota_limit || provider.quota_limit <= 0) return 0;
  return ((provider.quota_used ?? 0) / provider.quota_limit) * 100;
}

async function resetWebSearchUsage(idx: number) {
  const provider = webSearchConfig.providers[idx];
  if (!provider) return;
  if (!confirm(t("admin.settings.webSearchEmulation.resetUsageConfirm")))
    return;
  try {
    await adminAPI.settings.resetWebSearchUsage({
      provider_type: provider.type,
    });
    provider.quota_used = 0;
    appStore.showSuccess(
      t("admin.settings.webSearchEmulation.resetUsageSuccess"),
    );
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  }
}

async function copyApiKey(idx: number) {
  const key = webSearchConfig.providers[idx]?.api_key;
  if (!key) {
    appStore.showError(
      t("admin.settings.webSearchEmulation.apiKeyPlaceholder"),
    );
    return;
  }
  try {
    await navigator.clipboard.writeText(key);
    appStore.showSuccess(t("admin.settings.webSearchEmulation.copied"));
  } catch {
    appStore.showError(t("common.error"));
  }
}

async function testWebSearchProvider() {
  wsTestLoading.value = true;
  wsTestResult.value = null;
  try {
    const query =
      wsTestQuery.value.trim() ||
      t("admin.settings.webSearchEmulation.testDefaultQuery");
    wsTestResult.value = await adminAPI.settings.testWebSearchEmulation(query);
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    wsTestLoading.value = false;
  }
}

async function loadWebSearchConfig() {
  try {
    const [resp, proxiesResp] = await Promise.all([
      adminAPI.settings.getWebSearchEmulationConfig(),
      adminAPI.proxies.list().catch(() => ({ items: [] as Proxy[] })),
    ]);
    if (resp) {
      webSearchConfig.enabled = resp.enabled || false;
      webSearchConfig.providers = resp.providers || [];
    }
    webSearchProxies.value = proxiesResp.items || [];
  } catch (err: unknown) {
    // 404 is expected when config hasn't been created yet; show error for other failures
    const status = (err as { status?: number })?.status;
    if (status !== 404 && status !== undefined) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    }
  }
}

async function saveWebSearchConfig(): Promise<boolean> {
  try {
    for (const p of webSearchConfig.providers) {
      const raw = p.quota_limit;
      if (raw != null && Number(raw) !== 0 && Number(raw) < 1) {
        appStore.showError(
          t("admin.settings.webSearchEmulation.quotaLimitMustBePositive"),
        );
        return false;
      }
    }
    const providers = webSearchConfig.providers.map(
      (p: WebSearchProviderConfig) => ({
        ...p,
        quota_limit: Number(p.quota_limit) > 0 ? Number(p.quota_limit) : null,
      }),
    );
    await adminAPI.settings.updateWebSearchEmulationConfig({
      enabled: webSearchConfig.enabled,
      providers,
    });
    return true;
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
    return false;
  }
}

const defaultSubscriptionGroupOptions = computed<
  DefaultSubscriptionGroupOption[]
>(() =>
  subscriptionGroups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    platform: group.platform,
    subscriptionType: group.subscription_type,
    rate: group.rate_multiplier,
  })),
);

const registrationEmailSuffixWhitelistSeparatorKeys = new Set([
  " ",
  ",",
  "，",
  "Enter",
  "Tab",
]);

function removeRegistrationEmailSuffixWhitelistTag(suffix: string) {
  registrationEmailSuffixWhitelistTags.value =
    registrationEmailSuffixWhitelistTags.value.filter(
      (item) => item !== suffix,
    );
}

function addRegistrationEmailSuffixWhitelistTag(raw: string) {
  const suffix = normalizeRegistrationEmailSuffixDomain(raw);
  if (
    !isRegistrationEmailSuffixDomainValid(suffix) ||
    registrationEmailSuffixWhitelistTags.value.includes(suffix)
  ) {
    return;
  }
  registrationEmailSuffixWhitelistTags.value = [
    ...registrationEmailSuffixWhitelistTags.value,
    suffix,
  ];
}

function commitRegistrationEmailSuffixWhitelistDraft() {
  if (!registrationEmailSuffixWhitelistDraft.value) {
    return;
  }
  addRegistrationEmailSuffixWhitelistTag(
    registrationEmailSuffixWhitelistDraft.value,
  );
  registrationEmailSuffixWhitelistDraft.value = "";
}

function handleRegistrationEmailSuffixWhitelistDraftInput() {
  registrationEmailSuffixWhitelistDraft.value =
    normalizeRegistrationEmailSuffixDomain(
      registrationEmailSuffixWhitelistDraft.value,
    );
}

function handleRegistrationEmailSuffixWhitelistDraftKeydown(
  event: KeyboardEvent,
) {
  if (event.isComposing) {
    return;
  }

  if (registrationEmailSuffixWhitelistSeparatorKeys.has(event.key)) {
    event.preventDefault();
    commitRegistrationEmailSuffixWhitelistDraft();
    return;
  }

  if (
    event.key === "Backspace" &&
    !registrationEmailSuffixWhitelistDraft.value &&
    registrationEmailSuffixWhitelistTags.value.length > 0
  ) {
    registrationEmailSuffixWhitelistTags.value.pop();
  }
}

function handleRegistrationEmailSuffixWhitelistPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData("text") || "";
  if (!text.trim()) {
    return;
  }
  event.preventDefault();
  const tokens = parseRegistrationEmailSuffixWhitelistInput(text);
  for (const token of tokens) {
    addRegistrationEmailSuffixWhitelistTag(token);
  }
}

// Quota notify email helpers
const addQuotaNotifyEmail = () => {
  if (!form.account_quota_notify_emails) {
    form.account_quota_notify_emails = [];
  }
  form.account_quota_notify_emails.push({
    email: "",
    disabled: false,
    verified: true,
  });
};

const currentOrigin =
  typeof window !== "undefined" ? window.location.origin : "";

// LinuxDo OAuth redirect URL suggestion
const linuxdoRedirectUrlSuggestion = computed(() => {
  if (typeof window === "undefined") return "";
  const origin =
    window.location.origin ||
    `${window.location.protocol}//${window.location.host}`;
  return `${origin}/api/v1/auth/oauth/linuxdo/callback`;
});

async function setAndCopyLinuxdoRedirectUrl() {
  const url = linuxdoRedirectUrlSuggestion.value;
  if (!url) return;

  form.linuxdo_connect_redirect_url = url;
  await copyToClipboard(
    url,
    t("admin.settings.linuxdo.redirectUrlSetAndCopied"),
  );
}

type EmailOAuthProvider = "github" | "google";

const githubOAuthRedirectUrlSuggestion = computed(() => {
  if (typeof window === "undefined") return "";
  const origin =
    window.location.origin ||
    `${window.location.protocol}//${window.location.host}`;
  return `${origin}/api/v1/auth/oauth/github/callback`;
});

const googleOAuthRedirectUrlSuggestion = computed(() => {
  if (typeof window === "undefined") return "";
  const origin =
    window.location.origin ||
    `${window.location.protocol}//${window.location.host}`;
  return `${origin}/api/v1/auth/oauth/google/callback`;
});

async function setAndCopyEmailOAuthRedirectUrl(provider: EmailOAuthProvider) {
  const url =
    provider === "github"
      ? githubOAuthRedirectUrlSuggestion.value
      : googleOAuthRedirectUrlSuggestion.value;
  if (!url) return;

  if (provider === "github") {
    form.github_oauth_redirect_url = url;
  } else {
    form.google_oauth_redirect_url = url;
  }
  await copyToClipboard(
    url,
    localText("回调地址已写入并复制。", "Callback URL set and copied."),
  );
}

const wechatRedirectUrlSuggestion = computed(() => {
  if (typeof window === "undefined") return "";
  const origin =
    window.location.origin ||
    `${window.location.protocol}//${window.location.host}`;
  return `${origin}/api/v1/auth/oauth/wechat/callback`;
});

function syncWeChatConnectMode(preferredMode?: WeChatConnectMode) {
  if (form.wechat_connect_mp_enabled && form.wechat_connect_mobile_enabled) {
    if (preferredMode === "mobile") {
      form.wechat_connect_mp_enabled = false;
    } else {
      form.wechat_connect_mobile_enabled = false;
    }
  }

  const capabilities = resolveWeChatConnectModeCapabilities(
    form.wechat_connect_open_enabled,
    form.wechat_connect_mp_enabled,
    form.wechat_connect_mobile_enabled,
    form.wechat_connect_mode,
  );
  form.wechat_connect_open_enabled = capabilities.openEnabled;
  form.wechat_connect_mp_enabled = capabilities.mpEnabled;
  form.wechat_connect_mobile_enabled = capabilities.mobileEnabled;
  form.wechat_connect_mode = deriveWeChatConnectStoredMode(
    capabilities.openEnabled,
    capabilities.mpEnabled,
    capabilities.mobileEnabled,
    form.wechat_connect_mode,
  );
  form.wechat_connect_scopes = defaultWeChatConnectScopesForMode(
    form.wechat_connect_mode,
  );
}

function handleWeChatOpenEnabledChange(value: boolean) {
  form.wechat_connect_open_enabled = value;
  syncWeChatConnectMode(value ? "open" : undefined);
}

function handleWeChatMPEnabledChange(value: boolean) {
  form.wechat_connect_mp_enabled = value;
  if (value) {
    form.wechat_connect_mobile_enabled = false;
  }
  syncWeChatConnectMode(value ? "mp" : undefined);
}

function handleWeChatMobileEnabledChange(value: boolean) {
  form.wechat_connect_mobile_enabled = value;
  if (value) {
    form.wechat_connect_mp_enabled = false;
  }
  syncWeChatConnectMode(value ? "mobile" : undefined);
}

async function setAndCopyWeChatRedirectUrl() {
  const url = wechatRedirectUrlSuggestion.value;
  if (!url) return;

  form.wechat_connect_redirect_url = url;
  await copyToClipboard(
    url,
    t("admin.settings.wechatConnect.redirectUrlSetAndCopied"),
  );
}

const oidcRedirectUrlSuggestion = computed(() => {
  if (typeof window === "undefined") return "";
  const origin =
    window.location.origin ||
    `${window.location.protocol}//${window.location.host}`;
  return `${origin}/api/v1/auth/oauth/oidc/callback`;
});

async function setAndCopyOIDCRedirectUrl() {
  const url = oidcRedirectUrlSuggestion.value;
  if (!url) return;

  form.oidc_connect_redirect_url = url;
  await copyToClipboard(url, t("admin.settings.oidc.redirectUrlSetAndCopied"));
}

// Custom menu item management
function addMenuItem() {
  form.custom_menu_items.push({
    id: "",
    label: "",
    icon_svg: "",
    url: "",
    visibility: "user",
    sort_order: form.custom_menu_items.length,
  });
}

function removeMenuItem(index: number) {
  form.custom_menu_items.splice(index, 1);
  // Re-index sort_order
  form.custom_menu_items.forEach((item, i) => {
    item.sort_order = i;
  });
}

function moveMenuItem(index: number, direction: -1 | 1) {
  const targetIndex = index + direction;
  if (targetIndex < 0 || targetIndex >= form.custom_menu_items.length) return;
  const items = form.custom_menu_items;
  const temp = items[index];
  items[index] = items[targetIndex];
  items[targetIndex] = temp;
  // Re-index sort_order
  items.forEach((item, i) => {
    item.sort_order = i;
  });
}

// Custom endpoint management
function addEndpoint() {
  form.custom_endpoints.push({ name: "", endpoint: "", description: "" });
}

function removeEndpoint(index: number) {
  form.custom_endpoints.splice(index, 1);
}

function addLoginAgreementDocument() {
  form.login_agreement_documents.push({
    id: `custom-${Date.now().toString(36)}`,
    title: "",
    content_md: "",
  });
}

function removeLoginAgreementDocument(index: number) {
  form.login_agreement_documents.splice(index, 1);
}

function normalizeLoginAgreementDocumentsForSave(): LoginAgreementDocument[] {
  return form.login_agreement_documents
    .map((doc, index) => ({
      id:
        normalizeLoginAgreementDocumentId(doc.id || doc.title) ||
        `doc-${index + 1}`,
      title: doc.title.trim(),
      content_md: doc.content_md.trim(),
    }))
    .filter((doc) => doc.title || doc.content_md);
}

function findDuplicateLoginAgreementDocumentId(
  documents: LoginAgreementDocument[],
): string | null {
  const seen = new Set<string>();
  for (const doc of documents) {
    if (seen.has(doc.id)) {
      return doc.id;
    }
    seen.add(doc.id);
  }
  return null;
}

function formatTablePageSizeOptions(options: number[]): string {
  return options.join(", ");
}

function parseTablePageSizeOptionsInput(raw: string): number[] | null {
  const tokens = raw
    .split(",")
    .map((token) => token.trim())
    .filter((token) => token.length > 0);

  if (tokens.length === 0) {
    return null;
  }

  const parsed = tokens.map((token) => Number(token));
  if (parsed.some((value) => !Number.isInteger(value))) {
    return null;
  }

  const deduped = Array.from(new Set(parsed)).sort((a, b) => a - b);
  if (
    deduped.some(
      (value) => value < tablePageSizeMin || value > tablePageSizeMax,
    )
  ) {
    return null;
  }

  return deduped;
}

async function loadSettings() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const settings = await adminAPI.settings.getSettings();
    settings.payment_load_balance_strategy =
      settings.payment_load_balance_strategy || "round-robin";
    // Only assign non-null values from backend (null means unconfigured, keep defaults)
    for (const [key, value] of Object.entries(settings)) {
      if (value !== null && value !== undefined) {
        (form as Record<string, unknown>)[key] = value;
      }
    }
    // 历史 cockpit_tools/codex_direct 全局模式不再生效，设置页统一展示为关闭。
    form.openai_oauth_compat_mode = "off";
    form.openai_cockpit_tools_compat = false;
    form.openai_codex_direct_force_ws =
      settings.openai_codex_direct_force_ws === true;
    form.openai_codex_direct_tls_fingerprint_profile_id =
      typeof settings.openai_codex_direct_tls_fingerprint_profile_id ===
      "number"
        ? settings.openai_codex_direct_tls_fingerprint_profile_id
        : 0;
    form.login_agreement_mode =
      settings.login_agreement_mode === "checkbox" ? "checkbox" : "modal";
    form.login_agreement_updated_at =
      settings.login_agreement_updated_at || "2026-03-31";
    form.login_agreement_documents =
      Array.isArray(settings.login_agreement_documents) &&
      settings.login_agreement_documents.length > 0
        ? settings.login_agreement_documents.map((doc) => ({
            id: doc.id || "",
            title: doc.title || "",
            content_md: doc.content_md || "",
          }))
        : defaultLoginAgreementDocuments();
    Object.assign(authSourceDefaults, buildAuthSourceDefaultsState(settings));
    form.backend_mode_enabled = settings.backend_mode_enabled;
    form.default_subscriptions = normalizeDefaultSubscriptionSettings(
      settings.default_subscriptions,
    );
    registrationEmailSuffixWhitelistTags.value =
      normalizeRegistrationEmailSuffixDomains(
        settings.registration_email_suffix_whitelist,
      );
    tablePageSizeOptionsInput.value = formatTablePageSizeOptions(
      Array.isArray(settings.table_page_size_options)
        ? settings.table_page_size_options
        : [10, 20, 50, 100],
    );
    registrationEmailSuffixWhitelistDraft.value = "";
    form.smtp_password = "";
    smtpPasswordManuallyEdited.value = false;
    form.turnstile_secret_key = "";
    form.linuxdo_connect_client_secret = "";
    form.dingtalk_connect_client_secret = "";
    form.github_oauth_client_secret = "";
    form.google_oauth_client_secret = "";
    form.wechat_connect_app_secret = "";
    form.wechat_connect_open_app_secret = "";
    form.wechat_connect_mp_app_secret = "";
    form.wechat_connect_mobile_app_secret = "";
    const wechatCapabilities = resolveWeChatConnectModeCapabilities(
      settings.wechat_connect_open_enabled,
      settings.wechat_connect_mp_enabled,
      settings.wechat_connect_mobile_enabled,
      settings.wechat_connect_mode,
    );
    form.wechat_connect_open_enabled = wechatCapabilities.openEnabled;
    form.wechat_connect_mp_enabled = wechatCapabilities.mpEnabled;
    form.wechat_connect_mobile_enabled = wechatCapabilities.mobileEnabled;
    form.wechat_connect_mode = deriveWeChatConnectStoredMode(
      wechatCapabilities.openEnabled,
      wechatCapabilities.mpEnabled,
      wechatCapabilities.mobileEnabled,
      settings.wechat_connect_mode,
    );
    const legacyWeChatAppID = String(settings.wechat_connect_app_id || "").trim();
    const legacyWeChatSecretConfigured = Boolean(
      settings.wechat_connect_app_secret_configured,
    );
    if (!form.wechat_connect_open_app_id && wechatCapabilities.openEnabled) {
      form.wechat_connect_open_app_id = legacyWeChatAppID;
    }
    if (!form.wechat_connect_mp_app_id && wechatCapabilities.mpEnabled) {
      form.wechat_connect_mp_app_id = legacyWeChatAppID;
    }
    if (!form.wechat_connect_mobile_app_id && wechatCapabilities.mobileEnabled) {
      form.wechat_connect_mobile_app_id = legacyWeChatAppID;
    }
    if (
      !form.wechat_connect_open_app_secret_configured &&
      wechatCapabilities.openEnabled
    ) {
      form.wechat_connect_open_app_secret_configured =
        legacyWeChatSecretConfigured;
    }
    if (
      !form.wechat_connect_mp_app_secret_configured &&
      wechatCapabilities.mpEnabled
    ) {
      form.wechat_connect_mp_app_secret_configured = legacyWeChatSecretConfigured;
    }
    if (
      !form.wechat_connect_mobile_app_secret_configured &&
      wechatCapabilities.mobileEnabled
    ) {
      form.wechat_connect_mobile_app_secret_configured =
        legacyWeChatSecretConfigured;
    }
    form.wechat_connect_scopes = defaultWeChatConnectScopesForMode(
      form.wechat_connect_mode,
    );
    form.oidc_connect_client_secret = "";

    // Load OpenAI fast/flex policy rules from bulk settings.
    // 仅当 payload 真的包含该字段时填充并标记为已加载；否则保持表单空值，
    // 让 saveSettings 在未加载时跳过该字段，防止覆盖后端默认规则。
    if (
      settings.openai_fast_policy_settings &&
      Array.isArray(settings.openai_fast_policy_settings.rules)
    ) {
      openaiFastPolicyForm.rules =
        settings.openai_fast_policy_settings.rules.map((rule) => ({
          ...rule,
          user_ids: rule.user_ids ? [...rule.user_ids] : [],
          model_whitelist: rule.model_whitelist
            ? [...rule.model_whitelist]
            : [],
        }));
      openaiFastPolicyLoaded.value = true;
    }

    // Load web search emulation config separately
    await loadWebSearchConfig();
  } catch (error: unknown) {
    loadFailed.value = true;
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.failedToLoad")),
    );
  } finally {
    loading.value = false;
  }
}

async function loadCodexDirectTLSFingerprintProfiles() {
  try {
    codexDirectTLSFingerprintProfiles.value =
      await adminAPI.tlsFingerprintProfiles.list();
  } catch (_error: unknown) {
    codexDirectTLSFingerprintProfiles.value = [];
  }
}

async function loadSubscriptionGroups() {
  try {
    const groups = await adminAPI.groups.getAll();
    subscriptionGroups.value = groups.filter(
      (group) =>
        group.subscription_type === "subscription" && group.status === "active",
    );
  } catch (_error: unknown) {
    subscriptionGroups.value = [];
  }
}

function findNextAvailableSubscriptionGroup(
  existingGroupIDs: number[],
): AdminGroup | undefined {
  const existing = new Set(existingGroupIDs);
  return subscriptionGroups.value.find((group) => !existing.has(group.id));
}

function addDefaultSubscription() {
  if (subscriptionGroups.value.length === 0) return;
  const candidate = findNextAvailableSubscriptionGroup(
    form.default_subscriptions.map((item) => item.group_id),
  );
  if (!candidate) return;
  form.default_subscriptions.push({
    group_id: candidate.id,
    validity_days: 30,
  });
}

function removeDefaultSubscription(index: number) {
  form.default_subscriptions.splice(index, 1);
}

function addAuthSourceDefaultSubscription(source: AuthSourceType) {
  if (subscriptionGroups.value.length === 0) return;
  const candidate = findNextAvailableSubscriptionGroup(
    authSourceDefaults[source].subscriptions.map((item) => item.group_id),
  );
  if (!candidate) return;
  authSourceDefaults[source].subscriptions.push({
    group_id: candidate.id,
    validity_days: 30,
  });
}

function removeAuthSourceDefaultSubscription(
  source: AuthSourceType,
  index: number,
) {
  authSourceDefaults[source].subscriptions.splice(index, 1);
}

function findDuplicateDefaultSubscription(
  subscriptions: DefaultSubscriptionSetting[],
): DefaultSubscriptionSetting | undefined {
  const seenGroupIDs = new Set<number>();

  return subscriptions.find((item) => {
    if (seenGroupIDs.has(item.group_id)) {
      return true;
    }
    seenGroupIDs.add(item.group_id);
    return false;
  });
}

async function saveSettings() {
  saving.value = true;
  try {
    const normalizedTableDefaultPageSize = Math.floor(
      Number(form.table_default_page_size),
    );
    if (
      !Number.isInteger(normalizedTableDefaultPageSize) ||
      normalizedTableDefaultPageSize < tablePageSizeMin ||
      normalizedTableDefaultPageSize > tablePageSizeMax
    ) {
      appStore.showError(
        t("admin.settings.site.tableDefaultPageSizeRangeError", {
          min: tablePageSizeMin,
          max: tablePageSizeMax,
        }),
      );
      return;
    }

    const normalizedTablePageSizeOptions = parseTablePageSizeOptionsInput(
      tablePageSizeOptionsInput.value,
    );
    if (!normalizedTablePageSizeOptions) {
      appStore.showError(
        t("admin.settings.site.tablePageSizeOptionsFormatError", {
          min: tablePageSizeMin,
          max: tablePageSizeMax,
        }),
      );
      return;
    }

    form.table_default_page_size = normalizedTableDefaultPageSize;
    form.table_page_size_options = normalizedTablePageSizeOptions;

    const normalizedLoginAgreementDocuments =
      normalizeLoginAgreementDocumentsForSave();
    if (form.login_agreement_enabled && normalizedLoginAgreementDocuments.length === 0) {
      appStore.showError(
        localText(
          "启用登录条款确认时，至少需要保留一份文档。",
          "At least one document is required when login agreement is enabled.",
        ),
      );
      return;
    }
    const emptyTitleDocument = normalizedLoginAgreementDocuments.find(
      (doc) => !doc.title,
    );
    if (emptyTitleDocument) {
      appStore.showError(
        localText(
          "登录条款文档名称不能为空。",
          "Login agreement document title cannot be empty.",
        ),
      );
      return;
    }
    const duplicateLoginAgreementDocumentId =
      findDuplicateLoginAgreementDocumentId(normalizedLoginAgreementDocuments);
    if (duplicateLoginAgreementDocumentId) {
      appStore.showError(
        localText(
          `登录条款文档路由不能重复：/legal/${duplicateLoginAgreementDocumentId}`,
          `Login agreement document routes cannot be duplicated: /legal/${duplicateLoginAgreementDocumentId}`,
        ),
      );
      return;
    }
    form.login_agreement_mode =
      form.login_agreement_mode === "checkbox" ? "checkbox" : "modal";
    form.login_agreement_documents = normalizedLoginAgreementDocuments;

    const normalizedDefaultSubscriptions = normalizeDefaultSubscriptionSettings(
      form.default_subscriptions,
    );
    const duplicateDefaultSubscription = findDuplicateDefaultSubscription(
      normalizedDefaultSubscriptions,
    );
    if (duplicateDefaultSubscription) {
      appStore.showError(
        t("admin.settings.defaults.defaultSubscriptionsDuplicate", {
          groupId: duplicateDefaultSubscription.group_id,
        }),
      );
      return;
    }

    for (const authSource of authSourceDefaultsMeta.value) {
      authSourceDefaults[authSource.source].subscriptions =
        normalizeDefaultSubscriptionSettings(
          authSourceDefaults[authSource.source].subscriptions,
        );
      const duplicate = findDuplicateDefaultSubscription(
        authSourceDefaults[authSource.source].subscriptions,
      );
      if (duplicate) {
        appStore.showError(
          `${authSource.title}: ${t(
            "admin.settings.defaults.defaultSubscriptionsDuplicate",
            {
              groupId: duplicate.group_id,
            },
          )}`,
        );
        return;
      }
    }

    if (form.wechat_connect_mp_enabled && form.wechat_connect_mobile_enabled) {
      appStore.showError(
        localText(
          "公众号和移动应用不能同时启用。",
          "Official Account and Mobile App cannot be enabled at the same time.",
        ),
      );
      return;
    }
    // Validate URL fields — novalidate disables browser-native checks, so we validate here
    const isValidHttpUrl = (url: string): boolean => {
      if (!url) return true;
      try {
        const u = new URL(url);
        return u.protocol === "http:" || u.protocol === "https:";
      } catch {
        return false;
      }
    };
    // Optional URL fields: auto-clear invalid values so they don't cause backend 400 errors
    if (!isValidHttpUrl(form.frontend_url)) form.frontend_url = "";
    if (!isValidHttpUrl(form.doc_url)) form.doc_url = "";
    syncWeChatConnectMode();
    const wechatStoredMode = deriveWeChatConnectStoredMode(
      form.wechat_connect_open_enabled,
      form.wechat_connect_mp_enabled,
      form.wechat_connect_mobile_enabled,
      form.wechat_connect_mode,
    );

    const payload: UpdateSettingsRequest = {
      registration_enabled: form.registration_enabled,
      email_verify_enabled: form.email_verify_enabled,
      registration_email_suffix_whitelist:
        registrationEmailSuffixWhitelistTags.value.map((suffix) =>
          suffix.startsWith("*.") ? suffix : `@${suffix}`,
        ),
      promo_code_enabled: form.promo_code_enabled,
      invitation_code_enabled: form.invitation_code_enabled,
      password_reset_enabled: form.password_reset_enabled,
      totp_enabled: form.totp_enabled,
      passkey_enabled: form.passkey_enabled,
      login_agreement_enabled: form.login_agreement_enabled,
      login_agreement_mode: form.login_agreement_mode,
      login_agreement_updated_at: form.login_agreement_updated_at,
      login_agreement_documents: form.login_agreement_documents,
      default_balance: form.default_balance,
      affiliate_rebate_rate: Math.min(
        100,
        Math.max(0, Number(form.affiliate_rebate_rate) || 0),
      ),
      affiliate_rebate_freeze_hours: Math.max(0, Math.min(720, Number(form.affiliate_rebate_freeze_hours) || 0)),
      affiliate_rebate_duration_days: Math.max(0, Math.min(3650, Math.floor(Number(form.affiliate_rebate_duration_days) || 0))),
      affiliate_rebate_per_invitee_cap: Math.max(0, Number(form.affiliate_rebate_per_invitee_cap) || 0),
      default_concurrency: form.default_concurrency,
      default_subscriptions: normalizedDefaultSubscriptions,
      force_email_on_third_party_signup: form.force_email_on_third_party_signup,
      default_user_rpm_limit: form.default_user_rpm_limit,
      site_name: form.site_name,
      site_logo: form.site_logo,
      site_subtitle: form.site_subtitle,
      api_base_url: form.api_base_url,
      contact_info: form.contact_info,
      doc_url: form.doc_url,
      home_content: form.home_content,
      backend_mode_enabled: form.backend_mode_enabled,
      hide_ccs_import_button: form.hide_ccs_import_button,
      table_default_page_size: form.table_default_page_size,
      table_page_size_options: form.table_page_size_options,
      custom_menu_items: form.custom_menu_items,
      custom_endpoints: form.custom_endpoints,
      frontend_url: form.frontend_url,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password || undefined,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls,
      turnstile_enabled: form.turnstile_enabled,
      turnstile_site_key: form.turnstile_site_key,
      turnstile_secret_key: form.turnstile_secret_key || undefined,
      api_key_acl_trust_forwarded_ip: form.api_key_acl_trust_forwarded_ip,
      linuxdo_connect_enabled: form.linuxdo_connect_enabled,
      linuxdo_connect_client_id: form.linuxdo_connect_client_id,
      linuxdo_connect_client_secret:
        form.linuxdo_connect_client_secret || undefined,
      linuxdo_connect_redirect_url: form.linuxdo_connect_redirect_url,
      dingtalk_connect_enabled: form.dingtalk_connect_enabled,
      dingtalk_connect_client_id: form.dingtalk_connect_client_id,
      dingtalk_connect_client_secret:
        form.dingtalk_connect_client_secret || undefined,
      dingtalk_connect_redirect_url: form.dingtalk_connect_redirect_url,
      dingtalk_connect_corp_restriction_policy:
        form.dingtalk_connect_corp_restriction_policy,
      dingtalk_connect_internal_corp_id: form.dingtalk_connect_internal_corp_id,
      dingtalk_connect_bypass_registration: form.dingtalk_connect_bypass_registration,
      dingtalk_connect_sync_corp_email: form.dingtalk_connect_sync_corp_email,
      dingtalk_connect_sync_display_name: form.dingtalk_connect_sync_display_name,
      dingtalk_connect_sync_dept: form.dingtalk_connect_sync_dept,
      dingtalk_connect_sync_corp_email_attr_key: form.dingtalk_connect_sync_corp_email_attr_key,
      dingtalk_connect_sync_display_name_attr_key: form.dingtalk_connect_sync_display_name_attr_key,
      dingtalk_connect_sync_dept_attr_key: form.dingtalk_connect_sync_dept_attr_key,
      dingtalk_connect_sync_corp_email_attr_name: form.dingtalk_connect_sync_corp_email_attr_name,
      dingtalk_connect_sync_display_name_attr_name: form.dingtalk_connect_sync_display_name_attr_name,
      dingtalk_connect_sync_dept_attr_name: form.dingtalk_connect_sync_dept_attr_name,
      wechat_connect_enabled: form.wechat_connect_enabled,
      wechat_connect_app_id:
        form.wechat_connect_open_app_id ||
        form.wechat_connect_mp_app_id ||
        form.wechat_connect_mobile_app_id ||
        form.wechat_connect_app_id,
      wechat_connect_app_secret: form.wechat_connect_app_secret || undefined,
      wechat_connect_open_app_id: form.wechat_connect_open_app_id,
      wechat_connect_open_app_secret:
        form.wechat_connect_open_app_secret || undefined,
      wechat_connect_mp_app_id: form.wechat_connect_mp_app_id,
      wechat_connect_mp_app_secret:
        form.wechat_connect_mp_app_secret || undefined,
      wechat_connect_mobile_app_id: form.wechat_connect_mobile_app_id,
      wechat_connect_mobile_app_secret:
        form.wechat_connect_mobile_app_secret || undefined,
      wechat_connect_open_enabled: form.wechat_connect_open_enabled,
      wechat_connect_mp_enabled: form.wechat_connect_mp_enabled,
      wechat_connect_mobile_enabled: form.wechat_connect_mobile_enabled,
      wechat_connect_mode: wechatStoredMode,
      wechat_connect_scopes:
        defaultWeChatConnectScopesForMode(wechatStoredMode),
      wechat_connect_redirect_url: form.wechat_connect_redirect_url,
      wechat_connect_frontend_redirect_url:
        form.wechat_connect_frontend_redirect_url,
      oidc_connect_enabled: form.oidc_connect_enabled,
      oidc_connect_provider_name: form.oidc_connect_provider_name,
      oidc_connect_client_id: form.oidc_connect_client_id,
      oidc_connect_client_secret: form.oidc_connect_client_secret || undefined,
      oidc_connect_issuer_url: form.oidc_connect_issuer_url,
      oidc_connect_discovery_url: form.oidc_connect_discovery_url,
      oidc_connect_authorize_url: form.oidc_connect_authorize_url,
      oidc_connect_token_url: form.oidc_connect_token_url,
      oidc_connect_userinfo_url: form.oidc_connect_userinfo_url,
      oidc_connect_jwks_url: form.oidc_connect_jwks_url,
      oidc_connect_scopes: form.oidc_connect_scopes,
      oidc_connect_redirect_url: form.oidc_connect_redirect_url,
      oidc_connect_frontend_redirect_url:
        form.oidc_connect_frontend_redirect_url,
      oidc_connect_token_auth_method: form.oidc_connect_token_auth_method,
      oidc_connect_use_pkce: form.oidc_connect_use_pkce,
      oidc_connect_validate_id_token: form.oidc_connect_validate_id_token,
      oidc_connect_allowed_signing_algs: form.oidc_connect_allowed_signing_algs,
      oidc_connect_clock_skew_seconds: form.oidc_connect_clock_skew_seconds,
      oidc_connect_require_email_verified:
        form.oidc_connect_require_email_verified,
      oidc_connect_userinfo_email_path: form.oidc_connect_userinfo_email_path,
      oidc_connect_userinfo_id_path: form.oidc_connect_userinfo_id_path,
      oidc_connect_userinfo_username_path:
        form.oidc_connect_userinfo_username_path,
      github_oauth_enabled: form.github_oauth_enabled,
      github_oauth_client_id: form.github_oauth_client_id,
      github_oauth_client_secret:
        form.github_oauth_client_secret || undefined,
      github_oauth_redirect_url: form.github_oauth_redirect_url,
      github_oauth_frontend_redirect_url:
        form.github_oauth_frontend_redirect_url,
      google_oauth_enabled: form.google_oauth_enabled,
      google_oauth_client_id: form.google_oauth_client_id,
      google_oauth_client_secret:
        form.google_oauth_client_secret || undefined,
      google_oauth_redirect_url: form.google_oauth_redirect_url,
      google_oauth_frontend_redirect_url:
        form.google_oauth_frontend_redirect_url,
      enable_model_fallback: form.enable_model_fallback,
      fallback_model_anthropic: form.fallback_model_anthropic,
      fallback_model_openai: form.fallback_model_openai,
      fallback_model_gemini: form.fallback_model_gemini,
      fallback_model_antigravity: form.fallback_model_antigravity,
      enable_identity_patch: form.enable_identity_patch,
      identity_patch_prompt: form.identity_patch_prompt,
      min_claude_code_version: form.min_claude_code_version,
      max_claude_code_version: form.max_claude_code_version,
      allow_ungrouped_key_scheduling: form.allow_ungrouped_key_scheduling,
      enable_fingerprint_unification: form.enable_fingerprint_unification,
      enable_metadata_passthrough: form.enable_metadata_passthrough,
      enable_cch_signing: form.enable_cch_signing,
      enable_anthropic_cache_ttl_1h_injection:
        form.enable_anthropic_cache_ttl_1h_injection,
      rewrite_message_cache_control: form.rewrite_message_cache_control,
      antigravity_user_agent_version:
        form.antigravity_user_agent_version?.trim() || "",
      openai_codex_user_agent:
        form.openai_codex_user_agent?.trim() || "",
      openai_allow_claude_code_codex_plugin:
        form.openai_allow_claude_code_codex_plugin,
      openai_oauth_compat_mode: "off",
      openai_cockpit_tools_compat: false,
      openai_codex_direct_force_ws:
        form.openai_codex_direct_force_ws,
      openai_codex_direct_tls_fingerprint_profile_id:
        Math.max(
          -1,
          Math.floor(
            Number(form.openai_codex_direct_tls_fingerprint_profile_id) || 0,
          ),
        ),
      client_request_debug_log_enabled: form.client_request_debug_log_enabled,
      codex_stability_mode: form.codex_stability_mode || "codex",
      codex_stability_dynamic_header_timeout_enabled:
        form.codex_stability_dynamic_header_timeout_enabled,
      codex_stability_request_phase_failover_enabled:
        form.codex_stability_request_phase_failover_enabled,
      codex_stability_suppress_client_timeout_headers:
        form.codex_stability_suppress_client_timeout_headers,
      codex_stability_stream_keepalive_enabled:
        form.codex_stability_stream_keepalive_enabled,
      codex_autopilot_enabled: form.codex_autopilot_enabled,
      codex_autopilot_observe_only: form.codex_autopilot_observe_only,
      openai_path_health_circuit_breaker_enabled:
        form.openai_path_health_circuit_breaker_enabled,
      openai_fast_lane_enabled: form.openai_fast_lane_enabled,
      realtime_balance_confirm_top_n: Math.max(
        0,
        Math.floor(Number(form.realtime_balance_confirm_top_n) || 0),
      ),
      realtime_balance_confirm_timeout_ms: Math.max(
        0,
        Math.floor(Number(form.realtime_balance_confirm_timeout_ms) || 0),
      ),
      openai_header_race_enabled: form.openai_header_race_enabled,
      openai_header_race_delay_ms: Math.max(
        0,
        Math.floor(Number(form.openai_header_race_delay_ms) || 0),
      ),
      openai_header_race_daily_budget: Math.max(
        0,
        Math.floor(Number(form.openai_header_race_daily_budget) || 0),
      ),
      openai_request_snapshot_enabled: form.openai_request_snapshot_enabled,
      openai_request_snapshot_retention_hours: Math.max(
        1,
        Math.floor(Number(form.openai_request_snapshot_retention_hours) || 72),
      ),
      context_journal_backend: form.context_journal_backend || "memory",
      context_journal_ttl_hours: Math.max(
        0,
        Math.floor(Number(form.context_journal_ttl_hours) || 0),
      ),
      // Payment configuration
      payment_enabled: form.payment_enabled,
      risk_control_enabled: form.risk_control_enabled,
      payment_min_amount: Number(form.payment_min_amount) || 0,
      payment_max_amount: Number(form.payment_max_amount) || 0,
      payment_daily_limit: Number(form.payment_daily_limit) || 0,
      payment_max_pending_orders: Number(form.payment_max_pending_orders) || 0,
      payment_order_timeout_minutes:
        Number(form.payment_order_timeout_minutes) || 0,
      payment_balance_disabled: form.payment_balance_disabled,
      payment_balance_recharge_multiplier:
        Number(form.payment_balance_recharge_multiplier) || 1,
      payment_recharge_fee_rate: Number(form.payment_recharge_fee_rate) || 0,
      payment_enabled_types: form.payment_enabled_types,
      payment_load_balance_strategy: form.payment_load_balance_strategy,
      payment_product_name_prefix: form.payment_product_name_prefix,
      payment_product_name_suffix: form.payment_product_name_suffix,
      payment_help_image_url: form.payment_help_image_url,
      payment_help_text: form.payment_help_text,
      payment_cancel_rate_limit_enabled: form.payment_cancel_rate_limit_enabled,
      payment_cancel_rate_limit_max:
        Number(form.payment_cancel_rate_limit_max) || 10,
      payment_cancel_rate_limit_window:
        Number(form.payment_cancel_rate_limit_window) || 1,
      payment_cancel_rate_limit_unit: form.payment_cancel_rate_limit_unit,
      payment_cancel_rate_limit_window_mode:
        form.payment_cancel_rate_limit_window_mode,
      payment_alipay_force_qrcode: form.payment_alipay_force_qrcode,
      openai_advanced_scheduler_enabled: form.openai_advanced_scheduler_enabled,
      openai_scheduler_exhaustion_probe_infinite_wait_enabled:
        form.openai_scheduler_exhaustion_probe_infinite_wait_enabled,
      openai_scheduler_exhaustion_probe_notify_enabled:
        form.openai_scheduler_exhaustion_probe_notify_enabled,
      openai_scheduler_exhaustion_probe_notify_channel:
        form.openai_scheduler_exhaustion_probe_notify_channel || "feishu_webhook",
      openai_scheduler_exhaustion_probe_notify_after_seconds:
        Number(form.openai_scheduler_exhaustion_probe_notify_after_seconds) ||
        60,
      openai_scheduler_exhaustion_probe_notify_repeat_seconds:
        Number(form.openai_scheduler_exhaustion_probe_notify_repeat_seconds) ||
        300,
      openai_scheduler_exhaustion_probe_notify_feishu_webhook_url:
        form.openai_scheduler_exhaustion_probe_notify_feishu_webhook_url.trim(),
      openai_scheduler_exhaustion_probe_notify_feishu_app_id:
        form.openai_scheduler_exhaustion_probe_notify_feishu_app_id.trim(),
      openai_scheduler_exhaustion_probe_notify_feishu_app_secret:
        form.openai_scheduler_exhaustion_probe_notify_feishu_app_secret.trim(),
      openai_scheduler_exhaustion_probe_notify_feishu_domain:
        form.openai_scheduler_exhaustion_probe_notify_feishu_domain.trim(),
      openai_scheduler_exhaustion_probe_notify_feishu_receive_id_type:
        form.openai_scheduler_exhaustion_probe_notify_feishu_receive_id_type ||
        "chat_id",
      openai_scheduler_exhaustion_probe_notify_feishu_receive_id:
        form.openai_scheduler_exhaustion_probe_notify_feishu_receive_id.trim(),
      openai_scheduler_exhaustion_probe_notify_recovered_enabled:
        form.openai_scheduler_exhaustion_probe_notify_recovered_enabled,
      // 余额、订阅到期与账号限额通知
      balance_low_notify_enabled: form.balance_low_notify_enabled,
      balance_low_notify_threshold:
        Number(form.balance_low_notify_threshold) || 0,
      balance_low_notify_recharge_url: (form.balance_low_notify_recharge_url =
        form.balance_low_notify_recharge_url || currentOrigin),
      subscription_expiry_notify_enabled:
        form.subscription_expiry_notify_enabled,
      account_quota_notify_enabled: form.account_quota_notify_enabled,
      account_quota_notify_emails: (
        form.account_quota_notify_emails || []
      ).filter((e) => e.email.trim() !== ""),
      // Channel Monitor feature switch
      channel_monitor_enabled: form.channel_monitor_enabled,
      channel_monitor_default_interval_seconds:
        Number(form.channel_monitor_default_interval_seconds) || 60,
      // Available Channels feature switch
      available_channels_enabled: form.available_channels_enabled,
      // 模型广场功能开关与页面说明。
      model_plaza_enabled: form.model_plaza_enabled,
      model_plaza_require_auth: form.model_plaza_require_auth,
      model_plaza_description: form.model_plaza_description,
      // Affiliate (邀请返利) feature switch
      affiliate_enabled: form.affiliate_enabled,
    };

    // 仅当 openai_fast_policy_settings 已成功从后端加载时才回写，
    // 否则省略整个字段，让后端保留既有规则（含默认值）。
    if (openaiFastPolicyLoaded.value) {
      payload.openai_fast_policy_settings = {
        rules: openaiFastPolicyForm.rules.map((rule) => {
          const whitelist = (rule.model_whitelist || [])
            .map((p) => p.trim())
            .filter((p) => p !== "");
          const hasWhitelist = whitelist.length > 0;
          return {
            service_tier: rule.service_tier,
            action: rule.action,
            scope: rule.scope,
            user_ids:
              rule.user_ids && rule.user_ids.length > 0
                ? [...rule.user_ids]
                : undefined,
            error_message:
              rule.action === "block" ? rule.error_message : undefined,
            model_whitelist: hasWhitelist ? whitelist : undefined,
            fallback_action: hasWhitelist
              ? rule.fallback_action || "pass"
              : undefined,
            fallback_error_message:
              hasWhitelist && rule.fallback_action === "block"
                ? rule.fallback_error_message
                : undefined,
          };
        }),
      };
    }

    appendAuthSourceDefaultsToUpdateRequest(payload, authSourceDefaults);

    const updated = await adminAPI.settings.updateSettings(payload);
    for (const [key, value] of Object.entries(updated)) {
      if (key === "openai_fast_policy_settings") continue;
      if (value !== null && value !== undefined) {
        (form as Record<string, unknown>)[key] = value;
      }
    }
    form.openai_oauth_compat_mode = "off";
    form.openai_cockpit_tools_compat = false;
    form.openai_codex_direct_force_ws =
      updated.openai_codex_direct_force_ws === true;
    form.openai_codex_direct_tls_fingerprint_profile_id =
      typeof updated.openai_codex_direct_tls_fingerprint_profile_id ===
      "number"
        ? updated.openai_codex_direct_tls_fingerprint_profile_id
        : 0;
    Object.assign(authSourceDefaults, buildAuthSourceDefaultsState(updated));
    registrationEmailSuffixWhitelistTags.value =
      normalizeRegistrationEmailSuffixDomains(
        updated.registration_email_suffix_whitelist,
      );
    tablePageSizeOptionsInput.value = formatTablePageSizeOptions(
      Array.isArray(updated.table_page_size_options)
        ? updated.table_page_size_options
        : [10, 20, 50, 100],
    );
    registrationEmailSuffixWhitelistDraft.value = "";
    form.smtp_password = "";
    smtpPasswordManuallyEdited.value = false;
    form.turnstile_secret_key = "";
    form.linuxdo_connect_client_secret = "";
    form.dingtalk_connect_client_secret = "";
    form.github_oauth_client_secret = "";
    form.google_oauth_client_secret = "";
    form.wechat_connect_app_secret = "";
    form.wechat_connect_open_app_secret = "";
    form.wechat_connect_mp_app_secret = "";
    form.wechat_connect_mobile_app_secret = "";
    const updatedWechatCapabilities = resolveWeChatConnectModeCapabilities(
      updated.wechat_connect_open_enabled,
      updated.wechat_connect_mp_enabled,
      updated.wechat_connect_mobile_enabled,
      updated.wechat_connect_mode,
    );
    form.wechat_connect_open_enabled = updatedWechatCapabilities.openEnabled;
    form.wechat_connect_mp_enabled = updatedWechatCapabilities.mpEnabled;
    form.wechat_connect_mobile_enabled =
      updatedWechatCapabilities.mobileEnabled;
    form.wechat_connect_mode = deriveWeChatConnectStoredMode(
      updatedWechatCapabilities.openEnabled,
      updatedWechatCapabilities.mpEnabled,
      updatedWechatCapabilities.mobileEnabled,
      updated.wechat_connect_mode,
    );
    form.wechat_connect_scopes = defaultWeChatConnectScopesForMode(
      form.wechat_connect_mode,
    );
    form.oidc_connect_client_secret = "";
    // Refresh OpenAI fast/flex policy from server response
    if (
      updated.openai_fast_policy_settings &&
      Array.isArray(updated.openai_fast_policy_settings.rules)
    ) {
      openaiFastPolicyForm.rules =
        updated.openai_fast_policy_settings.rules.map((rule) => ({
          ...rule,
          user_ids: rule.user_ids ? [...rule.user_ids] : [],
          model_whitelist: rule.model_whitelist
            ? [...rule.model_whitelist]
            : [],
        }));
      openaiFastPolicyLoaded.value = true;
    }
    // Save web search emulation config separately (errors handled internally)
    const wsOk = await saveWebSearchConfig();
    // Refresh cached settings so sidebar/header update immediately
    await appStore.fetchPublicSettings(true);
    await adminSettingsStore.fetch(true);
    if (wsOk) {
      appStore.showSuccess(t("admin.settings.settingsSaved"));
    }
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.failedToSave")),
    );
  } finally {
    saving.value = false;
  }
}

async function testSmtpConnection() {
  testingSmtp.value = true;
  try {
    const smtpPasswordForTest = smtpPasswordManuallyEdited.value
      ? form.smtp_password
      : "";
    const result = await adminAPI.settings.testSmtpConnection({
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: smtpPasswordForTest,
      smtp_use_tls: form.smtp_use_tls,
    });
    // API returns { message: "..." } on success, errors are thrown as exceptions
    appStore.showSuccess(
      result.message || t("admin.settings.smtpConnectionSuccess"),
    );
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.failedToTestSmtp")),
    );
  } finally {
    testingSmtp.value = false;
  }
}

async function sendTestEmail() {
  if (!testEmailAddress.value) {
    appStore.showError(t("admin.settings.testEmail.enterRecipientHint"));
    return;
  }

  sendingTestEmail.value = true;
  try {
    const smtpPasswordForSend = smtpPasswordManuallyEdited.value
      ? form.smtp_password
      : "";
    const result = await adminAPI.settings.sendTestEmail({
      email: testEmailAddress.value,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: smtpPasswordForSend,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls,
    });
    // API returns { message: "..." } on success, errors are thrown as exceptions
    appStore.showSuccess(result.message || t("admin.settings.testEmailSent"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.failedToSendTestEmail")),
    );
  } finally {
    sendingTestEmail.value = false;
  }
}

// Admin API Key 方法
async function loadAdminApiKey() {
  adminApiKeyLoading.value = true;
  try {
    const status = await adminAPI.settings.getAdminApiKey();
    adminApiKeyExists.value = status.exists;
    adminApiKeyMasked.value = status.masked_key;
  } catch (_error: unknown) {
    // Silent fail - admin API key status is non-critical
  } finally {
    adminApiKeyLoading.value = false;
  }
}

async function createAdminApiKey() {
  adminApiKeyOperating.value = true;
  try {
    const result = await adminAPI.settings.regenerateAdminApiKey();
    newAdminApiKey.value = result.key;
    adminApiKeyExists.value = true;
    adminApiKeyMasked.value =
      result.key.substring(0, 10) + "..." + result.key.slice(-4);
    appStore.showSuccess(t("admin.settings.adminApiKey.keyGenerated"));
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("common.error")));
  } finally {
    adminApiKeyOperating.value = false;
  }
}

async function regenerateAdminApiKey() {
  if (!confirm(t("admin.settings.adminApiKey.regenerateConfirm"))) return;
  await createAdminApiKey();
}

async function deleteAdminApiKey() {
  if (!confirm(t("admin.settings.adminApiKey.deleteConfirm"))) return;
  adminApiKeyOperating.value = true;
  try {
    await adminAPI.settings.deleteAdminApiKey();
    adminApiKeyExists.value = false;
    adminApiKeyMasked.value = "";
    newAdminApiKey.value = "";
    appStore.showSuccess(t("admin.settings.adminApiKey.keyDeleted"));
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("common.error")));
  } finally {
    adminApiKeyOperating.value = false;
  }
}

function copyNewKey() {
  navigator.clipboard
    .writeText(newAdminApiKey.value)
    .then(() => {
      appStore.showSuccess(t("admin.settings.adminApiKey.keyCopied"));
    })
    .catch(() => {
      appStore.showError(t("common.copyFailed"));
    });
}

// Overload Cooldown 方法
async function loadOverloadCooldownSettings() {
  overloadCooldownLoading.value = true;
  try {
    const settings = await adminAPI.settings.getOverloadCooldownSettings();
    Object.assign(overloadCooldownForm, settings);
  } catch (_error: unknown) {
    // Silent fail - settings will use defaults
  } finally {
    overloadCooldownLoading.value = false;
  }
}

async function saveOverloadCooldownSettings() {
  overloadCooldownSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateOverloadCooldownSettings({
      enabled: overloadCooldownForm.enabled,
      cooldown_minutes: overloadCooldownForm.cooldown_minutes,
    });
    Object.assign(overloadCooldownForm, updated);
    appStore.showSuccess(t("admin.settings.overloadCooldown.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.overloadCooldown.saveFailed"),
      ),
    );
  } finally {
    overloadCooldownSaving.value = false;
  }
}

// Rate Limit Cooldown (429) 方法
async function loadRateLimit429CooldownSettings() {
  rateLimit429CooldownLoading.value = true;
  try {
    const settings = await adminAPI.settings.getRateLimit429CooldownSettings();
    Object.assign(rateLimit429CooldownForm, settings);
  } catch (_error: unknown) {
    // Silent fail - settings will use defaults
  } finally {
    rateLimit429CooldownLoading.value = false;
  }
}

async function saveRateLimit429CooldownSettings() {
  rateLimit429CooldownSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateRateLimit429CooldownSettings({
      enabled: rateLimit429CooldownForm.enabled,
      cooldown_seconds: rateLimit429CooldownForm.cooldown_seconds,
    });
    Object.assign(rateLimit429CooldownForm, updated);
    appStore.showSuccess(t("admin.settings.rateLimit429Cooldown.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.rateLimit429Cooldown.saveFailed"),
      ),
    );
  } finally {
    rateLimit429CooldownSaving.value = false;
  }
}

// Stream Timeout 方法
async function loadStreamTimeoutSettings() {
  streamTimeoutLoading.value = true;
  try {
    const settings = await adminAPI.settings.getStreamTimeoutSettings();
    Object.assign(streamTimeoutForm, settings);
  } catch (_error: unknown) {
    // Silent fail - settings will use defaults
  } finally {
    streamTimeoutLoading.value = false;
  }
}

async function saveStreamTimeoutSettings() {
  streamTimeoutSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateStreamTimeoutSettings({
      enabled: streamTimeoutForm.enabled,
      action: streamTimeoutForm.action,
      temp_unsched_minutes: streamTimeoutForm.temp_unsched_minutes,
      threshold_count: streamTimeoutForm.threshold_count,
      threshold_window_minutes: streamTimeoutForm.threshold_window_minutes,
    });
    Object.assign(streamTimeoutForm, updated);
    appStore.showSuccess(t("admin.settings.streamTimeout.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.streamTimeout.saveFailed"),
      ),
    );
  } finally {
    streamTimeoutSaving.value = false;
  }
}

// Rectifier 方法
async function loadRectifierSettings() {
  rectifierLoading.value = true;
  try {
    const settings = await adminAPI.settings.getRectifierSettings();
    Object.assign(rectifierForm, settings);
    // 确保 patterns 是数组（旧数据可能为 null）
    if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
      rectifierForm.apikey_signature_patterns = [];
    }
  } catch (_error: unknown) {
    // Silent fail - settings will use defaults
  } finally {
    rectifierLoading.value = false;
  }
}

async function saveRectifierSettings() {
  rectifierSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateRectifierSettings({
      enabled: rectifierForm.enabled,
      thinking_signature_enabled: rectifierForm.thinking_signature_enabled,
      thinking_budget_enabled: rectifierForm.thinking_budget_enabled,
      apikey_signature_enabled: rectifierForm.apikey_signature_enabled,
      apikey_signature_patterns: rectifierForm.apikey_signature_patterns.filter(
        (p) => p.trim() !== "",
      ),
    });
    Object.assign(rectifierForm, updated);
    if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
      rectifierForm.apikey_signature_patterns = [];
    }
    appStore.showSuccess(t("admin.settings.rectifier.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.rectifier.saveFailed")),
    );
  } finally {
    rectifierSaving.value = false;
  }
}

const betaPolicyActionOptions = computed(() => [
  { value: "pass", label: t("admin.settings.betaPolicy.actionPass") },
  { value: "filter", label: t("admin.settings.betaPolicy.actionFilter") },
  { value: "block", label: t("admin.settings.betaPolicy.actionBlock") },
]);

const betaPolicyScopeOptions = computed(() => [
  { value: "all", label: t("admin.settings.betaPolicy.scopeAll") },
  { value: "oauth", label: t("admin.settings.betaPolicy.scopeOAuth") },
  { value: "apikey", label: t("admin.settings.betaPolicy.scopeAPIKey") },
  { value: "bedrock", label: t("admin.settings.betaPolicy.scopeBedrock") },
]);

// Beta Policy 方法
const betaDisplayNames: Record<string, string> = {
  "fast-mode-2026-02-01": "Fast Mode",
  "context-1m-2025-08-07": "Context 1M",
};

// 快捷预设：按 beta_token 定义预设方案
const betaPresets: Record<
  string,
  Array<{
    label: string;
    description: string;
    action: "pass" | "filter" | "block";
    model_whitelist: string[];
    fallback_action: "pass" | "filter" | "block";
  }>
> = {
  "context-1m-2025-08-07": [
    {
      label: t("admin.settings.betaPolicy.presetOpusOnly"),
      description: t("admin.settings.betaPolicy.presetOpusOnlyDesc"),
      action: "pass",
      model_whitelist: ["claude-opus-4-6"],
      fallback_action: "filter",
    },
  ],
};

// 常用模型模式（具体 ID + 通配符示例）
const commonModelPatterns = [
  "claude-opus-4-6",
  "claude-fable-5",
  "claude-sonnet-4-6",
  "claude-opus-*",
  "claude-sonnet-*",
];

function getBetaDisplayName(token: string): string {
  return betaDisplayNames[token] || token;
}

function applyBetaPreset(
  rule: (typeof betaPolicyForm.rules)[number],
  preset: {
    action: "pass" | "filter" | "block";
    model_whitelist: string[];
    fallback_action: "pass" | "filter" | "block";
  },
) {
  rule.action = preset.action;
  rule.model_whitelist = [...preset.model_whitelist];
  rule.fallback_action = preset.fallback_action;
}

function addQuickPattern(
  rule: (typeof betaPolicyForm.rules)[number],
  pattern: string,
) {
  if (!rule.model_whitelist) rule.model_whitelist = [];
  if (!rule.model_whitelist.includes(pattern)) {
    rule.model_whitelist.push(pattern);
  }
}

async function loadBetaPolicySettings() {
  betaPolicyLoading.value = true;
  try {
    const settings = await adminAPI.settings.getBetaPolicySettings();
    betaPolicyForm.rules = settings.rules;
  } catch (_error: unknown) {
    // Silent fail - settings will use defaults
  } finally {
    betaPolicyLoading.value = false;
  }
}

// ==================== OpenAI Fast/Flex Policy ====================

const openaiFastPolicyTierOptions = computed(() => [
  { value: "all", label: t("admin.settings.openaiFastPolicy.tierAll") },
  {
    value: "priority",
    label: t("admin.settings.openaiFastPolicy.tierPriority"),
  },
  { value: "flex", label: t("admin.settings.openaiFastPolicy.tierFlex") },
]);

const openaiFastPolicyActionOptions = computed(() => [
  { value: "pass", label: t("admin.settings.openaiFastPolicy.actionPass") },
  { value: "filter", label: t("admin.settings.openaiFastPolicy.actionFilter") },
  { value: "block", label: t("admin.settings.openaiFastPolicy.actionBlock") },
]);

const openaiFastPolicyScopeOptions = computed(() => [
  { value: "all", label: t("admin.settings.openaiFastPolicy.scopeAll") },
  { value: "oauth", label: t("admin.settings.openaiFastPolicy.scopeOAuth") },
  { value: "apikey", label: t("admin.settings.openaiFastPolicy.scopeAPIKey") },
  {
    value: "bedrock",
    label: t("admin.settings.openaiFastPolicy.scopeBedrock"),
  },
]);

type OpenAIRoutePolicyPresetKey =
  | "codex_stable"
  | "first_token_fast"
  | "quota_saver"
  | "long_context";

const openaiRoutePolicyPresets = computed(() => [
  {
    key: "codex_stable" as const,
    label: t("admin.settings.openaiFastPolicy.presetCodexStable"),
  },
  {
    key: "first_token_fast" as const,
    label: t("admin.settings.openaiFastPolicy.presetFirstTokenFast"),
  },
  {
    key: "quota_saver" as const,
    label: t("admin.settings.openaiFastPolicy.presetQuotaSaver"),
  },
  {
    key: "long_context" as const,
    label: t("admin.settings.openaiFastPolicy.presetLongContext"),
  },
]);

function cloneOpenAIFastPolicyRules(
  rules: OpenAIFastPolicyRule[],
): OpenAIFastPolicyRule[] {
  return rules.map((rule) => ({
    ...rule,
    model_whitelist: rule.model_whitelist ? [...rule.model_whitelist] : [],
  }));
}

function applyOpenAIRoutePolicyPreset(key: OpenAIRoutePolicyPresetKey) {
  const presets: Record<OpenAIRoutePolicyPresetKey, OpenAIFastPolicyRule[]> = {
    codex_stable: [
      { service_tier: "priority", action: "filter", scope: "all", model_whitelist: [] },
      { service_tier: "flex", action: "filter", scope: "all", model_whitelist: [] },
    ],
    first_token_fast: [
      { service_tier: "priority", action: "pass", scope: "all", model_whitelist: [] },
      { service_tier: "flex", action: "filter", scope: "all", model_whitelist: [] },
    ],
    quota_saver: [
      { service_tier: "priority", action: "filter", scope: "all", model_whitelist: [] },
      { service_tier: "flex", action: "filter", scope: "all", model_whitelist: [] },
    ],
    long_context: [
      {
        service_tier: "priority",
        action: "filter",
        scope: "all",
        model_whitelist: ["gpt-5.4*", "gpt-5.5*"],
        fallback_action: "filter",
      },
      { service_tier: "flex", action: "filter", scope: "all", model_whitelist: [] },
    ],
  };
  openaiFastPolicyForm.rules = cloneOpenAIFastPolicyRules(presets[key]);
  openaiFastPolicyLoaded.value = true;
  appStore.showInfo(t("admin.settings.openaiFastPolicy.presetApplied"));
}

function addOpenAIFastPolicyRule() {
  openaiFastPolicyForm.rules.push({
    service_tier: "priority",
    action: "filter",
    scope: "all",
    user_ids: [],
    error_message: "",
    model_whitelist: [],
    fallback_action: "pass",
    fallback_error_message: "",
  });
}

function removeOpenAIFastPolicyRule(index: number) {
  openaiFastPolicyForm.rules.splice(index, 1);
}

function addOpenAIFastPolicyUserID(rule: OpenAIFastPolicyRule) {
  if (!rule.user_ids) rule.user_ids = [];
  rule.user_ids.push(0);
}

function removeOpenAIFastPolicyUserID(rule: OpenAIFastPolicyRule, idx: number) {
  rule.user_ids?.splice(idx, 1);
}

function addOpenAIFastPolicyModelPattern(rule: OpenAIFastPolicyRule) {
  if (!rule.model_whitelist) rule.model_whitelist = [];
  rule.model_whitelist.push("");
}

function removeOpenAIFastPolicyModelPattern(
  rule: OpenAIFastPolicyRule,
  idx: number,
) {
  rule.model_whitelist?.splice(idx, 1);
}

async function saveBetaPolicySettings() {
  betaPolicySaving.value = true;
  try {
    // Clean up empty patterns before saving
    const cleanedRules = betaPolicyForm.rules.map((rule) => {
      const whitelist = rule.model_whitelist?.filter((p) => p.trim() !== "");
      const hasWhitelist = whitelist && whitelist.length > 0;
      return {
        beta_token: rule.beta_token,
        action: rule.action,
        scope: rule.scope,
        error_message: rule.error_message,
        model_whitelist: hasWhitelist ? whitelist : undefined,
        fallback_action: hasWhitelist
          ? rule.fallback_action || "pass"
          : undefined,
        fallback_error_message:
          hasWhitelist && rule.fallback_action === "block"
            ? rule.fallback_error_message
            : undefined,
      };
    });
    const updated = await adminAPI.settings.updateBetaPolicySettings({
      rules: cleanedRules,
    });
    betaPolicyForm.rules = updated.rules;
    appStore.showSuccess(t("admin.settings.betaPolicy.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.betaPolicy.saveFailed")),
    );
  } finally {
    betaPolicySaving.value = false;
  }
}

// ==================== Provider Management ====================

const allPaymentTypes = computed(() => [
  { value: "easypay", label: t("payment.methods.easypay") },
  { value: "alipay", label: t("payment.methods.alipay") },
  { value: "wxpay", label: t("payment.methods.wxpay") },
  { value: "stripe", label: t("payment.methods.stripe") },
  { value: "airwallex", label: t("payment.methods.airwallex") },
]);

function isPaymentTypeEnabled(type: string): boolean {
  return form.payment_enabled_types.includes(type);
}

const hasAnyPaymentTypeEnabled = computed(
  () => form.payment_enabled_types.length > 0,
);

function togglePaymentType(type: string) {
  if (form.payment_enabled_types.includes(type)) {
    form.payment_enabled_types = form.payment_enabled_types.filter(
      (t) => t !== type,
    );
    // Disable all provider instances matching this type
    disableProvidersByType(type);
  } else {
    form.payment_enabled_types = [...form.payment_enabled_types, type];
  }
}

async function disableProvidersByType(type: string) {
  const matching = providers.value.filter(
    (p) => p.provider_key === type && p.enabled,
  );
  for (const p of matching) {
    try {
      await adminAPI.payment.updateProvider(p.id, { enabled: false });
      p.enabled = false;
    } catch (err: unknown) {
      slog("disable provider failed", p.id, err);
    }
  }
}

function slog(...args: unknown[]) {
  console.warn("[payment]", ...args);
}

const providersLoading = ref(false);
const providerSaving = ref(false);
const providers = ref<ProviderInstance[]>([]);
const showProviderDialog = ref(false);
const showDeleteProviderDialog = ref(false);
const editingProvider = ref<ProviderInstance | null>(null);
const deletingProviderId = ref<number | null>(null);
const providerDialogRef = ref<InstanceType<
  typeof PaymentProviderDialog
> | null>(null);

const providerKeyOptions = computed(() => [
  { value: "easypay", label: t("admin.settings.payment.providerEasypay") },
  { value: "alipay", label: t("admin.settings.payment.providerAlipay") },
  { value: "wxpay", label: t("admin.settings.payment.providerWxpay") },
  { value: "stripe", label: t("admin.settings.payment.providerStripe") },
  { value: "airwallex", label: t("admin.settings.payment.providerAirwallex") },
]);

const enabledProviderKeyOptions = computed(() => {
  const enabled = form.payment_enabled_types;
  return providerKeyOptions.value.filter((opt) => enabled.includes(opt.value));
});

const loadBalanceOptions = computed(() => [
  {
    value: "round-robin",
    label: t("admin.settings.payment.strategyRoundRobin"),
  },
  {
    value: "least-amount",
    label: t("admin.settings.payment.strategyLeastAmount"),
  },
]);

const cancelRateLimitUnitOptions = computed(() => [
  {
    value: "minute",
    label: t("admin.settings.payment.cancelRateLimitUnitMinute"),
  },
  { value: "hour", label: t("admin.settings.payment.cancelRateLimitUnitHour") },
  { value: "day", label: t("admin.settings.payment.cancelRateLimitUnitDay") },
]);

const cancelRateLimitModeOptions = computed(() => [
  {
    value: "rolling",
    label: t("admin.settings.payment.cancelRateLimitWindowModeRolling"),
  },
  {
    value: "fixed",
    label: t("admin.settings.payment.cancelRateLimitWindowModeFixed"),
  },
]);

type ProviderEnablementCandidate = Pick<
  ProviderInstance,
  "id" | "provider_key" | "supported_types" | "enabled" | "name"
>;

function getProviderVisibleMethods(
  provider: ProviderEnablementCandidate,
): Array<"alipay" | "wxpay"> {
  if (!provider.enabled) {
    return [];
  }

  const supportedTypes = Array.isArray(provider.supported_types)
    ? provider.supported_types
    : [];
  const methods = new Set<"alipay" | "wxpay">();
  const addMethod = (type: string) => {
    const method = normalizeVisibleMethod(type);
    if (method === "alipay" || method === "wxpay") {
      methods.add(method);
    }
  };

  if (provider.provider_key === "alipay") {
    if (supportedTypes.length === 0) {
      methods.add("alipay");
    } else {
      supportedTypes.forEach((type) => {
        if (normalizeVisibleMethod(type) === "alipay") {
          methods.add("alipay");
        }
      });
    }
  } else if (provider.provider_key === "wxpay") {
    if (supportedTypes.length === 0) {
      methods.add("wxpay");
    } else {
      supportedTypes.forEach((type) => {
        if (normalizeVisibleMethod(type) === "wxpay") {
          methods.add("wxpay");
        }
      });
    }
  } else if (provider.provider_key === "easypay") {
    supportedTypes.forEach(addMethod);
  }

  return Array.from(methods);
}

function findProviderEnablementConflict(
  candidate: ProviderEnablementCandidate,
): { method: "alipay" | "wxpay"; conflicting: ProviderInstance } | null {
  const claimedMethods = getProviderVisibleMethods(candidate);
  if (claimedMethods.length === 0) {
    return null;
  }

  for (const other of providers.value) {
    if (other.id === candidate.id || !other.enabled) {
      continue;
    }

    const otherMethods = getProviderVisibleMethods(other);
    const matchedMethod = claimedMethods.find((method) =>
      otherMethods.includes(method),
    );
    if (matchedMethod) {
      return {
        method: matchedMethod,
        conflicting: other,
      };
    }
  }

  return null;
}

function showProviderEnablementConflict(
  conflict: { method: "alipay" | "wxpay"; conflicting: ProviderInstance },
) {
  appStore.showError(
    t("admin.settings.payment.enableConflict", {
      method: t(`payment.methods.${conflict.method}`),
      provider: conflict.conflicting.name,
    }),
  );
}

async function loadProviders() {
  providersLoading.value = true;
  try {
    const res = await adminAPI.payment.getProviders();
    providers.value = res.data || [];
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  } finally {
    providersLoading.value = false;
  }
}

function openCreateProvider() {
  editingProvider.value = null;
  providerDialogRef.value?.reset(
    enabledProviderKeyOptions.value[0]?.value || "easypay",
  );
  showProviderDialog.value = true;
}

function openEditProvider(provider: ProviderInstance) {
  editingProvider.value = provider;
  providerDialogRef.value?.loadProvider(provider);
  showProviderDialog.value = true;
}

async function handleSaveProvider(payload: Partial<ProviderInstance>) {
  providerSaving.value = true;
  try {
    const candidate: ProviderEnablementCandidate = {
      id: editingProvider.value?.id ?? 0,
      provider_key:
        payload.provider_key ?? editingProvider.value?.provider_key ?? "",
      supported_types:
        payload.supported_types ?? editingProvider.value?.supported_types ?? [],
      enabled: payload.enabled ?? editingProvider.value?.enabled ?? false,
      name: payload.name ?? editingProvider.value?.name ?? "",
    };
    const conflict = findProviderEnablementConflict(candidate);
    if (conflict) {
      showProviderEnablementConflict(conflict);
      return;
    }

    if (editingProvider.value) {
      await adminAPI.payment.updateProvider(editingProvider.value.id, payload);
    } else {
      await adminAPI.payment.createProvider(payload);
    }
    showProviderDialog.value = false;
    // Reload full list (API returns decrypted/formatted data with correct sort order)
    await loadProviders();
    // Auto-save settings so provider changes take effect immediately
    await saveSettings();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  } finally {
    providerSaving.value = false;
  }
}

async function handleToggleField(
  provider: ProviderInstance,
  field: "enabled" | "refund_enabled" | "allow_user_refund",
) {
  let newValue: boolean;
  if (field === "enabled") newValue = !provider.enabled;
  else if (field === "refund_enabled") newValue = !provider.refund_enabled;
  else newValue = !provider.allow_user_refund;

  if (field === "enabled" && newValue) {
    const conflict = findProviderEnablementConflict({
      id: provider.id,
      provider_key: provider.provider_key,
      supported_types: provider.supported_types,
      enabled: true,
      name: provider.name,
    });
    if (conflict) {
      showProviderEnablementConflict(conflict);
      return;
    }
  }

  const payload: Record<string, boolean> = { [field]: newValue };
  // Cascade: turning off refund_enabled also turns off allow_user_refund
  if (field === "refund_enabled" && !newValue) {
    payload.allow_user_refund = false;
  }
  try {
    await adminAPI.payment.updateProvider(provider.id, payload);
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  }
}

async function handleToggleType(provider: ProviderInstance, type: string) {
  const updated = provider.supported_types.includes(type)
    ? provider.supported_types.filter((t) => t !== type)
    : [...provider.supported_types, type];
  const conflict = findProviderEnablementConflict({
    id: provider.id,
    provider_key: provider.provider_key,
    supported_types: updated,
    enabled: provider.enabled,
    name: provider.name,
  });
  if (conflict) {
    showProviderEnablementConflict(conflict);
    return;
  }
  try {
    await adminAPI.payment.updateProvider(provider.id, {
      supported_types: updated,
    } as any);
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  }
}

function confirmDeleteProvider(provider: ProviderInstance) {
  deletingProviderId.value = provider.id;
  showDeleteProviderDialog.value = true;
}

async function handleReorderProviders(
  updates: { id: number; sort_order: number }[],
) {
  try {
    await Promise.all(
      updates.map((u) =>
        adminAPI.payment.updateProvider(u.id, {
          sort_order: u.sort_order,
        } as Partial<ProviderInstance>),
      ),
    );
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
    loadProviders();
  }
}

async function handleDeleteProvider() {
  if (!deletingProviderId.value) return;
  try {
    await adminAPI.payment.deleteProvider(deletingProviderId.value);
    appStore.showSuccess(t("common.deleted"));
    showDeleteProviderDialog.value = false;
    loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  }
}

onMounted(() => {
  loadSettings();
  loadSubscriptionGroups();
  loadCodexDirectTLSFingerprintProfiles();
  loadAdminApiKey();
  loadOverloadCooldownSettings();
  loadRateLimit429CooldownSettings();
  loadStreamTimeoutSettings();
  loadRectifierSettings();
  loadBetaPolicySettings();
  loadProviders();
});

// =========================
// Affiliate (邀请返利) 专属用户管理
// =========================

interface AffiliateState {
  loading: boolean;
  entries: AffiliateAdminEntry[];
  total: number;
  page: number;
  pageSize: number;
  search: string;
  selected: number[];
  searchTimer: number | null;
}

const affiliateState = reactive<AffiliateState>({
  loading: false,
  entries: [],
  total: 0,
  page: 1,
  pageSize: 20,
  search: "",
  selected: [],
  searchTimer: null,
});

// `rate` is typed as string|number because <input type="number"> makes Vue's
// v-model auto-cast the bound value to a Number on every keystroke. We keep
// both shapes and normalize at read time.
interface AffiliateModalState {
  open: boolean;
  mode: "add" | "edit";
  saving: boolean;
  userQuery: string;
  userResults: AffiliateSimpleUser[];
  selectedUser: AffiliateSimpleUser | null;
  editingEntry: AffiliateAdminEntry | null;
  code: string;
  rate: string | number;
  searchTimer: number | null;
}

const affiliateModal = reactive<AffiliateModalState>({
  open: false,
  mode: "add",
  saving: false,
  userQuery: "",
  userResults: [],
  selectedUser: null,
  editingEntry: null,
  code: "",
  rate: "",
  searchTimer: null,
});

const affiliateBatchModal = reactive<{
  open: boolean;
  saving: boolean;
  rate: string | number;
}>({
  open: false,
  saving: false,
  rate: "",
});

// affiliateConfirmDialog drives the project-standard <ConfirmDialog>. We can't
// `await` the user's response from the dialog component, so the confirm action
// runs from the @confirm callback once the user clicks the dialog's confirm
// button.
const affiliateConfirmDialog = reactive<{
  show: boolean;
  title: string;
  message: string;
  confirmText: string;
  pending: (() => Promise<unknown>) | null;
}>({
  show: false,
  title: "",
  message: "",
  confirmText: "",
  pending: null,
});

function openAffiliateConfirm(
  title: string,
  message: string,
  confirmText: string,
  fn: () => Promise<unknown>,
) {
  affiliateConfirmDialog.title = title;
  affiliateConfirmDialog.message = message;
  affiliateConfirmDialog.confirmText = confirmText;
  affiliateConfirmDialog.pending = fn;
  affiliateConfirmDialog.show = true;
}

async function handleAffiliateConfirm() {
  const fn = affiliateConfirmDialog.pending;
  affiliateConfirmDialog.show = false;
  affiliateConfirmDialog.pending = null;
  if (!fn) return;
  try {
    await fn();
    appStore.showSuccess(t("common.saved"));
    await loadAffiliateUsers();
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  }
}

function cancelAffiliateConfirm() {
  affiliateConfirmDialog.show = false;
  affiliateConfirmDialog.pending = null;
}

// debounceTimer wires a single timer slot to a callback with a delay,
// canceling any pending invocation. Used for type-as-you-go search inputs.
function debounceTimer(slot: { searchTimer: number | null }, delayMs: number, run: () => void) {
  if (slot.searchTimer != null) window.clearTimeout(slot.searchTimer);
  slot.searchTimer = window.setTimeout(run, delayMs);
}

// parseRebateRate validates 0-100 numeric input. Returns the parsed number on
// success, null when the field is empty (caller decides empty semantics), or
// undefined on invalid input (after surfacing a toast).
//
// Accepts unknown because <input type="number"> makes Vue's v-model coerce
// the value to Number on each keystroke (e.g. typing "30" lands a `30: number`
// in state, not a `"30": string`). String("") and (30).trim() would crash, so
// we normalize here instead of forcing every caller to remember.
function parseRebateRate(raw: unknown): number | null | undefined {
  const s = String(raw ?? "").trim();
  if (s === "") return null;
  const parsed = Number(s);
  if (Number.isNaN(parsed) || parsed < 0 || parsed > 100) {
    appStore.showError(t("admin.settings.features.affiliate.modal.errorBadRate"));
    return undefined;
  }
  return parsed;
}

async function loadAffiliateUsers() {
  affiliateState.loading = true;
  try {
    const res = await affiliatesAPI.listUsers({
      page: affiliateState.page,
      page_size: affiliateState.pageSize,
      search: affiliateState.search,
    });
    affiliateState.entries = res.items ?? [];
    affiliateState.total = res.total ?? 0;
    // Drop selections that are no longer visible.
    const visibleIds = new Set(affiliateState.entries.map((e) => e.user_id));
    affiliateState.selected = affiliateState.selected.filter((id) => visibleIds.has(id));
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    affiliateState.loading = false;
  }
}

function onAffiliateSearchInput() {
  debounceTimer(affiliateState, 300, () => {
    affiliateState.page = 1;
    loadAffiliateUsers();
  });
}

function changeAffiliatePage(page: number) {
  if (page < 1) return;
  affiliateState.page = page;
  loadAffiliateUsers();
}

function toggleAffiliateSelectAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked;
  affiliateState.selected = checked ? affiliateState.entries.map((entry) => entry.user_id) : [];
}

function toggleAffiliateSelect(userId: number) {
  const idx = affiliateState.selected.indexOf(userId);
  if (idx >= 0) affiliateState.selected.splice(idx, 1);
  else affiliateState.selected.push(userId);
}

// openAffiliateModal opens the add/edit modal, prefilling fields from the
// edited entry when present and resetting them otherwise.
function openAffiliateModal(entry: AffiliateAdminEntry | null) {
  affiliateModal.open = true;
  affiliateModal.mode = entry ? "edit" : "add";
  affiliateModal.userQuery = "";
  affiliateModal.userResults = [];
  affiliateModal.selectedUser = null;
  affiliateModal.editingEntry = entry;
  affiliateModal.code = entry?.aff_code_custom ? entry.aff_code : "";
  affiliateModal.rate =
    entry?.aff_rebate_rate_percent != null ? String(entry.aff_rebate_rate_percent) : "";
}

function closeAffiliateModal() {
  affiliateModal.open = false;
  if (affiliateModal.searchTimer != null) {
    window.clearTimeout(affiliateModal.searchTimer);
    affiliateModal.searchTimer = null;
  }
}

function onAffiliateUserSearchInput() {
  const q = affiliateModal.userQuery.trim();
  if (!q) {
    affiliateModal.userResults = [];
    return;
  }
  debounceTimer(affiliateModal, 300, async () => {
    try {
      affiliateModal.userResults = await affiliatesAPI.lookupUsers(q);
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    }
  });
}

// selectAffiliateUser picks a user from the dropdown and collapses the search
// UI. Clearing the result list also clears the visual dropdown.
function selectAffiliateUser(user: AffiliateSimpleUser) {
  affiliateModal.selectedUser = user;
  affiliateModal.userQuery = "";
  affiliateModal.userResults = [];
}

function clearSelectedAffiliateUser() {
  affiliateModal.selectedUser = null;
}

// affiliateModalCanSubmit guards the Save button: must have a user picked AND
// produce at least one field change. Without this the admin could "save" an
// empty payload that silently does nothing — the user reported exactly that
// confusion.
const affiliateModalCanSubmit = computed(() => {
  if (affiliateModal.mode === "add") {
    if (!affiliateModal.selectedUser) return false;
  } else if (!affiliateModal.editingEntry) {
    return false;
  }
  const codeFilled = affiliateModal.code.trim() !== "";
  const rateFilled = String(affiliateModal.rate ?? "").trim() !== "";
  if (codeFilled || rateFilled) return true;
  // Edit mode + empty rate input is a meaningful "clear" only if the user
  // currently has an exclusive rate to clear.
  return (
    affiliateModal.mode === "edit" &&
    affiliateModal.editingEntry?.aff_rebate_rate_percent != null
  );
});

async function submitAffiliateModal() {
  if (!affiliateModalCanSubmit.value) {
    // Should be unreachable because the button is disabled, but keep a guard.
    appStore.showError(t("admin.settings.features.affiliate.modal.errorEmpty"));
    return;
  }

  let userId: number;
  if (affiliateModal.mode === "add") {
    userId = affiliateModal.selectedUser!.id;
  } else {
    userId = affiliateModal.editingEntry!.user_id;
  }

  const payload: Parameters<typeof affiliatesAPI.updateUserSettings>[1] = {};
  const codeRaw = affiliateModal.code.trim();
  if (codeRaw) payload.aff_code = codeRaw.toUpperCase();

  const rateInput = parseRebateRate(affiliateModal.rate);
  if (rateInput === undefined) return; // toast already shown
  if (rateInput === null) {
    if (affiliateModal.mode === "edit" && affiliateModal.editingEntry?.aff_rebate_rate_percent != null) {
      payload.clear_rebate_rate = true;
    }
  } else {
    payload.aff_rebate_rate_percent = rateInput;
  }

  affiliateModal.saving = true;
  try {
    await affiliatesAPI.updateUserSettings(userId, payload);
    appStore.showSuccess(t("common.saved"));
    closeAffiliateModal();
    affiliateState.page = 1;
    await loadAffiliateUsers();
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    affiliateModal.saving = false;
  }
}

// askResetAffiliateUser prompts via the project ConfirmDialog, then on confirm
// calls the backend "reset all" endpoint that clears both the exclusive rate
// AND regenerates the invite code as a system random one.
function askResetAffiliateUser(entry: AffiliateAdminEntry) {
  openAffiliateConfirm(
    t("admin.settings.features.affiliate.customUsers.resetTitle"),
    t("admin.settings.features.affiliate.customUsers.resetMessage", {
      email: entry.email || `#${entry.user_id}`,
    }),
    t("common.delete"),
    () => affiliatesAPI.clearUserSettings(entry.user_id),
  );
}

function openAffiliateBatchModal() {
  if (affiliateState.selected.length === 0) return;
  affiliateBatchModal.open = true;
  affiliateBatchModal.rate = "";
}

async function submitAffiliateBatchModal() {
  const rateInput = parseRebateRate(affiliateBatchModal.rate);
  if (rateInput === undefined) return;
  const userIDs = [...affiliateState.selected];
  const payload: Parameters<typeof affiliatesAPI.batchSetRate>[0] =
    rateInput === null
      ? { user_ids: userIDs, clear: true }
      : { user_ids: userIDs, aff_rebate_rate_percent: rateInput };

  affiliateBatchModal.saving = true;
  try {
    await affiliatesAPI.batchSetRate(payload);
    appStore.showSuccess(t("common.saved"));
    affiliateBatchModal.open = false;
    affiliateState.selected = [];
    await loadAffiliateUsers();
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    affiliateBatchModal.saving = false;
  }
}

// Load the per-user table the first time the affiliate switch is observed
// as enabled. The form starts disabled and is updated to the server's value
// after the settings load — so this fires either when the saved value is
// truthy on first paint, or when the admin manually toggles it on.
watch(
  () => form.affiliate_enabled,
  (enabled, prev) => {
    if (enabled && !prev) {
      loadAffiliateUsers();
    }
  },
);

// bypass_registration 与身份同步三开关仅在 internal_only 模式下生效。切换 policy 到其它值时，
// 立即把相关字段重置为 false，避免保存请求里残留旧值。后端 admin handler 与
// 配置加载层都有 coerce 兜底，这里是 UX 层的同步而非安全防线。
watch(
  () => form.dingtalk_connect_corp_restriction_policy,
  (policy) => {
    if (policy !== "internal_only") {
      if (form.dingtalk_connect_bypass_registration) form.dingtalk_connect_bypass_registration = false;
      if (form.dingtalk_connect_sync_corp_email) form.dingtalk_connect_sync_corp_email = false;
      if (form.dingtalk_connect_sync_display_name) form.dingtalk_connect_sync_display_name = false;
      if (form.dingtalk_connect_sync_dept) form.dingtalk_connect_sync_dept = false;
    }
  },
);
</script>

<style scoped>
.default-sub-group-select :deep(.select-trigger) {
  @apply h-[42px];
}

.default-sub-delete-btn {
  @apply h-[42px];
}

</style>
