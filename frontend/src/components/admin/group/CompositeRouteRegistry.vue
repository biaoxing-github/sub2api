<template>
  <BaseDialog
    :show="show"
    :title="copy.title(group?.name)"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="grid gap-5 lg:grid-cols-[minmax(0,1.15fr)_minmax(340px,0.85fr)]">
      <section class="min-w-0">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ copy.routes }}</h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ copy.routesHint }}</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="loading"
            :title="copy.refresh"
            data-testid="composite-routes-refresh"
            @click="loadRoutes"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
          <div
            v-if="loading"
            class="flex h-36 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
          >
            {{ copy.loading }}
          </div>
          <div
            v-else-if="routes.length === 0"
            class="flex h-36 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            data-testid="composite-routes-empty"
          >
            {{ copy.empty }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
              <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-3 py-2">{{ copy.publicModel }}</th>
                  <th class="px-3 py-2">{{ copy.target }}</th>
                  <th class="px-3 py-2">{{ copy.scope }}</th>
                  <th class="px-3 py-2 text-right">{{ copy.actions }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr
                  v-for="route in routes"
                  :key="route.id"
                  :class="!route.enabled && 'opacity-60'"
                  :data-testid="`composite-route-${route.id}`"
                >
                  <td class="max-w-[15rem] px-3 py-2">
                    <div class="break-all font-medium text-gray-900 dark:text-white">
                      {{ route.public_model }}
                    </div>
                    <div class="mt-1 flex flex-wrap gap-1.5">
                      <span class="badge badge-gray">{{ copy.match[route.match_type] }}</span>
                      <span v-if="!route.enabled" class="badge badge-danger">{{ copy.disabled }}</span>
                    </div>
                  </td>
                  <td class="px-3 py-2">
                    <div class="flex items-center gap-1.5 text-gray-900 dark:text-white">
                      <PlatformIcon :platform="route.target_platform" size="xs" />
                      <span>{{ platformLabel(route.target_platform) }}</span>
                    </div>
                    <div class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">
                      {{ route.upstream_model || route.public_model }}
                    </div>
                  </td>
                  <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                    <div>{{ endpointLabel(route.endpoint) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ copy.priority }}: {{ route.priority }}
                    </div>
                  </td>
                  <td class="px-3 py-2">
                    <div class="flex justify-end gap-1">
                      <button
                        type="button"
                        class="rounded p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                        :title="copy.edit"
                        :data-testid="`edit-composite-route-${route.id}`"
                        @click="editRoute(route)"
                      >
                        <Icon name="edit" size="sm" />
                      </button>
                      <button
                        type="button"
                        class="rounded p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                        :title="copy.delete"
                        :data-testid="`delete-composite-route-${route.id}`"
                        @click="removeRoute(route)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="space-y-5">
        <form class="space-y-3" data-testid="composite-route-form" @submit.prevent="saveRoute">
          <div class="flex items-center justify-between gap-3">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ editingId ? copy.editRoute : copy.addRoute }}
            </h4>
            <button
              v-if="editingId"
              type="button"
              class="text-xs font-medium text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-white"
              @click="resetForm"
            >
              {{ copy.cancel }}
            </button>
          </div>

          <Input
            id="composite-route-public-model"
            v-model="form.public_model"
            :label="copy.publicModel"
            placeholder="openrouter/gpt-5"
            required
          />

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="block">
              <span class="input-label mb-1.5 block">{{ copy.matchType }}</span>
              <select v-model="form.match_type" class="input w-full" data-testid="route-match-type">
                <option value="exact">{{ copy.match.exact }}</option>
                <option value="prefix">{{ copy.match.prefix }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label mb-1.5 block">{{ copy.endpoint }}</span>
              <select v-model="form.endpoint" class="input w-full" data-testid="route-endpoint">
                <option v-for="endpoint in endpoints" :key="endpoint" :value="endpoint">
                  {{ endpointLabel(endpoint) }}
                </option>
              </select>
            </label>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="block">
              <span class="input-label mb-1.5 block">{{ copy.targetPlatform }}</span>
              <select
                v-model="form.target_platform"
                class="input w-full"
                data-testid="route-target-platform"
              >
                <option v-for="platform in targetPlatforms" :key="platform" :value="platform">
                  {{ platformLabel(platform) }}
                </option>
              </select>
            </label>
            <Input
              id="composite-route-priority"
              v-model="priorityText"
              type="number"
              :label="copy.priority"
            />
          </div>

          <Input
            id="composite-route-upstream-model"
            v-model="form.upstream_model"
            :label="copy.upstreamModel"
            :hint="copy.upstreamHint"
            placeholder="gpt-5"
          />
          <TextArea
            id="composite-route-notes"
            v-model="form.notes"
            :label="copy.notes"
            :rows="2"
          />

          <div class="flex items-center justify-between gap-3">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="form.enabled" />
              {{ copy.enabled }}
            </label>
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="saving"
              data-testid="save-composite-route"
            >
              <Icon name="check" size="sm" class="mr-2" />
              {{ saving ? copy.saving : editingId ? copy.update : copy.create }}
            </button>
          </div>
        </form>

        <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
          <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ copy.preview }}</h4>
          <div class="space-y-3">
            <Input
              id="composite-route-preview-model"
              v-model="previewModel"
              :placeholder="copy.previewPlaceholder"
              @enter="previewRoute"
            />
            <div class="flex gap-2">
              <select
                v-model="previewEndpoint"
                class="input min-w-0 flex-1"
                data-testid="preview-route-endpoint"
              >
                <option v-for="endpoint in endpoints" :key="endpoint" :value="endpoint">
                  {{ endpointLabel(endpoint) }}
                </option>
              </select>
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="previewing"
                data-testid="preview-composite-route"
                @click="previewRoute"
              >
                {{ previewing ? copy.previewing : copy.preview }}
              </button>
            </div>
            <div
              v-if="previewDecision"
              class="rounded-lg border p-3 text-sm"
              :class="
                previewDecision.matched
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-200'
                  : 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200'
              "
              data-testid="composite-route-preview-result"
            >
              <div class="font-medium">
                {{ previewDecision.matched ? copy.matched : copy.notMatched }}
              </div>
              <template v-if="previewDecision.matched">
                <div class="mt-2 flex items-center gap-1.5">
                  <PlatformIcon :platform="previewTargetPlatform || undefined" size="xs" />
                  <span>{{ platformLabel(previewDecision.target_platform) }}</span>
                  <span class="text-current/60">→</span>
                  <span class="break-all">{{ previewDecision.upstream_model }}</span>
                </div>
                <div class="mt-1 text-xs opacity-75">
                  {{ copy.source }}: {{ sourceLabel(previewDecision.source) }}
                </div>
              </template>
              <div v-else-if="previewDecision.reason" class="mt-1 text-xs opacity-75">
                {{ previewDecision.reason }}
              </div>
            </div>
          </div>
        </div>

        <p v-if="errorMessage" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300" role="status">
          {{ successMessage }}
        </p>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import groupsAPI from '@/api/admin/groups'
