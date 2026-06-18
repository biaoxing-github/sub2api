import { describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";

import SettingsSectionSaveButton from "@/components/admin/settings/SettingsSectionSaveButton.vue";

vi.mock("vue-i18n", async () => {
  const actual = await vi.importActual<typeof import("vue-i18n")>("vue-i18n");
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  };
});

describe("SettingsSectionSaveButton", () => {
  it("renders an idle section save button and emits click", async () => {
    const wrapper = mount(SettingsSectionSaveButton, {
      props: {
        saving: false,
      },
    });

    const button = wrapper.get('[data-testid="settings-section-save-button"]');
    expect(button.attributes("type")).toBe("button");
    expect(button.text()).toContain("common.save");
    expect(button.attributes("disabled")).toBeUndefined();

    await button.trigger("click");
    expect(wrapper.emitted("click")).toHaveLength(1);
  });

  it("disables the button and shows saving state while saving", () => {
    const wrapper = mount(SettingsSectionSaveButton, {
      props: {
        saving: true,
      },
    });

    const button = wrapper.get('[data-testid="settings-section-save-button"]');
    expect(button.attributes("disabled")).toBeDefined();
    expect(button.text()).toContain("common.saving");
    expect(wrapper.get('[data-testid="settings-section-save-spinner"]').exists()).toBe(true);
  });
});
