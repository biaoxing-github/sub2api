import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive } from "vue";

import UsersSettingsTab from "../UsersSettingsTab.vue";
import type {
  AuthSourceDefaultsState,
  AuthSourceType,
  DefaultSubscriptionSetting,
} from "@/api/admin/settings";
import type { AdminGroup } from "@/types";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
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
            emit("update:modelValue", Number((event.target as HTMLSelectElement).value));
          },
        },
        (props.options as Array<{ value: number; label: string }>).map((option) =>
          h("option", { value: option.value }, option.label),
        ),
      );
  },
});

const mountTab = () => {
  const form = reactive({
    default_balance: 1,
    default_concurrency: 2,
    default_user_rpm_limit: 60,
    force_email_on_third_party_signup: false,
    default_subscriptions: [
      {
        group_id: 10,
        validity_days: 30,
      },
    ] as DefaultSubscriptionSetting[],
  });

  const subscriptionGroups: AdminGroup[] = [
    {
      id: 10,
      name: "Basic",
      platform: "openai",
      description: "basic group",
      subscription_type: "monthly",
      rate_multiplier: 1,
    } as AdminGroup,
    {
      id: 11,
      name: "Pro",
      platform: "claude",
      description: "pro group",
      subscription_type: "yearly",
      rate_multiplier: 2,
    } as AdminGroup,
  ];

  const defaultSubscriptionGroupOptions = subscriptionGroups.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    platform: group.platform,
    subscriptionType: group.subscription_type,
    rate: group.rate_multiplier,
  }));

  const authSourceDefaults = reactive<AuthSourceDefaultsState>({
    email: {
      grant_on_signup: true,
      grant_on_first_bind: false,
      balance: 5,
      concurrency: 3,
      subscriptions: [
        {
          group_id: 10,
          validity_days: 7,
        },
      ],
    },
    linuxdo: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
    oidc: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
    wechat: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
    github: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
    google: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
    dingtalk: {
      grant_on_signup: false,
      grant_on_first_bind: false,
      balance: 0,
      concurrency: 1,
      subscriptions: [],
    },
  });

  const callbacks = {
    addDefaultSubscription: vi.fn(),
    removeDefaultSubscription: vi.fn(),
    addAuthSourceDefaultSubscription: vi.fn(),
    removeAuthSourceDefaultSubscription: vi.fn(),
  };

  const wrapper = mount(UsersSettingsTab, {
    props: {
      form,
      subscriptionGroups,
      defaultSubscriptionGroupOptions,
      authSourceDefaults,
      authSourceDefaultsMeta: [
        {
          source: "email" as AuthSourceType,
          title: "Email signup",
          description: "Email defaults",
        },
      ],
      ...callbacks,
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        Select: SelectStub,
        GroupBadge: { template: '<span data-test="group-badge">{{ name }}</span>', props: ["name"] },
        GroupOptionItem: { template: '<span data-test="group-option">{{ name }}</span>', props: ["name"] },
      },
    },
  });

  return { wrapper, form, authSourceDefaults, callbacks };
};

describe("UsersSettingsTab", () => {
  it("renders user defaults and delegates subscription actions", async () => {
    const { wrapper, form, authSourceDefaults, callbacks } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.defaults.title");
    expect(wrapper.text()).toContain("admin.settings.authSourceDefaults.title");
    expect(wrapper.text()).toContain("Email signup");

    await wrapper.get('input[placeholder="0.00"]').setValue("8.5");
    expect(form.default_balance).toBe(8.5);

    await wrapper.get(".toggle-stub").setValue(true);
    expect(form.force_email_on_third_party_signup).toBe(true);

    await wrapper.get("select.select-stub").setValue("11");
    expect(form.default_subscriptions[0].group_id).toBe(11);

    await wrapper.get(".btn.btn-secondary.btn-sm").trigger("click");
    expect(callbacks.addDefaultSubscription).toHaveBeenCalledTimes(1);

    await wrapper.get(".default-sub-delete-btn").trigger("click");
    expect(callbacks.removeDefaultSubscription).toHaveBeenCalledWith(0);

    await wrapper.get('[data-testid="auth-source-email-enabled"]').setValue(false);
    expect(authSourceDefaults.email.grant_on_signup).toBe(false);

    authSourceDefaults.email.grant_on_signup = true;
    await wrapper.vm.$nextTick();
    await wrapper.findAll(".btn.btn-secondary.btn-sm")[1].trigger("click");
    expect(callbacks.addAuthSourceDefaultSubscription).toHaveBeenCalledWith("email");

    await wrapper.findAll(".btn.btn-secondary").at(-1)?.trigger("click");
    expect(callbacks.removeAuthSourceDefaultSubscription).toHaveBeenCalledWith("email", 0);
  });
});
