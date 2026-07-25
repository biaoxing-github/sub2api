<template>
  <AppLayout>
    <section class="admin-tools">
      <div class="admin-tools__tabs" role="tablist" :aria-label="t('admin.tools.tabsLabel')">
        <button
          v-for="tab in toolTabs"
          :id="`admin-tools-tab-${tab.key}`"
          :key="tab.key"
          type="button"
          role="tab"
          class="admin-tools__tab"
          :class="{ 'admin-tools__tab--active': tab.key === activeTab.key }"
          :aria-selected="tab.key === activeTab.key"
          :aria-controls="'admin-tools-panel'"
          @click="selectTab(tab.key)"
        >
          {{ t(tab.labelKey) }}
        </button>
      </div>

      <div
        id="admin-tools-panel"
        class="admin-tools__frame"
        role="tabpanel"
        :aria-labelledby="`admin-tools-tab-${activeTab.key}`"
        :aria-label="t(activeTab.titleKey)"
      >
        <component :is="activeTab.component" :key="activeTab.key" class="admin-tools__embedded" />
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import NewApiCheckinTool from './tools/NewApiCheckinTool.vue'
import NewApiRedeemTool from './tools/NewApiRedeemTool.vue'
import TokenCostTool from './tools/TokenCostTool.vue'

type AdminToolTabKey = 'newapi' | 'newapi-redeem' | 'token-cost'

interface AdminToolTab {
  key: AdminToolTabKey
  labelKey: string
  titleKey: string
  component: Component
}

const toolTabs: AdminToolTab[] = [
  {
    key: 'newapi',
    labelKey: 'admin.tools.tabs.newapi',
    titleKey: 'admin.tools.frames.newapi',
    component: NewApiCheckinTool
  },
  {
    key: 'newapi-redeem',
    labelKey: 'admin.tools.tabs.newapiRedeem',
    titleKey: 'admin.tools.frames.newapiRedeem',
    component: NewApiRedeemTool
  },
  {
    key: 'token-cost',
    labelKey: 'admin.tools.tabs.tokenCost',
    titleKey: 'admin.tools.frames.tokenCost',
    component: TokenCostTool
  }
]

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

function normalizeTabKey(value: unknown): AdminToolTabKey {
  if (value === 'newapi-redeem') return 'newapi-redeem'
  return value === 'token-cost' ? 'token-cost' : 'newapi'
}

const activeTab = computed(() => {
  const key = normalizeTabKey(route.query.tab)
  return toolTabs.find(tab => tab.key === key) ?? toolTabs[0]
})

function selectTab(key: AdminToolTabKey) {
  if (key === activeTab.value.key) return
  router.replace({
    query: {
      ...route.query,
      tab: key
    }
  })
}
</script>

<style scoped>
.admin-tools {
  display: flex;
  min-width: 0;
  max-width: 100%;
  min-height: calc(100vh - 96px);
  contain: paint;
  flex-direction: column;
  gap: 0.75rem;
  overflow-x: hidden;
}

.admin-tools__tabs {
  display: inline-flex;
  width: fit-content;
  max-width: 100%;
  gap: 0.25rem;
  overflow-x: auto;
  border-bottom: 1px solid rgb(226 232 240);
}

.dark .admin-tools__tabs {
  border-bottom-color: rgb(55 65 81);
}

.admin-tools__tab {
  min-height: 2.5rem;
  flex: 0 0 auto;
  border-bottom: 2px solid transparent;
  padding: 0.5rem 0.875rem;
  color: rgb(71 85 105);
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1.25rem;
  transition:
    border-color 0.15s ease,
    color 0.15s ease,
    background-color 0.15s ease;
}

.admin-tools__tab:hover {
  background: rgb(248 250 252);
  color: rgb(15 23 42);
}

.admin-tools__tab--active {
  border-bottom-color: rgb(59 130 246);
  color: rgb(37 99 235);
}

.dark .admin-tools__tab {
  color: rgb(203 213 225);
}

.dark .admin-tools__tab:hover {
  background: rgb(31 41 55);
  color: rgb(248 250 252);
}

.dark .admin-tools__tab--active {
  border-bottom-color: rgb(96 165 250);
  color: rgb(147 197 253);
}

.admin-tools__frame {
  min-height: 0;
  flex: 1 1 auto;
  overflow: auto;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: #f8fafc;
}

.dark .admin-tools__frame {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
}

.admin-tools__embedded {
  display: block;
  width: 100%;
  min-height: calc(100vh - 148px);
  background: #f8fafc;
}
</style>
