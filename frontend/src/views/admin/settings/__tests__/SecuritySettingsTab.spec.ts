import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive, ref } from "vue";

import SecuritySettingsTab from "../SecuritySettingsTab.vue";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: ref("zh-CN"),
  }),
}));

const IconStub = defineComponent({
  props: {
    name: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    return () => h("span", { "data-icon": props.name });
  },
});

const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit, attrs }) {
    return () =>
      h("input", {
        ...attrs,
        class: "toggle-stub",
        type: "checkbox",
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit("update:modelValue", (event.target as HTMLInputElement).checked);
        },
      });
  },
});

const mountTab = () => {
  const form = reactive({
    registration_enabled: true,
    email_verify_enabled: false,
    promo_code_enabled: true,
    invitation_code_enabled: false,
    password_reset_enabled: true,
    totp_enabled: false,
    totp_encryption_key_configured: false,
    api_key_acl_trust_forwarded_ip: false,
    turnstile_enabled: false,
    turnstile_site_key: "",
    turnstile_secret_key: "",
    turnstile_secret_key_configured: false,
    linuxdo_connect_enabled: false,
    linuxdo_connect_client_id: "",
    linuxdo_connect_client_secret: "",
    linuxdo_connect_client_secret_configured: false,
    linuxdo_connect_redirect_url: "",
    github_oauth_enabled: true,
    github_oauth_client_id: "",
    github_oauth_client_secret: "",
    github_oauth_client_secret_configured: false,
    github_oauth_redirect_url: "",
    google_oauth_enabled: false,
    google_oauth_client_id: "",
    google_oauth_client_secret: "",
    google_oauth_client_secret_configured: false,
    google_oauth_redirect_url: "",
    wechat_connect_enabled: true,
    wechat_connect_app_id: "wx-main",
    wechat_connect_app_secret: "",
    wechat_connect_app_secret_configured: true,
    wechat_connect_open_enabled: false,
    wechat_connect_mp_enabled: true,
    wechat_connect_mobile_enabled: false,
    wechat_connect_mode: "mp",
    wechat_connect_open_app_id: "",
    wechat_connect_open_app_secret: "",
    wechat_connect_open_app_secret_configured: false,
    wechat_connect_mp_app_id: "wx-mp",
    wechat_connect_mp_app_secret: "",
    wechat_connect_mp_app_secret_configured: true,
    wechat_connect_mobile_app_id: "",
    wechat_connect_mobile_app_secret: "",
    wechat_connect_mobile_app_secret_configured: false,
    wechat_connect_scopes: "",
    wechat_connect_redirect_url: "",
    wechat_connect_frontend_redirect_url: "/auth/wechat/callback",
    dingtalk_connect_enabled: false,
    dingtalk_connect_client_id: "",
    dingtalk_connect_client_secret: "",
    dingtalk_connect_client_secret_configured: false,
    dingtalk_connect_redirect_url: "",
    dingtalk_connect_sync_display_name: false,
    dingtalk_connect_sync_corp_email: false,
    dingtalk_connect_sync_dept: false,
    dingtalk_connect_bypass_registration: false,
    oidc_connect_enabled: true,
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
    oidc_connect_use_pkce: true,
    oidc_connect_validate_id_token: true,
    oidc_connect_allowed_signing_algs: "RS256",
    oidc_connect_clock_skew_seconds: 120,
    oidc_connect_require_email_verified: false,
    oidc_connect_userinfo_email_path: "",
    oidc_connect_userinfo_id_path: "",
    oidc_connect_userinfo_username_path: "",
  });

  const callbacks = {
    createAdminApiKey: vi.fn(),
    regenerateAdminApiKey: vi.fn(),
    deleteAdminApiKey: vi.fn(),
    copyNewKey: vi.fn(),
    removeRegistrationEmailSuffixWhitelistTag: vi.fn(),
    handleRegistrationEmailSuffixWhitelistDraftInput: vi.fn(),
    handleRegistrationEmailSuffixWhitelistDraftKeydown: vi.fn(),
    commitRegistrationEmailSuffixWhitelistDraft: vi.fn(),
    handleRegistrationEmailSuffixWhitelistPaste: vi.fn(),
    setAndCopyLinuxdoRedirectUrl: vi.fn(),
    setAndCopyEmailOAuthRedirectUrl: vi.fn(),
    setAndCopyWeChatRedirectUrl: vi.fn(),
    setAndCopyOIDCRedirectUrl: vi.fn(),
    handleWeChatOpenEnabledChange: vi.fn(),
    handleWeChatMPEnabledChange: vi.fn(),
    handleWeChatMobileEnabledChange: vi.fn(),
  };
  const draftState = { value: "" };

  const wrapper = mount(SecuritySettingsTab, {
    props: {
      form,
      adminApiKeyLoading: false,
      adminApiKeyExists: false,
      adminApiKeyMasked: "",
      adminApiKeyOperating: false,
      newAdminApiKey: "sk-admin-new",
      registrationEmailSuffixWhitelistTags: ["example.com"],
      registrationEmailSuffixWhitelistDraft: draftState.value,
      "onUpdate:registrationEmailSuffixWhitelistDraft": async (value: string) => {
        draftState.value = value;
        await wrapper.setProps({ registrationEmailSuffixWhitelistDraft: value });
      },
      linuxdoRedirectUrlSuggestion: "https://example.test/api/v1/auth/oauth/linuxdo/callback",
      githubOAuthRedirectUrlSuggestion: "https://example.test/api/v1/auth/oauth/github/callback",
      googleOAuthRedirectUrlSuggestion: "https://example.test/api/v1/auth/oauth/google/callback",
      wechatRedirectUrlSuggestion: "https://example.test/api/v1/auth/oauth/wechat/callback",
      oidcRedirectUrlSuggestion: "https://example.test/api/v1/auth/oauth/oidc/callback",
      localText: (zh: string) => zh,
      ...callbacks,
    },
    global: {
      stubs: {
        Icon: IconStub,
        Toggle: ToggleStub,
      },
    },
  });

  return { wrapper, form, callbacks, draftState };
};

describe("SecuritySettingsTab", () => {
  it("renders security settings and delegates sensitive actions", async () => {
    const { wrapper, form, callbacks, draftState } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.adminApiKey.title");
    expect(wrapper.text()).toContain("admin.settings.registration.title");

    await wrapper.get("button.btn.btn-primary.btn-sm").trigger("click");
    expect(callbacks.createAdminApiKey).toHaveBeenCalledTimes(1);

    await wrapper
      .get('input[placeholder="admin.settings.registration.emailSuffixWhitelistPlaceholder"]')
      .setValue("blocked");
    expect(draftState.value).toBe("blocked");
    expect(callbacks.handleRegistrationEmailSuffixWhitelistDraftInput).toHaveBeenCalledTimes(1);

    await wrapper.get(".toggle-stub").setValue(false);
    expect(form.registration_enabled).toBe(false);

    await wrapper.get('[data-testid="wechat-connect-open-enabled"]').setValue(true);
    expect(callbacks.handleWeChatOpenEnabledChange).toHaveBeenCalledWith(true);

    await wrapper.get('[data-testid="github-oauth-apps-guide-link"]').trigger("click");
    expect(wrapper.get('[data-testid="github-oauth-apps-guide-link"]').attributes("href")).toBe(
      "https://github.com/settings/developers",
    );
  });
});
