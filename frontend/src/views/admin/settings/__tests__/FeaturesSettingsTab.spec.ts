import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive, ref } from "vue";

import FeaturesSettingsTab from "../FeaturesSettingsTab.vue";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.count ? `${key}:${String(params.count)}` : key,
  }),
}));

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

const mountTab = () => {
  const form = reactive({
    channel_monitor_enabled: true,
    channel_monitor_default_interval_seconds: 60,
    available_channels_enabled: false,
    model_plaza_enabled: false,
    model_plaza_require_auth: false,
    model_plaza_description: "",
    risk_control_enabled: false,
    affiliate_enabled: true,
    affiliate_rebate_rate: 20,
    affiliate_rebate_freeze_hours: 0,
    affiliate_rebate_duration_days: 0,
    affiliate_rebate_per_invitee_cap: 0,
  });
  const affiliateState = reactive({
    search: "",
    selected: [] as number[],
    entries: [
      {
        user_id: 7,
        email: "demo@example.com",
        username: "demo",
        aff_code: "DEMO7",
        aff_code_custom: true,
        aff_rebate_rate_percent: 15,
      },
    ],
    loading: false,
    total: 1,
    page: 1,
    pageSize: 20,
  });
  const affiliateModal = reactive({
    open: false,
    mode: "add",
    selectedUser: null,
    userQuery: "",
    userResults: [] as Array<{ id: number; email: string; username: string }>,
    editingEntry: null,
    code: "",
    rate: "",
    saving: false,
  });
  const affiliateBatchModal = reactive({
    open: false,
    rate: "",
    saving: false,
  });

  const callbacks = {
    openAffiliateModal: vi.fn(),
    onAffiliateSearchInput: vi.fn(),
    openAffiliateBatchModal: vi.fn(),
    toggleAffiliateSelectAll: vi.fn(),
    toggleAffiliateSelect: vi.fn(),
    changeAffiliatePage: vi.fn(),
    askResetAffiliateUser: vi.fn(),
    closeAffiliateModal: vi.fn(),
    clearSelectedAffiliateUser: vi.fn(),
    onAffiliateUserSearchInput: vi.fn(),
    selectAffiliateUser: vi.fn(),
    submitAffiliateModal: vi.fn(),
    submitAffiliateBatchModal: vi.fn(),
  };

  const wrapper = mount(FeaturesSettingsTab, {
    props: {
      form,
      affiliateState,
      affiliateModal,
      affiliateBatchModal,
      affiliateModalCanSubmit: ref(true),
      ...callbacks,
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        RouterLink: {
          props: ["to"],
          template: '<a :href="String(to)"><slot /></a>',
        },
      },
    },
  });

  return { wrapper, form, callbacks };
};

describe("FeaturesSettingsTab", () => {
  it("renders feature cards and delegates affiliate actions", async () => {
    const { wrapper, form, callbacks } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.features.channelMonitor.title");
    expect(wrapper.text()).toContain("admin.settings.features.modelPlaza.title");
    expect(wrapper.text()).toContain("admin.settings.features.affiliate.title");
    expect(wrapper.text()).toContain("demo@example.com");

    await wrapper.get('[data-test="affiliate-add-user"]').trigger("click");
    expect(callbacks.openAffiliateModal).toHaveBeenCalledWith(null);

    await wrapper.get('[data-test="affiliate-edit-user-7"]').trigger("click");
    expect(callbacks.openAffiliateModal).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 7 }),
    );

    await wrapper.get('[data-test="affiliate-delete-user-7"]').trigger("click");
    expect(callbacks.askResetAffiliateUser).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 7 }),
    );

    await wrapper.get('input[type="number"]').setValue("90");
    expect(form.channel_monitor_default_interval_seconds).toBe(90);
  });
});
