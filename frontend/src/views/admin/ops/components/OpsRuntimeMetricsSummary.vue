<template>
  <section
    v-if="snapshot"
    data-testid="ops-runtime-metrics-summary"
    class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
  >
    <header class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
        {{ t('admin.ops.runtimeMetrics.title') }}
      </h3>
      <time class="text-xs text-gray-500 dark:text-gray-400" :datetime="snapshot.generated_at">
        {{ t('admin.ops.runtimeMetrics.generatedAt') }} {{ formatDateTime(snapshot.generated_at) }}
      </time>
    </header>

    <div class="grid divide-y divide-gray-200 dark:divide-dark-700 lg:grid-cols-2 lg:divide-x lg:divide-y-0">
      <div class="min-w-0 p-4">
        <h4 class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.runtimeMetrics.requestStages') }}
        </h4>
        <div v-if="stageRows.length" class="mt-3 space-y-2">
          <div
            v-for="row in stageRows"
            :key="row.stage"
            data-testid="runtime-stage-row"
            class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 text-xs"
          >
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-800 dark:text-gray-100">{{ stageLabel(row.stage) }}</div>
              <div class="mt-0.5 text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtimeMetrics.average') }} {{ formatDuration(row.totalDurationMs / row.count) }}
                · {{ t('admin.ops.runtimeMetrics.maximum') }} {{ formatDuration(row.maxDurationMs) }}
              </div>
            </div>
            <div class="text-right font-mono text-gray-700 dark:text-gray-200">
              {{ formatCount(row.count) }}
              <div v-if="row.failureCount" class="text-rose-600 dark:text-rose-300">
                {{ t('admin.ops.runtimeMetrics.failures') }} {{ formatCount(row.failureCount) }}
              </div>
            </div>
          </div>
        </div>
        <p v-else class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtimeMetrics.noData') }}</p>
      </div>

      <div class="min-w-0 p-4">
        <h4 class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.runtimeMetrics.caches') }}
        </h4>
        <div v-if="cacheRows.length" class="mt-3 space-y-2">
          <div
            v-for="row in cacheRows"
            :key="row.name"
            data-testid="runtime-cache-row"
            class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 text-xs"
          >
            <div class="truncate font-medium text-gray-800 dark:text-gray-100">{{ cacheLabel(row.name) }}</div>
            <div class="text-right text-gray-500 dark:text-gray-400">
              <span class="font-mono text-gray-700 dark:text-gray-200">{{ formatRate(row.hit, row.hit + row.miss) }}</span>
              <span class="ml-2">{{ t('admin.ops.runtimeMetrics.hitMiss', { hit: formatCount(row.hit), miss: formatCount(row.miss) }) }}</span>
              <span v-if="row.writeIssues" class="ml-2 text-rose-600 dark:text-rose-300">
                {{ t('admin.ops.runtimeMetrics.writeIssues') }} {{ formatCount(row.writeIssues) }}
              </span>
            </div>
          </div>
        </div>
        <p v-else class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtimeMetrics.noData') }}</p>
      </div>
    </div>

    <div class="grid divide-y divide-gray-200 border-t border-gray-200 dark:divide-dark-700 dark:border-dark-700 lg:grid-cols-2 lg:divide-x lg:divide-y-0">
      <div class="min-w-0 p-4">
        <h4 class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.runtimeMetrics.failover') }}
        </h4>
        <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
          <div v-for="item in schedulingSummary" :key="item.key" class="flex items-center justify-between gap-2">
            <dt class="text-gray-500 dark:text-gray-400">{{ item.label }}</dt>
            <dd class="font-mono font-medium text-gray-800 dark:text-gray-100">{{ item.value }}</dd>
          </div>
        </dl>
      </div>

      <div class="min-w-0 p-4">
        <h4 class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.runtimeMetrics.connectionPools') }}
        </h4>
        <div v-if="connectionPoolRows.length" class="mt-3 space-y-2">
          <div
            v-for="row in connectionPoolRows"
            :key="row.name"
            data-testid="runtime-connection-pool-row"
            class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 text-xs"
          >
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-800 dark:text-gray-100">{{ connectionPoolLabel(row.name) }}</div>
              <div v-if="row.hasHTTPStats" class="mt-0.5 text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtimeMetrics.poolHTTPCurrent', { entries: formatCount(row.entries), capacity: formatCount(row.capacity), inFlight: formatCount(row.inFlight), idleAge: formatDuration(row.oldestIdleAgeMs) }) }}
              </div>
              <div v-else-if="row.hasStats" class="mt-0.5 text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.runtimeMetrics.poolCurrent', { inUse: formatCount(row.inUse), open: formatCount(row.open), idle: formatCount(row.idle), capacity: formatCount(row.capacity) }) }}
              </div>
            </div>
            <div class="text-right text-gray-500 dark:text-gray-400">
              <template v-if="row.hasHTTPStats">
                <div>
                  {{ t('admin.ops.runtimeMetrics.poolHitRate') }}
                  <span class="font-mono text-gray-700 dark:text-gray-200">{{ formatRate(row.cacheHits, row.cacheHits + row.cacheMisses) }}</span>
                </div>
                <div class="text-gray-400 dark:text-gray-500">
                  {{ t('admin.ops.runtimeMetrics.poolHTTPChanges', { created: formatCount(row.cacheCreates), evicted: formatCount(row.cacheEvicts) }) }}
                </div>
              </template>
              <template v-else-if="row.hasStats">
                <div v-if="row.baseSize">
                  {{ t('admin.ops.runtimeMetrics.poolBaseSize') }}
                  <span class="font-mono text-gray-700 dark:text-gray-200">{{ formatCount(row.baseSize) }}</span>
                </div>
                <div v-if="row.hits || row.misses">
                  {{ t('admin.ops.runtimeMetrics.poolHitRate') }}
                  <span class="font-mono text-gray-700 dark:text-gray-200">{{ formatRate(row.hits, row.hits + row.misses) }}</span>
                </div>
                <div v-if="row.waitCount || row.waitDurationMs || row.timeouts" :class="row.timeouts ? 'text-rose-600 dark:text-rose-300' : ''">
                  {{ t('admin.ops.runtimeMetrics.poolWait', { count: formatCount(row.waitCount), duration: formatDuration(row.waitDurationMs), timeout: formatCount(row.timeouts) }) }}
                </div>
                <div v-if="row.closed || row.stale" class="text-gray-400 dark:text-gray-500">
                  {{ t('admin.ops.runtimeMetrics.poolRetired', { closed: formatCount(row.closed), stale: formatCount(row.stale) }) }}
                </div>
              </template>
              <template v-else>
                {{ t('admin.ops.runtimeMetrics.poolActivity', { acquire: formatCount(row.acquire), reuse: formatCount(row.reuse) }) }}
                <span v-if="row.errors || row.waitTimeouts" class="ml-2 text-rose-600 dark:text-rose-300">
                  {{ t('admin.ops.runtimeMetrics.poolIssues', { error: formatCount(row.errors), timeout: formatCount(row.waitTimeouts) }) }}
                </span>
              </template>
            </div>
          </div>
        </div>
        <p v-else class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.runtimeMetrics.noData') }}</p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  opsAPI,
  type OpsConnectionPoolStatsSnapshot,
  type OpsRuntimeCounterSnapshot,
  type OpsRuntimeMetricsSnapshot,
} from '@/api/admin/ops'
import { formatDateTime } from '../utils/opsFormatters'

