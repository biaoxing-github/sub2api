<template>
  <LegacyHtmlTool
    tool-key="token-cost"
    title="API Token 余额动态计算器"
    :head-html="tokenCostLegacyHeadHtml"
    :styles="tokenCostLegacyStyles"
    :body-html="tokenCostLegacyBodyHtml"
    :mount-tool="mountTokenCostLegacyTool"
  />
</template>

<script setup lang="ts">
import LegacyHtmlTool from './LegacyHtmlTool.vue'
import {
  mountTokenCostLegacyTool,
  tokenCostLegacyBodyHtml,
  tokenCostLegacyHeadHtml,
  tokenCostLegacyStyles
} from './tokenCostLegacy.generated'
</script>
