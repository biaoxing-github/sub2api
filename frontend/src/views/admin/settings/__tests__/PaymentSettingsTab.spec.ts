import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive } from "vue";

import PaymentSettingsTab from "../PaymentSettingsTab.vue";
import type { ProviderInstance } from "@/types/payment";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
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

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number, Boolean],
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
        (props.options as Array<{ value: string; label: string }>).map((option) =>
          h("option", { value: option.value }, option.label),
        ),
      );
  },
});

const ImageUploadStub = defineComponent({
  props: {
    modelValue: {
      type: String,
      default: "",
    },
    placeholder: {
      type: String,
      default: "",
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    return () =>
      h("input", {
        class: "image-upload-stub",
        "data-placeholder": props.placeholder,
        value: props.modelValue,
        onInput: (event: Event) => {
          emit("update:modelValue", (event.target as HTMLInputElement).value);
        },
      });
  },
});

const PaymentProviderListStub = defineComponent({
  props: {
    providers: {
      type: Array,
      default: () => [],
    },
    loading: {
      type: Boolean,
      default: false,
    },
    canCreate: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["refresh", "create", "edit", "delete", "toggleField", "toggleType", "reorder"],
  setup(props, { emit }) {
    const provider = (props.providers as ProviderInstance[])[0];
    return () =>
      h("div", { "data-test": "payment-provider-list" }, [
        h("span", { "data-test": "provider-count" }, String((props.providers as unknown[]).length)),
        h("button", { "data-test": "provider-refresh", onClick: () => emit("refresh") }, "refresh"),
        h("button", { "data-test": "provider-create", onClick: () => emit("create") }, "create"),
        provider
          ? h("button", { "data-test": "provider-edit", onClick: () => emit("edit", provider) }, "edit")
          : null,
        provider
          ? h("button", { "data-test": "provider-delete", onClick: () => emit("delete", provider) }, "delete")
          : null,
        provider
          ? h(
              "button",
              { "data-test": "provider-toggle-field", onClick: () => emit("toggleField", provider, "enabled") },
              "toggle field",
            )
          : null,
        provider
          ? h(
              "button",
              { "data-test": "provider-toggle-type", onClick: () => emit("toggleType", provider, "alipay") },
              "toggle type",
            )
          : null,
        h("button", { "data-test": "provider-reorder", onClick: () => emit("reorder", [{ id: 9, sort_order: 0 }]) }, "reorder"),
      ]);
  },
});

const mountTab = () => {
  const form = reactive({
    payment_enabled: true,
    payment_product_name_prefix: "Sub2API",
    payment_product_name_suffix: "CNY",
    payment_min_amount: 1,
    payment_max_amount: 10000,
    payment_daily_limit: 50000,
    payment_balance_recharge_multiplier: 1,
    payment_recharge_fee_rate: 0,
    payment_order_timeout_minutes: 30,
    payment_max_pending_orders: 3,
    payment_load_balance_strategy: "round-robin",
    payment_cancel_rate_limit_enabled: false,
    payment_cancel_rate_limit_window_mode: "rolling",
    payment_cancel_rate_limit_window: 1,
    payment_cancel_rate_limit_unit: "day",
    payment_cancel_rate_limit_max: 10,
    payment_alipay_force_qrcode: false,
    payment_enabled_types: ["alipay"],
    payment_help_image_url: "",
    payment_help_text: "",
  });
  const provider: ProviderInstance = {
    id: 9,
    provider_key: "alipay",
    name: "Alipay",
    config: {},
    supported_types: ["alipay"],
    enabled: true,
    payment_mode: "",
    refund_enabled: false,
    allow_user_refund: false,
    limits: "",
    sort_order: 0,
  };
  const callbacks = {
    togglePaymentType: vi.fn(),
    isPaymentTypeEnabled: vi.fn((type: string) => type === "alipay"),
    loadProviders: vi.fn(),
    openCreateProvider: vi.fn(),
    openEditProvider: vi.fn(),
    confirmDeleteProvider: vi.fn(),
    handleToggleField: vi.fn(),
    handleToggleType: vi.fn(),
    handleReorderProviders: vi.fn(),
  };

  const wrapper = mount(PaymentSettingsTab, {
    props: {
      form,
      paymentGuideHref: "https://example.test/PAYMENT_CN.md",
      paymentMethodsHref: "https://example.test/PAYMENT_CN.md#methods",
      providers: [provider],
      providersLoading: false,
      hasAnyPaymentTypeEnabled: true,
      allPaymentTypes: [
        { value: "alipay", label: "Alipay" },
        { value: "wxpay", label: "WeChat Pay" },
      ],
      loadBalanceOptions: [
        { value: "round-robin", label: "Round robin" },
      ],
      cancelRateLimitModeOptions: [
        { value: "rolling", label: "Rolling" },
      ],
      cancelRateLimitUnitOptions: [
        { value: "day", label: "Day" },
      ],
      ...callbacks,
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        Select: SelectStub,
        ImageUpload: ImageUploadStub,
        PaymentProviderList: PaymentProviderListStub,
      },
    },
  });

  return { wrapper, form, provider, callbacks };
};

describe("PaymentSettingsTab", () => {
  it("renders payment settings and delegates provider actions", async () => {
    const { wrapper, form, provider, callbacks } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.payment.title");
    expect(wrapper.get('a[href="https://example.test/PAYMENT_CN.md"]').exists()).toBe(true);
    expect(wrapper.get('a[href="https://example.test/PAYMENT_CN.md#methods"]').exists()).toBe(true);

    await wrapper.get('input[placeholder="Sub2API"]').setValue("Gateway");
    expect(form.payment_product_name_prefix).toBe("Gateway");

    await wrapper.get('[data-test="payment-type-wxpay"]').trigger("click");
    expect(callbacks.togglePaymentType).toHaveBeenCalledWith("wxpay");

    await wrapper.get('[data-test="provider-refresh"]').trigger("click");
    expect(callbacks.loadProviders).toHaveBeenCalledTimes(1);

    await wrapper.get('[data-test="provider-create"]').trigger("click");
    expect(callbacks.openCreateProvider).toHaveBeenCalledTimes(1);

    await wrapper.get('[data-test="provider-edit"]').trigger("click");
    expect(callbacks.openEditProvider).toHaveBeenCalledWith(provider);

    await wrapper.get('[data-test="provider-delete"]').trigger("click");
    expect(callbacks.confirmDeleteProvider).toHaveBeenCalledWith(provider);

    await wrapper.get('[data-test="provider-toggle-field"]').trigger("click");
    expect(callbacks.handleToggleField).toHaveBeenCalledWith(provider, "enabled");

    await wrapper.get('[data-test="provider-toggle-type"]').trigger("click");
    expect(callbacks.handleToggleType).toHaveBeenCalledWith(provider, "alipay");

    await wrapper.get('[data-test="provider-reorder"]').trigger("click");
    expect(callbacks.handleReorderProviders).toHaveBeenCalledWith([{ id: 9, sort_order: 0 }]);
  });
});
