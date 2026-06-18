import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { defineComponent, h, reactive } from "vue";

import EmailSettingsTab from "../EmailSettingsTab.vue";
import type { NotifyEmailEntry } from "@/types";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("../EmailTemplateEditor.vue", () => ({
  default: defineComponent({
    name: "EmailTemplateEditorStub",
    setup() {
      return () => h("div", { "data-test": "email-template-editor" });
    },
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

const IconStub = defineComponent({
  props: {
    name: {
      type: String,
      default: "",
    },
  },
  setup(props) {
    return () => h("span", { "data-icon": props.name });
  },
});

const mountTab = (enabled = true) => {
  const form = reactive({
    email_verify_enabled: enabled,
    smtp_host: "smtp.example.test",
    smtp_port: 587,
    smtp_username: "robot",
    smtp_password: "",
    smtp_password_configured: true,
    smtp_from_email: "robot@example.test",
    smtp_from_name: "Robot",
    smtp_use_tls: true,
    subscription_expiry_notify_enabled: true,
    balance_low_notify_enabled: true,
    balance_low_notify_threshold: 5,
    balance_low_notify_recharge_url: "",
    account_quota_notify_enabled: true,
    account_quota_notify_emails: [
      { email: "ops@example.test", disabled: false, verified: true },
    ] as NotifyEmailEntry[],
  });
  const callbacks = {
    testSmtpConnection: vi.fn(),
    sendTestEmail: vi.fn(),
    addQuotaNotifyEmail: vi.fn(() => {
      form.account_quota_notify_emails.push({
        email: "",
        disabled: false,
        verified: true,
      });
    }),
    markSmtpPasswordEdited: vi.fn(),
  };
  const wrapper = mount(EmailSettingsTab, {
    props: {
      form,
      testEmailAddress: "",
      testingSmtp: false,
      sendingTestEmail: false,
      loadFailed: false,
      currentOrigin: "https://gateway.example.test",
      ...callbacks,
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        Icon: IconStub,
        EmailTemplateEditor: {
          template: '<div data-test="email-template-editor"></div>',
        },
      },
    },
  });

  return { wrapper, form, callbacks };
};

describe("EmailSettingsTab", () => {
  it("shows disabled hint when email verification is off", () => {
    const { wrapper } = mountTab(false);

    expect(wrapper.text()).toContain("admin.settings.emailTabDisabledTitle");
    expect(wrapper.find('[data-test="email-template-editor"]').exists()).toBe(true);
  });

  it("renders SMTP settings and delegates email actions", async () => {
    const { wrapper, form, callbacks } = mountTab(true);

    expect(wrapper.text()).toContain("admin.settings.smtp.title");
    expect(wrapper.find('[data-test="email-template-editor"]').exists()).toBe(true);

    await wrapper.get('input[placeholder="admin.settings.smtp.hostPlaceholder"]').setValue("mail.example.test");
    expect(form.smtp_host).toBe("mail.example.test");

    await wrapper.get('input[type="password"]').trigger("keydown");
    expect(callbacks.markSmtpPasswordEdited).toHaveBeenCalledTimes(1);

    await wrapper.get('[data-test="test-smtp"]').trigger("click");
    expect(callbacks.testSmtpConnection).toHaveBeenCalledTimes(1);

    await wrapper.get('[data-test="test-email-address"]').setValue("admin@example.test");
    expect(wrapper.emitted("update:testEmailAddress")?.[0]).toEqual(["admin@example.test"]);

    await wrapper.setProps({ testEmailAddress: "admin@example.test" });
    await wrapper.get('[data-test="send-test-email"]').trigger("click");
    expect(callbacks.sendTestEmail).toHaveBeenCalledTimes(1);

    await wrapper.get('[data-test="remove-quota-email-0"]').trigger("click");
    expect(form.account_quota_notify_emails).toHaveLength(0);

    await wrapper.get('[data-test="add-quota-email"]').trigger("click");
    expect(callbacks.addQuotaNotifyEmail).toHaveBeenCalledTimes(1);
    expect(form.account_quota_notify_emails).toHaveLength(1);
  });
});