interface Props {
  // active 控制弹窗可见时才读取指标，避免后台页面产生无效请求。
  active?: boolean
  // refreshKey 在查看另一条请求时触发新的进程快照。
  refreshKey?: string | number | null
}

interface StageSummary {
  // stage 是后端固定请求阶段名称。
  stage: string
  // count 是该阶段所有标签桶的累计样本数。
  count: number
  // totalDurationMs 是累计耗时，用于计算加权平均值。
  totalDurationMs: number
  // maxDurationMs 是该阶段所有桶中的最大耗时。
  maxDurationMs: number
  // failureCount 是 result=failure 的累计样本数。
  failureCount: number
}

interface CacheSummary {
  // name 是后端固定缓存名称。
  name: string
  // hit 是缓存命中次数。
  hit: number
  // miss 是缓存未命中次数。
  miss: number
  // writeIssues 汇总写入丢弃与写入错误次数。
  writeIssues: number
}

interface ConnectionPoolSummary {
  // name 是后端固定连接池名称。
  name: string
  // acquire 是连接获取次数。
  acquire: number
  // reuse 是连接复用次数。
  reuse: number
  // errors 是连接池错误次数。
  errors: number
  // waitTimeouts 是连接等待超时次数。
  waitTimeouts: number
  // hasStats 表示后端是否提供标准驱动连接池状态。
  hasStats: boolean
  // hasHTTPStats 表示该行使用上游 HTTP 客户端缓存专用字段。
  hasHTTPStats: boolean
  // capacity 是配置的连接池容量。
  capacity: number
  // baseSize 是 go-redis 的基础连接池大小。
  baseSize: number
  // open 是当前打开连接数。
  open: number
  // inUse 是当前占用连接数。
  inUse: number
  // idle 是当前空闲连接数。
  idle: number
  // waitCount 是累计等待连接次数。
  waitCount: number
  // waitDurationMs 是累计等待时长。
  waitDurationMs: number
  // hits 是空闲连接命中次数。
  hits: number
  // misses 是新建连接次数。
  misses: number
  // timeouts 是等待连接超时次数。
  timeouts: number
  // closed 是驱动主动关闭连接的累计次数。
  closed: number
  // stale 是 Redis 移除失效连接的累计次数。
  stale: number
  // cacheHits 与 cacheMisses 是客户端缓存累计命中和未命中次数。
  cacheHits: number
  cacheMisses: number
  // cacheCreates 与 cacheEvicts 是客户端累计创建和淘汰次数。
  cacheCreates: number
  cacheEvicts: number
  // entries 是当前活动客户端条目数。
  entries: number
  // inFlight 是活动与退休条目上的进行中请求数。
  inFlight: number
  // oldestIdleAgeMs 是最久空闲活动条目的空闲时间。
  oldestIdleAgeMs: number
}

