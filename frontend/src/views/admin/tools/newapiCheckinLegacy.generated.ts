/* eslint-disable */
// @ts-nocheck
// 本文件由 frontend\public\newapi-checkin\index.html 机械拆分生成，用于在管理端原生挂载旧工具页面。
import type { LegacyToolCleanup, LegacyToolScope } from "./legacyToolRuntime"

export const newapiCheckinLegacyHeadHtml = "<link rel=\"preconnect\" href=\"https://fonts.googleapis.com\">\n<link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin>\n<link href=\"https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;600;700&family=Fira+Sans:wght@300;400;500;600;700&display=swap\" rel=\"stylesheet\">"
export const newapiCheckinLegacyStyles = "\r\n    :root {\r\n      color-scheme: light;\r\n      --bg: #F8FAFC;\r\n      --bg-2: #F1F5F9;\r\n      --surface: #FFFFFF;\r\n      --surface-2: #F8FAFC;\r\n      --surface-3: #E2E8F0;\r\n      --ink: #0F172A;\r\n      --muted: #64748B;\r\n      --line: #E2E8F0;\r\n      --line-strong: #CBD5E1;\r\n      --accent: #0891B2;\r\n      --accent-strong: #0E7490;\r\n      --accent-soft: #CFFAFE;\r\n      --ok: #16A34A;\r\n      --ok-soft: #DCFCE7;\r\n      --warn: #D97706;\r\n      --warn-soft: #FEF3C7;\r\n      --danger: #DC2626;\r\n      --danger-soft: #FEE2E2;\r\n      --shadow: 0 12px 32px rgba(15, 23, 42, 0.04), 0 4px 12px rgba(15, 23, 42, 0.02);\r\n      --radius: 4px;\r\n      --radius-lg: 6px;\r\n      --content: 1620px;\r\n\r\n      /* Overdrive: Spring Physics & Motion */\r\n      --spring-bouncy: linear(0, 0.009, 0.035 2.1%, 0.141, 0.281 6.7%, 0.723 12.9%, 0.938 16.7%, 1.017, 1.077, 1.121, 1.149 24.3%, 1.159, 1.163, 1.161, 1.154 29.9%, 1.129 32.8%, 1.051 39.6%, 1.017 43.1%, 0.991, 0.977 51%, 0.974 53.8%, 0.975 57.1%, 0.997 69.8%, 1.003 76.9%, 1.004 83.8%, 1);\r\n      --spring-smooth: cubic-bezier(0.175, 0.885, 0.32, 1.1);\r\n      --spring-stiff: cubic-bezier(0.1, 0.7, 0.1, 1);\r\n      --motion-fast: 300ms;\r\n      --motion-normal: 500ms;\r\n    }\r\n\r\n    * {\r\n      box-sizing: border-box;\r\n    }\r\n\r\n    html {\r\n      scroll-behavior: smooth;\r\n    }\r\n\r\n    /* Staggered Entry Animations */\r\n    @keyframes slideFadeIn {\r\n      from {\r\n        opacity: 0;\r\n        transform: translateY(16px) scale(0.98);\r\n      }\r\n      to {\r\n        opacity: 1;\r\n        transform: translateY(0) scale(1);\r\n      }\r\n    }\r\n\r\n    .metric-card, .overview-card, .feed-item, .panel {\r\n      animation: slideFadeIn var(--motion-normal) var(--spring-bouncy) both;\r\n    }\r\n\r\n    .metric-card:nth-child(1) { animation-delay: 0ms; }\r\n    .metric-card:nth-child(2) { animation-delay: 60ms; }\r\n    .metric-card:nth-child(3) { animation-delay: 120ms; }\r\n    .metric-card:nth-child(4) { animation-delay: 180ms; }\r\n    .metric-card:nth-child(5) { animation-delay: 240ms; }\r\n    .metric-card:nth-child(6) { animation-delay: 300ms; }\r\n\r\n    .feed-item:nth-child(1) { animation-delay: 0ms; }\r\n    .feed-item:nth-child(2) { animation-delay: 50ms; }\r\n    .feed-item:nth-child(3) { animation-delay: 100ms; }\r\n    .feed-item:nth-child(4) { animation-delay: 150ms; }\r\n    .feed-item:nth-child(5) { animation-delay: 200ms; }\r\n\r\n    @starting-style {\r\n      .metric-card, .overview-card, .feed-item, .panel {\r\n        opacity: 0;\r\n        transform: translateY(16px) scale(0.98);\r\n      }\r\n    }\r\n\r\n    body {\r\n      margin: 0;\r\n      background: var(--bg);\r\n      color: var(--ink);\r\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif;\r\n      line-height: 1.45;\r\n    }\r\n\r\n    button,\r\n    select,\r\n    input,\r\n    textarea {\r\n      font: inherit;\r\n    }\r\n\r\n    button {\r\n      cursor: pointer;\r\n    }\r\n\r\n    code {\r\n      padding: 0 4px;\r\n      border-radius: 4px;\r\n      background: color-mix(in oklab, var(--surface-3) 88%, white);\r\n      font-size: 12px;\r\n      color: var(--accent-strong);\r\n    }\r\n\r\n    .shell {\r\n      max-width: var(--content);\r\n      margin: 0 auto;\r\n      padding: 32px 32px 48px;\r\n    }\r\n\r\n    .header {\r\n      display: grid;\r\n      grid-template-columns: minmax(0, 1fr);\r\n      gap: 18px;\r\n      align-items: end;\r\n      margin-bottom: 16px;\r\n    }\r\n\r\n    .eyebrow {\r\n      display: inline-flex;\r\n      align-items: center;\r\n      gap: 8px;\r\n      min-height: 28px;\r\n      padding: 0 11px;\r\n      border: 1px solid color-mix(in oklab, var(--accent) 16%, white);\r\n      border-radius: 999px;\r\n      background: color-mix(in oklab, var(--accent-soft) 74%, white);\r\n      color: var(--accent-strong);\r\n      font-size: 12px;\r\n      font-weight: 700;\r\n      letter-spacing: 0.04em;\r\n      text-transform: uppercase;\r\n    }\r\n\r\n    .eyebrow-dot {\r\n      width: 8px;\r\n      height: 8px;\r\n      border-radius: 50%;\r\n      background: var(--accent);\r\n      flex: 0 0 auto;\r\n    }\r\n\r\n    h1 {\r\n      margin: 10px 0 0;\r\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif; font-weight: 600;\r\n      font-size: clamp(2.25rem, 4vw, 3.45rem);\r\n      line-height: 0.96;\r\n      letter-spacing: 0;\r\n    }\r\n\r\n    .header-copy {\r\n      max-width: 860px;\r\n      margin: 12px 0 0;\r\n      color: var(--muted);\r\n      font-size: 15px;\r\n    }\r\n\r\n    button,\r\n    select,\r\n    input,\r\n    textarea {\r\n      min-height: 40px;\r\n      border: 1px solid var(--line);\r\n      border-radius: var(--radius);\r\n      background: var(--surface);\r\n      color: var(--ink);\r\n    }\r\n\r\n    button {\r\n      padding: 8px 14px;\r\n      transition:\r\n        background-color var(--motion-fast) var(--spring-stiff),\r\n        border-color var(--motion-fast) var(--spring-stiff),\r\n        transform var(--motion-fast) var(--spring-bouncy),\r\n        box-shadow var(--motion-fast) var(--spring-bouncy);\r\n    }\r\n\r\n    button:hover:not(:disabled) {\r\n      transform: translateY(-2px) scale(1.02);\r\n      border-color: var(--line-strong);\r\n      background: color-mix(in oklab, var(--surface) 90%, white);\r\n      box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);\r\n    }\r\n\r\n    button:active:not(:disabled) {\r\n      transform: translateY(1px) scale(0.96);\r\n      transition-duration: 100ms;\r\n    }\r\n\r\n    button:disabled {\n      opacity: 0.58;\n      cursor: not-allowed;\n      transform: none;\n    }\n\r\n    button.primary {\r\n      border-color: var(--accent-strong);\r\n      background: var(--accent);\r\n      color: white;\r\n    }\r\n\r\n    button.secondary {\r\n      background: color-mix(in oklab, var(--surface-2) 90%, white);\r\n    }\r\n\r\n    button.ghost {\r\n      background: transparent;\r\n    }\r\n\r\n    .summary-strip {\r\n      display: grid;\r\n      grid-template-columns: repeat(6, minmax(0, 1fr));\r\n      gap: 10px;\r\n      margin-bottom: 16px;\r\n    }\r\n\r\n    .metric-card {\r\n      min-height: 110px;\r\n      padding: 20px 24px;\r\n      border: none;\r\n      border-radius: var(--radius-lg);\r\n      background: var(--surface);\r\n      box-shadow: var(--shadow);\r\n      transition: transform var(--motion-fast) var(--spring-bouncy), box-shadow var(--motion-fast) var(--spring-bouncy);\r\n    }\r\n\r\n    .metric-card:hover {\r\n      transform: translateY(-4px) scale(1.01);\r\n      box-shadow: 0 20px 40px rgba(15, 23, 42, 0.06), 0 8px 16px rgba(15, 23, 42, 0.03);\r\n    }\r\n\r\n    .metric-label {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n      font-weight: 700;\r\n      letter-spacing: 0.04em;\r\n      text-transform: uppercase;\r\n    }\r\n\r\n    .metric-value {\r\n      margin-top: 16px;\r\n      font-family: \"Fira Code\", monospace;\r\n      font-weight: 600;\r\n      font-size: clamp(1.8rem, 2.5vw, 2.8rem);\r\n      line-height: 1;\r\n      font-variant-numeric: tabular-nums;\r\n    }\r\n\r\n    .metric-note {\r\n      margin-top: 10px;\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    .workspace {\r\n      display: grid;\r\n      grid-template-columns: minmax(308px, 348px) minmax(0, 1fr);\r\n      gap: 16px;\r\n      align-items: start;\r\n    }\r\n\r\n    .sidebar {\n      position: sticky;\n      top: 16px;\n      padding: 24px;\n      display: grid;\n      gap: 14px;\r\n    }\r\n\r\n    .sidebar-section {\r\n      display: grid;\r\n      gap: 12px;\r\n    }\r\n\r\n    .panel {\n      position: static;\n      padding: 24px;\n      display: grid;\n      gap: 14px;\n      overflow: hidden;\n    }\n\r\n    .panel + .panel {\r\n      margin-top: 14px;\r\n    }\r\n\r\n    .panel-head {\n      display: flex;\n      flex-wrap: wrap;\n      justify-content: space-between;\n      align-items: end;\n      gap: 10px;\n      margin-bottom: 14px;\n    }\n\n    .panel-head > :first-child {\n      min-width: 0;\n      display: grid;\n      gap: 6px;\n    }\n\n    .panel-title {\n      margin: 0;\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif; font-weight: 600;\n      font-size: 19px;\n      line-height: 1.12;\n    }\n\n    .panel-meta {\n      color: var(--muted);\n      font-size: 13px;\n      line-height: 1.45;\n    }\n\r\n    @keyframes pulseBackground {\r\n      0% { background-position: 0% 50%; }\r\n      50% { background-position: 100% 50%; }\r\n      100% { background-position: 0% 50%; }\r\n    }\r\n\r\n    .status-box {\r\n      min-height: 48px;\r\n      padding: 12px 14px;\r\n      border: 1px solid var(--line);\r\n      border-radius: var(--radius);\r\n      background: color-mix(in oklab, var(--surface-2) 84%, white);\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n      transition: all var(--motion-fast) var(--spring-smooth);\r\n    }\r\n\r\n    .status-box[data-tone=\"busy\"] {\r\n      border-color: color-mix(in oklab, var(--accent) 26%, var(--line));\r\n      background: linear-gradient(90deg,\r\n        color-mix(in oklab, var(--accent-soft) 42%, white) 0%,\r\n        color-mix(in oklab, var(--accent-soft) 80%, white) 50%,\r\n        color-mix(in oklab, var(--accent-soft) 42%, white) 100%);\r\n      background-size: 200% 200%;\r\n      animation: pulseBackground 2s ease infinite;\r\n      color: var(--accent-strong);\r\n    }\r\n\r\n    .status-box[data-tone=\"error\"] {\r\n      border-color: color-mix(in oklab, var(--danger) 30%, var(--line));\r\n      background: color-mix(in oklab, var(--danger-soft) 72%, white);\r\n      color: var(--danger);\r\n    }\r\n\r\n    .status-box[data-tone=\"ok\"] {\n      border-color: color-mix(in oklab, var(--ok) 24%, var(--line));\n      background: color-mix(in oklab, var(--ok-soft) 72%, white);\n      color: var(--ok);\n    }\n\n    .status-box.is-pending {\n      position: relative;\n      overflow: hidden;\n    }\n\n    .status-box.is-pending::after {\n      content: \"\";\n      position: absolute;\n      inset: 0;\n      background: linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.35) 48%, transparent 100%);\n      transform: translateX(-120%);\n      animation: sweep 1.1s ease-in-out infinite;\n      pointer-events: none;\n    }\n\n    @keyframes sweep {\n      to {\n        transform: translateX(120%);\n      }\n    }\n\r\n    .control-grid,\r\n    .field-stack,\r\n    .note-list,\r\n    .view-stack,\r\n    .overview-stack {\r\n      display: grid;\r\n      gap: 12px;\r\n    }\r\n\r\n    .toolbar {\n      display: flex;\n      flex-wrap: wrap;\n      gap: 10px;\n    }\n\n    .toolbar button[data-busy=\"true\"],\n    .site-actions button[data-busy=\"true\"],\n    .action-cluster button[data-busy=\"true\"] {\n      position: relative;\n      pointer-events: none;\n      opacity: 0.78;\n      overflow: hidden;\n    }\n\n    .toolbar button[data-busy=\"true\"]::after,\n    .site-actions button[data-busy=\"true\"]::after,\n    .action-cluster button[data-busy=\"true\"]::after {\n      content: \"\";\n      position: absolute;\n      inset: 0;\n      background: linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.35) 50%, transparent 100%);\n      transform: translateX(-120%);\n      animation: sweep 1.1s ease-in-out infinite;\n      pointer-events: none;\n    }\n\n    .status-row {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 10px;\n    }\n\n    .status-row .status-mini {\n      color: var(--muted);\n      font-size: 12px;\n      white-space: nowrap;\n    }\n\n    .row-pending td {\n      background: color-mix(in oklab, var(--accent-soft) 26%, white);\n    }\n\n    .row-done td {\n      background: color-mix(in oklab, var(--ok-soft) 22%, white);\n    }\n\n    .row-error td {\n      background: color-mix(in oklab, var(--danger-soft) 24%, white);\n    }\n\n    .action-cluster {\n      display: grid;\n      grid-template-columns: repeat(auto-fit, minmax(86px, 1fr));\n      gap: 6px;\n      min-width: 0;\n    }\n\n    .action-cluster .quick-action {\n      width: 100%;\n      min-width: 0;\n      padding-inline: 8px;\n      white-space: nowrap;\n    }\n\r\n    label {\r\n      display: grid;\r\n      gap: 6px;\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    select,\r\n    input,\r\n    textarea {\r\n      width: 100%;\r\n      padding: 9px 11px;\r\n    }\r\n\r\n    select:focus,\r\n    input:focus,\r\n    textarea:focus,\r\n    button:focus {\r\n      outline: 2px solid color-mix(in oklab, var(--accent) 46%, white);\r\n      outline-offset: 2px;\r\n    }\r\n\r\n    .target-card {\r\n      padding: 12px 14px;\r\n      border: 1px solid var(--line);\r\n      border-radius: var(--radius);\r\n      background: color-mix(in oklab, var(--surface-2) 88%, white);\r\n      display: grid;\r\n      gap: 8px;\r\n    }\r\n\r\n    .target-title {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n      text-transform: uppercase;\r\n      font-weight: 700;\r\n      letter-spacing: 0.04em;\r\n    }\r\n\r\n    .target-main {\r\n      display: grid;\r\n      gap: 4px;\r\n    }\r\n\r\n    .target-main strong {\r\n      font-size: 18px;\r\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif; font-weight: 600;\r\n      line-height: 1;\r\n    }\r\n\r\n    .target-meta {\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    details {\r\n      border-top: 1px solid var(--line);\r\n      padding-top: 12px;\r\n    }\r\n\r\n    summary {\r\n      cursor: pointer;\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n      user-select: none;\r\n    }\r\n\r\n    pre {\r\n      margin: 10px 0 0;\r\n      min-height: 120px;\r\n      padding: 14px;\r\n      border-radius: var(--radius);\r\n      background: oklch(0.212 0.018 255);\r\n      color: oklch(0.962 0.012 226);\r\n      overflow: auto;\r\n      white-space: pre-wrap;\r\n      word-break: break-word;\r\n      font-size: 12px;\r\n      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;\r\n    }\r\n\r\n    .note-list {\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    .main {\r\n      display: grid;\r\n      gap: 14px;\r\n    }\r\n\r\n    .viewbar {\r\n      border: none;\r\n      border-radius: var(--radius-lg);\r\n      background: var(--surface);\r\n      box-shadow: var(--shadow);\r\n      padding: 18px 24px;\r\n      display: grid;\r\n      gap: 12px;\r\n    }\r\n\r\n    .tab-row {\r\n      display: flex;\r\n      flex-wrap: wrap;\r\n      gap: 8px;\r\n    }\r\n\r\n    .tab-button {\r\n      min-height: 38px;\r\n      padding: 8px 13px;\r\n      border-radius: 999px;\r\n      background: transparent;\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n      font-weight: 600;\r\n      transition: all var(--motion-fast) var(--spring-bouncy);\r\n      border: 1px solid transparent;\r\n    }\r\n\r\n    .tab-button:hover {\r\n      background: var(--surface-2);\r\n      transform: scale(1.05);\r\n    }\r\n\r\n    .tab-button:active {\r\n      transform: scale(0.95);\r\n    }\r\n\r\n    .tab-button[aria-selected=\"true\"] {\r\n      border-color: color-mix(in oklab, var(--accent) 24%, var(--line));\r\n      background: color-mix(in oklab, var(--accent-soft) 78%, white);\r\n      color: var(--accent-strong);\r\n      transform: scale(1.05);\r\n    }\r\n\r\n    .filter-row {\r\n      display: flex;\r\n      flex-wrap: wrap;\r\n      gap: 8px;\r\n      align-items: center;\r\n    }\r\n\r\n    .site-actions {\n      display: grid;\n      grid-template-columns: repeat(3, minmax(0, 1fr));\n      gap: 8px;\n      align-items: stretch;\n      padding-top: 2px;\n    }\n\n    .site-actions button {\n      min-width: 0;\n      width: 100%;\n      white-space: nowrap;\n    }\n\r\n    .filter-pill {\r\n      min-height: 34px;\r\n      padding: 6px 12px;\r\n      border-radius: 999px;\r\n      background: transparent;\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    .filter-pill.active {\r\n      border-color: color-mix(in oklab, var(--accent) 24%, var(--line));\r\n      background: color-mix(in oklab, var(--accent-soft) 80%, white);\r\n      color: var(--accent-strong);\r\n    }\r\n\r\n    .filter-meta {\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    .view-panel[hidden] {\n      display: none;\n    }\n\n    .directory-trigger {\n      margin-left: auto;\n      min-height: 38px;\n    }\n\n    .directory-dialog {\n      width: min(1500px, calc(100vw - 32px));\n      max-width: none;\n      max-height: calc(100vh - 32px);\n      margin: auto;\n      padding: 0;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-lg);\n      background: var(--bg);\n      color: var(--ink);\n      box-shadow: 0 24px 80px rgba(15, 23, 42, 0.24);\n      overflow: hidden;\n    }\n\n    .directory-dialog::backdrop {\n      background: rgba(15, 23, 42, 0.52);\n    }\n\n    .credential-dialog {\n      width: min(560px, calc(100vw - 32px));\n    }\n\n    .credential-form {\n      display: grid;\n      gap: 14px;\n      padding: 20px 22px 22px;\n      background: var(--surface);\n    }\n\n    .credential-target {\n      padding-bottom: 12px;\n      border-bottom: 1px solid var(--line);\n      color: var(--muted);\n      font-size: 13px;\n    }\n\n    .directory-dialog-shell {\n      max-height: calc(100vh - 34px);\n      display: grid;\n      grid-template-rows: auto minmax(0, 1fr);\n    }\n\n    .directory-dialog-head {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 12px;\n      padding: 18px 22px;\n      border-bottom: 1px solid var(--line);\n      background: var(--surface);\n    }\n\n    .directory-dialog-body {\n      padding: 18px;\n      overflow: auto;\n    }\n\n    .api-key-stack {\n      display: grid;\n      gap: 8px;\n      min-width: 260px;\n    }\n\n    .api-key-item {\n      display: grid;\n      gap: 7px;\n      padding: 9px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius);\n      background: var(--surface-2);\n    }\n\n    .api-key-item.referenced {\n      border-color: color-mix(in oklab, var(--ok) 46%, var(--line));\n      background: color-mix(in oklab, var(--ok-soft) 48%, var(--surface-2));\n    }\n\n    .account-reference-list {\n      display: grid;\n      gap: 6px;\n    }\n\n    .account-reference-row {\n      display: flex;\n      align-items: center;\n      flex-wrap: wrap;\n      gap: 6px;\n      font-size: 12px;\n    }\n\n    .api-key-line,\n    .group-control {\n      display: flex;\n      align-items: center;\n      flex-wrap: wrap;\n      gap: 6px;\n    }\n\n    .group-control select {\n      width: auto;\n      min-width: 126px;\n      min-height: 34px;\n      padding-block: 5px;\n    }\n\n    .group-capability {\n      color: var(--muted);\n      font-size: 12px;\n      line-height: 1.4;\n    }\n\r\n    .view-grid-2 {\r\n      display: grid;\r\n      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);\r\n      gap: 14px;\r\n    }\r\n\r\n    .overview-site-grid,\r\n    .overview-balance-grid {\r\n      display: grid;\r\n      grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr));\n      gap: 10px;\r\n    }\r\n\r\n    .overview-card {\r\n      padding: 24px;\r\n      border: none;\r\n      box-shadow: var(--shadow);\r\n      border-radius: var(--radius-lg);\r\n      background: var(--surface);\r\n      display: grid;\r\n      gap: 12px;\r\n      min-height: 140px;\r\n      transition: transform var(--motion-fast) var(--spring-bouncy), box-shadow var(--motion-fast) var(--spring-bouncy);\r\n    }\r\n\r\n    .overview-card:hover {\r\n      transform: translateY(-4px) scale(1.01);\r\n      box-shadow: 0 20px 40px rgba(15, 23, 42, 0.06), 0 8px 16px rgba(15, 23, 42, 0.03);\r\n    }\r\n\r\n    .overview-top {\n      display: flex;\n      flex-wrap: wrap;\n      justify-content: space-between;\n      gap: 10px;\r\n      align-items: start;\r\n    }\r\n\r\n    .overview-site {\n      min-width: 0;\n      flex: 1 1 150px;\n      display: grid;\n      gap: 4px;\r\n    }\r\n\r\n    .overview-site strong {\r\n      font-size: 17px;\r\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif; font-weight: 600;\r\n      line-height: 1;\n      overflow-wrap: anywhere;\n    }\n\n    .overview-top .chip-row {\n      flex: 0 1 auto;\n      justify-content: flex-end;\n    }\n\r\n    .overview-sub {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n    }\r\n\r\n    .chip-row,\r\n    .badge-row {\r\n      display: flex;\r\n      flex-wrap: wrap;\r\n      gap: 8px;\r\n    }\r\n\r\n    .chip,\r\n    .badge {\r\n      font-family: \"Fira Code\", monospace;\r\n      display: inline-flex;\r\n      align-items: center;\r\n      min-height: 28px;\r\n      padding: 4px 9px;\r\n      border-radius: 999px;\r\n      border: 1px solid transparent;\r\n      font-size: 12px;\r\n      font-weight: 600;\r\n      white-space: nowrap;\r\n    }\r\n\r\n    .chip.ok,\r\n    .badge.ok {\r\n      background: var(--ok-soft);\r\n      color: var(--ok);\r\n    }\r\n\r\n    .chip.warn,\r\n    .badge.warn {\r\n      background: var(--warn-soft);\r\n      color: var(--warn);\r\n    }\r\n\r\n    .chip.danger,\r\n    .badge.danger {\r\n      background: var(--danger-soft);\r\n      color: var(--danger);\r\n    }\r\n\r\n    .chip.neutral,\r\n    .badge.neutral {\r\n      background: color-mix(in oklab, var(--surface-3) 88%, white);\r\n      color: var(--ink);\r\n      border-color: color-mix(in oklab, var(--line) 82%, white);\r\n    }\r\n\r\n    .overview-metrics {\r\n      display: grid;\r\n      grid-template-columns: repeat(auto-fit, minmax(92px, 1fr));\n      gap: 10px;\n      margin-top: auto;\r\n    }\r\n\r\n    .overview-metric {\n      min-width: 0;\n      padding-top: 8px;\n      border-top: 1px solid color-mix(in oklab, var(--line) 78%, white);\r\n    }\r\n\r\n    .overview-metric-label {\n      color: var(--muted);\n      font-size: 12px;\n      white-space: nowrap;\n    }\r\n\r\n    .overview-metric-value {\n      font-family: \"Fira Code\", monospace;\n      margin-top: 4px;\n      font-size: 16px;\n      font-weight: 700;\n      font-variant-numeric: tabular-nums;\n      white-space: nowrap;\n      word-break: normal;\n      overflow-wrap: normal;\n    }\n\r\n    .feed-list {\r\n      display: grid;\r\n      gap: 8px;\r\n    }\r\n\r\n    .feed-item {\r\n      padding: 11px 12px;\r\n      border: 1px solid var(--line);\r\n      border-radius: var(--radius);\r\n      background: color-mix(in oklab, var(--surface-2) 88%, white);\r\n      display: grid;\r\n      gap: 4px;\r\n    }\r\n\r\n    .feed-item-head {\r\n      display: flex;\r\n      justify-content: space-between;\r\n      gap: 10px;\r\n      align-items: center;\r\n    }\r\n\r\n    .feed-item strong {\r\n      font-size: 14px;\r\n    }\r\n\r\n    .feed-item .dim {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n    }\r\n\r\n    .feed-item-body {\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    .table-wrap {\n      overflow: auto;\n      border: none;\n      box-shadow: var(--shadow);\n      border-radius: var(--radius-lg);\n      background: var(--surface);\n    }\n\n    .config-editor {\n      display: grid;\n      grid-template-columns: minmax(0, 1fr) minmax(220px, 280px);\n      gap: 12px;\n      align-items: stretch;\n    }\n\n    .json-input {\n      min-height: 220px;\n      resize: vertical;\n      font-family: \"Fira Code\", Consolas, monospace;\n      font-size: 12px;\n      line-height: 1.55;\n      white-space: pre;\n    }\n\n    .editor-aside {\n      display: grid;\n      gap: 10px;\n      align-content: start;\n      padding: 14px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-lg);\n      background: color-mix(in oklab, var(--surface-2) 82%, white);\n    }\n\n    .chart-shell {\n      display: grid;\n      gap: 12px;\n    }\n\n    .trend-chart {\n      min-height: 280px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-lg);\n      background:\n        linear-gradient(180deg, rgba(255,255,255,0.82), rgba(248,250,252,0.92)),\n        repeating-linear-gradient(0deg, transparent 0 45px, color-mix(in oklab, var(--line) 60%, transparent) 46px 47px);\n      overflow: hidden;\n    }\n\n    .trend-chart svg {\n      display: block;\n      width: 100%;\n      min-height: 280px;\n    }\n\n    .trend-line {\n      fill: none;\n      stroke-width: 3;\n      stroke-linecap: round;\n      stroke-linejoin: round;\n    }\n\n    .trend-line.award {\n      stroke: var(--accent);\n    }\n\n    .trend-line.balance {\n      stroke: var(--ok);\n    }\n\n    .trend-dot {\n      stroke: white;\n      stroke-width: 2;\n    }\n\n    .chart-legend {\n      display: flex;\n      flex-wrap: wrap;\n      gap: 8px;\n      color: var(--muted);\n      font-size: 12px;\n    }\n\n    .legend-item {\n      display: inline-flex;\n      align-items: center;\n      gap: 6px;\n    }\n\n    .legend-swatch {\n      width: 22px;\n      height: 3px;\n      border-radius: 999px;\n      background: var(--accent);\n    }\n\n    .legend-swatch.balance {\n      background: var(--ok);\n    }\n\r\n    table {\r\n      font-family: \"Fira Code\", monospace;\r\n      width: 100%;\r\n      min-width: 760px;\r\n      border-collapse: collapse;\r\n      font-size: 13px;\r\n      font-variant-numeric: tabular-nums;\r\n    }\r\n\r\n    th,\r\n    td {\r\n      padding: 8px 10px;\r\n      text-align: left;\r\n      vertical-align: top;\r\n      border-bottom: 1px solid var(--line);\r\n    }\r\n\r\n    th {\r\n      position: sticky;\r\n      top: 0;\r\n      z-index: 1;\r\n      background: color-mix(in oklab, var(--surface) 86%, var(--surface-3));\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n      font-weight: 700;\r\n      letter-spacing: 0.04em;\r\n      text-transform: uppercase;\r\n    }\r\n\r\n    tr:last-child td {\r\n      border-bottom: 0;\r\n    }\r\n\r\n    tbody tr:hover td {\r\n      background: var(--surface-2);\r\n    }\r\n\r\n    .table-wrap.catalog-wrap table {\r\n      font-family: \"Fira Code\", monospace;\r\n      min-width: 980px;\r\n    }\r\n\r\n    .catalog-row td {\r\n      padding-top: 13px;\r\n      padding-bottom: 13px;\r\n    }\r\n\r\n    .catalog-row.site-start td {\r\n      border-top: 1px solid color-mix(in oklab, var(--line-strong) 72%, white);\r\n    }\r\n\r\n    .site-block {\r\n      min-width: 190px;\r\n      display: grid;\r\n      gap: 6px;\r\n    }\r\n\r\n    .site-top {\r\n      display: flex;\r\n      align-items: center;\r\n      gap: 8px;\r\n      flex-wrap: wrap;\r\n    }\r\n\r\n    .site-top strong {\r\n      font-family: \"Fira Sans\", \"Microsoft YaHei\", sans-serif; font-weight: 600;\r\n      font-size: 17px;\r\n      line-height: 1;\r\n    }\r\n\r\n    .site-url {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n      word-break: break-all;\r\n    }\r\n\r\n    .site-note {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n    }\r\n\r\n    .site-repeat {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n    }\r\n\r\n    .identity {\r\n      display: grid;\r\n      gap: 3px;\r\n    }\r\n\r\n    .identity strong {\r\n      font-weight: 700;\r\n      line-height: 1.15;\r\n    }\r\n\r\n    .identity .subline,\r\n    .dim {\r\n      color: var(--muted);\r\n      font-size: 12px;\r\n    }\r\n\r\n    .route-badge {\n      display: inline-flex;\r\n      min-height: 26px;\r\n      align-items: center;\r\n      padding: 4px 9px;\r\n      border-radius: 999px;\r\n      background: color-mix(in oklab, var(--surface-3) 90%, white);\r\n      color: var(--ink);\r\n      border: 1px solid color-mix(in oklab, var(--line) 82%, white);\r\n      font-size: 12px;\r\n      font-weight: 600;\r\n      white-space: nowrap;\r\n    }\r\n\r\n    .quick-action {\r\n      min-height: 34px;\r\n      padding: 6px 11px;\r\n      border-radius: 999px;\r\n      background: color-mix(in oklab, var(--accent-soft) 66%, white);\r\n      border-color: color-mix(in oklab, var(--accent) 22%, var(--line));\r\n      color: var(--accent-strong);\r\n      font-size: 12px;\r\n      font-weight: 700;\r\n    }\r\n\r\n    .quick-action.secondary {\n      background: color-mix(in oklab, var(--surface-2) 88%, white);\n      border-color: var(--line);\n      color: var(--ink);\n    }\n\n    .access-key-cell {\n      display: flex;\n      align-items: flex-start;\n      gap: 6px;\n      min-width: 190px;\n      max-width: 380px;\n    }\n\n    .access-key-cell code {\n      display: block;\n      min-width: 0;\n      max-width: 310px;\n      white-space: normal;\n      overflow-wrap: anywhere;\n      user-select: all;\n    }\n\n    .access-key-cell .quick-action {\n      min-height: 28px;\n      flex: 0 0 auto;\n      padding: 3px 8px;\n    }\n\n    .model-list {\n      max-width: 360px;\n      margin-top: 6px;\n      color: var(--muted);\n      line-height: 1.65;\n      white-space: normal;\n      overflow-wrap: anywhere;\n    }\n\n    #balanceAccountTable table {\n      min-width: 980px;\n    }\n\n    #balanceAccountTable td {\n      vertical-align: top;\n    }\n\n    .cell-stack {\n      display: grid;\n      gap: 4px;\n      min-width: 120px;\n      white-space: normal;\n    }\n\n    .quick-action.danger {\n      background: color-mix(in oklab, var(--danger-soft) 76%, white);\n      border-color: color-mix(in oklab, var(--danger) 28%, var(--line));\n      color: var(--danger);\n    }\n\n    .action-cluster {\n      display: grid;\n      grid-template-columns: repeat(auto-fit, minmax(86px, 1fr));\n      gap: 6px;\n      min-width: 0;\n    }\n\r\n    .table-summary {\r\n      display: flex;\r\n      flex-wrap: wrap;\r\n      gap: 8px;\r\n      margin-bottom: 12px;\r\n    }\r\n\r\n    .empty {\r\n      padding: 18px 14px;\r\n      color: var(--muted);\r\n      font-size: 14px;\r\n    }\r\n\r\n    .inline-note {\r\n      color: var(--muted);\r\n      font-size: 13px;\r\n    }\r\n\r\n    @media (max-width: 1400px) {\n      .summary-strip {\n        grid-template-columns: repeat(3, minmax(0, 1fr));\n      }\n\r\n      .view-grid-2 {\n        grid-template-columns: 1fr;\n      }\n    }\n\n    @media (max-width: 1180px) {\n      .site-actions,\n      .action-cluster {\n        grid-template-columns: repeat(2, minmax(0, 1fr));\n      }\n    }\n\n    @media (max-width: 1240px) {\n      .workspace {\n        grid-template-columns: 1fr;\n      }\n\n      .sidebar {\n        position: static;\n      }\n\n      .overview-site-grid,\n      .overview-balance-grid {\n        grid-template-columns: 1fr;\n      }\n\n      .config-editor {\n        grid-template-columns: 1fr;\n      }\n    }\n\r\n    @media (max-width: 820px) {\r\n      .shell {\r\n        padding: 18px 14px 24px;\r\n      }\r\n\r\n      .header {\r\n        grid-template-columns: 1fr;\r\n      }\r\n\r\n      .toolbar {\r\n        justify-content: stretch;\r\n      }\r\n\r\n      .toolbar button {\r\n        flex: 1 1 auto;\r\n      }\r\n\r\n      .summary-strip {\n        grid-template-columns: repeat(2, minmax(0, 1fr));\n      }\n\n      .overview-metrics {\n        grid-template-columns: 1fr 1fr;\n      }\n    }\n\n    @media (max-width: 560px) {\n      .site-actions,\n      .action-cluster {\n        grid-template-columns: 1fr;\n      }\n\n      .summary-strip {\n        grid-template-columns: 1fr;\n      }\n\n      .overview-metrics {\n        grid-template-columns: 1fr;\n      }\n\n      .directory-trigger {\n        width: 100%;\n        margin-left: 0;\n      }\n\n      .directory-dialog {\n        width: 100vw;\n        max-height: 100vh;\n        border-radius: 0;\n      }\n\n      .directory-dialog-shell {\n        max-height: 100vh;\n      }\n    }\n\n    @media (prefers-reduced-motion: reduce) {\n      *,\n      *::before,\n      *::after {\r\n        scroll-behavior: auto !important;\n        transition: none !important;\n      }\n    }\n  "
export const newapiCheckinLegacyBodyHtml = "\r\n  <div class=\"shell\">\r\n    <header class=\"header\">\r\n      <div>\r\n        <div class=\"eyebrow\"><span class=\"eyebrow-dot\"></span> Personal Operations Console</div>\r\n        <h1>NewApi Checkin Dashboard</h1>\r\n        <p class=\"header-copy\">页面默认只读 sub2api SQL 存储；需要更新远端数据时，按站点或账号手动刷新。这样打开很快，也尽量减少同一站点的连续请求。</p>\n      </div>\r\n    </header>\r\n\r\n    <section class=\"summary-strip\" id=\"statsGrid\">\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">启用站点</div>\r\n        <div class=\"metric-value\">-</div>\r\n        <div class=\"metric-note\">当前配置</div>\r\n      </div>\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">启用账号</div>\r\n        <div class=\"metric-value\">-</div>\r\n        <div class=\"metric-note\">今日轮询范围</div>\r\n      </div>\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">最近成功</div>\r\n        <div class=\"metric-value\">-</div>\r\n        <div class=\"metric-note\">上次收口结果</div>\r\n      </div>\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">今日已签</div>\r\n        <div class=\"metric-value\">-</div>\r\n        <div class=\"metric-note\">无需补跑</div>\r\n      </div>\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">最近奖励</div>\r\n        <div class=\"metric-value\">-</div>\r\n        <div class=\"metric-note\">上次累计奖励</div>\r\n      </div>\r\n      <div class=\"metric-card\">\n        <div class=\"metric-label\">运行状态</div>\r\n        <div class=\"metric-value\" id=\"statusInline\">-</div>\r\n        <div class=\"metric-note\" id=\"statusSubline\">等待加载</div>\r\n      </div>\r\n    </section>\r\n\r\n    <div class=\"workspace\">\r\n      <aside class=\"sidebar\">\r\n        <section class=\"sidebar-section\">\r\n          <div class=\"panel-head\" style=\"margin-bottom: 0;\">\r\n            <div>\r\n              <h2 class=\"panel-title\">控制台</h2>\r\n              <div class=\"panel-meta\">补签动作和页面状态</div>\r\n            </div>\r\n          </div>\r\n          <div class=\"status-box\" id=\"appStatus\" data-tone=\"normal\">准备加载…</div>\r\n        </section>\r\n\r\n        <section class=\"sidebar-section\">\r\n          <div class=\"field-stack\">\r\n            <label>\r\n              站点\r\n              <select id=\"siteSelect\"></select>\r\n            </label>\r\n            <label>\r\n              账号\r\n              <select id=\"accountSelect\"></select>\r\n            </label>\r\n          </div>\r\n          <div class=\"toolbar\">\n            <button class=\"primary\" id=\"singleRunBtn\">执行单账号签到</button>\n            <button class=\"secondary\" id=\"fullRunBtn\">后台完整签到</button>\n          </div>\n          <div class=\"target-card\" id=\"selectedTargetCard\">\n            <div class=\"target-title\">当前补签目标</div>\r\n            <div class=\"target-main\">\r\n              <strong>-</strong>\r\n              <div class=\"target-meta\">等待选择账号</div>\r\n            </div>\r\n          </div>\n          <div class=\"status-box\" id=\"singleRunStatus\" data-tone=\"normal\">等待执行。</div>\n          <div class=\"status-box\" id=\"fullRunStatus\" data-tone=\"normal\">完整签到等待触发。</div>\n        </section>\n\r\n        <details open>\r\n          <summary>查看单账号脚本输出</summary>\r\n          <pre id=\"singleRunOutput\"></pre>\r\n        </details>\r\n\r\n        <section class=\"note-list\">\r\n          <div>名称同步会把站点返回的 <code>display_name</code>、<code>username</code> 写回配置，同时把展示名称一起更新。</div>\r\n          <div>“最近一次签到”区域展示的是本地最终收口结果；如果做过补跑，结果会按补跑后的最新状态呈现。</div>\r\n          <div>余额默认来自 sub2api SQL 存储；只有点击站点或账号级刷新按钮时，才会请求远端并写回余额与签到状态。</div>\n        </section>\r\n      </aside>\r\n\r\n      <main class=\"main\">\r\n        <section class=\"viewbar\">\r\n          <div class=\"tab-row\" id=\"tabRow\">\n            <button class=\"tab-button\" type=\"button\" data-tab=\"overview\" aria-selected=\"true\">总览</button>\n            <button class=\"tab-button\" type=\"button\" data-tab=\"history\" aria-selected=\"false\">签到记录</button>\n            <button class=\"tab-button\" type=\"button\" data-tab=\"balances\" aria-selected=\"false\">实时余额</button>\n            <button class=\"tab-button\" type=\"button\" data-tab=\"monthly\" aria-selected=\"false\">月度历史</button>\n            <button class=\"secondary directory-trigger\" id=\"openPlatformDirectoryBtn\" type=\"button\">平台目录</button>\n          </div>\n          <div class=\"filter-row\">\r\n            <div class=\"filter-row\" id=\"siteFilterRow\"></div>\r\n            <div class=\"filter-meta\" id=\"activeFilterMeta\">按全部站点查看</div>\r\n          </div>\r\n          <div class=\"site-actions\">\r\n            <button class=\"secondary\" id=\"reloadLocalBtn\">读取 SQL 数据</button>\n            <button class=\"primary\" id=\"refreshSiteBalanceBtn\">刷新当前站点余额/签到</button>\n            <button class=\"secondary\" id=\"syncSiteNamesBtn\">同步当前站点名称</button>\r\n          </div>\r\n        </section>\r\n\r\n        <section class=\"view-panel\" data-tab-panel=\"overview\">\r\n          <div class=\"view-grid-2\">\r\n            <section class=\"panel\">\n              <div class=\"panel-head\">\r\n                <div>\r\n                  <h2 class=\"panel-title\">今日签到收口</h2>\r\n                  <div class=\"panel-meta\" id=\"overviewRunMeta\">等待最近一次结果</div>\r\n                </div>\r\n                <div class=\"panel-meta\">按站点看执行强弱</div>\r\n              </div>\r\n              <div class=\"overview-site-grid\" id=\"overviewRunGrid\"></div>\r\n            </section>\r\n\r\n            <section class=\"panel\">\n              <div class=\"panel-head\">\r\n                <div>\r\n                  <h2 class=\"panel-title\">平台余额总览</h2>\r\n                  <div class=\"panel-meta\" id=\"overviewBalanceMeta\">等待实时余额</div>\r\n                </div>\r\n                <div class=\"panel-meta\">来自 sub2api SQL 存储</div>\n              </div>\r\n              <div class=\"overview-balance-grid\" id=\"overviewBalanceGrid\"></div>\r\n            </section>\r\n          </div>\r\n\r\n          <div class=\"view-grid-2\">\r\n            <section class=\"panel\">\n              <div class=\"panel-head\">\r\n                <div>\r\n                  <h2 class=\"panel-title\">关注项</h2>\r\n                  <div class=\"panel-meta\">失败、已签和补跑相关提示</div>\r\n                </div>\r\n              </div>\r\n              <div class=\"feed-list\" id=\"overviewFeed\"></div>\r\n            </section>\r\n\r\n            <section class=\"panel\">\n              <div class=\"panel-head\">\r\n                <div>\r\n                  <h2 class=\"panel-title\">当前筛选摘要</h2>\r\n                  <div class=\"panel-meta\">站点切换后，这里会跟着收窄</div>\r\n                </div>\r\n              </div>\r\n              <div class=\"feed-list\" id=\"overviewContextFeed\"></div>\r\n            </section>\r\n          </div>\r\n        </section>\r\n\r\n        <section class=\"view-panel\" data-tab-panel=\"history\" hidden>\n          <section class=\"panel\">\n            <div class=\"panel-head\">\r\n              <div>\r\n                <h2 class=\"panel-title\">最近一次全量签到</h2>\r\n                <div class=\"panel-meta\" id=\"lastRunMeta\">尚未读取</div>\r\n              </div>\r\n              <div class=\"panel-meta\">执行结果与当时余额快照</div>\r\n            </div>\r\n            <div class=\"badge-row\" id=\"lastRunBadges\"></div>\r\n          </section>\r\n\r\n          <section class=\"panel\">\n            <div class=\"panel-head\">\r\n              <div>\r\n                <h2 class=\"panel-title\">站点汇总</h2>\r\n                <div class=\"panel-meta\">成功、已签、失败和奖励折算</div>\r\n              </div>\r\n            </div>\r\n            <div class=\"table-wrap\" id=\"lastRunSiteTable\"></div>\r\n          </section>\r\n\r\n          <section class=\"panel\">\n            <div class=\"panel-head\">\r\n              <div>\r\n                <h2 class=\"panel-title\">账号结果</h2>\r\n                <div class=\"panel-meta\">按当前站点筛选展示</div>\r\n              </div>\r\n            </div>\r\n            <div class=\"table-wrap\" id=\"lastRunAccountTable\"></div>\r\n          </section>\r\n\r\n          <section class=\"panel\">\n            <div class=\"panel-head\">\r\n              <div>\r\n                <h2 class=\"panel-title\">原始摘要</h2>\r\n                <div class=\"panel-meta\">保留脚本输出文本，方便复制</div>\r\n              </div>\r\n            </div>\r\n            <pre id=\"lastRunSummaryText\"></pre>\r\n          </section>\r\n        </section>\r\n\r\n        <section class=\"view-panel\" data-tab-panel=\"balances\" hidden>\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">实时余额与额度</h2>\n                <div class=\"panel-meta\" id=\"balanceMeta\">尚未读取 SQL 存储</div>\n              </div>\r\n              <div class=\"panel-meta\">手动刷新后写回 SQL</div>\n            </div>\r\n            <div class=\"badge-row\" id=\"balanceOverallBadges\"></div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">每日变化趋势</h2>\n                <div class=\"panel-meta\" id=\"trendMeta\">等待本地历史</div>\n              </div>\n              <div class=\"field-stack\" style=\"min-width: 260px; max-width: 420px;\">\n                <label>\n                  趋势范围\n                  <select id=\"trendScopeSelect\"></select>\n                </label>\n                <div class=\"chart-legend\">\n                  <span class=\"legend-item\"><span class=\"legend-swatch\"></span>每日签到新增</span>\n                  <span class=\"legend-item\"><span class=\"legend-swatch balance\"></span>当日总余额</span>\n                </div>\n              </div>\n            </div>\n            <div class=\"chart-shell\">\n              <div class=\"trend-chart\" id=\"trendChart\"></div>\n              <div class=\"table-wrap\" id=\"trendTable\"></div>\n            </div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\r\n                <h2 class=\"panel-title\">站点额度汇总</h2>\r\n                <div class=\"panel-meta\">适合先看平台层面的总余额</div>\r\n              </div>\r\n            </div>\r\n            <div class=\"table-wrap\" id=\"balanceSiteTable\"></div>\r\n          </section>\r\n\r\n          <section class=\"panel\">\n            <div class=\"panel-head\">\r\n              <div>\r\n                <h2 class=\"panel-title\">账号余额明细</h2>\r\n                <div class=\"panel-meta\">按当前站点筛选展示</div>\r\n              </div>\r\n            </div>\r\n            <div class=\"table-wrap\" id=\"balanceAccountTable\"></div>\n          </section>\n        </section>\n\n        <section class=\"view-panel\" data-tab-panel=\"monthly\" hidden>\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">月度签到缓存</h2>\n                <div class=\"panel-meta\" id=\"monthlyMeta\">读取 sub2api SQL 存储，不主动请求远端</div>\n              </div>\n              <div class=\"field-stack\" style=\"min-width: 260px; max-width: 460px;\">\n                <label>\n                  历史月份\n                  <input id=\"monthlyMonthInput\" type=\"month\">\n                </label>\n                <label>\n                  签到趋势范围\n                  <select id=\"monthlyTrendScopeSelect\"></select>\n                </label>\n              </div>\n            </div>\n            <div class=\"site-actions\">\n              <button class=\"secondary\" id=\"loadMonthlyBtn\">读取本地月份</button>\n              <button class=\"primary\" id=\"syncMonthlySiteBtn\">同步当前站点月份</button>\n              <button class=\"secondary\" id=\"syncMonthlyAccountBtn\">同步当前账号月份</button>\n            </div>\n            <div class=\"status-box\" id=\"monthlySyncStatus\" data-tone=\"normal\">等待读取月度缓存。</div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">每日签到奖励趋势</h2>\n                <div class=\"panel-meta\" id=\"monthlyTrendMeta\">按 SQL 月度记录统计每日签到奖励</div>\n              </div>\n              <div class=\"chart-legend\">\n                <span class=\"legend-item\"><span class=\"legend-swatch\"></span>每日签到奖励($)</span>\n              </div>\n            </div>\n            <div class=\"chart-shell\">\n              <div class=\"trend-chart\" id=\"monthlyTrendChart\"></div>\n              <div class=\"table-wrap\" id=\"monthlyTrendTable\"></div>\n            </div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">平台月度汇总</h2>\n                <div class=\"panel-meta\">按站点统计当月已缓存签到奖励</div>\n              </div>\n            </div>\n            <div class=\"table-wrap\" id=\"monthlySiteTable\"></div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">账号月度汇总</h2>\n                <div class=\"panel-meta\">按账号统计当月签到天数和奖励</div>\n              </div>\n            </div>\n            <div class=\"table-wrap\" id=\"monthlyAccountTable\"></div>\n          </section>\n\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">月度明细</h2>\n                <div class=\"panel-meta\">每条记录来自 /api/user/checkin?month=YYYY-MM 后写入 SQL</div>\n              </div>\n            </div>\n            <div class=\"table-wrap\" id=\"monthlyRecordsTable\"></div>\n          </section>\n        </section>\n      </main>\n    </div>\n  </div>\n\n  <dialog class=\"directory-dialog\" id=\"platformDirectoryDialog\" aria-labelledby=\"platformDirectoryTitle\">\n    <div class=\"directory-dialog-shell\">\n      <div class=\"directory-dialog-head\">\n        <div>\n          <h2 class=\"panel-title\" id=\"platformDirectoryTitle\">平台目录</h2>\n          <div class=\"panel-meta\">集中管理站点、账号、API Key 和分组</div>\n        </div>\n        <div class=\"toolbar\">\n          <button class=\"secondary\" id=\"refreshDirectoryKeysBtn\" type=\"button\">重新读取 Key / 分组</button>\n          <button class=\"secondary\" id=\"closePlatformDirectoryBtn\" type=\"button\">关闭</button>\n        </div>\n      </div>\n      <div class=\"directory-dialog-body\">\n        <section class=\"panel\">\n          <div class=\"panel-head\">\n            <div>\n              <h2 class=\"panel-title\">新增平台 / 账号</h2>\n              <div class=\"panel-meta\">粘贴平台或账号 JSON 配置片段；同名平台会按 user_id 合并账号</div>\n            </div>\n            <button class=\"primary\" id=\"saveConfigJsonBtn\" type=\"button\">写入配置</button>\n          </div>\n          <div class=\"config-editor\">\n            <textarea class=\"json-input\" id=\"configJsonInput\" spellcheck=\"false\"></textarea>\n            <div class=\"editor-aside\">\n              <div class=\"badge-row\">\n                <span class=\"badge neutral\">JSON</span>\n                <span class=\"badge neutral\">明文 key</span>\n              </div>\n              <div class=\"inline-note\">可以粘贴单个平台对象，也可以粘贴 <code>{\"sites\":[...]}</code>。字段格式沿用 NewApi 签到配置。</div>\n              <div class=\"status-box\" id=\"configJsonStatus\" data-tone=\"normal\">等待输入。</div>\n            </div>\n          </div>\n        </section>\n\n        <section class=\"panel\">\n          <div class=\"panel-head\">\n            <div>\n              <h2 class=\"panel-title\">站点与账号</h2>\n              <div class=\"panel-meta\" id=\"configMeta\">等待配置</div>\n            </div>\n            <div class=\"panel-meta\">NewAPI 可按需显示生成 Key 并更新 token 分组；sub2api 需要账号登录态</div>\n          </div>\n          <div class=\"table-summary\" id=\"configSummaryBadges\"></div>\n          <div class=\"table-wrap catalog-wrap\" id=\"configTableWrap\"></div>\n        </section>\n      </div>\n    </div>\n  </dialog>\n\n  <dialog class=\"directory-dialog credential-dialog\" id=\"loginCredentialDialog\" aria-labelledby=\"loginCredentialTitle\">\n    <div class=\"directory-dialog-shell\">\n      <div class=\"directory-dialog-head\">\n        <div>\n          <h2 class=\"panel-title\" id=\"loginCredentialTitle\">登录凭据</h2>\n          <div class=\"panel-meta\">用于读取和修改 sub2api API Key 分组</div>\n        </div>\n        <button class=\"secondary\" id=\"closeLoginCredentialBtn\" type=\"button\">关闭</button>\n      </div>\n      <form class=\"credential-form\" id=\"loginCredentialForm\">\n        <div class=\"credential-target\" id=\"loginCredentialTarget\">等待选择账号</div>\n        <label>\n          登录账号 / 邮箱\n          <input id=\"loginUsernameInput\" type=\"email\" autocomplete=\"username\" required>\n        </label>\n        <label>\n          登录密码\n          <input id=\"loginPasswordInput\" type=\"password\" autocomplete=\"current-password\" placeholder=\"留空则使用已保存密码\">\n        </label>\n        <div class=\"status-box\" id=\"loginCredentialStatus\" data-tone=\"normal\">密码不会在页面中回显。</div>\n        <div class=\"toolbar\">\n          <button class=\"secondary\" id=\"testLoginCredentialBtn\" type=\"button\">测试登录</button>\n          <button class=\"primary\" id=\"saveLoginCredentialBtn\" type=\"submit\">验证并保存</button>\n        </div>\n      </form>\n    </div>\n  </dialog>\n\n  "