import type {
  AdminGroup,
  CompositeModelRoute,
  CompositeModelRouteInput,
  CompositeRouteDecision,
  CompositeRouteEndpoint,
  GroupPlatform
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import TextArea from '@/components/common/TextArea.vue'
import Toggle from '@/components/common/Toggle.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { locale } = useI18n()
const isZh = computed(() => locale.value.toLowerCase().startsWith('zh'))
const copy = computed(() => {
  if (isZh.value) {
    return {
      title: (name?: string) => (name ? `Composite 路由：${name}` : 'Composite 路由'),
      routes: '已保存路由', routesHint: '优先级越小越先匹配；显式路由优先于内置模型识别。', empty: '暂无 Composite 路由',
      loading: '加载中...', refresh: '刷新', publicModel: '公开模型', target: '目标', scope: '范围', actions: '操作',
      priority: '优先级', disabled: '已停用', edit: '编辑', delete: '删除', addRoute: '添加路由', editRoute: '编辑路由',
      cancel: '取消编辑', matchType: '匹配方式', endpoint: '端点', targetPlatform: '目标平台', upstreamModel: '上游模型',
      upstreamHint: '留空时沿用公开模型名。', notes: '备注', enabled: '启用', saving: '保存中...', update: '更新', create: '创建',
      preview: '预览', previewing: '预览中...', previewPlaceholder: '输入请求模型', matched: '已匹配', notMatched: '未匹配', source: '来源',
      publicModelRequired: '请输入公开模型', loadFailed: '加载 Composite 路由失败', saveFailed: '保存 Composite 路由失败',
      deleteFailed: '删除 Composite 路由失败', previewFailed: '预览 Composite 路由失败', deleteConfirm: '确定删除此 Composite 路由？',
      created: 'Composite 路由已创建', updated: 'Composite 路由已更新', deleted: 'Composite 路由已删除',
      match: { exact: '精确', prefix: '前缀' },
      sources: { route: '显式路由', detector: '内置识别' }
    }
  }
  return {
    title: (name?: string) => (name ? `Composite Routes: ${name}` : 'Composite Routes'),
    routes: 'Saved routes', routesHint: 'Lower priorities match first. Explicit routes take precedence over built-in detection.', empty: 'No composite routes configured',
    loading: 'Loading...', refresh: 'Refresh', publicModel: 'Public model', target: 'Target', scope: 'Scope', actions: 'Actions',
    priority: 'Priority', disabled: 'Disabled', edit: 'Edit', delete: 'Delete', addRoute: 'Add route', editRoute: 'Edit route',
    cancel: 'Cancel edit', matchType: 'Match', endpoint: 'Endpoint', targetPlatform: 'Target platform', upstreamModel: 'Upstream model',
    upstreamHint: 'Leave blank to keep the public model name.', notes: 'Notes', enabled: 'Enabled', saving: 'Saving...', update: 'Update', create: 'Create',
    preview: 'Preview', previewing: 'Previewing...', previewPlaceholder: 'Request model', matched: 'Matched', notMatched: 'No match', source: 'Source',
    publicModelRequired: 'Public model is required', loadFailed: 'Failed to load composite routes', saveFailed: 'Failed to save composite route',
    deleteFailed: 'Failed to delete composite route', previewFailed: 'Failed to preview composite route', deleteConfirm: 'Delete this composite route?',
    created: 'Composite route created', updated: 'Composite route updated', deleted: 'Composite route deleted',
    match: { exact: 'Exact', prefix: 'Prefix' },
    sources: { route: 'Explicit route', detector: 'Built-in detector' }
  }
})

type ConcretePlatform = Exclude<GroupPlatform, 'composite'>

const endpoints: CompositeRouteEndpoint[] = [
  'any', 'messages', 'count_tokens', 'responses', 'chat_completions', 'embeddings', 'images', 'gemini'
]
const targetPlatforms: ConcretePlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok']
const endpointLabels: Record<CompositeRouteEndpoint, string> = {
  any: 'Any', messages: 'Messages', count_tokens: 'Count Tokens', responses: 'Responses',
  chat_completions: 'Chat Completions', embeddings: 'Embeddings', images: 'Images', gemini: 'Gemini Native'
}

const routes = ref<CompositeModelRoute[]>([])
const loading = ref(false)
const saving = ref(false)
const previewing = ref(false)
const editingId = ref<number | null>(null)
const previewModel = ref('')
const previewEndpoint = ref<CompositeRouteEndpoint>('any')
const previewDecision = ref<CompositeRouteDecision | null>(null)
const errorMessage = ref('')
const successMessage = ref('')

const form = reactive<Required<CompositeModelRouteInput>>({
  public_model: '', match_type: 'exact', target_platform: 'openai', upstream_model: '',
  endpoint: 'any', priority: 0, enabled: true, notes: ''
})

const priorityText = computed({
  get: () => String(form.priority),
  set: (value: string) => { form.priority = Number.isFinite(Number(value)) ? Math.trunc(Number(value)) : 0 }
})

const previewTargetPlatform = computed<GroupPlatform | ''>(() => {
  return previewDecision.value?.target_platform || ''
})

function endpointLabel(endpoint: CompositeRouteEndpoint): string {
  return endpointLabels[endpoint] || endpoint
}

function sourceLabel(source?: string): string {
  if (!source) return ''
  if (source === 'route') return copy.value.sources.route
  if (source === 'detector') return copy.value.sources.detector
  return source
}

function clearMessages(): void {
  errorMessage.value = ''
  successMessage.value = ''
}

function resetForm(): void {
  editingId.value = null
  Object.assign(form, {
    public_model: '', match_type: 'exact', target_platform: 'openai', upstream_model: '',
    endpoint: 'any', priority: 0, enabled: true, notes: ''
  })
}

async function loadRoutes(): Promise<void> {
  if (!props.group) return
  loading.value = true
  clearMessages()
  try {
    routes.value = await groupsAPI.listCompositeRoutes(props.group.id)
  } catch {
    errorMessage.value = copy.value.loadFailed
  } finally {
    loading.value = false
  }
}

function editRoute(route: CompositeModelRoute): void {
  clearMessages()
  editingId.value = route.id
  Object.assign(form, {
    public_model: route.public_model,
    match_type: route.match_type,
    target_platform: route.target_platform,
    upstream_model: route.upstream_model,
    endpoint: route.endpoint,
    priority: route.priority,
    enabled: route.enabled,
    notes: route.notes
  })
}

async function saveRoute(): Promise<void> {
  if (!props.group || saving.value) return
  clearMessages()
  if (!form.public_model.trim()) {
    errorMessage.value = copy.value.publicModelRequired
    return
  }
  saving.value = true
  const payload: CompositeModelRouteInput = {
    ...form,
    public_model: form.public_model.trim(),
    upstream_model: form.upstream_model.trim(),
    notes: form.notes.trim()
  }
  try {
    const message = editingId.value ? copy.value.updated : copy.value.created
    if (editingId.value) {
      await groupsAPI.updateCompositeRoute(props.group.id, editingId.value, payload)
    } else {
      await groupsAPI.createCompositeRoute(props.group.id, payload)
    }
    resetForm()
    await loadRoutes()
    successMessage.value = message
  } catch {
    errorMessage.value = copy.value.saveFailed
  } finally {
    saving.value = false
  }
}

async function removeRoute(route: CompositeModelRoute): Promise<void> {
  if (!props.group || !globalThis.confirm(copy.value.deleteConfirm)) return
  clearMessages()
  try {
    await groupsAPI.deleteCompositeRoute(props.group.id, route.id)
    if (editingId.value === route.id) resetForm()
    await loadRoutes()
    successMessage.value = copy.value.deleted
  } catch {
    errorMessage.value = copy.value.deleteFailed
  }
}

async function previewRoute(): Promise<void> {
  if (!props.group || previewing.value || !previewModel.value.trim()) return
  clearMessages()
  previewing.value = true
  previewDecision.value = null
  try {
    previewDecision.value = await groupsAPI.previewCompositeRoute(props.group.id, {
      model: previewModel.value.trim(),
      endpoint: previewEndpoint.value
    })
  } catch {
    errorMessage.value = copy.value.previewFailed
  } finally {
    previewing.value = false
  }
}

watch(
  () => [props.show, props.group?.id] as const,
  ([show]) => {
    if (!show) return
    resetForm()
    previewModel.value = ''
    previewEndpoint.value = 'any'
    previewDecision.value = null
    void loadRoutes()
  },
  { immediate: true }
)
</script>
