<template>
  <LegacyHtmlTool
    tool-key="newapi-checkin"
    title="NewApi Checkin Dashboard"
    :head-html="newapiCheckinLegacyHeadHtml"
    :styles="newapiCheckinLegacyStyles"
    :body-html="newapiCheckinLegacyBodyHtml"
    :mount-tool="mountNewapiCheckinLegacyTool"
  />
</template>

<script setup lang="ts">
import LegacyHtmlTool from './LegacyHtmlTool.vue'
import {
  mountNewapiCheckinLegacyTool,
  newapiCheckinLegacyBodyHtml,
  newapiCheckinLegacyHeadHtml,
  newapiCheckinLegacyStyles
} from './newapiCheckinLegacy.generated'
</script>