const props = withDefaults(defineProps<Props>(), {
  active: true,
  refreshKey: null,
})

const { t } = useI18n()
const snapshot = ref<OpsRuntimeMetricsSnapshot | null>(null)
let requestVersion = 0
let abortController: AbortController | null = null

const stageOrder = ['selection', 'slot_wait', 'connect', 'header_wait', 'ttft', 'stream', 'unknown']

// safeCount 将异常响应值收敛为非负有限数，避免只读面板渲染错位。
function safeCount(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0
}

// counterValue 汇总固定名称与事件对应的计数。
function counterValue(rows: OpsRuntimeCounterSnapshot[], event: string): number {
  return rows.reduce((total, row) => total + (row.event === event ? safeCount(row.count) : 0), 0)
}

// translatedMetricLabel 为已知枚举返回本地化名称，并让未来新增枚举仍可读。
function translatedMetricLabel(scope: string, value: string): string {
  const normalized = value || 'unknown'
  const key = `admin.ops.runtimeMetrics.${scope}.${normalized}`
  const translated = t(key)
  return translated === key ? normalized : translated
}

// stageLabel 返回请求阶段的本地化名称。
function stageLabel(stage: string): string {
  return translatedMetricLabel('stages', stage)
}

// cacheLabel 返回缓存的本地化名称。
function cacheLabel(name: string): string {
  return translatedMetricLabel('cacheNames', name)
}

