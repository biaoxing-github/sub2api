import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";

import AccountAutoRefreshControl from "../AccountAutoRefreshControl.vue";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      key === "admin.accounts.autoRefreshCountdown"
        ? `${key}:${String(params?.seconds ?? "")}`
        : key,
  }),
}));

describe("AccountAutoRefreshControl", () => {
  it("opens the menu and emits enable/interval choices", async () => {
    const wrapper = mount(AccountAutoRefreshControl, {
      props: {
        open: false,
        enabled: false,
        countdown: 0,
        intervals: [5, 10, 15, 30],
        intervalSeconds: 30,
        intervalLabel: (seconds: number) => `${seconds}s`,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    });

    await wrapper.get("button").trigger("click");
    expect(wrapper.emitted("update:open")?.[0]).toEqual([true]);

    await wrapper.setProps({ open: true });
    expect(wrapper.text()).toContain("admin.accounts.enableAutoRefresh");
    expect(wrapper.text()).toContain("10s");

    await wrapper
      .findAll("button")
      .find((button) => button.text().includes("admin.accounts.enableAutoRefresh"))
      ?.trigger("click");
    expect(wrapper.emitted("set-enabled")?.[0]).toEqual([true]);

    await wrapper
      .findAll("button")
      .find((button) => button.text().includes("10s"))
      ?.trigger("click");
    expect(wrapper.emitted("set-interval")?.[0]).toEqual([10]);
  });
});