export function mountNewapiCheckinLegacyTool(scope: LegacyToolScope): LegacyToolCleanup {
  const document = scope.document
  const window = scope.window
  const localStorage = scope.localStorage
  const fetch = scope.fetch
  const Headers = scope.Headers
  const CSS = scope.CSS
  const structuredClone = scope.structuredClone
  const console = scope.console

    const state = {
      config: null,
      apiKeys: null,
      lastRun: null,
      balances: null,
      history: null,
      monthly: null,
      activeSite: "all",
      trendScope: "all",
      monthlyTrendScope: "all",
      monthlyStatusTimer: null,
      checkinStatusTimer: null,
      activeTab: "overview",
      revealedAccessKeys: new Set(),
      revealedGeneratedKeys: new Map(),
      loginCredentialTarget: null,
    };

    const statsGrid = document.getElementById("statsGrid");
    const statusInline = document.getElementById("statusInline");
    const statusSubline = document.getElementById("statusSubline");
    const appStatus = document.getElementById("appStatus");
    const activeFilterMeta = document.getElementById("activeFilterMeta");
    const siteFilterRow = document.getElementById("siteFilterRow");
    const tabRow = document.getElementById("tabRow");
    const openPlatformDirectoryBtn = document.getElementById("openPlatformDirectoryBtn");
    const closePlatformDirectoryBtn = document.getElementById("closePlatformDirectoryBtn");
    const refreshDirectoryKeysBtn = document.getElementById("refreshDirectoryKeysBtn");
    const platformDirectoryDialog = document.getElementById("platformDirectoryDialog");
    const loginCredentialDialog = document.getElementById("loginCredentialDialog");
    const closeLoginCredentialBtn = document.getElementById("closeLoginCredentialBtn");
    const loginCredentialForm = document.getElementById("loginCredentialForm");
    const loginCredentialTarget = document.getElementById("loginCredentialTarget");
    const loginUsernameInput = document.getElementById("loginUsernameInput");
    const loginPasswordInput = document.getElementById("loginPasswordInput");
    const loginCredentialStatus = document.getElementById("loginCredentialStatus");
    const testLoginCredentialBtn = document.getElementById("testLoginCredentialBtn");
    const saveLoginCredentialBtn = document.getElementById("saveLoginCredentialBtn");

    const siteSelect = document.getElementById("siteSelect");
    const accountSelect = document.getElementById("accountSelect");
    const singleRunBtn = document.getElementById("singleRunBtn");
    const fullRunBtn = document.getElementById("fullRunBtn");
    const singleRunStatus = document.getElementById("singleRunStatus");
    const fullRunStatus = document.getElementById("fullRunStatus");
    const singleRunOutput = document.getElementById("singleRunOutput");
    const selectedTargetCard = document.getElementById("selectedTargetCard");

    const reloadLocalBtn = document.getElementById("reloadLocalBtn");
    const refreshSiteBalanceBtn = document.getElementById("refreshSiteBalanceBtn");
    const syncSiteNamesBtn = document.getElementById("syncSiteNamesBtn");

    const overviewRunMeta = document.getElementById("overviewRunMeta");
    const overviewRunGrid = document.getElementById("overviewRunGrid");
    const overviewBalanceMeta = document.getElementById("overviewBalanceMeta");
    const overviewBalanceGrid = document.getElementById("overviewBalanceGrid");
    const overviewFeed = document.getElementById("overviewFeed");
    const overviewContextFeed = document.getElementById("overviewContextFeed");

    const configMeta = document.getElementById("configMeta");
    const configSummaryBadges = document.getElementById("configSummaryBadges");
    const configTableWrap = document.getElementById("configTableWrap");
    const configJsonInput = document.getElementById("configJsonInput");
    const saveConfigJsonBtn = document.getElementById("saveConfigJsonBtn");
    const configJsonStatus = document.getElementById("configJsonStatus");

    const lastRunMeta = document.getElementById("lastRunMeta");
    const lastRunBadges = document.getElementById("lastRunBadges");
    const lastRunSiteTable = document.getElementById("lastRunSiteTable");
    const lastRunAccountTable = document.getElementById("lastRunAccountTable");
    const lastRunSummaryText = document.getElementById("lastRunSummaryText");

    const balanceMeta = document.getElementById("balanceMeta");
    const balanceOverallBadges = document.getElementById("balanceOverallBadges");
    const balanceSiteTable = document.getElementById("balanceSiteTable");
    const balanceAccountTable = document.getElementById("balanceAccountTable");
    const trendMeta = document.getElementById("trendMeta");
    const trendChart = document.getElementById("trendChart");
    const trendTable = document.getElementById("trendTable");
    const trendScopeSelect = document.getElementById("trendScopeSelect");
    const monthlyMeta = document.getElementById("monthlyMeta");
    const monthlyMonthInput = document.getElementById("monthlyMonthInput");
    const monthlyTrendScopeSelect = document.getElementById("monthlyTrendScopeSelect");
    const loadMonthlyBtn = document.getElementById("loadMonthlyBtn");
    const syncMonthlySiteBtn = document.getElementById("syncMonthlySiteBtn");
    const syncMonthlyAccountBtn = document.getElementById("syncMonthlyAccountBtn");
    const monthlySyncStatus = document.getElementById("monthlySyncStatus");
    const monthlyTrendMeta = document.getElementById("monthlyTrendMeta");
    const monthlyTrendChart = document.getElementById("monthlyTrendChart");
    const monthlyTrendTable = document.getElementById("monthlyTrendTable");
    const monthlySiteTable = document.getElementById("monthlySiteTable");
    const monthlyAccountTable = document.getElementById("monthlyAccountTable");
    const monthlyRecordsTable = document.getElementById("monthlyRecordsTable");

    function escapeHtml(value) {
      let str = String(value ?? "");
      return str
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;");
    }

    function preferredName(record) {
      return record?.display_name || record?.username || record?.name || record?.account || record?.account_name || record?.user_id || "-";
    }

    function getConfigSites() {
      return state.config?.sites || [];
    }

    function getEnabledSites() {
      return getConfigSites().filter((site) => site.enabled);
    }

    function isSiteEnabled(siteName) {
      const site = getConfigSite(siteName);
      return Boolean(site?.enabled);
    }

    function getConfigSite(siteName) {
      return getConfigSites().find((site) => site.name === siteName) || null;
    }

    function siteProvider(site) {
      return site?.provider === "sub2api" ? "sub2api" : "newapi";
    }

    function isReadOnlySite(siteOrName) {
      const site = typeof siteOrName === "string" ? getConfigSite(siteOrName) : siteOrName;
      return siteProvider(site) === "sub2api";
    }

    function getConfigAccount(siteName, userId) {
      const site = getConfigSite(siteName);
      return site?.accounts?.find((item) => String(item.user_id) === String(userId)) || null;
    }

    function getActiveSiteLabel() {
      return state.activeSite === "all" ? "全部站点" : state.activeSite;
    }

    function getTrendScopeLabel() {
      if (state.trendScope === "all") {
        return "总余额";
      }
      if (state.trendScope.startsWith("site:")) {
        return state.trendScope.slice(5);
      }
      if (state.trendScope.startsWith("account:")) {
        const [, site, userId] = state.trendScope.split(":");
        const account = getConfigAccount(site, userId);
        return `${site} / ${preferredName(account || { user_id: userId })}`;
      }
      return state.trendScope;
    }

    function currentMonthValue() {
      const now = new Date();
      return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
    }

    function getMonthlyTrendScopeLabel() {
      if (state.monthlyTrendScope === "all") {
        return "全部站点";
      }
      if (state.monthlyTrendScope.startsWith("site:")) {
        return state.monthlyTrendScope.slice(5);
      }
      if (state.monthlyTrendScope.startsWith("account:")) {
        const [, site, userId] = state.monthlyTrendScope.split(":");
        const account = getConfigAccount(site, userId);
        return `${site} / ${preferredName(account || { user_id: userId })}`;
      }
      return state.monthlyTrendScope;
    }

    function matchesActiveSite(row) {
      return state.activeSite === "all" || row.site === state.activeSite;
    }

    function balanceTrendScopeForActiveSite() {
      return state.activeSite === "all" ? "all" : `site:${state.activeSite}`;
    }

    function syncBalanceTrendToActiveSite() {
      state.trendScope = balanceTrendScopeForActiveSite();
    }

    function renderTrendScopeSelector(history) {
      const scopedSiteNames = (history?.site_summaries || [])
        .filter(matchesActiveSite)
        .map((row) => row.site)
        .filter(Boolean);
      const siteNames = state.activeSite === "all"
        ? Array.from(new Set(scopedSiteNames))
        : [state.activeSite];
      const accountItems = Array.from(new Map((history?.account_summaries || [])
        .filter(matchesActiveSite)
        .map((row) => [`${row.site}:${row.user_id}`, row])
      ).values());
      const baseOptions = state.activeSite === "all" ? [{ value: "all", label: "总余额" }] : [];
      const options = baseOptions.concat(
        siteNames.map((siteName) => ({ value: `site:${siteName}`, label: `${siteName} 余额` })),
        accountItems.map((row) => ({ value: `account:${row.site}:${row.user_id}`, label: `${row.site} / ${preferredName(row)} 余额` })),
      );

      trendScopeSelect.innerHTML = options.map((item) => `
        <option value="${escapeHtml(item.value)}">${escapeHtml(item.label)}</option>
      `).join("");

      if (!options.some((item) => item.value === state.trendScope)) {
        state.trendScope = "all";
      }
      trendScopeSelect.value = state.trendScope;
    }

    function formatDisplayValue(value) {
      if (value === null || value === undefined || value === "") {
        return "-";
      }

      return String(value)
        .split("|")
        .map((part) => {
          const trimmed = part.trim();
          if (!trimmed || trimmed === "-") {
            return "-";
          }

          if (/^[-+]?\d+(\.\d+)?$/.test(trimmed)) {
            return Number(trimmed).toFixed(2);
          }

          const match = trimmed.match(/^([^0-9+\-]*)([-+]?\d+(\.\d+)?)(.*)$/);
          if (!match) {
            return trimmed;
          }

          const [, prefix, amount, , suffix] = match;
          const numeric = Number(amount);
          if (!Number.isFinite(numeric)) {
            return trimmed;
          }
          return `${prefix}${numeric.toFixed(2)}${suffix}`.trim();
        })
        .join(" | ");
    }

    function formatResult(row) {
      if (row?.checkin_status) {
        return row.checkin_status;
      }
      if (row?.checked_in_today) {
        return "今日已签到";
      }
      if (row?.ok === false) {
        return "失败";
      }
      if (row?.success) {
        return "签到成功";
      }
      return row?.message || "失败";
    }

    function resultTone(row) {
      if (row?.checkin_status_tone) {
        return row.checkin_status_tone;
      }
      if (row?.checked_in_today) {
        return "warn";
      }
      if (row?.ok === false) {
        return "danger";
      }
      if (row?.success) {
        return "ok";
      }
      if ((row?.message || "").includes("已签到")) {
        return "warn";
      }
      return "neutral";
    }

    function statusChip(text, tone = "neutral") {
      return `<span class="chip ${tone}">${escapeHtml(text)}</span>`;
    }

    function badge(text, tone = "neutral") {
      return `<span class="badge ${tone}">${escapeHtml(text)}</span>`;
    }

    function identityHtml(record) {
      const name = preferredName(record);
      const secondary = [];
      if (record?.username && record.username !== name) {
        secondary.push(`@${record.username}`);
      }
      if (record?.user_id) {
        secondary.push(`ID ${record.user_id}`);
      }
      if (record?.email) {
        secondary.push(record.email);
      }
      if (record?.login_username) {
        secondary.push(`登录 ${record.login_username}`);
      }
      if (record?.has_login_password) {
        secondary.push("密码已保存");
      }
      return `
        <div class="identity">
          <strong>${escapeHtml(name)}</strong>
          <div class="subline">${escapeHtml(secondary.join(" · ") || "未记录更多标识")}</div>
        </div>
      `;
    }

    function renderTable(container, columns, rows, rowClassName = "") {
      if (!rows?.length) {
        container.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      const head = columns.map((column) => `<th>${escapeHtml(column.label)}</th>`).join("");
      const body = rows.map((row) => {
        const cells = columns.map((column) => {
          const value = typeof column.value === "function" ? column.value(row) : row[column.value];
          return `<td>${value ?? "-"}</td>`;
        }).join("");
        const className = [rowClassName, typeof row._rowClass === "string" ? row._rowClass : ""].filter(Boolean).join(" ");
        const dataAttrs = [];
        if (row.site) {
          dataAttrs.push(`data-site="${escapeHtml(row.site)}"`);
        }
        if (row.user_id) {
          dataAttrs.push(`data-user-id="${escapeHtml(row.user_id)}"`);
        }
        return `<tr class="${className}" ${dataAttrs.join(" ")}>${cells}</tr>`;
      }).join("");

      container.innerHTML = `<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`;
    }

    function setAppStatus(text, tone = "normal") {
      appStatus.textContent = text;
      appStatus.dataset.tone = tone;
      appStatus.classList.toggle("is-pending", tone === "busy");
      statusInline.textContent = tone === "error" ? "异常" : tone === "busy" ? "处理中" : "就绪";
      statusSubline.textContent = text;
    }

    function setInlineStatus(node, text, tone = "normal") {
      if (!node) {
        return;
      }
      node.textContent = text;
      node.dataset.tone = tone;
      node.classList.toggle("is-pending", tone === "busy");
    }

    function defaultConfigJsonText() {
      return JSON.stringify({
        name: "new-platform",
        provider: "newapi",
        enabled: true,
        base_url: "https://example.com",
        accounts: [
          {
            name: "account-name",
            user_id: "1001",
            access_key: "paste-key-here",
            ip_profile: "ip-slot-29"
          }
        ]
      }, null, 2);
    }

    function setButtonBusy(button, busy, label) {
      if (!button) {
        return;
      }
      if (busy) {
        if (button.dataset.originalLabel === undefined) {
          button.dataset.originalLabel = button.textContent;
        }
        button.dataset.busy = "true";
        button.disabled = true;
        if (label) {
          button.textContent = label;
        }
      } else {
        button.removeAttribute("data-busy");
        button.disabled = false;
        if (button.dataset.originalLabel !== undefined) {
          button.textContent = button.dataset.originalLabel;
        }
      }
    }

    function pulseRow(site, userId, tone) {
      if (!window.CSS || typeof window.CSS.escape !== "function") {
        return;
      }
      const siteSelector = CSS.escape(site);
      const userSelector = userId ? `[data-user-id="${CSS.escape(userId)}"]` : "";
      const target = document.querySelector(`tr[data-site="${siteSelector}"]${userSelector}`);
      if (!target) {
        return;
      }
      target.classList.remove("row-pending", "row-done", "row-error");
      target.classList.add(tone === "ok" ? "row-done" : tone === "error" ? "row-error" : "row-pending");
      window.setTimeout(() => {
        target.classList.remove("row-pending", "row-done", "row-error");
      }, 1800);
    }

    const SUB2API_NEWAPI_PREFIX = "/api/v1/admin/newapi-checkin";

    function normalizeDashboardApiUrl(url) {
      if (typeof url !== "string" || !url.startsWith("/api/")) {
        return url;
      }
      return `${SUB2API_NEWAPI_PREFIX}${url.slice(4)}`;
    }

    function buildSub2APIRequestOptions(options = {}) {
      const headers = new Headers(options.headers || {});
      const token = localStorage.getItem("auth_token");
      if (token && !headers.has("Authorization")) {
        headers.set("Authorization", `Bearer ${token}`);
      }
      return {
        ...options,
        headers,
      };
    }

    async function fetchJson(url, options) {
      const response = await fetch(normalizeDashboardApiUrl(url), buildSub2APIRequestOptions(options));
      const payload = await response.json();
      if (!response.ok || payload.ok === false || (payload.code !== undefined && payload.code !== 0)) {
        throw new Error(payload.message || payload.reason || `请求失败: ${response.status}`);
      }
      return payload.data;
    }

    function setStats() {
      const cards = Array.from(statsGrid.querySelectorAll(".metric-card"));
      const values = [
        state.config?.enabled_site_count ?? "-",
        state.config?.enabled_account_count ?? "-",
        state.lastRun?.success_count ?? "-",
        state.lastRun?.already_done_count ?? "-",
        formatDisplayValue(state.lastRun?.quota_awarded_display),
      ];

      cards.slice(0, 5).forEach((card, index) => {
        const node = card.querySelector(".metric-value");
        if (node) {
          node.textContent = values[index];
        }
      });

      activeFilterMeta.textContent = `当前筛选：${getActiveSiteLabel()}`;
    }

    function setActiveTab(tab) {
      state.activeTab = tab;
      Array.from(tabRow.querySelectorAll("[data-tab]")).forEach((button) => {
        button.setAttribute("aria-selected", button.dataset.tab === tab ? "true" : "false");
      });
      Array.from(document.querySelectorAll("[data-tab-panel]")).forEach((panel) => {
        panel.hidden = panel.getAttribute("data-tab-panel") !== tab;
      });
    }

    function renderSiteFilters() {
      const items = [{ key: "all", label: "全部站点" }].concat(
        getConfigSites().map((site) => ({
          key: site.name,
          label: site.enabled ? site.name : `${site.name}（停用）`,
        })),
      );

      siteFilterRow.innerHTML = items.map((item) => `
        <button class="filter-pill ${state.activeSite === item.key ? "active" : ""}" type="button" data-site-filter="${escapeHtml(item.key)}">
          ${escapeHtml(item.label)}
        </button>
      `).join("");

      Array.from(siteFilterRow.querySelectorAll("[data-site-filter]")).forEach((button) => {
        button.addEventListener("click", () => {
          state.activeSite = button.getAttribute("data-site-filter");
          syncBalanceTrendToActiveSite();
          renderAllViews();
        });
      });
    }

    function openPlatformDirectory() {
      renderConfig(state.config);
      if (typeof platformDirectoryDialog.showModal === "function") {
        platformDirectoryDialog.showModal();
      } else {
        platformDirectoryDialog.setAttribute("open", "");
      }
    }

    function closePlatformDirectory() {
      if (typeof platformDirectoryDialog.close === "function") {
        platformDirectoryDialog.close();
      } else {
        platformDirectoryDialog.removeAttribute("open");
      }
    }

    function openLoginCredentials(site, userId) {
      const account = getConfigAccount(site, userId);
      if (!account) {
        setAppStatus("未找到要维护的账号。", "error");
        return;
      }
      state.loginCredentialTarget = { site, userId };
      loginCredentialTarget.textContent = `${site} / ${preferredName(account)} / ID ${userId}`;
      loginUsernameInput.value = account.login_username || account.display_name || account.username || "";
      loginPasswordInput.value = "";
      setInlineStatus(loginCredentialStatus, account.has_login_password ? "已保存密码；留空可直接测试现有凭据。" : "尚未保存密码。", account.has_login_password ? "ok" : "normal");
      if (typeof loginCredentialDialog.showModal === "function") {
        loginCredentialDialog.showModal();
      } else {
        loginCredentialDialog.setAttribute("open", "");
      }
      loginUsernameInput.focus();
    }

    function closeLoginCredentials() {
      loginPasswordInput.value = "";
      state.loginCredentialTarget = null;
      if (typeof loginCredentialDialog.close === "function") {
        loginCredentialDialog.close();
      } else {
        loginCredentialDialog.removeAttribute("open");
      }
    }

    async function submitLoginCredentials(save) {
      const target = state.loginCredentialTarget;
      const loginUsername = loginUsernameInput.value.trim();
      if (!target || !loginUsername) {
        setInlineStatus(loginCredentialStatus, "登录账号不能为空。", "error");
        return;
      }
      const button = save ? saveLoginCredentialBtn : testLoginCredentialBtn;
      setButtonBusy(button, true, save ? "验证中" : "测试中");
      setInlineStatus(loginCredentialStatus, save ? "正在验证并保存…" : "正在测试登录…", "busy");
      try {
        const result = await fetchJson(save ? "/api/login-credentials" : "/api/test-login-credentials", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            site: target.site,
            user_id: target.userId,
            login_username: loginUsername,
            login_password: loginPasswordInput.value,
          }),
        });
        loginPasswordInput.value = "";
        if (result.config) {
          state.config = result.config;
          renderConfigSelectors();
        }
        await loadAPIKeys();
        renderAllViews();
        setInlineStatus(loginCredentialStatus, `${result.message}；Key ${result.api_key_count} 个，分组 ${result.group_count} 个。`, "ok");
        setAppStatus(`${target.site} / ${target.userId} 登录成功，读取到 ${result.group_count} 个分组`, "ok");
      } catch (error) {
        setInlineStatus(loginCredentialStatus, `${save ? "保存" : "测试"}失败: ${error.message}`, "error");
        setAppStatus(`登录凭据${save ? "保存" : "测试"}失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    function setSelectedTarget(siteName, userId) {
      if (!siteName || !userId) {
        return;
      }
      siteSelect.value = siteName;
      updateAccountOptions(userId);
      updateSelectedTargetCard();
    }

    function updateSelectedTargetCard() {
      const site = getConfigSite(siteSelect.value);
      const account = getConfigAccount(siteSelect.value, accountSelect.value);
      const title = selectedTargetCard.querySelector("strong");
      const meta = selectedTargetCard.querySelector(".target-meta");
      if (!site || !account) {
        title.textContent = "-";
        meta.textContent = "等待选择账号";
        return;
      }
      title.textContent = preferredName(account);
      meta.textContent = isReadOnlySite(site)
        ? `${site.name} · sub2api 只读数据源 · ${account.access_key_masked || "访问 Key 已保存"}`
        : site.enabled
        ? `${site.name} · ID ${account.user_id} · ${account.ip_profile || "-"}`
        : `${site.name} 已停用签到 · ID ${account.user_id} · ${site.disabled_reason || "仍可查看记录和刷新余额"}`;
      singleRunBtn.disabled = !site.enabled || isReadOnlySite(site);
      updateProviderActions();
    }

    function updateProviderActions() {
      const remoteSite = getConfigSite(selectedSiteForRemoteAction());
      const readOnly = isReadOnlySite(remoteSite);
      syncSiteNamesBtn.disabled = readOnly;
      syncMonthlySiteBtn.disabled = readOnly;
      syncMonthlyAccountBtn.disabled = readOnly;
      refreshSiteBalanceBtn.textContent = readOnly ? "刷新套餐/用量/模型" : "刷新当前站点余额/签到";
    }

    function renderConfigSelectors() {
      const sites = getConfigSites();
      if (!sites.length) {
        siteSelect.innerHTML = "";
        accountSelect.innerHTML = "";
        singleRunBtn.disabled = true;
        updateSelectedTargetCard();
        return;
      }

      siteSelect.innerHTML = sites.map((site) => `
        <option value="${escapeHtml(site.name)}">${escapeHtml(site.enabled ? site.name : `${site.name}（停用）`)}</option>
      `).join("");

      if (!sites.find((site) => site.name === siteSelect.value)) {
        siteSelect.value = sites[0].name;
      }
      updateAccountOptions();
    }

    function updateAccountOptions(preferredUserId = "") {
      const site = getConfigSite(siteSelect.value);
      if (!site) {
        accountSelect.innerHTML = "";
        singleRunBtn.disabled = true;
        updateSelectedTargetCard();
        return;
      }

      accountSelect.innerHTML = (site.accounts || []).map((account) => `
        <option value="${escapeHtml(account.user_id)}">
          ${escapeHtml(preferredName(account))} · ${escapeHtml(account.user_id)} · ${escapeHtml(account.ip_profile || "-")}
        </option>
      `).join("");

      if (preferredUserId && site.accounts.some((item) => String(item.user_id) === String(preferredUserId))) {
        accountSelect.value = preferredUserId;
      }
      updateSelectedTargetCard();
    }

    function attachConfigActions() {
      Array.from(configTableWrap.querySelectorAll("[data-pick-site][data-pick-user]")).forEach((button) => {
        button.addEventListener("click", () => {
          if (!isSiteEnabled(button.dataset.pickSite) || isReadOnlySite(button.dataset.pickSite)) {
            singleRunStatus.textContent = isReadOnlySite(button.dataset.pickSite)
              ? `${button.dataset.pickSite} 是只读数据源，不能设为补签目标。`
              : `${button.dataset.pickSite} 已停用签到，不能设为补签目标。`;
            singleRunStatus.dataset.tone = "warn";
            return;
          }
          setSelectedTarget(button.dataset.pickSite, button.dataset.pickUser);
          singleRunStatus.textContent = `已把 ${button.dataset.pickSite} / ${button.dataset.pickUser} 设为补签目标。`;
          singleRunStatus.dataset.tone = "ok";
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-refresh-account][data-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          refreshAccountBalance(button.dataset.site, button.dataset.refreshAccount, button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-sync-account][data-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          syncAccountName(button.dataset.site, button.dataset.syncAccount, button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-edit-account-label][data-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          editAccountDisplayName(button.dataset.site, button.dataset.editAccountLabel, button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-edit-login-credential][data-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          openLoginCredentials(button.dataset.site, button.dataset.editLoginCredential);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-toggle-access-key][data-site][data-user-id]")).forEach((button) => {
        button.addEventListener("click", () => {
          const accountKey = `${button.dataset.site}\u0000${button.dataset.userId}`;
          if (state.revealedAccessKeys.has(accountKey)) {
            state.revealedAccessKeys.delete(accountKey);
          } else {
            state.revealedAccessKeys.add(accountKey);
          }
          renderConfig(state.config);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-reveal-generated-key][data-site][data-user-id]")).forEach((button) => {
        button.addEventListener("click", () => {
          revealGeneratedAPIKey(button.dataset.site, button.dataset.userId, Number(button.dataset.revealGeneratedKey), button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-update-generated-group][data-site][data-user-id]")).forEach((button) => {
        button.addEventListener("click", () => {
          const tokenId = Number(button.dataset.updateGeneratedGroup);
          const select = Array.from(configTableWrap.querySelectorAll("[data-group-select][data-site][data-user-id]")).find((item) => (
            Number(item.dataset.groupSelect) === tokenId
            && item.dataset.site === button.dataset.site
            && item.dataset.userId === button.dataset.userId
          ));
          const sub2 = select?.dataset.provider === "sub2api";
          const selectedOption = select?.selectedOptions?.[0];
          updateGeneratedAPIKeyGroup(
            button.dataset.site,
            button.dataset.userId,
            tokenId,
            selectedOption?.dataset.groupName || (sub2 ? "" : select?.value || ""),
            sub2 ? Number(select?.value || 0) : 0,
            button,
          );
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-link-generated-key][data-site][data-user-id][data-target-account-id]")).forEach((button) => {
        button.addEventListener("click", () => {
          linkGeneratedAPIKey(
            button.dataset.site,
            button.dataset.userId,
            Number(button.dataset.linkGeneratedKey),
            Number(button.dataset.targetAccountId),
            button.dataset.targetAccountName,
            button.dataset.operation,
            button,
          );
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-delete-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          deleteSite(button.dataset.deleteSite, button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-toggle-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          toggleSiteEnabled(button.dataset.toggleSite, button.dataset.siteEnabledTarget === "true", button);
        });
      });
      Array.from(configTableWrap.querySelectorAll("[data-delete-account-site]")).forEach((button) => {
        button.addEventListener("click", () => {
          deleteAccount(button.dataset.deleteAccountSite, button.dataset.deleteAccountUser, button.dataset.deleteAccountLabel, button);
        });
      });
    }

    function generatedKeyStateKey(site, userId, apiKeyId) {
      return `${site}\u0000${userId}\u0000${apiKeyId}`;
    }

    async function revealGeneratedAPIKey(site, userId, apiKeyId, button) {
      const stateKey = generatedKeyStateKey(site, userId, apiKeyId);
      if (state.revealedGeneratedKeys.has(stateKey)) {
        state.revealedGeneratedKeys.delete(stateKey);
        renderConfig(state.config);
        return;
      }
      setButtonBusy(button, true, "读取中");
      try {
        const result = await fetchJson("/api/reveal-api-key", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId, api_key_id: apiKeyId }),
        });
        state.revealedGeneratedKeys.set(stateKey, result.key);
        renderConfig(state.config);
        setAppStatus(`${site} / ${userId} 的 ${result.name || "API Key"} 已显示`, "ok");
      } catch (error) {
        setAppStatus(`读取完整 API Key 失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function updateGeneratedAPIKeyGroup(site, userId, apiKeyId, group, groupId, button) {
      if (!group && !groupId) {
        setAppStatus("请选择目标分组。", "error");
        return;
      }
      setButtonBusy(button, true, "更新中");
      try {
        const result = await fetchJson("/api/api-key-group", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId, api_key_id: apiKeyId, group, group_id: groupId }),
        });
        await loadAPIKeys();
        setAppStatus(`${site} / ${userId} 的 ${result.name || "API Key"} 已切换到 ${result.group}`, "ok");
      } catch (error) {
        setAppStatus(`更新 API Key 分组失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function linkGeneratedAPIKey(site, userId, apiKeyId, targetAccountId, targetAccountName, operation, button) {
      const operationLabel = operation === "replace" ? "替换" : "追加";
      const warning = operation === "replace" ? "这会删除该账号当前保存的其他 Key。" : "现有 Key 会保留。";
      if (!window.confirm(`确认将此签到 Key ${operationLabel}到主平台账号「${targetAccountName}」？\n${warning}`)) return;
      setButtonBusy(button, true, "处理中");
      try {
        const result = await fetchJson("/api/link-api-key", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            site,
            user_id: userId,
            api_key_id: apiKeyId,
            target_account_id: targetAccountId,
            operation,
          }),
        });
        await loadAPIKeys();
        setAppStatus(`已${operationLabel}到主平台账号 ${result.target_account_name}，当前 ${result.key_count} 个 Key`, "ok");
      } catch (error) {
        setAppStatus(`${operationLabel}主平台账号 Key 失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    function renderOverview() {
      const siteSummaries = (state.lastRun?.site_summaries || []).filter((row) => state.activeSite === "all" || row.site === state.activeSite);
      const balanceSummaries = (state.balances?.site_summaries || []).filter((row) => state.activeSite === "all" || row.site === state.activeSite);
      const accountResults = (state.lastRun?.account_results || []).filter(matchesActiveSite);
      const selectedAccount = getConfigAccount(siteSelect.value, accountSelect.value);

      overviewRunMeta.textContent = state.lastRun?.started_at
        ? `${state.lastRun.started_at} -> ${state.lastRun.ended_at}，共 ${siteSummaries.length} 个站点视图`
        : "等待最近一次结果";
      overviewBalanceMeta.textContent = state.balances?.generated_at
        ? `${state.balances.generated_at} 在线查询`
        : "等待实时余额";

      if (!siteSummaries.length) {
        overviewRunGrid.innerHTML = '<div class="empty">暂无执行汇总</div>';
      } else {
        overviewRunGrid.innerHTML = siteSummaries.map((row) => `
          <article class="overview-card">
            <div class="overview-top">
              <div class="overview-site">
                <strong>${escapeHtml(row.site)}</strong>
                <div class="overview-sub">任务 ${escapeHtml(row.task_count)} · 奖励 ${escapeHtml(formatDisplayValue(row.quota_display_total))}</div>
              </div>
              <div class="chip-row">
                ${statusChip(`成功 ${row.success_count}`, row.success_count > 0 ? "ok" : "neutral")}
                ${statusChip(`已签 ${row.already_done_count}`, row.already_done_count > 0 ? "warn" : "neutral")}
                ${statusChip(`失败 ${row.failed_count}`, row.failed_count > 0 ? "danger" : "neutral")}
              </div>
            </div>
            <div class="overview-metrics">
              <div class="overview-metric">
                <div class="overview-metric-label" title="当前签到任务的总数量" style="cursor: help;">任务数</div>
                <div class="overview-metric-value">${escapeHtml(row.task_count)}</div>
              </div>
              <div class="overview-metric">
                <div class="overview-metric-label" title="成功签到的账号数量" style="cursor: help;">成功数</div>
                <div class="overview-metric-value">${escapeHtml(row.success_count)}</div>
              </div>
              <div class="overview-metric">
                <div class="overview-metric-label" title="奖励额度折算后的实际价值" style="cursor: help;">奖励($)</div>
                <div class="overview-metric-value">${escapeHtml(formatDisplayValue(row.quota_display_total))}</div>
              </div>
            </div>
          </article>
        `).join("");
      }

      if (!balanceSummaries.length) {
        overviewBalanceGrid.innerHTML = '<div class="empty">暂无余额汇总</div>';
      } else {
        overviewBalanceGrid.innerHTML = balanceSummaries.map((row) => `
          <article class="overview-card">
            <div class="overview-top">
              <div class="overview-site">
                <strong>${escapeHtml(row.site)}</strong>
                <div class="overview-sub">${row.enabled ? "启用站点" : "停用站点"} · 账号 ${escapeHtml(row.account_count)}</div>
              </div>
              <div class="chip-row">
                ${statusChip(row.enabled ? "在线巡检" : "停用", row.enabled ? "ok" : "warn")}
              </div>
            </div>
            <div class="overview-metrics">
              <div class="overview-metric">
                <div class="overview-metric-label" title="该站点下所有账号的折算余额总和" style="cursor: help;">余额合计</div>
                <div class="overview-metric-value">${escapeHtml(formatDisplayValue(row.quota_display_total))}</div>
              </div>
              <div class="overview-metric">
                <div class="overview-metric-label" title="该站点下所有账号的已用折算总和" style="cursor: help;">已用合计</div>
                <div class="overview-metric-value">${escapeHtml(formatDisplayValue(row.used_display_total))}</div>
              </div>
              <div class="overview-metric">
                <div class="overview-metric-label" title="该站点当前纳入统计的账号数量" style="cursor: help;">账号数</div>
                <div class="overview-metric-value">${escapeHtml(row.account_count)}</div>
              </div>
            </div>
          </article>
        `).join("");
      }

      const failedAccounts = accountResults.filter((row) => row.ok === false || row.success === false && !(row.message || "").includes("已签到"));
      const signedAccounts = accountResults.filter((row) => (row.message || "").includes("已签到"));
      const feedItems = [];

      if (failedAccounts.length) {
        failedAccounts.forEach((row) => {
          feedItems.push({
            title: `${row.site} / ${preferredName(row)}`,
            tone: "danger",
            tag: "需要处理",
            meta: `ID ${row.user_id} · ${row.ip_profile || "-"}`,
            body: row.message || row.quota_query_message || "签到失败",
          });
        });
      }

      signedAccounts.slice(0, 4).forEach((row) => {
        feedItems.push({
          title: `${row.site} / ${preferredName(row)}`,
          tone: "warn",
          tag: "已签过",
          meta: `ID ${row.user_id} · ${row.ip_profile || "-"}`,
          body: "今天已签到，无需补跑。",
        });
      });

      if (!feedItems.length) {
        feedItems.push({
          title: "今天没有异常项",
          tone: "ok",
          tag: "运行平稳",
          meta: `${getActiveSiteLabel()} 范围内未发现失败项`,
          body: "需要更新账号数据时，可以从平台目录使用该数据源支持的刷新操作。",
        });
      }

      overviewFeed.innerHTML = feedItems.map((item) => `
        <article class="feed-item">
          <div class="feed-item-head">
            <strong>${escapeHtml(item.title)}</strong>
            ${badge(item.tag, item.tone)}
          </div>
          <div class="dim">${escapeHtml(item.meta)}</div>
          <div class="feed-item-body">${escapeHtml(item.body)}</div>
        </article>
      `).join("");

      const contextItems = [];
      contextItems.push({
        title: `当前视图：${getActiveSiteLabel()}`,
        tone: "neutral",
        tag: "筛选",
        meta: state.activeSite === "all" ? "正在看所有站点" : "页面里的表格和卡片都已经收窄",
        body: state.activeSite === "all" ? "适合看整体趋势和平台对比。" : "适合处理单个站点的补签和余额核对。",
      });
      if (selectedAccount) {
        const selectedSite = getConfigSite(siteSelect.value);
        const selectedReadOnly = isReadOnlySite(selectedSite);
        contextItems.push({
          title: preferredName(selectedAccount),
          tone: "ok",
          tag: selectedReadOnly ? "只读账号" : "补签目标",
          meta: `${siteSelect.value} · ID ${selectedAccount.user_id} · ${selectedAccount.ip_profile || "-"}`,
          body: selectedReadOnly
            ? "该账号只读取套餐、用量和可用模型，不会触发签到或模型请求。"
            : "单账号签到会沿用配置里的 token 和出口档位。",
        });
      }
      if (state.lastRun?.quota_awarded_display) {
        contextItems.push({
          title: "最近奖励",
          tone: "neutral",
          tag: "执行结果",
          meta: `${state.lastRun.success_count} 成功 · ${state.lastRun.already_done_count} 已签`,
          body: formatDisplayValue(state.lastRun.quota_awarded_display),
        });
      }

      overviewContextFeed.innerHTML = contextItems.map((item) => `
        <article class="feed-item">
          <div class="feed-item-head">
            <strong>${escapeHtml(item.title)}</strong>
            ${badge(item.tag, item.tone)}
          </div>
          <div class="dim">${escapeHtml(item.meta)}</div>
          <div class="feed-item-body">${escapeHtml(item.body)}</div>
        </article>
      `).join("");
    }

    function generatedAPIKeysHtml(site, account, apiKeySummary) {
      const provider = apiKeySummary?.provider || site.provider || "newapi";
      const sub2 = provider === "sub2api";
      const baseOptions = sub2
        ? (apiKeySummary?.available_group_options || []).map((option) => ({ id: Number(option.id), name: option.name }))
        : Array.from(new Set((apiKeySummary?.available_groups || []).filter(Boolean))).map((name) => ({ id: 0, name }));
      const items = (apiKeySummary?.api_keys || []).map((item) => {
        const stateKey = generatedKeyStateKey(site.name, account.user_id, item.id);
        const revealedKey = state.revealedGeneratedKeys.get(stateKey);
        const options = baseOptions.slice();
        if (item.group && !options.some((option) => sub2 ? option.id === Number(item.group_id) : option.name === item.group)) {
          options.unshift({ id: Number(item.group_id || 0), name: item.group });
        }
        const references = item.referenced_accounts || [];
        const targets = item.target_accounts || [];
        const referenceHtml = targets.length ? `
          <div class="account-reference-list">
            ${targets.map((target) => `
              <div class="account-reference-row">
                ${badge(target.referenced ? `已引用 · ${target.name}` : `可关联 · ${target.name}`, target.referenced ? "ok" : "neutral")}
                <button class="quick-action secondary" type="button"
                  data-link-generated-key="${escapeHtml(item.id)}" data-site="${escapeHtml(site.name)}"
                  data-user-id="${escapeHtml(account.user_id)}" data-target-account-id="${escapeHtml(target.id)}"
                  data-target-account-name="${escapeHtml(target.name)}" data-operation="append">追加</button>
                <button class="quick-action ${target.referenced ? "secondary" : "danger"}" type="button"
                  data-link-generated-key="${escapeHtml(item.id)}" data-site="${escapeHtml(site.name)}"
                  data-user-id="${escapeHtml(account.user_id)}" data-target-account-id="${escapeHtml(target.id)}"
                  data-target-account-name="${escapeHtml(target.name)}" data-operation="replace">替换</button>
              </div>
            `).join("")}
          </div>
        ` : '<div class="group-capability">主平台暂无相同站点 URL 的 OpenAI API Key 账号</div>';
        return `
          <div class="api-key-item ${references.length ? "referenced" : ""}">
            <div class="api-key-line">
              <code title="${escapeHtml(item.name || "未命名")}">${escapeHtml(revealedKey || item.masked_key)}</code>
              <button
                class="quick-action secondary"
                type="button"
                data-reveal-generated-key="${escapeHtml(item.id)}"
                data-site="${escapeHtml(site.name)}"
                data-user-id="${escapeHtml(account.user_id)}"
              >${revealedKey ? "隐藏" : "显示"}</button>
            </div>
            <div class="group-control">
              <select
                data-group-select="${escapeHtml(item.id)}"
                data-site="${escapeHtml(site.name)}"
                data-user-id="${escapeHtml(account.user_id)}"
                data-provider="${escapeHtml(provider)}"
                aria-label="${escapeHtml(item.name || "API Key")} 分组"
                ${options.length ? "" : "disabled"}
              >
                ${options.map((option) => {
                  const value = sub2 ? String(option.id) : option.name;
                  const selected = sub2 ? Number(option.id) === Number(item.group_id) : option.name === item.group;
                  return `<option value="${escapeHtml(value)}" data-group-name="${escapeHtml(option.name)}" ${selected ? "selected" : ""}>${escapeHtml(option.name)}</option>`;
                }).join("")}
              </select>
              <button
                class="quick-action secondary"
                type="button"
                data-update-generated-group="${escapeHtml(item.id)}"
                data-site="${escapeHtml(site.name)}"
                data-user-id="${escapeHtml(account.user_id)}"
                ${options.length ? "" : "disabled"}
              >更新分组</button>
            </div>
            ${referenceHtml}
          </div>
        `;
      }).join("");
      const capability = apiKeySummary?.group_status === "ready" ? "完整分组已读取" : apiKeySummary?.group_message || "仅展示已知分组";
      return `<div class="api-key-stack">${items || '<span class="dim">未生成</span>'}<div class="group-capability" title="${escapeHtml(apiKeySummary?.group_message || "")}">${escapeHtml(capability)}</div></div>`;
    }

    function renderConfig(config) {
      if (!config) {
        configMeta.textContent = "暂无配置数据";
        configSummaryBadges.innerHTML = "";
        configTableWrap.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      configMeta.textContent = `共 ${config.all_site_count} 个站点，启用 ${config.enabled_site_count} 个；共 ${config.all_account_count} 个账号，启用 ${config.enabled_account_count} 个。`;

      const visibleSites = config.sites.filter((site) => state.activeSite === "all" || site.name === state.activeSite);
      configSummaryBadges.innerHTML = [
        badge(`查看范围 ${getActiveSiteLabel()}`, "neutral"),
        badge(`启用站点 ${config.enabled_site_count}`, "ok"),
        badge(`启用账号 ${config.enabled_account_count}`, "ok"),
        badge(`停用站点 ${config.all_site_count - config.enabled_site_count}`, config.all_site_count > config.enabled_site_count ? "warn" : "neutral"),
      ].join("");

      const apiKeyAccounts = new Map(
        (state.apiKeys?.accounts || []).map((account) => [`${account.site}\u0000${account.user_id}`, account]),
      );
      const rows = [];
      visibleSites.forEach((site) => {
        (site.accounts || []).forEach((account, index) => {
          const readOnly = isReadOnlySite(site);
          const apiKeySummary = apiKeyAccounts.get(`${site.name}\u0000${account.user_id}`);
          let apiKeyHtml = '<span class="dim">读取中</span>';
          if (readOnly) {
            const accountKey = `${site.name}\u0000${account.user_id}`;
            const keyRevealed = state.revealedAccessKeys.has(accountKey) && Boolean(account.access_key);
            const displayedKey = keyRevealed ? account.access_key : account.access_key_masked;
            apiKeyHtml = displayedKey
              ? `
                  <div class="access-key-cell">
                    <code title="${keyRevealed ? "完整 sub2api 访问 Key" : "sub2api 访问 Key（已脱敏）"}">${escapeHtml(displayedKey)}</code>
                    ${account.access_key ? `
                      <button
                        class="quick-action secondary"
                        type="button"
                        data-toggle-access-key
                        data-site="${escapeHtml(site.name)}"
                        data-user-id="${escapeHtml(account.user_id)}"
                        aria-pressed="${keyRevealed ? "true" : "false"}"
                        title="${keyRevealed ? "隐藏完整 Key" : "显示完整 Key"}"
                      >${keyRevealed ? "隐藏" : "显示"}</button>
                    ` : ""}
                  </div>
              `
              : '<span class="dim">已保存</span>';
            const generated = apiKeySummary?.status === "ready" || apiKeySummary?.status === "missing"
              ? generatedAPIKeysHtml(site, account, apiKeySummary)
              : `<div class="group-capability">${escapeHtml(apiKeySummary?.group_message || "填写登录凭据后可读取和更新分组")}</div>`;
            apiKeyHtml = `<div class="api-key-stack">${apiKeyHtml}${generated}</div>`;
          } else if (state.apiKeys?.load_error) {
            apiKeyHtml = '<span class="dim">读取失败</span>';
          } else if (apiKeySummary?.status === "ready") {
            apiKeyHtml = generatedAPIKeysHtml(site, account, apiKeySummary);
          } else if (apiKeySummary?.status === "missing") {
            apiKeyHtml = `<div class="api-key-stack"><span class="dim">未生成</span><span class="group-capability">${escapeHtml(apiKeySummary.group_message || "暂无 token 可切换分组")}</span></div>`;
          } else if (apiKeySummary?.status === "unsupported") {
            apiKeyHtml = `<div class="api-key-stack"><span class="dim">使用配置 Key</span><span class="group-capability">${escapeHtml(apiKeySummary.group_message || "需要账号登录态")}</span></div>`;
          } else if (apiKeySummary?.status === "error") {
            apiKeyHtml = `<span class="dim" title="${escapeHtml(apiKeySummary.message || "API Key 读取失败")}">读取失败</span>`;
          } else if (state.apiKeys) {
            apiKeyHtml = '<span class="dim">读取失败</span>';
          }
          const siteHtml = index === 0
            ? `
                <div class="site-block">
                  <div class="site-top">
                    <strong>${escapeHtml(site.name)}</strong>
                    ${badge(site.enabled ? "启用" : "停用", site.enabled ? "ok" : "warn")}
                    ${badge(readOnly ? "sub2api 只读" : "NewAPI", readOnly ? "neutral" : "ok")}
                  </div>
                  <div class="site-url">${escapeHtml(site.base_url)}</div>
                  <div class="site-note">${escapeHtml(site.disabled_reason || "按当前配置参与轮询")}</div>
                </div>
              `
            : `<span class="site-repeat">↳ 同站点账号</span>`;

          rows.push({
            _rowClass: `catalog-row ${index === 0 ? "site-start" : ""}`,
            site: site.name,
            user_id: account.user_id,
            siteHtml,
            identity: account,
            route: account.ip_profile || "-",
            endpoint: readOnly ? "/v1/usage + /v1/models" : "/api/user/checkin",
            display: account.display_name || "-",
            apiKeyHtml,
            action: `
              <div class="action-cluster">
                <button
                  class="quick-action"
                  type="button"
                  data-pick-site="${escapeHtml(site.name)}"
                  data-pick-user="${escapeHtml(account.user_id)}"
                  ${site.enabled && !readOnly ? "" : "disabled"}
                  title="${readOnly ? "sub2api 是只读数据源" : site.enabled ? "设为单账号补签目标" : "站点已停用签到，不能补签"}"
                >
                  设为补签
                </button>
                <button
                  class="quick-action secondary"
                  type="button"
                  data-site="${escapeHtml(site.name)}"
                  data-refresh-account="${escapeHtml(account.user_id)}"
                >
                  ${readOnly ? "刷套餐/模型" : site.enabled ? "刷余额/签到" : "刷余额"}
                </button>
                <button
                  class="quick-action secondary"
                  type="button"
                  data-site="${escapeHtml(site.name)}"
                  data-sync-account="${escapeHtml(account.user_id)}"
                  ${readOnly ? "disabled" : ""}
                >
                  同步名
                </button>
                <button
                  class="quick-action secondary"
                  type="button"
                  data-site="${escapeHtml(site.name)}"
                  data-edit-account-label="${escapeHtml(account.user_id)}"
                >
                  编辑标识
                </button>
                ${readOnly ? `
                  <button
                    class="quick-action secondary"
                    type="button"
                    data-site="${escapeHtml(site.name)}"
                    data-edit-login-credential="${escapeHtml(account.user_id)}"
                    title="填写账号密码，用于读取和修改 API Key 分组"
                  >
                    登录凭据
                  </button>
                ` : ""}
                <button
                  class="quick-action danger"
                  type="button"
                  data-delete-account-site="${escapeHtml(site.name)}"
                  data-delete-account-user="${escapeHtml(account.user_id)}"
                  data-delete-account-label="${escapeHtml(preferredName(account))}"
                  title="删除该账号及本地缓存记录"
                >
                  删账号
                </button>
                ${index === 0 ? `
                  <button
                    class="quick-action ${site.enabled ? "danger" : "secondary"}"
                    type="button"
                    data-toggle-site="${escapeHtml(site.name)}"
                    data-site-enabled-target="${site.enabled ? "false" : "true"}"
                    title="${readOnly ? "控制该只读数据源是否纳入余额汇总" : site.enabled ? "禁用该站点后不会再执行签到" : "重新允许该站点参与签到"}"
                  >
                    ${readOnly ? (site.enabled ? "停用数据源" : "启用数据源") : (site.enabled ? "禁用签到" : "启用签到")}
                  </button>
                  <button
                    class="quick-action danger"
                    type="button"
                    data-delete-site="${escapeHtml(site.name)}"
                    title="删除该站点及本地缓存记录"
                  >
                    删站点
                  </button>
                ` : ""}
              </div>
            `,
          });
        });
      });

      renderTable(
        configTableWrap,
        [
          { label: "站点", value: (row) => row.siteHtml },
          { label: "名称 / 账号", value: (row) => identityHtml(row.identity) },
          { label: "展示名称", value: (row) => `<span class="dim">${escapeHtml(row.display)}</span>` },
          { label: "API Key", value: (row) => row.apiKeyHtml },
          { label: "出口档位", value: (row) => `<span class="route-badge">${escapeHtml(row.route)}</span>` },
          { label: "数据接口", value: (row) => `<span class="dim">${escapeHtml(row.endpoint)}</span>` },
          { label: "操作", value: (row) => row.action },
        ],
        rows,
      );
      attachConfigActions();
    }

    function renderHistory(report) {
      if (!report?.started_at) {
        lastRunMeta.textContent = "暂无最近执行结果";
        lastRunBadges.innerHTML = "";
        lastRunSiteTable.innerHTML = '<div class="empty">暂无数据</div>';
        lastRunAccountTable.innerHTML = '<div class="empty">暂无数据</div>';
        lastRunSummaryText.textContent = "";
        return;
      }

      lastRunMeta.textContent = `${report.started_at} -> ${report.ended_at}，耗时 ${formatDisplayValue(report.duration_seconds)}s`;
      lastRunBadges.innerHTML = [
        badge(`任务 ${report.task_count}`, "neutral"),
        badge(`成功 ${report.success_count}`, "ok"),
        badge(`已签 ${report.already_done_count}`, report.already_done_count > 0 ? "warn" : "neutral"),
        badge(`失败 ${report.failed_count}`, report.failed_count > 0 ? "danger" : "neutral"),
        badge(`奖励 ${formatDisplayValue(report.quota_awarded_display)}`, "neutral"),
      ].join("");

      renderTable(
        lastRunSiteTable,
        [
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "任务数", value: (row) => escapeHtml(row.task_count) },
          { label: "成功", value: (row) => escapeHtml(row.success_count) },
          { label: "已签", value: (row) => escapeHtml(row.already_done_count) },
          { label: "失败", value: (row) => escapeHtml(row.failed_count) },
          { label: "奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_display_total)) },
        ],
        (report.site_summaries || []).filter((row) => state.activeSite === "all" || row.site === state.activeSite),
      );

      const accountRows = (report.account_results || [])
        .filter(matchesActiveSite)
        .map((row) => {
          const configAccount = getConfigAccount(row.site, row.user_id) || {};
          return {
            ...row,
            name: configAccount.name || row.account,
            username: configAccount.username || row.username || "",
            display_name: configAccount.display_name || row.display_name || "",
          };
        });

      renderTable(
        lastRunAccountTable,
        [
          { label: "签到日期", value: (row) => escapeHtml(row.checkin_date || (report.ended_at || "").slice(0, 10) || "-") },
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "名称 / 账号", value: (row) => identityHtml(row) },
          { label: "IP 档位", value: (row) => `<span class="route-badge">${escapeHtml(row.ip_profile || "-")}</span>` },
          { label: "结果", value: (row) => badge(formatResult(row), resultTone(row)) },
          { label: "奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
          { label: "当前余额($)", value: (row) => escapeHtml(formatDisplayValue(row.remaining_quota_display)) },
          { label: "已用($)", value: (row) => escapeHtml(formatDisplayValue(row.used_quota_display)) },
          { label: "消息", value: (row) => `<span class="dim">${escapeHtml(row.message || row.quota_query_message || "-")}</span>` },
        ],
        accountRows,
      );

      lastRunSummaryText.textContent = report.summary_text || "";
    }

    function chartPath(rows, key, width, height, padding) {
      if (!rows.length) {
        return "";
      }
      const values = rows.map((row) => Number(row[key] || 0));
      const max = Math.max(...values, 1);
      const step = rows.length > 1 ? (width - padding * 2) / (rows.length - 1) : 0;
      return rows.map((row, index) => {
        const x = rows.length > 1 ? padding + step * index : width / 2;
        const y = height - padding - (Number(row[key] || 0) / max) * (height - padding * 2);
        return `${index === 0 ? "M" : "L"} ${x.toFixed(1)} ${y.toFixed(1)}`;
      }).join(" ");
    }

    function chartDots(rows, key, width, height, padding, className) {
      const values = rows.map((row) => Number(row[key] || 0));
      const max = Math.max(...values, 1);
      const step = rows.length > 1 ? (width - padding * 2) / (rows.length - 1) : 0;
      return rows.map((row, index) => {
        const x = rows.length > 1 ? padding + step * index : width / 2;
        const y = height - padding - (Number(row[key] || 0) / max) * (height - padding * 2);
        return `<circle class="trend-dot ${className}" cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="4" fill="currentColor"><title>${escapeHtml(row.date)} ${escapeHtml(formatDisplayValue(row[key]))}</title></circle>`;
      }).join("");
    }

    function renderTrend(history) {
      if (!history?.daily_summaries?.length) {
        renderTrendScopeSelector(history);
        trendMeta.textContent = "暂无历史趋势数据";
        trendChart.innerHTML = '<div class="empty">刷新余额/签到或完成每日脚本后，会在这里形成每日趋势。</div>';
        trendTable.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      renderTrendScopeSelector(history);
      let visibleRows = history.daily_summaries.slice(-30);
      if (state.trendScope.startsWith("site:")) {
        const siteName = state.trendScope.slice(5);
        visibleRows = (history.site_summaries || []).filter((row) => row.site === siteName).slice(-30);
      } else if (state.trendScope.startsWith("account:")) {
        const [, siteName, userId] = state.trendScope.split(":");
        visibleRows = (history.account_summaries || [])
          .filter((row) => row.site === siteName && String(row.user_id) === String(userId))
          .slice(-30);
      }

      if (!visibleRows.length) {
        trendMeta.textContent = `${history.generated_at || "-"}，${getTrendScopeLabel()} 暂无趋势数据，存储 ${history.history_path || ""}`;
        trendChart.innerHTML = '<div class="empty">当前趋势范围还没有写入历史记录。</div>';
        trendTable.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      trendMeta.textContent = `${history.generated_at || "-"}，${getTrendScopeLabel()} 最近 ${visibleRows.length} 天，存储 ${history.history_path || ""}`;
      const width = 920;
      const height = 280;
      const padding = 34;
      const awardPath = chartPath(visibleRows, "quota_awarded_display_value", width, height, padding);
      const balancePath = chartPath(visibleRows, "balance_display_value", width, height, padding);
      const labels = visibleRows.map((row, index) => {
        const step = visibleRows.length > 1 ? (width - padding * 2) / (visibleRows.length - 1) : 0;
        const x = visibleRows.length > 1 ? padding + step * index : width / 2;
        return `<text x="${x.toFixed(1)}" y="${height - 8}" text-anchor="middle" font-size="11" fill="var(--muted)">${escapeHtml(row.date.slice(5))}</text>`;
      }).join("");

      trendChart.innerHTML = `
        <svg viewBox="0 0 ${width} ${height}" role="img" aria-label="${escapeHtml(getTrendScopeLabel())}每日签到新增与余额趋势">
          <line x1="${padding}" y1="${height - padding}" x2="${width - padding}" y2="${height - padding}" stroke="var(--line)" />
          <line x1="${padding}" y1="${padding}" x2="${padding}" y2="${height - padding}" stroke="var(--line)" />
          <path class="trend-line balance" d="${balancePath}" />
          <path class="trend-line award" d="${awardPath}" />
          <g style="color: var(--ok)">${chartDots(visibleRows, "balance_display_value", width, height, padding, "balance")}</g>
          <g style="color: var(--accent)">${chartDots(visibleRows, "quota_awarded_display_value", width, height, padding, "award")}</g>
          ${labels}
        </svg>
      `;

      renderTable(
        trendTable,
        [
          { label: "日期", value: (row) => escapeHtml(row.date) },
          { label: "任务", value: (row) => escapeHtml(row.task_count) },
          { label: "成功", value: (row) => escapeHtml(row.success_count) },
          { label: "已签", value: (row) => escapeHtml(row.already_done_count) },
          { label: "异常", value: (row) => escapeHtml(row.failed_count) },
          { label: "新增($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
          { label: "余额($)", value: (row) => escapeHtml(formatDisplayValue(row.balance_display)) },
        ],
        visibleRows,
      );
    }

    function renderMonthlyTrendScopeSelector(monthly) {
      const siteNames = Array.from(new Set((monthly?.site_summaries || []).map((row) => row.site).filter(Boolean)));
      const accountItems = monthly?.account_summaries || [];
      const options = [{ value: "all", label: "全部站点" }].concat(
        siteNames.map((siteName) => ({ value: `site:${siteName}`, label: `${siteName} 签到奖励` })),
        accountItems.map((row) => ({ value: `account:${row.site}:${row.user_id}`, label: `${row.site} / ${preferredName(row)} 签到奖励` })),
      );

      monthlyTrendScopeSelect.innerHTML = options.map((item) => `
        <option value="${escapeHtml(item.value)}">${escapeHtml(item.label)}</option>
      `).join("");

      if (!options.some((item) => item.value === state.monthlyTrendScope)) {
        state.monthlyTrendScope = "all";
      }
      monthlyTrendScopeSelect.value = state.monthlyTrendScope;
    }

    function compactDailyRows(rows) {
      const daily = new Map();
      (rows || []).forEach((row) => {
        const date = row.date;
        if (!daily.has(date)) {
          daily.set(date, { date, quota_awarded_display_value: 0, checkin_count: 0 });
        }
        const item = daily.get(date);
        item.quota_awarded_display_value += Number(row.quota_awarded_display_value || 0);
        item.checkin_count += Number(row.checkin_count || 0);
      });
      return Array.from(daily.values())
        .sort((a, b) => a.date.localeCompare(b.date))
        .map((row) => ({
          ...row,
          quota_awarded_display: `$${Number(row.quota_awarded_display_value || 0).toFixed(2)}`,
        }));
    }

    function monthlyTrendRows(monthly) {
      if (!monthly) {
        return [];
      }
      if (state.monthlyTrendScope === "all") {
        return compactDailyRows(monthly.site_daily_summaries || []);
      }
      if (state.monthlyTrendScope.startsWith("site:")) {
        const siteName = state.monthlyTrendScope.slice(5);
        return compactDailyRows((monthly.site_daily_summaries || []).filter((row) => row.site === siteName));
      }
      if (state.monthlyTrendScope.startsWith("account:")) {
        const [, siteName, userId] = state.monthlyTrendScope.split(":");
        return compactDailyRows((monthly.account_daily_summaries || [])
          .filter((row) => row.site === siteName && String(row.user_id) === String(userId)));
      }
      return [];
    }

    function renderMonthlyTrend(monthly) {
      renderMonthlyTrendScopeSelector(monthly);
      const rows = monthlyTrendRows(monthly);
      if (!rows.length) {
        monthlyTrendMeta.textContent = `${getMonthlyTrendScopeLabel()} 暂无月度趋势数据`;
        monthlyTrendChart.innerHTML = '<div class="empty">点击同步当前站点月份或账号月份后，这里会从 SQL 记录形成折线图。</div>';
        monthlyTrendTable.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      const width = 920;
      const height = 260;
      const padding = 34;
      const awardPath = chartPath(rows, "quota_awarded_display_value", width, height, padding);
      const labels = rows.map((row, index) => {
        const step = rows.length > 1 ? (width - padding * 2) / (rows.length - 1) : 0;
        const x = rows.length > 1 ? padding + step * index : width / 2;
        return `<text x="${x.toFixed(1)}" y="${height - 8}" text-anchor="middle" font-size="11" fill="var(--muted)">${escapeHtml(row.date.slice(5))}</text>`;
      }).join("");

      monthlyTrendMeta.textContent = `${monthly?.generated_at || "-"}，${getMonthlyTrendScopeLabel()} 最近 ${rows.length} 天签到奖励`;
      monthlyTrendChart.innerHTML = `
        <svg viewBox="0 0 ${width} ${height}" role="img" aria-label="${escapeHtml(getMonthlyTrendScopeLabel())}月度签到奖励趋势">
          <line x1="${padding}" y1="${height - padding}" x2="${width - padding}" y2="${height - padding}" stroke="var(--line)" />
          <line x1="${padding}" y1="${padding}" x2="${padding}" y2="${height - padding}" stroke="var(--line)" />
          <path class="trend-line award" d="${awardPath}" />
          <g style="color: var(--accent)">${chartDots(rows, "quota_awarded_display_value", width, height, padding, "award")}</g>
          ${labels}
        </svg>
      `;

      renderTable(
        monthlyTrendTable,
        [
          { label: "日期", value: (row) => escapeHtml(row.date) },
          { label: "签到次数", value: (row) => escapeHtml(row.checkin_count) },
          { label: "签到奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
        ],
        rows,
      );
    }

    function renderMonthly(monthly) {
      if (!monthly) {
        monthlyMeta.textContent = "尚未读取 SQL 月度记录";
        monthlySiteTable.innerHTML = '<div class="empty">暂无数据</div>';
        monthlyAccountTable.innerHTML = '<div class="empty">暂无数据</div>';
        monthlyRecordsTable.innerHTML = '<div class="empty">暂无数据</div>';
        renderMonthlyTrend(null);
        return;
      }

      const records = (monthly.records || []).filter(matchesActiveSite);
      const siteRows = (monthly.site_summaries || []).filter(matchesActiveSite);
      const accountRows = (monthly.account_summaries || []).filter(matchesActiveSite);
      const sync = monthly.sync_state || {};
      monthlyMeta.textContent = `${monthly.generated_at}，月份 ${monthly.selected_month}，SQL ${monthly.initialized ? "已初始化" : "初始化中"}，记录 ${records.length} 条。`;
      setInlineStatus(
        monthlySyncStatus,
        sync.running
          ? `${sync.message || "正在同步"}：${sync.processed || 0}/${sync.total || 0} ${sync.last_site || ""} ${sync.last_user_id || ""}`.trim()
          : (sync.message || "月度同步空闲"),
        sync.running ? "busy" : sync.error ? "error" : "ok",
      );

      renderMonthlyTrend(monthly);
      renderTable(
        monthlySiteTable,
        [
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "账号数", value: (row) => escapeHtml(row.account_count) },
          { label: "签到次数", value: (row) => escapeHtml(row.checkin_count) },
          { label: "签到奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
        ],
        siteRows,
      );
      renderTable(
        monthlyAccountTable,
        [
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "名称 / 账号", value: (row) => identityHtml(row) },
          { label: "签到次数", value: (row) => escapeHtml(row.checkin_count) },
          { label: "签到奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
        ],
        accountRows,
      );
      renderTable(
        monthlyRecordsTable,
        [
          { label: "日期", value: (row) => escapeHtml(row.checkin_date) },
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "名称 / 账号", value: (row) => identityHtml(row) },
          { label: "奖励($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_awarded_display)) },
          { label: "来源", value: (row) => `<span class="dim">${escapeHtml(row.source || "-")}</span>` },
          { label: "写入时间", value: (row) => `<span class="dim">${escapeHtml(row.fetched_at || "-")}</span>` },
        ],
        records.slice(-320).reverse(),
      );
    }

    function renderBalances(payload) {
      if (!payload) {
        balanceMeta.textContent = "暂无余额数据";
        balanceOverallBadges.innerHTML = "";
        balanceSiteTable.innerHTML = '<div class="empty">暂无数据</div>';
        balanceAccountTable.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }

      balanceMeta.textContent = `${payload.generated_at}，共 ${payload.site_count} 个站点，${payload.account_count} 个账号。`;
      const overall = payload.overall || {};
      const overallBadges = [badge(`缓存账号 ${payload.account_count}`, "neutral")];
      (overall.display_totals || []).forEach((item) => {
        overallBadges.push(badge(`总余额 ${formatDisplayValue(item.quota_display_total)}`, "ok"));
        overallBadges.push(badge(`总已用 ${formatDisplayValue(item.used_display_total)}`, "neutral"));
      });
      balanceOverallBadges.innerHTML = overallBadges.join("");

      const siteRows = (payload.site_summaries || []).filter((row) => state.activeSite === "all" || row.site === state.activeSite);
      if (state.activeSite === "all" && siteRows.length > 1) {
        siteRows.push({
          site: "总计",
          enabled: true,
          account_count: payload.account_count,
          quota_display_total: (overall.display_totals || []).map((item) => item.quota_display_total).join(" | "),
          used_display_total: (overall.display_totals || []).map((item) => item.used_display_total).join(" | "),
        });
      }

      renderTable(
        balanceSiteTable,
        [
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "状态", value: (row) => badge(row.enabled ? "启用" : "停用", row.enabled ? "ok" : "warn") },
          { label: "账号数", value: (row) => escapeHtml(row.account_count) },
          { label: "签到成功", value: (row) => escapeHtml(row.checkin_success_count ?? "-") },
          { label: "今日已签", value: (row) => escapeHtml(row.already_done_count ?? "-") },
          { label: "异常", value: (row) => escapeHtml(row.checkin_failed_count ?? "-") },
          { label: "余额($)", value: (row) => escapeHtml(formatDisplayValue(row.quota_display_total)) },
          { label: "已用($)", value: (row) => escapeHtml(formatDisplayValue(row.used_display_total)) },
        ],
        siteRows,
      );

      renderTable(
        balanceAccountTable,
        [
          { label: "站点", value: (row) => escapeHtml(row.site) },
          { label: "名称 / 账号", value: (row) => identityHtml(row) },
          { label: "类型 / 状态", value: (row) => `<div class="cell-stack">
            ${badge(row.provider === "sub2api" ? "sub2api 只读" : "NewAPI", row.provider === "sub2api" ? "neutral" : "ok")}
            ${badge(row.checkin_status || row.status, row.checkin_status_tone || (row.status?.includes("失败") ? "danger" : "ok"))}
          </div>` },
          { label: "余额 / 成本", value: (row) => `<div class="cell-stack">
            <strong>${escapeHtml(formatDisplayValue(row.quota_display))}</strong>
            <span class="dim">累计 ${escapeHtml(formatDisplayValue(row.used_quota_display))}</span>
          </div>` },
          { label: "套餐 / 到期", value: (row) => `<div class="cell-stack">
            <strong>${escapeHtml(row.provider_data?.plan_name || "-")}</strong>
            <span class="dim">${escapeHtml(row.provider_data?.expires_at || "-")}</span>
          </div>` },
          { label: "用量 / 模型", value: (row) => {
            const total = row.provider_data?.usage?.total;
            const models = row.provider_data?.models || [];
            return `<div class="cell-stack">
              <span class="dim">${total ? `${escapeHtml(total.requests ?? 0)} 请求 · ${escapeHtml(total.total_tokens ?? 0)} Token` : "-"}</span>
              ${models.length ? `<details><summary>${escapeHtml(models.length)} 个模型</summary><div class="model-list">${models.map(escapeHtml).join(" · ")}</div></details>` : '<span class="dim">-</span>'}
            </div>`;
          } },
          { label: "消息", value: (row) => `<span class="dim">${escapeHtml(row.checkin_message || row.message || "-")}</span>` },
        ],
        (payload.accounts || []).filter(matchesActiveSite),
      );
    }

    function renderAllViews() {
      renderSiteFilters();
      renderConfig(state.config);
      renderHistory(state.lastRun);
      renderBalances(state.balances);
      renderTrend(state.history);
      renderMonthly(state.monthly);
      renderOverview();
      setStats();
      updateProviderActions();
    }

    async function loadConfig() {
      state.config = await fetchJson("/api/config");
      renderConfigSelectors();
    }

    async function loadAPIKeys() {
      try {
        state.apiKeys = await fetchJson("/api/api-keys");
      } catch (error) {
        state.apiKeys = { load_error: true, accounts: [] };
        console.error("读取 API Key 摘要失败", error);
      }
      renderConfig(state.config);
    }

    async function refreshDirectoryKeys() {
      setButtonBusy(refreshDirectoryKeysBtn, true, "读取中");
      try {
        state.revealedGeneratedKeys.clear();
        await loadAPIKeys();
        setAppStatus("API Key 与分组信息已重新读取", "ok");
      } catch (error) {
        setAppStatus(`重新读取 API Key 与分组失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(refreshDirectoryKeysBtn, false);
      }
    }

    async function loadLastRun() {
      state.lastRun = await fetchJson("/api/last-run");
    }

    async function loadBalances() {
      state.balances = await fetchJson("/api/balances");
    }

    async function loadHistory() {
      state.history = await fetchJson("/api/history");
    }

    async function loadMonthly(month = monthlyMonthInput.value || currentMonthValue()) {
      const url = `/api/monthly?month=${encodeURIComponent(month)}`;
      state.monthly = await fetchJson(url);
      if (state.monthly?.selected_month) {
        monthlyMonthInput.value = state.monthly.selected_month;
      }
    }

    async function loadLocalState() {
      setAppStatus("正在读取 SQL 存储…", "busy");
      [reloadLocalBtn, refreshSiteBalanceBtn, syncSiteNamesBtn, singleRunBtn, fullRunBtn, loadMonthlyBtn, syncMonthlySiteBtn, syncMonthlyAccountBtn].forEach((button) => {
        setButtonBusy(button, true, "读取中");
      });
      try {
        state.apiKeys = null;
        await loadConfig();
        void loadAPIKeys();
        await loadLastRun();
        await loadBalances();
        await loadHistory();
        await loadMonthly();
        renderAllViews();
        setAppStatus(`本地缓存已读取 ${new Date().toLocaleTimeString("zh-CN", { hour12: false })}`, "ok");
      } catch (error) {
        setAppStatus(`读取本地缓存失败: ${error.message}`, "error");
      } finally {
        [reloadLocalBtn, refreshSiteBalanceBtn, syncSiteNamesBtn, singleRunBtn, fullRunBtn, loadMonthlyBtn, syncMonthlySiteBtn, syncMonthlyAccountBtn].forEach((button) => {
          setButtonBusy(button, false);
        });
        updateSelectedTargetCard();
      }
    }

    function selectedSiteForRemoteAction() {
      if (state.activeSite !== "all") {
        return state.activeSite;
      }
      return siteSelect.value;
    }

    async function refreshSiteBalance() {
      const site = selectedSiteForRemoteAction();
      if (!site) {
        setAppStatus("请先选择一个站点。", "error");
        return;
      }
      const readOnly = isReadOnlySite(site);
      setAppStatus(readOnly ? `正在读取 ${site} 的套餐、用量和模型…` : `正在刷新 ${site} 的余额与签到状态…`, "busy");
      setButtonBusy(refreshSiteBalanceBtn, true, "刷新中");
      try {
        const result = await fetchJson("/api/refresh-site-balances", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site }),
        });
        state.balances = result.balances;
        state.history = result.history || state.history;
        state.lastRun = result.last_run || state.lastRun;
        state.monthly = result.monthly || state.monthly;
        renderAllViews();
        setAppStatus(readOnly
          ? `${site} 只读数据已写入 SQL 缓存：账号 ${result.account_count}，异常 ${result.failed_count}`
          : `${site} 已写入 SQL：签到成功 ${result.success_count}，今日已签 ${result.already_done_count}，异常 ${result.failed_count}`,
        result.failed_count > 0 ? "error" : "ok");
        pulseRow(site, "", result.failed_count > 0 ? "error" : "ok");
      } catch (error) {
        setAppStatus(`刷新站点余额失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(refreshSiteBalanceBtn, false);
      }
    }

    async function syncSiteNames() {
      const site = selectedSiteForRemoteAction();
      if (!site) {
        setAppStatus("请先选择一个站点。", "error");
        return;
      }
      if (isReadOnlySite(site)) {
        setAppStatus(`${site} 是只读数据源，不支持名称同步。`, "warn");
        return;
      }
      setAppStatus(`正在同步 ${site} 的账号名称…`, "busy");
      setButtonBusy(syncSiteNamesBtn, true, "同步中");
      try {
        const result = await fetchJson("/api/sync-site-names", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site }),
        });
        state.balances = result.balances;
        await loadConfig();
        renderAllViews();
        setAppStatus(`${site} 名称同步完成：成功 ${result.success_count}，失败 ${result.failed_count}，${result.changed ? "配置已更新" : "没有变更"}`, result.failed_count > 0 ? "error" : "ok");
        pulseRow(site, "", result.failed_count > 0 ? "error" : "ok");
      } catch (error) {
        setAppStatus(`同步站点名称失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(syncSiteNamesBtn, false);
      }
    }

    async function refreshAccountBalance(site, userId, button) {
      const readOnly = isReadOnlySite(site);
      setAppStatus(readOnly ? `正在读取 ${site} / ${userId} 的套餐、用量和模型…` : `正在刷新 ${site} / ${userId} 的余额与签到状态…`, "busy");
      setButtonBusy(button, true, "刷新中");
      try {
        const result = await fetchJson("/api/refresh-account-balance", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId }),
        });
        state.balances = result.balances;
        state.history = result.history || state.history;
        state.lastRun = result.last_run || state.lastRun;
        state.monthly = result.monthly || state.monthly;
        renderAllViews();
        setAppStatus(`${site} / ${userId} 已写入 SQL：${readOnly ? (result.message || "只读数据已刷新") : (result.checkin?.checkin_status || result.message || "状态已刷新")}`, result.ok ? "ok" : "error");
        pulseRow(site, userId, result.ok ? "ok" : "error");
      } catch (error) {
        setAppStatus(`刷新账号余额失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function syncAccountName(site, userId, button) {
      setAppStatus(`正在同步 ${site} / ${userId} 的名称…`, "busy");
      setButtonBusy(button, true, "同步中");
      try {
        const result = await fetchJson("/api/sync-account-name", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId }),
        });
        state.balances = result.balances;
        await loadConfig();
        renderAllViews();
        setAppStatus(`${site} / ${userId} 名称同步完成，${result.changed ? "配置已更新" : "没有变更"}`, result.ok ? "ok" : "error");
        pulseRow(site, userId, result.changed ? "ok" : "pending");
      } catch (error) {
        setAppStatus(`同步账号名称失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function editAccountDisplayName(site, userId, button) {
      const account = getConfigAccount(site, userId);
      const displayName = window.prompt("输入该账号的用户名或邮箱", account?.display_name || account?.username || "");
      if (displayName === null) {
        return;
      }
      const normalized = displayName.trim();
      if (!normalized) {
        setAppStatus("用户名或邮箱不能为空。", "error");
        return;
      }

      setAppStatus(`正在保存 ${site} / ${userId} 的账号标识…`, "busy");
      setButtonBusy(button, true, "保存中");
      try {
        state.config = await fetchJson("/api/account-display-name", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId, display_name: normalized }),
        });
        await loadBalances();
        renderConfigSelectors();
        renderAllViews();
        setAppStatus(`${site} / ${userId} 的账号标识已保存为 ${normalized}`, "ok");
        pulseRow(site, userId, "ok");
      } catch (error) {
        setAppStatus(`保存账号标识失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function saveConfigJson() {
      let payload;
      try {
        payload = JSON.parse(configJsonInput.value || "{}");
      } catch (error) {
        setInlineStatus(configJsonStatus, `JSON 解析失败: ${error.message}`, "error");
        return;
      }

      setInlineStatus(configJsonStatus, "正在写入配置…", "busy");
      setButtonBusy(saveConfigJsonBtn, true, "写入中");
      try {
        const result = await fetchJson("/api/config-site", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        state.config = result.config;
        state.balances = result.balances;
        renderConfigSelectors();
        renderAllViews();
        setInlineStatus(configJsonStatus, `已写入 ${result.site_count} 个平台，新增账号 ${result.added_accounts}，更新账号 ${result.updated_accounts}`, "ok");
        setAppStatus("平台配置已写入 JSON", "ok");
      } catch (error) {
        setInlineStatus(configJsonStatus, `写入失败: ${error.message}`, "error");
        setAppStatus(`平台配置写入失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(saveConfigJsonBtn, false);
      }
    }

    async function toggleSiteEnabled(site, enabled, button) {
      if (!site) {
        setAppStatus("缺少要切换的站点。", "error");
        return;
      }
      const actionText = enabled ? "启用签到" : "禁用签到";
      if (!enabled) {
        const confirmed = window.confirm(`确认禁用 ${site} 的签到？后续全量签到、单账号补签和余额刷新里的签到查询都会跳过它。`);
        if (!confirmed) {
          return;
        }
      }

      setAppStatus(`正在${actionText}：${site}…`, "busy");
      setButtonBusy(button, true, enabled ? "启用中" : "禁用中");
      try {
        const result = await fetchJson("/api/site-enabled", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            site,
            enabled,
            disabled_reason: enabled ? "" : "页面禁用签到",
          }),
        });
        state.config = result.config;
        state.balances = result.balances || state.balances;
        renderConfigSelectors();
        renderAllViews();
        const message = `${site} 已${enabled ? "启用" : "禁用"}签到`;
        setInlineStatus(configJsonStatus, message, "ok");
        setAppStatus(message, "ok");
      } catch (error) {
        setInlineStatus(configJsonStatus, `${actionText}失败: ${error.message}`, "error");
        setAppStatus(`${actionText}失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function deleteSite(site, button) {
      if (!site) {
        setAppStatus("缺少要删除的站点。", "error");
        return;
      }
      const confirmed = window.confirm(`确认删除站点 ${site}？这会同步移除本地配置、余额缓存和历史趋势记录。`);
      if (!confirmed) {
        return;
      }

      setAppStatus(`正在删除 ${site} 的本地配置和缓存…`, "busy");
      setButtonBusy(button, true, "删除中");
      try {
        const result = await fetchJson("/api/delete-site", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site }),
        });
        state.config = result.config;
        state.balances = result.balances;
        state.history = result.history;
        state.lastRun = result.last_run || state.lastRun;
        if (state.activeSite === site) {
          state.activeSite = "all";
        }
        renderConfigSelectors();
        renderAllViews();
        const message = `已删除 ${site}：配置账号 ${result.removed_accounts} 个，缓存 ${result.removed_cache_accounts} 条，历史 ${result.removed_history_entries} 条`;
        setInlineStatus(configJsonStatus, message, "ok");
        setAppStatus(message, "ok");
      } catch (error) {
        setInlineStatus(configJsonStatus, `删除失败: ${error.message}`, "error");
        setAppStatus(`删除站点失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function deleteAccount(site, userId, label, button) {
      if (!site || !userId) {
        setAppStatus("缺少要删除的站点或账号。", "error");
        return;
      }
      const name = label || userId;
      const confirmed = window.confirm(`确认删除 ${site} / ${name}（ID ${userId}）？这会同步移除该账号的本地配置、余额缓存和历史记录。`);
      if (!confirmed) {
        return;
      }

      setAppStatus(`正在删除 ${site} / ${userId} 的本地配置和缓存…`, "busy");
      setButtonBusy(button, true, "删除中");
      try {
        const result = await fetchJson("/api/delete-account", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId }),
        });
        state.config = result.config;
        state.balances = result.balances;
        state.history = result.history;
        state.lastRun = result.last_run || state.lastRun;
        state.monthly = result.monthly || state.monthly;
        if (siteSelect.value === site && accountSelect.value === userId) {
          accountSelect.value = "";
        }
        renderConfigSelectors();
        renderAllViews();
        const message = `已删除 ${site} / ${userId}：缓存 ${result.removed_cache_accounts} 条，历史 ${result.removed_history_entries} 条，月度 ${result.removed_monthly_entries} 条`;
        setInlineStatus(configJsonStatus, message, "ok");
        setAppStatus(message, "ok");
      } catch (error) {
        setInlineStatus(configJsonStatus, `删除账号失败: ${error.message}`, "error");
        setAppStatus(`删除账号失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function refreshMonthlyStatus() {
      const status = await fetchJson("/api/monthly-sync-status");
      if (state.monthly) {
        state.monthly.sync_state = status;
      }
      renderMonthly(state.monthly);
      if (!status.running && state.monthlyStatusTimer) {
        window.clearInterval(state.monthlyStatusTimer);
        state.monthlyStatusTimer = null;
        await loadMonthly(monthlyMonthInput.value || currentMonthValue());
        await loadLastRun();
        await loadHistory();
        renderAllViews();
      }
    }

    function startMonthlyStatusPolling() {
      if (state.monthlyStatusTimer) {
        window.clearInterval(state.monthlyStatusTimer);
      }
      state.monthlyStatusTimer = window.setInterval(() => {
        refreshMonthlyStatus().catch((error) => {
          setInlineStatus(monthlySyncStatus, `月度状态读取失败: ${error.message}`, "error");
        });
      }, 1800);
    }

    async function loadSelectedMonthly() {
      setInlineStatus(monthlySyncStatus, "正在读取 SQL 月度记录…", "busy");
      setButtonBusy(loadMonthlyBtn, true, "读取中");
      try {
        await loadMonthly(monthlyMonthInput.value || currentMonthValue());
        renderAllViews();
        setInlineStatus(monthlySyncStatus, `已读取 ${state.monthly?.selected_month || monthlyMonthInput.value} 本地缓存`, "ok");
      } catch (error) {
        setInlineStatus(monthlySyncStatus, `读取月度缓存失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(loadMonthlyBtn, false);
      }
    }

    async function syncMonthly(scope) {
      const month = monthlyMonthInput.value || currentMonthValue();
      const site = scope === "site" || scope === "account" ? selectedSiteForRemoteAction() : "";
      const userId = scope === "account" ? accountSelect.value : "";
      const button = scope === "account" ? syncMonthlyAccountBtn : syncMonthlySiteBtn;
      if (!site) {
        setInlineStatus(monthlySyncStatus, "请先选择一个站点。", "error");
        return;
      }
      if (scope === "account" && !userId) {
        setInlineStatus(monthlySyncStatus, "请先选择一个账号。", "error");
        return;
      }
      if (isReadOnlySite(site)) {
        setInlineStatus(monthlySyncStatus, `${site} 是只读数据源，不支持月度签到同步。`, "warn");
        return;
      }

      setInlineStatus(monthlySyncStatus, `已提交 ${month} ${site}${userId ? ` / ${userId}` : ""} 月度同步，后台会慢慢查。`, "busy");
      setButtonBusy(button, true, "同步中");
      try {
        const result = await fetchJson("/api/sync-monthly", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ month, site, user_id: userId }),
        });
        state.monthly = result.monthly || state.monthly;
        renderAllViews();
        if (result.started) {
          startMonthlyStatusPolling();
        } else {
          setInlineStatus(monthlySyncStatus, "已有月度同步正在执行，继续查看当前状态。", "busy");
        }
      } catch (error) {
        setInlineStatus(monthlySyncStatus, `提交月度同步失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(button, false);
      }
    }

    async function refreshCheckinJobStatus() {
      const status = await fetchJson("/api/checkin-job-status");
      setInlineStatus(
        fullRunStatus,
        status.running
          ? `${status.message || "完整签到运行中"}，开始于 ${status.started_at || "-"}`
          : `${status.message || "完整签到空闲"}，退出码 ${status.exit_code ?? "-"}`,
        status.running ? "busy" : status.exit_code && status.exit_code !== 0 ? "error" : "ok",
      );
      if (!status.running && state.checkinStatusTimer) {
        window.clearInterval(state.checkinStatusTimer);
        state.checkinStatusTimer = null;
        await loadLastRun();
        await loadBalances();
        await loadHistory();
        await loadMonthly(currentMonthValue());
        renderAllViews();
      }
    }

    function startCheckinStatusPolling() {
      if (state.checkinStatusTimer) {
        window.clearInterval(state.checkinStatusTimer);
      }
      state.checkinStatusTimer = window.setInterval(() => {
        refreshCheckinJobStatus().catch((error) => {
          setInlineStatus(fullRunStatus, `完整签到状态读取失败: ${error.message}`, "error");
        });
      }, 2400);
    }

    async function runFullCheckin() {
      setInlineStatus(fullRunStatus, "正在提交完整签到到后台…", "busy");
      setButtonBusy(fullRunBtn, true, "已提交");
      try {
        const result = await fetchJson("/api/run-full-checkin", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        });
        setInlineStatus(fullRunStatus, result.started ? "完整签到已在后台运行。" : "完整签到已经在运行。", "busy");
        startCheckinStatusPolling();
      } catch (error) {
        setInlineStatus(fullRunStatus, `完整签到启动失败: ${error.message}`, "error");
      } finally {
        setButtonBusy(fullRunBtn, false);
      }
    }

    async function runSingleCheckin() {
      const site = siteSelect.value;
      const userId = accountSelect.value;
      if (!site || !userId) {
        singleRunStatus.textContent = "请先选择站点和账号。";
        singleRunStatus.dataset.tone = "error";
        return;
      }
      if (!isSiteEnabled(site)) {
        singleRunStatus.textContent = `${site} 已停用签到，不能执行单账号签到；可以继续刷新余额、查看签到记录和月度记录。`;
        singleRunStatus.dataset.tone = "warn";
        return;
      }
      if (isReadOnlySite(site)) {
        singleRunStatus.textContent = `${site} 是只读数据源，只能读取套餐、用量和模型，不能执行签到。`;
        singleRunStatus.dataset.tone = "warn";
        return;
      }

      singleRunBtn.disabled = true;
      singleRunStatus.textContent = `正在执行 ${site} / ${userId} 的签到…`;
      singleRunStatus.dataset.tone = "busy";
      singleRunOutput.textContent = "";
      setAppStatus(`正在执行 ${site} / ${userId} 的单账号签到…`, "busy");

      try {
        const result = await fetchJson("/api/checkin-single", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ site, user_id: userId }),
        });
        const report = result.report || {};
        singleRunStatus.textContent = `执行完成，退出码 ${result.exit_code}；成功 ${report.success_count ?? "-"}，失败 ${report.failed_count ?? "-"}`;
        singleRunStatus.dataset.tone = result.exit_code === 0 ? "ok" : "error";
        singleRunOutput.textContent = [
          "===== SUMMARY =====",
          report.summary_text || "无摘要",
          "",
          "===== STDOUT =====",
          result.stdout || "",
          "",
          "===== STDERR =====",
          result.stderr || "",
        ].join("\n");

        state.monthly = result.monthly || state.monthly;
        state.lastRun = result.last_run || state.lastRun;
        await loadBalances();
        await loadHistory();
        renderAllViews();
        setAppStatus(`单账号签到完成 ${new Date().toLocaleTimeString("zh-CN", { hour12: false })}`, "ok");
      } catch (error) {
        singleRunStatus.textContent = `执行失败: ${error.message}`;
        singleRunStatus.dataset.tone = "error";
        singleRunOutput.textContent = error.stack || error.message;
        setAppStatus(`单账号签到失败: ${error.message}`, "error");
      } finally {
        singleRunBtn.disabled = false;
      }
    }

    tabRow.addEventListener("click", (event) => {
      const button = event.target.closest("[data-tab]");
      if (!button) {
        return;
      }
      setActiveTab(button.dataset.tab);
    });

    reloadLocalBtn.addEventListener("click", loadLocalState);
    openPlatformDirectoryBtn.addEventListener("click", openPlatformDirectory);
    closePlatformDirectoryBtn.addEventListener("click", closePlatformDirectory);
    closeLoginCredentialBtn.addEventListener("click", closeLoginCredentials);
    testLoginCredentialBtn.addEventListener("click", () => submitLoginCredentials(false));
    loginCredentialForm.addEventListener("submit", (event) => {
      event.preventDefault();
      submitLoginCredentials(true);
    });
    refreshDirectoryKeysBtn.addEventListener("click", refreshDirectoryKeys);
    refreshSiteBalanceBtn.addEventListener("click", refreshSiteBalance);
    syncSiteNamesBtn.addEventListener("click", syncSiteNames);
    saveConfigJsonBtn.addEventListener("click", saveConfigJson);
    singleRunBtn.addEventListener("click", runSingleCheckin);
    fullRunBtn.addEventListener("click", runFullCheckin);
    loadMonthlyBtn.addEventListener("click", loadSelectedMonthly);
    syncMonthlySiteBtn.addEventListener("click", () => syncMonthly("site"));
    syncMonthlyAccountBtn.addEventListener("click", () => syncMonthly("account"));
    siteSelect.addEventListener("change", () => updateAccountOptions());
    accountSelect.addEventListener("change", updateSelectedTargetCard);
    trendScopeSelect.addEventListener("change", () => {
      state.trendScope = trendScopeSelect.value || "all";
      renderTrend(state.history);
    });
    monthlyTrendScopeSelect.addEventListener("change", () => {
      state.monthlyTrendScope = monthlyTrendScopeSelect.value || "all";
      renderMonthly(state.monthly);
    });
    monthlyMonthInput.addEventListener("change", loadSelectedMonthly);

    configJsonInput.value = defaultConfigJsonText();
    monthlyMonthInput.value = currentMonthValue();
    setActiveTab("overview");
    loadLocalState();

  return scope.cleanup
}