// connectionPoolLabel 返回连接池的本地化名称。
function connectionPoolLabel(name: string): string {
  return translatedMetricLabel('connectionPoolNames', name)
}

// formatCount 格式化累计计数。
function formatCount(value: number): string {
  return Math.round(safeCount(value)).toLocaleString()
}

// formatDuration 以毫秒展示有限耗时。
function formatDuration(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '—'
  return `${Math.round(value * 10) / 10} ms`
}

// formatRate 计算命中或复用比例。
function formatRate(numerator: number, denominator: number): string {
  if (denominator <= 0) return '—'
  return `${Math.round((numerator / denominator) * 1000) / 10}%`
}

// stageRows 将协议、结果和错误分类桶聚合为阶段摘要。
const stageRows = computed<StageSummary[]>(() => {
  const grouped = new Map<string, StageSummary>()
  for (const metric of snapshot.value?.request_stages || []) {
    const stage = metric.stage || 'unknown'
    const row = grouped.get(stage) || { stage, count: 0, totalDurationMs: 0, maxDurationMs: 0, failureCount: 0 }
    const count = safeCount(metric.count)
    row.count += count
    row.totalDurationMs += safeCount(metric.total_duration_ms)
    row.maxDurationMs = Math.max(row.maxDurationMs, safeCount(metric.max_duration_ms))
    if (metric.result === 'failure') row.failureCount += count
    grouped.set(stage, row)
  }
  return Array.from(grouped.values()).sort((left, right) => {
    const leftIndex = stageOrder.indexOf(left.stage)
    const rightIndex = stageOrder.indexOf(right.stage)
    return (leftIndex < 0 ? stageOrder.length : leftIndex) - (rightIndex < 0 ? stageOrder.length : rightIndex)
  })
})

// cacheRows 将缓存固定事件聚合为命中率与写入异常摘要。
const cacheRows = computed<CacheSummary[]>(() => {
  const grouped = new Map<string, OpsRuntimeCounterSnapshot[]>()
  for (const metric of snapshot.value?.caches || []) {
    const name = metric.name || 'unknown'
    grouped.set(name, [...(grouped.get(name) || []), metric])
  }
  return Array.from(grouped, ([name, rows]) => ({
    name,
    hit: counterValue(rows, 'hit'),
    miss: counterValue(rows, 'miss'),
    writeIssues: counterValue(rows, 'write_drop') + counterValue(rows, 'write_error'),
  })).sort((left, right) => left.name.localeCompare(right.name))
})

// schedulingSummary 提取账号切换、快照复用和槽位等待关键计数。
const schedulingSummary = computed(() => {
  const rows = snapshot.value?.scheduling || []
  const snapshotHits = counterValue(rows, 'snapshot_hit')
  const snapshotMisses = counterValue(rows, 'snapshot_miss')
  return [
    { key: 'failover', label: t('admin.ops.runtimeMetrics.failovers'), value: formatCount(counterValue(rows, 'failover')) },
    { key: 'snapshot', label: t('admin.ops.runtimeMetrics.snapshotReuse'), value: formatRate(snapshotHits, snapshotHits + snapshotMisses) },
    { key: 'slot-wait', label: t('admin.ops.runtimeMetrics.slotWaits'), value: formatCount(counterValue(rows, 'slot_wait')) },
    { key: 'slot-timeout', label: t('admin.ops.runtimeMetrics.slotTimeouts'), value: formatCount(counterValue(rows, 'slot_wait_timeout')) },
  ]
})

// emptyConnectionPoolSummary 创建单个连接池的零值摘要。
function emptyConnectionPoolSummary(name: string): ConnectionPoolSummary {
  return {
    name,
    acquire: 0,
    reuse: 0,
    errors: 0,
    waitTimeouts: 0,
    hasStats: false,
    hasHTTPStats: false,
    capacity: 0,
    baseSize: 0,
    open: 0,
    inUse: 0,
    idle: 0,
    waitCount: 0,
    waitDurationMs: 0,
    hits: 0,
    misses: 0,
    timeouts: 0,
    closed: 0,
    stale: 0,
    cacheHits: 0,
    cacheMisses: 0,
    cacheCreates: 0,
    cacheEvicts: 0,
    entries: 0,
    inFlight: 0,
    oldestIdleAgeMs: 0,
  }
}

