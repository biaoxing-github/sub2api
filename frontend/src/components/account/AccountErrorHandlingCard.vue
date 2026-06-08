<template>
  <section class="error-policy-shell">
    <div class="flex items-center justify-between gap-3 border-b border-sky-100 px-4 py-3 dark:border-sky-900/30">
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-50">{{ title }}</h3>
          <span class="rounded-full bg-sky-100 px-2 py-0.5 text-xs font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
            {{ enabledRuleCount }}/{{ rules.length }} 启用
          </span>
        </div>
        <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ description }}</p>
      </div>
      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:text-slate-300 dark:hover:bg-dark-700" @click="guideOpen = true">
          <Icon name="questionCircle" size="sm" />
          配置指南
        </button>
        <div class="relative">
          <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:text-slate-300 dark:hover:bg-dark-700" @click="presetMenuOpen = !presetMenuOpen">
            <Icon name="plus" size="sm" />
            添加预设
          </button>
          <div v-if="presetMenuOpen" class="absolute right-0 z-10 mt-2 w-56 overflow-hidden rounded-md border border-slate-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800">
            <button v-for="preset in presets" :key="preset.key" type="button" class="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50 dark:text-slate-200 dark:hover:bg-dark-700" @click="handlePresetClick(preset.key)">
              <span>{{ preset.label }}</span>
              <span class="text-xs text-slate-400">{{ actionLabel(preset.rule.action) }}</span>
            </button>
          </div>
        </div>
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:text-slate-300 dark:hover:bg-dark-700" @click="addBlankRule">
          <Icon name="plus" size="sm" />
          自定义规则
        </button>
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:text-slate-300 dark:hover:bg-dark-700" @click="normalizePriorities">
          <Icon name="arrowsUpDown" size="sm" />
          重排优先级
        </button>
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:text-slate-300 dark:hover:bg-dark-700" :disabled="rules.length === 0" @click="clearRules">
          <Icon name="trash" size="sm" />
          清空
        </button>
      </div>
    </div>

    <div class="px-4 py-4">
      <div v-if="rules.length === 0" class="rounded-md border border-dashed border-slate-200 px-4 py-6 text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">
        {{ contextGuide.emptyDescription }}
        <div class="mt-3">
          <button type="button" class="inline-flex items-center gap-1 rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-sky-500" @click="addBlankRule">
            <Icon name="plus" size="sm" />
            添加专属规则
          </button>
        </div>
      </div>

      <div v-else class="space-y-3">
        <article v-for="(rule, index) in rules" :key="ruleKey(index)" class="overflow-hidden rounded-md border border-slate-200 bg-white dark:border-dark-600 dark:bg-dark-700/40">
          <button type="button" class="flex w-full items-start justify-between gap-3 px-3 py-3 text-left" @click="toggleRule(index)">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <label class="inline-flex items-center gap-1 text-xs font-medium">
                  <input v-model="rule.enabled" type="checkbox" class="rounded border-slate-300 text-sky-600 focus:ring-sky-500" @click.stop />
                  启用
                </label>
                <span class="rounded-full bg-sky-100 px-2 py-0.5 text-xs font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">P{{ rule.priority ?? '-' }}</span>
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="actionTagClass(rule.action)">{{ actionLabel(rule.action) }}</span>
              </div>
              <p class="mt-1 truncate text-sm font-medium text-slate-900 dark:text-slate-50">{{ rule.name || '未命名规则' }}</p>
              <p class="mt-1 truncate text-xs text-slate-500 dark:text-slate-400">{{ ruleConditionSummary(rule) }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-1" @click.stop>
              <button type="button" class="rounded-md p-1.5 text-slate-500 hover:bg-slate-50 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-dark-700 dark:hover:text-slate-200" :disabled="index === 0" @click="moveRule(index, -1)">
                <Icon name="arrowUp" size="sm" />
              </button>
              <button type="button" class="rounded-md p-1.5 text-slate-500 hover:bg-slate-50 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-dark-700 dark:hover:text-slate-200" :disabled="index === rules.length - 1" @click="moveRule(index, 1)">
                <Icon name="arrowDown" size="sm" />
              </button>
              <button type="button" class="rounded-md p-1.5 text-red-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20" @click="removeRule(index)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </button>

          <div v-if="expandedRuleKeys.has(ruleKey(index))" class="border-t border-slate-100 px-3 py-3 dark:border-dark-600">
            <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
              <div>
                <label class="input-label">{{ fieldLabels.name }}</label>
                <input v-model="rule.name" type="text" class="input" :placeholder="fieldHints.name" />
              </div>
              <div>
                <label class="input-label">{{ fieldLabels.priority }}</label>
                <input v-model.number="rule.priority" type="number" min="1" max="9999" class="input" />
              </div>
              <div>
                <label class="input-label">{{ fieldLabels.action }}</label>
                <Select v-model="rule.action" :options="actionOptions" />
              </div>
            </div>

            <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
              <div>
                <label class="input-label">{{ fieldLabels.statusCodes }}</label>
                <input v-model="rule.status_codes" type="text" class="input" :placeholder="fieldHints.statusCodes" />
              </div>
              <div>
                <label class="input-label">{{ fieldLabels.errorCodes }}</label>
                <input v-model="rule.error_codes" type="text" class="input" :placeholder="fieldHints.errorCodes" />
              </div>
              <div>
                <label class="input-label">{{ fieldLabels.errorTypes }}</label>
                <input v-model="rule.error_types" type="text" class="input" :placeholder="fieldHints.errorTypes" />
              </div>
              <div>
                <label class="input-label">{{ fieldLabels.keywords }}</label>
                <input v-model="rule.keywords" type="text" class="input" :placeholder="fieldHints.keywords" />
              </div>
            </div>

            <div v-if="rule.action === 'temp_unschedulable'" class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
              <div>
                <label class="input-label">{{ fieldLabels.durationMinutes }}</label>
                <input v-model.number="rule.durationMinutes" type="number" min="1" max="1440" class="input" />
              </div>
            </div>

            <div v-else-if="rule.action === 'rate_limited'" class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
              <div>
                <label class="input-label">{{ fieldLabels.resetStrategy }}</label>
                <Select v-model="rule.reset_strategy" :options="resetStrategyOptions" />
              </div>
              <div v-if="rule.reset_strategy === 'duration'">
                <label class="input-label">{{ fieldLabels.durationHours }}</label>
                <input v-model.number="rule.duration_hours" type="number" min="1" max="720" class="input" />
              </div>
              <div v-if="rule.reset_strategy === 'daily'">
                <label class="input-label">{{ fieldLabels.dailyResetHour }}</label>
                <input v-model.number="rule.daily_reset_hour" type="number" min="0" max="23" class="input" />
              </div>
              <div v-if="rule.reset_strategy === 'weekly'">
                <label class="input-label">{{ fieldLabels.weeklyResetDay }}</label>
                <input v-model.number="rule.weekly_reset_day" type="number" min="0" max="6" class="input" />
              </div>
              <div v-if="rule.reset_strategy === 'weekly'">
                <label class="input-label">{{ fieldLabels.weeklyResetHour }}</label>
                <input v-model.number="rule.weekly_reset_hour" type="number" min="0" max="23" class="input" />
              </div>
            </div>

            <div class="mt-3">
              <label class="input-label">{{ fieldLabels.description }}</label>
              <textarea v-model="rule.description" rows="2" class="input" :placeholder="fieldHints.description" />
            </div>
          </div>
        </article>
      </div>
    </div>

    <div class="border-t border-slate-100 px-4 py-3 text-xs text-slate-500 dark:border-dark-600 dark:text-slate-400">
      <div class="flex flex-wrap gap-2">
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2.5 py-1 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:hover:bg-dark-700" @click="expandAllRules">
          <Icon name="chevronDown" size="sm" />
          展开全部
        </button>
        <button type="button" class="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2.5 py-1 transition-colors hover:bg-slate-50 dark:border-dark-600 dark:hover:bg-dark-700" @click="collapseAllRules">
          <Icon name="chevronUp" size="sm" />
          收起全部
        </button>
      </div>
    </div>

    <BaseDialog :show="guideOpen" title="错误处理策略配置指南" width="wide" @close="guideOpen = false">
      <div class="space-y-5">
        <section class="space-y-2">
          <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-50">去哪里查错误</h4>
          <div class="overflow-hidden rounded-md border border-slate-200 dark:border-dark-600">
            <table class="w-full text-sm">
              <thead class="bg-slate-50 text-left text-xs uppercase text-slate-500 dark:bg-dark-700 dark:text-slate-400">
                <tr>
                  <th class="px-3 py-2 font-medium">来源</th>
                  <th class="px-3 py-2 font-medium">查看位置</th>
                  <th class="px-3 py-2 font-medium">说明</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in guideSources" :key="item.key" class="border-t border-slate-100 dark:border-dark-600">
                  <td class="px-3 py-2 font-medium text-slate-900 dark:text-slate-50">{{ item.name }}</td>
                  <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{{ item.where }}</td>
                  <td class="px-3 py-2 text-slate-500 dark:text-slate-400">{{ item.note }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="space-y-2">
          <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-50">字段怎么填</h4>
          <div class="overflow-hidden rounded-md border border-slate-200 dark:border-dark-600">
            <table class="w-full text-sm">
              <thead class="bg-slate-50 text-left text-xs uppercase text-slate-500 dark:bg-dark-700 dark:text-slate-400">
                <tr>
                  <th class="px-3 py-2 font-medium">字段</th>
                  <th class="px-3 py-2 font-medium">取值来源</th>
                  <th class="px-3 py-2 font-medium">例子</th>
                  <th class="px-3 py-2 font-medium">说明</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in guideFields" :key="item.key" class="border-t border-slate-100 dark:border-dark-600">
                  <td class="px-3 py-2 font-medium text-slate-900 dark:text-slate-50">{{ item.field }}</td>
                  <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{{ item.source }}</td>
                  <td class="px-3 py-2 text-slate-500 dark:text-slate-400">{{ item.example }}</td>
                  <td class="px-3 py-2 text-slate-500 dark:text-slate-400">{{ item.note }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="text-xs text-slate-500 dark:text-slate-400">多个值用逗号、分号或换行分隔；同一个字段里的多个值是“任一命中”，不同字段之间是“同时命中”。</p>
        </section>
      </div>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import {
  accountErrorHandlingActionColor,
  accountErrorHandlingActionLabel,
  accountErrorHandlingActionSelectOptions,
  accountErrorHandlingRuleConditionSummary,
  accountErrorHandlingRuleKey
} from './accountErrorHandlingDisplay'
import {
  accountErrorHandlingGuideFields,
  accountErrorHandlingGuideSources,
  resolveAccountErrorHandlingContextGuide
} from './accountErrorHandlingGuide'
import {
  accountErrorHandlingPresets,
  cloneAccountErrorHandlingRule,
  createBlankAccountErrorHandlingRule,
  getNextAccountErrorHandlingPriority,
  normalizeAccountErrorHandlingPriorities
} from './accountErrorHandlingRules'
import type {
  AccountErrorHandlingRuleForm
} from './accountErrorHandlingTypes'

const rules = defineModel<AccountErrorHandlingRuleForm[]>('rules', { required: true })

const props = withDefaults(defineProps<{
  accountType?: string
  baseUrl?: string
  providerCode?: string
  title?: string
  description?: string
}>(), {
  accountType: '',
  baseUrl: '',
  providerCode: '',
  title: '错误处理策略',
  description: '用于把上游状态码、错误码、错误类型和关键词映射为账号级处理动作'
})

const actionOptions = accountErrorHandlingActionSelectOptions.map((item) => ({
  value: item.value,
  label: item.label
}))

const presetMenuOpen = ref(false)
const guideOpen = ref(false)
const expandedRuleKeys = ref<Set<string>>(new Set())
const contextGuide = computed(() => resolveAccountErrorHandlingContextGuide({
  accountType: props.accountType,
  baseUrl: props.baseUrl,
  providerCode: props.providerCode
}))
const enabledRuleCount = computed(() => rules.value.filter((rule) => rule.enabled !== false).length)
const presets = accountErrorHandlingPresets
const guideFields = accountErrorHandlingGuideFields
const guideSources = accountErrorHandlingGuideSources
const fieldLabels = {
  name: '规则名称',
  priority: '优先级',
  action: '处理动作',
  statusCodes: '状态码',
  errorCodes: '错误码',
  errorTypes: '错误类型',
  keywords: '关键词',
  durationMinutes: '临时避让分钟数',
  resetStrategy: '恢复策略',
  durationHours: '恢复小时数',
  dailyResetHour: '每天恢复时间',
  weeklyResetDay: '每周恢复日',
  weeklyResetHour: '每周恢复时间',
  description: '说明'
}
const fieldHints = {
  name: '例如 429 临时限流',
  statusCodes: '429, 502, 503',
  errorCodes: 'insufficient_quota',
  errorTypes: 'rate_limit_exceeded',
  keywords: 'quota exceeded, 额度不足',
  description: '可写为什么要这样处理'
}
const resetStrategyOptions = [
  { label: '固定时长', value: 'duration' },
  { label: '每天固定时间', value: 'daily' },
  { label: '每周固定时间', value: 'weekly' }
]

watch(() => presetMenuOpen.value, (value) => {
  if (!value) return
  const onDocumentClick = (event: MouseEvent) => {
    const target = event.target as HTMLElement | null
    if (target?.closest('.error-policy-shell')) {
      return
    }
    presetMenuOpen.value = false
    document.removeEventListener('click', onDocumentClick)
  }
  document.addEventListener('click', onDocumentClick)
})

function toggleRule(index: number) {
  const key = accountErrorHandlingRuleKey(index)
  const next = new Set(expandedRuleKeys.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedRuleKeys.value = next
}

function addBlankRule() {
  rules.value.push(createBlankAccountErrorHandlingRule(getNextAccountErrorHandlingPriority(rules.value)))
  expandedRuleKeys.value = new Set([accountErrorHandlingRuleKey(rules.value.length - 1)])
  presetMenuOpen.value = false
}

function handlePresetClick(key: string) {
  const preset = presets.find((item) => item.key === key)
  if (!preset) return
  rules.value.push(cloneAccountErrorHandlingRule({
    ...preset.rule,
    priority: getNextAccountErrorHandlingPriority(rules.value)
  }))
  expandedRuleKeys.value = new Set([accountErrorHandlingRuleKey(rules.value.length - 1)])
  presetMenuOpen.value = false
}

function clearRules() {
  rules.value = []
  expandedRuleKeys.value = new Set()
  presetMenuOpen.value = false
}

function removeRule(index: number) {
  rules.value.splice(index, 1)
  expandedRuleKeys.value = new Set()
}

function moveRule(index: number, offset: number) {
  const nextIndex = index + offset
  if (nextIndex < 0 || nextIndex >= rules.value.length) return
  const [rule] = rules.value.splice(index, 1)
  rules.value.splice(nextIndex, 0, rule)
  expandedRuleKeys.value = new Set([accountErrorHandlingRuleKey(nextIndex)])
}

function normalizePriorities() {
  rules.value = normalizeAccountErrorHandlingPriorities(rules.value)
}

function expandAllRules() {
  expandedRuleKeys.value = new Set(rules.value.map((_, index) => accountErrorHandlingRuleKey(index)))
}

function collapseAllRules() {
  expandedRuleKeys.value = new Set()
}

const actionLabel = (action: AccountErrorHandlingRuleForm['action']) => accountErrorHandlingActionLabel(action)
const actionTagClass = (action: AccountErrorHandlingRuleForm['action']) => {
  const color = accountErrorHandlingActionColor(action)
  return color === 'blue'
    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    : color === 'orange'
      ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
      : color === 'red'
        ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
        : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}
const ruleKey = accountErrorHandlingRuleKey
const ruleConditionSummary = accountErrorHandlingRuleConditionSummary
</script>

<style scoped>
.error-policy-shell {
  border: 1px solid rgb(186 230 253 / 0.9);
  border-radius: 12px;
  background: linear-gradient(180deg, rgb(248 250 252) 0%, rgb(255 255 255) 100%);
}
</style>
