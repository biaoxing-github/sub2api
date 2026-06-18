import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { defineComponent, h } from "vue";

import AgreementSettingsTab from "../AgreementSettingsTab.vue";
import type { LoginAgreementDocument } from "@/types";

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
  const form = {
    login_agreement_enabled: false,
    login_agreement_mode: "modal",
    login_agreement_updated_at: "2026-03-31",
    login_agreement_documents: [
      {
        id: "terms",
        title: "服务条款",
        content_md: "# Terms",
      },
    ] as LoginAgreementDocument[],
  };

  const wrapper = mount(AgreementSettingsTab, {
    props: {
      form,
      localText: (zh: string, en: string) => `${zh}/${en}`,
      loginAgreementRoutePath: (doc: LoginAgreementDocument, index: number) =>
        `/legal/${doc.id || `doc-${index + 1}`}`,
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        Icon: true,
      },
    },
  });

  return { wrapper, form };
};

describe("AgreementSettingsTab", () => {
  it("emits document add/remove events and updates display mode locally", async () => {
    const { wrapper, form } = mountTab();

    expect(wrapper.text()).toContain("登录条款确认/Login agreement");
    expect(wrapper.text()).toContain("/legal/terms");

    const addButton = wrapper
      .findAll("button")
      .find((button) => button.text().includes("添加文档/Add document"));
    expect(addButton).toBeDefined();
    await addButton?.trigger("click");

    expect(wrapper.emitted("add-document")).toHaveLength(1);

    await wrapper.get('[data-test="remove-agreement-document"]').trigger("click");

    expect(wrapper.emitted("remove-document")?.[0]).toEqual([0]);

    const checkboxModeButton = wrapper
      .findAll("button")
      .find((button) => button.text().includes("复选框/Checkbox"));
    expect(checkboxModeButton).toBeDefined();
    await checkboxModeButton?.trigger("click");

    expect(form.login_agreement_mode).toBe("checkbox");
  });
});
