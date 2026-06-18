import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive } from "vue";

import GatewaySettingsTab from "../GatewaySettingsTab.vue";

vi.mock("vue-i18n", async (importOriginal) => {
  const actual = await importOriginal<typeof import("vue-i18n")>();
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  };
});

const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    return () =>
      h("input", {
        class: "toggle-stub",
        type: "checkbox",
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit("update:modelValue", (event.target as HTMLInputElement).checked);
        },
      });
  },
});

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number],
      default: "",
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    return () =>
      h(
        "select",
        {
          class: "select-stub",
          value: props.modelValue,
          onChange: (event: Event) => {
            emit("update:modelValue", (event.target as HTMLSelectElement).value);
          },
        },
        (props.options as Array<{ value: string | number; label: string }>).map((option) =>
          h("option", { value: option.value }, option.label),
        ),
      );
  },
});

const mountTab = () => {
  const form = reactive({
    allow_ungrouped_key_scheduling: false,
    min_claude_code_version: "",
    max_claude_code_version: "",
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
    enable_fingerprint_unification: true,
    enable_metadata_passthrough: false,
    enable_cch_signing: false,
    enable_anthropic_cache_ttl_1h_injection: false,
    rewrite_message_cache_control: false,
    antigravity_user_agent_version: "",
    openai_codex_user_agent: "",
    openai_allow_claude_code_codex_plugin: false,
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
    openai_header_race_enabled: false,
    openai_header_race_delay_ms: 3500,
    openai_header_race_daily_budget: 0,
    openai_request_snapshot_enabled: true,
    openai_request_snapshot_retention_hours: 72,
    openai_fast_lane_enabled: true,
    realtime_balance_confirm_top_n: 3,
    realtime_balance_confirm_timeout_ms: 1200,
    context_journal_backend: "memory",
    context_journal_ttl_hours: 24,
  });

  const wrapper = mount(GatewaySettingsTab, {
    props: {
      form,
      overloadCooldownLoading: false,
      overloadCooldownSaving: false,
      overloadCooldownForm: reactive({ enabled: true, cooldown_minutes: 10 }),
      saveOverloadCooldownSettings: vi.fn(),
      rateLimit429CooldownLoading: false,
      rateLimit429CooldownSaving: false,
      rateLimit429CooldownForm: reactive({ enabled: true, cooldown_seconds: 5 }),
      saveRateLimit429CooldownSettings: vi.fn(),
      streamTimeoutLoading: false,
      streamTimeoutSaving: false,
      streamTimeoutForm: reactive({
        enabled: true,
        action: "temp_unsched",
        temp_unsched_minutes: 5,
        threshold_count: 3,
        threshold_window_minutes: 10,
      }),
      saveStreamTimeoutSettings: vi.fn(),
      rectifierLoading: false,
      rectifierSaving: false,
      rectifierForm: reactive({
        enabled: true,
        thinking_signature_enabled: true,
        thinking_budget_enabled: true,
        apikey_signature_enabled: false,
        apikey_signature_patterns: [],
      }),
      saveRectifierSettings: vi.fn(),
      betaPolicyLoading: false,
      betaPolicySaving: false,
      betaPolicyForm: reactive({ rules: [] }),
      betaPolicyActionOptions: [],
      betaPolicyScopeOptions: [],
      betaPresets: {},
      commonModelPatterns: [],
      getBetaDisplayName: (token: string) => token,
      applyBetaPreset: vi.fn(),
      addQuickPattern: vi.fn(),
      saveBetaPolicySettings: vi.fn(),
      openaiFastPolicyForm: reactive({ rules: [] }),
      openaiRoutePolicyPresets: [],
      applyOpenAIRoutePolicyPreset: vi.fn(),
      openaiFastPolicyTierOptions: [],
      openaiFastPolicyActionOptions: [],
      openaiFastPolicyScopeOptions: [],
      addOpenAIFastPolicyRule: vi.fn(),
      removeOpenAIFastPolicyRule: vi.fn(),
      addOpenAIFastPolicyModelPattern: vi.fn(),
      removeOpenAIFastPolicyModelPattern: vi.fn(),
      codexDirectTLSFingerprintProfiles: [],
      webSearchConfig: reactive({ enabled: false, providers: [] }),
      expandedProviders: reactive({}),
      apiKeyVisible: reactive({}),
      webSearchProxies: [],
      wsTestQuery: "",
      "onUpdate:wsTestQuery": vi.fn(),
      wsTestDialogOpen: false,
      "onUpdate:wsTestDialogOpen": vi.fn(),
      wsTestLoading: false,
      wsTestResult: null,
      openTestDialog: vi.fn(),
      toggleProviderExpand: vi.fn(),
      removeWebSearchProvider: vi.fn(),
      addWebSearchProvider: vi.fn(),
      formatSubscribedAt: vi.fn(() => ""),
      parseSubscribedAt: vi.fn(() => null),
      quotaPercentage: vi.fn(() => 0),
      resetWebSearchUsage: vi.fn(),
      copyApiKey: vi.fn(),
      testWebSearchProvider: vi.fn(),
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        Select: SelectStub,
        ProxySelector: { template: '<div data-test="proxy-selector" />' },
        SettingsSectionSaveButton: {
          template: '<button type="button" data-test="section-save"><slot /></button>',
        },
      },
    },
  });

  return { wrapper, form };
};

describe("GatewaySettingsTab", () => {
  it("renders both gateway setting groups after extraction", () => {
    const { wrapper } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.overloadCooldown.title");
    expect(wrapper.text()).toContain("admin.settings.openaiExperimentalScheduler.title");
    expect(wrapper.text()).toContain("admin.settings.gatewayForwarding.title");
    expect(wrapper.text()).toContain("admin.settings.webSearchEmulation.title");
  });
});
