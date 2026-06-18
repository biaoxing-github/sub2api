import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { computed, defineComponent, h } from "vue";

import GeneralSettingsTab from "../GeneralSettingsTab.vue";
import type { CustomEndpoint, CustomMenuItem } from "@/types";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.n ? `${key}:${String(params.n)}` : key,
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

const ImageUploadStub = defineComponent({
  inheritAttrs: false,
  props: {
    modelValue: {
      type: String,
      default: "",
    },
    size: {
      type: String,
      default: "",
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    const value = computed({
      get: () => props.modelValue,
      set: (next: string) => emit("update:modelValue", next),
    });
    return () => h("input", { class: "image-upload-stub", value: value.value });
  },
});

const mountTab = () => {
  const form = {
    backend_mode_enabled: false,
    site_name: "Sub2API",
    site_subtitle: "Gateway",
    api_base_url: "",
    table_default_page_size: 20,
    custom_endpoints: [
      { name: "Responses", endpoint: "/v1/responses", description: "OpenAI Responses" },
    ] as CustomEndpoint[],
    contact_info: "",
    doc_url: "",
    site_logo: "",
    home_content: "",
    hide_ccs_import_button: false,
    custom_menu_items: [
      {
        id: "docs",
        label: "Docs",
        icon_svg: "",
        url: "/docs",
        visibility: "user",
        sort_order: 0,
      },
      {
        id: "admin",
        label: "Admin",
        icon_svg: "",
        url: "/admin",
        visibility: "admin",
        sort_order: 1,
      },
    ] as CustomMenuItem[],
  };

  const wrapper = mount(GeneralSettingsTab, {
    props: {
      form,
      tablePageSizeOptionsInput: "10, 20, 50, 100",
    },
    global: {
      stubs: {
        Toggle: ToggleStub,
        ImageUpload: ImageUploadStub,
      },
    },
  });

  return { wrapper, form };
};

describe("GeneralSettingsTab", () => {
  it("renders site settings and emits collection operations", async () => {
    const { wrapper, form } = mountTab();

    expect(wrapper.text()).toContain("admin.settings.site.title");
    expect(wrapper.text()).toContain("admin.settings.customMenu.title");

    await wrapper.get('[data-test="table-page-size-options"]').setValue("25, 50");
    expect(wrapper.emitted("update:tablePageSizeOptionsInput")?.[0]).toEqual(["25, 50"]);

    await wrapper.get('[data-test="add-endpoint"]').trigger("click");
    expect(wrapper.emitted("add-endpoint")).toHaveLength(1);

    await wrapper.get('[data-test="remove-endpoint-0"]').trigger("click");
    expect(wrapper.emitted("remove-endpoint")?.[0]).toEqual([0]);

    await wrapper.get('[data-test="add-menu-item"]').trigger("click");
    expect(wrapper.emitted("add-menu-item")).toHaveLength(1);

    await wrapper.get('[data-test="move-menu-item-down-0"]').trigger("click");
    expect(wrapper.emitted("move-menu-item")?.[0]).toEqual([0, 1]);

    await wrapper.get('[data-test="remove-menu-item-0"]').trigger("click");
    expect(wrapper.emitted("remove-menu-item")?.[0]).toEqual([0]);

    await wrapper.get('input[placeholder="admin.settings.site.siteNamePlaceholder"]').setValue("New Name");
    expect(form.site_name).toBe("New Name");
  });
});
