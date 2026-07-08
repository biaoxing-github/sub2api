<template>
  <div ref="hostRef" class="legacy-html-tool" :data-tool-key="toolKey" :aria-label="title"></div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  createLegacyToolScope,
  type LegacyToolCleanup,
  type LegacyToolScope,
  type LegacyToolServiceMap
} from './legacyToolRuntime'

interface Props {
  toolKey: string
  title: string
  headHtml: string
  styles: string
  bodyHtml: string
  mountTool: (scope: LegacyToolScope) => LegacyToolCleanup
  services?: LegacyToolServiceMap
}

const props = defineProps<Props>()
const hostRef = ref<HTMLElement | null>(null)
let shadowRoot: ShadowRoot | null = null
let cleanupTool: LegacyToolCleanup | null = null

function normalizeStyles(styles: string): string {
  return styles
    .replace(/:root/g, ':host')
    .replace(/(^|})\s*html\s*\{/g, '$1\n:host {')
    .replace(/(^|})\s*body\s*\{/g, '$1\n.legacy-tool-document {')
}

function clearTool() {
  cleanupTool?.()
  cleanupTool = null
  shadowRoot?.replaceChildren()
}

function renderNavigationNotice(target: string): boolean {
  const statusBox = shadowRoot?.querySelector<HTMLElement>('#statusBox')
  if (statusBox) {
    statusBox.className = 'status warn'
    statusBox.textContent = `独立页面 ${target} 已整合到当前工具页，无需跳转。`
  }
  return true
}

function renderMountError(error: unknown) {
  const message = error instanceof Error ? error.message : String(error)
  if (!shadowRoot) return
  shadowRoot.innerHTML = `
    <style>
      :host { display: block; min-height: 360px; background: #f8fafc; color: #0f172a; font-family: "Microsoft YaHei", sans-serif; }
      .legacy-tool-error { display: grid; min-height: 360px; place-items: center; padding: 32px; text-align: center; }
      .legacy-tool-error__box { max-width: 560px; border: 1px solid #fecaca; border-radius: 8px; background: #fff1f2; padding: 20px; color: #991b1b; }
      .legacy-tool-error__title { margin-bottom: 8px; font-size: 18px; font-weight: 700; }
    </style>
    <div class="legacy-tool-error">
      <div class="legacy-tool-error__box">
        <div class="legacy-tool-error__title">工具加载失败</div>
        <div>${escapeHtml(message)}</div>
      </div>
    </div>
  `
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

async function mountLegacyTool() {
  await nextTick()
  const host = hostRef.value
  if (!host) return

  clearTool()
  shadowRoot = shadowRoot ?? host.attachShadow({ mode: 'open' })
  shadowRoot.innerHTML = `
    ${props.headHtml}
    <style>
      :host { display: block; min-height: calc(100vh - 148px); background: #f8fafc; }
      .legacy-tool-document { min-height: calc(100vh - 148px); }
      ${normalizeStyles(props.styles)}
    </style>
    <div class="legacy-tool-document">${props.bodyHtml}</div>
  `

  const scope = createLegacyToolScope(shadowRoot, {
    onNavigate: renderNavigationNotice,
    services: props.services
  })

  try {
    cleanupTool = props.mountTool(scope)
  } catch (error) {
    scope.cleanup()
    console.error(`[AdminTools] ${props.toolKey} mount failed`, error)
    renderMountError(error)
  }
}

onMounted(() => {
  mountLegacyTool()
})

watch(
  () => [props.toolKey, props.headHtml, props.styles, props.bodyHtml, props.mountTool, props.services],
  () => {
    mountLegacyTool()
  }
)

onBeforeUnmount(() => {
  clearTool()
})
</script>

<style scoped>
.legacy-html-tool {
  min-height: calc(100vh - 148px);
}
</style>
