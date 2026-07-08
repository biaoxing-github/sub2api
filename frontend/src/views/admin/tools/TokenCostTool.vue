<template>
  <LegacyHtmlTool
    tool-key="token-cost"
    title="API Token 余额动态计算器"
    :head-html="tokenCostLegacyHeadHtml"
    :styles="tokenCostLegacyStyles"
    :body-html="tokenCostLegacyBodyHtml"
    :mount-tool="mountTokenCostLegacyTool"
    :services="tokenCostServices"
  />
</template>

<script setup lang="ts">
import { tokenCostAPI } from '@/api/admin'
import type { TokenCostHealth, TokenCostState } from '@/types'
import LegacyHtmlTool from './LegacyHtmlTool.vue'
import type { LegacyToolServiceMap } from './legacyToolRuntime'
import {
  mountTokenCostLegacyTool,
  tokenCostLegacyBodyHtml,
  tokenCostLegacyHeadHtml,
  tokenCostLegacyStyles
} from './tokenCostLegacy.generated'

interface TokenCostLegacyService {
  loadState: () => Promise<TokenCostState>
  saveState: (state: TokenCostState) => Promise<TokenCostState>
  health: () => Promise<TokenCostHealth>
}

const tokenCostService: TokenCostLegacyService = {
  loadState: () => tokenCostAPI.getState(),
  saveState: (state) => tokenCostAPI.saveState(state),
  health: () => tokenCostAPI.health()
}

const tokenCostServices: LegacyToolServiceMap = {
  tokenCost: tokenCostService
}
</script>