// applyConnectionPoolStats 将标准库与 Redis 的实时状态写入统一摘要。
function applyConnectionPoolStats(row: ConnectionPoolSummary, stats: OpsConnectionPoolStatsSnapshot): void {
  row.hasStats = true
  row.capacity = safeCount(stats.capacity)
  row.baseSize = safeCount(stats.base_size)
  row.open = safeCount(stats.open)
  row.inUse = safeCount(stats.in_use)
  row.idle = safeCount(stats.idle)
  row.waitCount = safeCount(stats.wait_count)
  row.waitDurationMs = safeCount(stats.wait_duration_ms)
  row.hits = safeCount(stats.hits)
  row.misses = safeCount(stats.misses)
  row.timeouts = safeCount(stats.timeouts)
  row.closed = safeCount(stats.closed_idle) + safeCount(stats.closed_idle_time) + safeCount(stats.closed_lifetime)
  row.stale = safeCount(stats.stale)
  if (stats.name === 'upstream_http') {
    row.hasHTTPStats = true
    row.cacheHits = safeCount(stats.cache_hit_total)
    row.cacheMisses = safeCount(stats.cache_miss_total)
    row.cacheCreates = safeCount(stats.cache_create_total)
    row.cacheEvicts = safeCount(stats.cache_evict_total)
    row.entries = safeCount(stats.entries)
    row.inFlight = safeCount(stats.in_flight)
    row.oldestIdleAgeMs = safeCount(stats.oldest_idle_age_ms)
  }
}

// connectionPoolRows 优先展示标准驱动统计，并兼容旧版固定事件计数。
const connectionPoolRows = computed<ConnectionPoolSummary[]>(() => {
  const grouped = new Map<string, OpsRuntimeCounterSnapshot[]>()
  for (const metric of snapshot.value?.connection_pools || []) {
    const name = metric.name || 'unknown'
    grouped.set(name, [...(grouped.get(name) || []), metric])
  }
  const summaries = new Map<string, ConnectionPoolSummary>()
  for (const [name, rows] of grouped) {
    const summary = emptyConnectionPoolSummary(name)
    summary.acquire = counterValue(rows, 'acquire')
    summary.reuse = counterValue(rows, 'reuse')
    summary.errors = counterValue(rows, 'error')
    summary.waitTimeouts = counterValue(rows, 'wait_timeout')
    summaries.set(name, summary)
  }
  for (const stats of snapshot.value?.connection_pool_stats || []) {
    const name = stats.name || 'unknown'
    const summary = summaries.get(name) || emptyConnectionPoolSummary(name)
    applyConnectionPoolStats(summary, stats)
    summaries.set(name, summary)
  }
  return Array.from(summaries.values()).sort((left, right) => left.name.localeCompare(right.name))
})

// loadSnapshot 独立加载运行时快照；失败仅隐藏摘要，不影响父级排障流程。
async function loadSnapshot(): Promise<void> {
  if (!props.active) return

  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const version = ++requestVersion
  try {
    const nextSnapshot = await opsAPI.getRuntimeMetrics({ signal: controller.signal })
    if (version === requestVersion) snapshot.value = nextSnapshot
  } catch {
    if (version === requestVersion) snapshot.value = null
  } finally {
    if (abortController === controller) abortController = null
  }
}

watch(
  () => [props.active, props.refreshKey] as const,
  ([active]) => {
    if (active) void loadSnapshot()
    else {
      abortController?.abort()
      abortController = null
      requestVersion += 1
      snapshot.value = null
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  abortController?.abort()
  abortController = null
  requestVersion += 1
})
</script>
