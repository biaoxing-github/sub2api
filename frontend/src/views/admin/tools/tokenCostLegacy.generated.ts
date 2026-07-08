/* eslint-disable */
// @ts-nocheck
// 本文件由 frontend\public\token-cost\index.html 机械拆分生成，用于在管理端原生挂载旧工具页面。
import type { LegacyToolCleanup, LegacyToolScope } from './legacyToolRuntime'

export const tokenCostLegacyHeadHtml = "<link rel=\"icon\" href=\"data:,\">"
export const tokenCostLegacyStyles = ":root {\n      color-scheme: light;\n      --bg: #F5F5F7;\n      --surface: #FFFFFF;\n      --surface-solid: #FFFFFF;\n      --surface-soft: rgba(0, 0, 0, 0.04);\n      --surface-strong: rgba(255, 255, 255, 0.8);\n      --text: #1D1D1F;\n      --text-soft: #424245;\n      --muted: #86868B;\n      --line: rgba(0, 0, 0, 0.08);\n      --line-strong: rgba(0, 0, 0, 0.16);\n      --accent: #0066CC;\n      --accent-strong: #0055AB;\n      --accent-soft: rgba(0, 102, 204, 0.1);\n      --warn: #B25000;\n      --warn-soft: #FFF4E6;\n      --danger: #E30000;\n      --danger-soft: #FFEBEB;\n      --ok: #008000;\n      --ok-soft: #E6F5E6;\n      --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.04);\n      --shadow: 0 4px 24px rgba(0, 0, 0, 0.06);\n      --shadow-lg: 0 16px 48px rgba(0, 0, 0, 0.12);\n      --radius-sm: 8px;\n      --radius: 12px;\n      --radius-lg: 16px;\n      --mono: \"SF Mono\", \"ui-monospace\", \"JetBrains Mono\", Consolas, monospace;\n      --sans: -apple-system, BlinkMacSystemFont, \"SF Pro Text\", \"Inter\", \"Helvetica Neue\", Arial, sans-serif;\n      --spring: cubic-bezier(0.175, 0.885, 0.32, 1.1);\n      --ease: cubic-bezier(0.25, 1, 0.5, 1);\n    }\n\n    * { box-sizing: border-box; }\n\n    html { scroll-behavior: smooth; }\n\n    body {\n      margin: 0;\n      min-width: 320px;\n      overflow-x: hidden;\n      background-color: var(--bg);\n      color: var(--text);\n      font-family: var(--sans);\n      font-size: 14px;\n      line-height: 1.5;\n      -webkit-font-smoothing: antialiased;\n      -moz-osx-font-smoothing: grayscale;\n    }\n\n    button, input, select, textarea { font: inherit; }\n\n    button {\n      min-height: 36px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-sm);\n      background: var(--surface);\n      color: var(--text);\n      padding: 6px 14px;\n      cursor: pointer;\n      font-weight: 500;\n      transition: all 0.2s var(--ease);\n      box-shadow: var(--shadow-sm);\n    }\n\n    button:hover:not(:disabled) {\n      background: #F5F5F7;\n      border-color: var(--line-strong);\n    }\n\n    button:active:not(:disabled) {\n      transform: scale(0.96);\n    }\n\n    button:disabled {\n      opacity: 0.4;\n      cursor: not-allowed;\n      box-shadow: none;\n    }\n\n    button.primary {\n      border: none;\n      background: var(--accent);\n      color: #fff;\n    }\n\n    button.primary:hover:not(:disabled) {\n      background: var(--accent-strong);\n      border-color: transparent;\n    }\n\n    button.danger {\n      border-color: transparent;\n      background: var(--danger-soft);\n      color: var(--danger);\n      box-shadow: none;\n    }\n\n    button.danger:hover:not(:disabled) {\n      background: color-mix(in srgb, var(--danger-soft) 80%, var(--danger));\n    }\n\n    input, select, textarea {\n      width: 100%;\n      min-height: 36px;\n      padding: 6px 12px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-sm);\n      background: var(--surface);\n      color: var(--text);\n      transition: border-color 0.2s var(--ease), box-shadow 0.2s var(--ease);\n    }\n\n    input:focus, select:focus, textarea:focus, button:focus-visible {\n      outline: none;\n      border-color: var(--accent);\n      box-shadow: 0 0 0 3px var(--accent-soft);\n    }\n\n    a { color: var(--accent); text-decoration: none; }\n    a:hover { text-decoration: underline; }\n\n    code, pre { font-family: var(--mono); }\n\n    .shell {\n      max-width: 1440px;\n      margin: 0 auto;\n      padding: 32px 24px 64px;\n    }\n\n    .topbar {\n      display: grid;\n      grid-template-columns: minmax(0, 1fr) auto;\n      gap: 24px;\n      align-items: end;\n      margin-bottom: 32px;\n    }\n\n    h1 {\n      margin: 0;\n      font-size: clamp(24px, 4vw, 36px);\n      line-height: 1.1;\n      font-weight: 700;\n      letter-spacing: -0.02em;\n    }\n\n    .lead {\n      max-width: 800px;\n      margin: 8px 0 0;\n      color: var(--text-soft);\n      font-size: 15px;\n    }\n\n    .actions {\n      display: flex;\n      flex-wrap: wrap;\n      justify-content: flex-end;\n      gap: 12px;\n      align-items: center;\n    }\n\n    .recharge-field {\n      width: 180px;\n      color: var(--text-soft);\n      font-size: 13px;\n      font-weight: 500;\n    }\n\n    .recharge-field input {\n      min-height: 36px;\n      font-family: var(--mono);\n      font-weight: 600;\n      margin-top: 4px;\n    }\n\n    .section-switch {\n      display: inline-flex;\n      gap: 4px;\n      margin: 0 0 24px;\n      padding: 4px;\n      border-radius: calc(var(--radius-sm) + 4px);\n      background: var(--surface-soft);\n    }\n\n    .section-switch button {\n      min-height: 32px;\n      border: none;\n      background: transparent;\n      color: var(--text-soft);\n      padding: 4px 16px;\n      font-weight: 600;\n      box-shadow: none;\n      border-radius: var(--radius-sm);\n    }\n\n    .section-switch button.active {\n      background: var(--surface);\n      color: var(--text);\n      box-shadow: var(--shadow-sm);\n    }\n\n    .stats {\n      display: grid;\n      grid-template-columns: repeat(6, minmax(0, 1fr));\n      gap: 16px;\n      margin-bottom: 24px;\n    }\n\n    .stat, .panel, .editor-card {\n      border-radius: var(--radius);\n      background: var(--surface);\n      box-shadow: var(--shadow);\n      border: 1px solid var(--line);\n      transition: transform 0.3s var(--spring), box-shadow 0.3s var(--ease);\n    }\n\n    .stat {\n      padding: 16px;\n      display: flex;\n      flex-direction: column;\n    }\n\n    .stat:hover {\n      transform: translateY(-2px);\n      box-shadow: var(--shadow-lg);\n    }\n\n    .stat-label {\n      color: var(--text-soft);\n      font-size: 12px;\n      font-weight: 600;\n      text-transform: uppercase;\n      letter-spacing: 0.05em;\n    }\n\n    .stat-value {\n      margin-top: 8px;\n      font-size: clamp(20px, 2vw, 24px);\n      font-weight: 700;\n      line-height: 1.1;\n      letter-spacing: -0.01em;\n    }\n\n    .stat-note {\n      margin-top: auto;\n      padding-top: 8px;\n      color: var(--muted);\n      font-size: 12px;\n    }\n\n    .layout {\n      display: grid;\n      grid-template-columns: minmax(0, 1fr) minmax(320px, 380px);\n      gap: 24px;\n      align-items: start;\n    }\n\n    .workspace-main {\n      display: grid;\n      gap: 24px;\n      min-width: 0;\n    }\n\n    .workspace-side {\n      position: sticky;\n      top: 24px;\n      display: grid;\n      gap: 24px;\n      align-self: start;\n    }\n\n    .panel, .editor-card {\n      padding: 24px;\n      overflow: hidden;\n    }\n\n    .panel-head {\n      display: flex;\n      flex-wrap: wrap;\n      justify-content: space-between;\n      align-items: center;\n      gap: 16px;\n      margin-bottom: 20px;\n    }\n\n    .panel-title {\n      margin: 0;\n      font-size: 20px;\n      font-weight: 600;\n      letter-spacing: -0.01em;\n    }\n\n    .panel-meta {\n      color: var(--muted);\n      font-size: 13px;\n      margin-top: 4px;\n    }\n\n    .form-grid {\n      display: grid;\n      grid-template-columns: repeat(2, minmax(0, 1fr));\n      gap: 16px;\n    }\n\n    .field {\n      display: grid;\n      gap: 6px;\n      color: var(--text-soft);\n      font-size: 13px;\n      font-weight: 500;\n    }\n\n    .field.full { grid-column: 1 / -1; }\n\n    .section-view {\n      display: none;\n      gap: 24px;\n    }\n\n    .section-view.active {\n      display: grid;\n      animation: fadeIn 0.4s var(--ease);\n    }\n\n    @keyframes fadeIn {\n      from { opacity: 0; transform: translateY(4px); }\n      to { opacity: 1; transform: translateY(0); }\n    }\n\n    .toolbar {\n      display: flex;\n      flex-wrap: wrap;\n      gap: 12px;\n      align-items: center;\n    }\n\n    .segmented {\n      display: inline-flex;\n      padding: 3px;\n      border-radius: var(--radius-sm);\n      background: var(--surface-soft);\n    }\n\n    .segmented button {\n      min-height: 28px;\n      border: none;\n      background: transparent;\n      padding: 4px 12px;\n      color: var(--text-soft);\n      box-shadow: none;\n    }\n\n    .segmented button.active {\n      background: var(--surface);\n      color: var(--text);\n      box-shadow: var(--shadow-sm);\n    }\n\n    .platform-grid {\n      display: grid;\n      grid-template-columns: repeat(2, minmax(0, 1fr));\n      gap: 16px;\n    }\n\n    .platform-pager {\n      display: flex;\n      flex-wrap: wrap;\n      justify-content: space-between;\n      align-items: center;\n      margin-top: 24px;\n      padding-top: 16px;\n      border-top: 1px solid var(--line);\n      gap: 16px;\n    }\n\n    .pager-meta {\n      color: var(--muted);\n      font-size: 13px;\n    }\n\n    .pager-meta strong {\n      color: var(--text);\n      font-weight: 600;\n    }\n\n    .pager-buttons, .pager-pages {\n      display: flex;\n      gap: 8px;\n      align-items: center;\n    }\n\n    .pager-button, .pager-page {\n      min-height: 32px;\n      padding: 4px 10px;\n      box-shadow: none;\n      background: var(--surface-soft);\n      border: none;\n      font-weight: 500;\n      color: var(--text-soft);\n    }\n\n    .pager-page {\n      min-width: 32px;\n      font-family: var(--mono);\n    }\n\n    .pager-page.active {\n      background: var(--accent);\n      color: #fff;\n    }\n\n    .platform-card {\n      position: relative;\n      overflow: hidden;\n      border: 1px solid var(--line);\n      border-radius: var(--radius);\n      background: var(--surface);\n      padding: 20px;\n      display: grid;\n      gap: 16px;\n      transition: box-shadow 0.3s var(--ease), transform 0.3s var(--spring);\n    }\n\n    .platform-card:hover {\n      box-shadow: var(--shadow-lg);\n      transform: translateY(-2px);\n    }\n\n    .platform-card-head {\n      display: flex;\n      justify-content: space-between;\n      gap: 12px;\n      align-items: flex-start;\n    }\n\n    .platform-card-title {\n      display: grid;\n      gap: 6px;\n    }\n\n    .platform-title-line {\n      display: flex;\n      flex-wrap: wrap;\n      gap: 8px;\n      align-items: center;\n    }\n\n    .platform-card-name {\n      font-size: 16px;\n      font-weight: 600;\n      line-height: 1.2;\n    }\n\n    .score-badge {\n      display: inline-flex;\n      align-items: center;\n      min-height: 22px;\n      padding: 2px 8px;\n      border-radius: 999px;\n      background: var(--tier-bg, var(--surface-soft));\n      color: var(--tier-color, var(--text));\n      font-size: 12px;\n      font-weight: 600;\n      white-space: nowrap;\n    }\n\n    .tier-super .score-badge { --tier-bg: #E6F4EA; --tier-color: #137333; }\n    .tier-good .score-badge { --tier-bg: #E8F0FE; --tier-color: #1967D2; }\n    .tier-normal .score-badge { --tier-bg: #FEF7E0; --tier-color: #B06000; }\n    .tier-expensive .score-badge { --tier-bg: #FCE8E6; --tier-color: #C5221F; }\n    .tier-unknown .score-badge { --tier-bg: #F1F3F4; --tier-color: #5F6368; }\n\n    .platform-card-meta {\n      color: var(--muted);\n      font-size: 12px;\n    }\n\n    .platform-card-meta strong {\n      color: var(--text-soft);\n      font-weight: 600;\n    }\n\n    .platform-metrics {\n      display: grid;\n      grid-template-columns: repeat(4, minmax(0, 1fr));\n      gap: 8px;\n    }\n\n    .metric-chip {\n      padding: 8px;\n      border-radius: var(--radius-sm);\n      background: var(--surface-soft);\n    }\n\n    .metric-chip-label {\n      color: var(--muted);\n      font-size: 11px;\n      font-weight: 500;\n    }\n\n    .metric-chip-value {\n      margin-top: 4px;\n      font-family: var(--mono);\n      font-size: 12px;\n      font-weight: 600;\n      color: var(--text);\n    }\n\n    .power-console {\n      display: grid;\n      gap: 12px;\n      padding: 16px;\n      border-radius: var(--radius-sm);\n      background: var(--surface-soft);\n    }\n\n    .power-row {\n      display: grid;\n      grid-template-columns: 40px minmax(0, 1fr) 70px;\n      gap: 12px;\n      align-items: center;\n      font-size: 13px;\n      font-weight: 500;\n      color: var(--text-soft);\n    }\n\n    .power-row strong {\n      font-family: var(--mono);\n      font-weight: 600;\n      color: var(--text);\n      text-align: right;\n    }\n\n    .power-track {\n      height: 6px;\n      overflow: hidden;\n      border-radius: 999px;\n      background: var(--line);\n    }\n\n    .power-fill {\n      display: block;\n      height: 100%;\n      width: var(--bar-width, 0%);\n      border-radius: inherit;\n      background: var(--accent);\n      transition: width 0.6s var(--spring);\n    }\n\n    .console-split {\n      display: grid;\n      grid-template-columns: repeat(3, minmax(0, 1fr));\n      gap: 8px;\n      margin-top: 4px;\n    }\n\n    .console-pill {\n      font-size: 11px;\n      color: var(--muted);\n    }\n\n    .console-pill strong {\n      display: block;\n      margin-top: 2px;\n      font-family: var(--mono);\n      font-size: 12px;\n      color: var(--text);\n      white-space: nowrap;\n      overflow: hidden;\n      text-overflow: ellipsis;\n    }\n\n    .platform-fields {\n      display: grid;\n      grid-template-columns: repeat(4, minmax(0, 1fr));\n      gap: 12px;\n    }\n\n    .platform-card .field {\n      gap: 4px;\n      font-size: 12px;\n    }\n\n    .cell-input {\n      min-height: 30px;\n      padding: 4px 8px;\n      font-size: 13px;\n      border-radius: 6px;\n    }\n\n    .platform-card .field.full { grid-column: 1 / -1; }\n\n    .muted { color: var(--muted); }\n\n    .badge {\n      display: inline-flex;\n      align-items: center;\n      padding: 2px 8px;\n      border-radius: 999px;\n      background: var(--surface-soft);\n      color: var(--text-soft);\n      font-size: 12px;\n      font-weight: 600;\n    }\n\n    .badge.ok { background: var(--ok-soft); color: var(--ok); }\n    .badge.warn { background: var(--warn-soft); color: var(--warn); }\n    .badge.danger { background: var(--danger-soft); color: var(--danger); }\n\n    .rank-list { display: grid; gap: 12px; }\n\n    .rank-item {\n      display: grid;\n      grid-template-columns: 28px minmax(0, 1fr) auto;\n      gap: 12px;\n      align-items: center;\n      padding: 12px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-sm);\n      background: var(--surface);\n      transition: transform 0.2s var(--ease);\n    }\n\n    .rank-item:hover {\n      transform: translateX(4px);\n    }\n\n    .rank-index {\n      width: 28px;\n      height: 28px;\n      display: grid;\n      place-items: center;\n      border-radius: 50%;\n      background: var(--surface-soft);\n      color: var(--text-soft);\n      font-weight: 600;\n      font-size: 12px;\n    }\n\n    .rank-item:nth-child(1) .rank-index { background: #FFD700; color: #000; }\n    .rank-item:nth-child(2) .rank-index { background: #E0E0E0; color: #000; }\n    .rank-item:nth-child(3) .rank-index { background: #CD7F32; color: #000; }\n\n    .rank-name {\n      font-weight: 600;\n      font-size: 14px;\n    }\n\n    .rank-value {\n      font-family: var(--mono);\n      font-size: 13px;\n      font-weight: 600;\n      text-align: right;\n    }\n\n    .status {\n      padding: 12px 16px;\n      border-radius: var(--radius-sm);\n      background: var(--surface-soft);\n      color: var(--text-soft);\n      font-size: 13px;\n      line-height: 1.4;\n    }\n\n    .status.ok { background: var(--ok-soft); color: var(--ok); }\n    .status.warn { background: var(--warn-soft); color: var(--warn); }\n\n    .history-list, .event-list {\n      display: grid;\n      gap: 12px;\n      max-height: 400px;\n      overflow-y: auto;\n      padding-right: 8px;\n    }\n\n    .history-editor {\n      display: grid;\n      grid-template-columns: minmax(0, 1.5fr) repeat(4, minmax(0, 1fr)) auto;\n      gap: 12px;\n      align-items: end;\n      margin-bottom: 24px;\n      padding: 16px;\n      border-radius: var(--radius-sm);\n      background: var(--surface-soft);\n    }\n\n    .tgb-grid {\n      display: grid;\n      grid-template-columns: repeat(3, minmax(0, 1fr));\n      gap: 16px;\n    }\n\n    .group-card {\n      border: 1px solid var(--line);\n      border-radius: var(--radius-sm);\n      padding: 16px;\n      display: grid;\n      gap: 12px;\n    }\n\n    .group-title {\n      display: flex;\n      justify-content: space-between;\n      align-items: center;\n      font-weight: 600;\n      font-size: 15px;\n    }\n\n    .group-meta {\n      color: var(--muted);\n      font-size: 13px;\n      line-height: 1.5;\n    }\n\n    .history-item, .event-item {\n      padding: 16px;\n      border: 1px solid var(--line);\n      border-radius: var(--radius-sm);\n      background: var(--surface);\n    }\n\n    .history-title, .event-title {\n      display: flex;\n      justify-content: space-between;\n      align-items: center;\n      font-weight: 600;\n      font-size: 14px;\n      margin-bottom: 8px;\n    }\n\n    .history-body, .event-body {\n      color: var(--text-soft);\n      font-size: 13px;\n      line-height: 1.5;\n    }\n\n    .modal-shell {\n      position: fixed;\n      inset: 0;\n      display: none;\n      place-items: center;\n      padding: 24px;\n      background: rgba(0, 0, 0, 0.2);\n      backdrop-filter: blur(8px);\n      -webkit-backdrop-filter: blur(8px);\n      z-index: 100;\n    }\n\n    .modal-shell.open {\n      display: grid;\n    }\n\n    .modal-card {\n      width: min(640px, 100%);\n      max-height: min(90vh, 800px);\n      overflow-y: auto;\n      border-radius: var(--radius-lg);\n      background: var(--surface-solid);\n      box-shadow: 0 24px 80px rgba(0, 0, 0, 0.16);\n      padding: 32px;\n      transform: scale(0.96) translateY(12px);\n      opacity: 0;\n      transition: opacity 0.3s var(--ease), transform 0.4s var(--spring);\n    }\n\n    .modal-shell.open .modal-card {\n      opacity: 1;\n      transform: scale(1) translateY(0);\n    }\n\n    @media (prefers-reduced-motion: reduce) {\n      *, *::before, *::after {\n        animation-duration: 0.01ms !important;\n        transition-duration: 0.01ms !important;\n      }\n    }\n\n    .hidden { display: none !important; }\n\n    /* View Transitions Setup */\n    ::view-transition-old(root),\n    ::view-transition-new(root) {\n      animation-duration: 0.4s;\n      animation-timing-function: var(--spring);\n    }\n\n    @media (max-width: 1240px) {\n      .stats { grid-template-columns: repeat(3, minmax(0, 1fr)); }\n      .layout { grid-template-columns: 1fr; }\n      .workspace-side { position: static; }\n      .tgb-grid { grid-template-columns: 1fr; }\n      .history-editor { grid-template-columns: repeat(2, minmax(0, 1fr)); }\n      .platform-grid, .platform-fields, .platform-metrics, .console-split { grid-template-columns: repeat(2, minmax(0, 1fr)); }\n    }\n\n    @media (max-width: 760px) {\n      .shell { padding: 24px 16px; }\n      .topbar { grid-template-columns: 1fr; gap: 16px; margin-bottom: 24px; }\n      .actions { justify-content: flex-start; }\n      .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }\n      .form-grid { grid-template-columns: 1fr; }\n      .field.full { grid-column: auto; }\n      .section-switch { display: flex; width: 100%; }\n      .section-switch button { flex: 1; text-align: center; }\n      .history-editor { grid-template-columns: 1fr; }\n      .platform-grid, .platform-fields, .platform-metrics, .console-split { grid-template-columns: 1fr; }\n      .platform-card-head { flex-direction: column; }\n    }"
export const tokenCostLegacyBodyHtml = "<main class=\"shell\">\n    <header class=\"topbar\">\n      <div>\n        <h1>API Token 余额动态计算器</h1>\n        <p class=\"lead\">在这里直接改平台余额、plus/pro 倍率和兑换比例，页面会实时重算购买力与性价比；通过管理端打开时，数据会自动保存到数据库。</p>\n      </div>\n      <div class=\"actions\">\n        <label class=\"field recharge-field\">\n          个人充值总价 r\n          <input id=\"personalRechargeInput\" type=\"number\" step=\"0.01\" min=\"0\" inputmode=\"decimal\">\n        </label>\n        <button id=\"openReportBtn\">重新载入动态页</button>\n        <button id=\"saveDataBtn\">保存数据</button>\n      </div>\n    </header>\n\n    <div class=\"section-switch\" aria-label=\"TGB 分区\">\n      <button class=\"active\" id=\"viewTBtn\" type=\"button\">T</button>\n      <button id=\"viewGBtn\" type=\"button\">G</button>\n      <button id=\"viewBBtn\" type=\"button\">B</button>\n    </div>\n\n    <section class=\"stats\" id=\"statsGrid\"></section>\n\n    <section class=\"layout\">\n      <section class=\"workspace-main\">\n        <section class=\"section-view active\" data-view=\"t\">\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">平台编辑表</h2>\n                <div class=\"panel-meta\" id=\"tableMeta\">只展示余额大于 0 的平台。</div>\n              </div>\n              <div class=\"toolbar\">\n                <button id=\"openAddPlatformBtn\" class=\"primary\" type=\"button\">新增平台</button>\n                <label class=\"field\" style=\"min-width: 220px;\">\n                  搜索\n                  <input id=\"searchInput\" placeholder=\"平台名 / 备注\">\n                </label>\n                <label class=\"field\" style=\"min-width: 150px;\">\n                  显示\n                  <select id=\"visibilitySelect\">\n                    <option value=\"active\">余额大于 0</option>\n                    <option value=\"all\">全部平台</option>\n                  </select>\n                </label>\n                <button id=\"resetBtn\" class=\"danger\">恢复默认</button>\n              </div>\n            </div>\n            <div class=\"platform-grid\" id=\"platformTableBody\"></div>\n            <nav class=\"platform-pager\" id=\"platformPager\" aria-label=\"平台分页\"></nav>\n          </section>\n        </section>\n\n        <section class=\"section-view\" data-view=\"g\">\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">TGB 视图</h2>\n                <div class=\"panel-meta\">把平台按三种关注方式拆开看，避免一页塞满所有表格。</div>\n              </div>\n            </div>\n            <div class=\"tgb-grid\" id=\"tgbGrid\"></div>\n          </section>\n        </section>\n\n        <section class=\"section-view\" data-view=\"b\">\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">历史留痕</h2>\n                <div class=\"panel-meta\">历史点记录总余额和购买力涨跌；事件记录每次编辑。</div>\n              </div>\n              <div class=\"toolbar\">\n                <button id=\"copyMarkdownBtn\">复制报告</button>\n                <button id=\"clearEventsBtn\">清空事件</button>\n              </div>\n            </div>\n            <div class=\"history-editor\">\n              <label class=\"field\">\n                历史摘要\n                <input id=\"historySummaryInput\" placeholder=\"例如 更新 dawclaude 余额后重新记录\">\n              </label>\n              <label class=\"field\">\n                总余额\n                <input id=\"historyBalanceInput\" type=\"number\" step=\"0.01\" min=\"0\">\n              </label>\n              <label class=\"field\">\n                Pro 购买力\n                <input id=\"historyProInput\" type=\"number\" step=\"0.01\" min=\"0\">\n              </label>\n              <label class=\"field\">\n                Plus 购买力\n                <input id=\"historyPlusInput\" type=\"number\" step=\"0.01\" min=\"0\">\n              </label>\n              <label class=\"field\">\n                记录日期\n                <input id=\"historyAtInput\" placeholder=\"例如 2026-06-28\">\n              </label>\n              <button class=\"primary\" id=\"recordHistoryBtn\" type=\"button\">记录历史点</button>\n            </div>\n            <div class=\"history-list\" id=\"historyList\"></div>\n          </section>\n          <section class=\"panel\">\n            <div class=\"panel-head\">\n              <div>\n                <h2 class=\"panel-title\">编辑事件</h2>\n                <div class=\"panel-meta\">保留最近 200 条，便于回看倍率和余额改动。</div>\n              </div>\n            </div>\n            <div class=\"event-list\" id=\"eventList\"></div>\n          </section>\n        </section>\n      </section>\n\n      <aside class=\"workspace-side\">\n        <section class=\"panel\">\n          <div class=\"panel-head\">\n            <div>\n              <h2 class=\"panel-title\">性价比排行</h2>\n              <div class=\"panel-meta\">按每 1r 可买标准$排序。</div>\n            </div>\n            <div class=\"segmented\" aria-label=\"排行口径\">\n              <button class=\"active\" id=\"rankPlusBtn\" type=\"button\">Plus</button>\n              <button id=\"rankProBtn\" type=\"button\">Pro</button>\n            </div>\n          </div>\n          <div class=\"rank-list\" id=\"rankList\"></div>\n        </section>\n\n        <section class=\"panel\">\n          <div class=\"panel-head\">\n            <div>\n              <h2 class=\"panel-title\">操作状态</h2>\n              <div class=\"panel-meta\">管理端服务把数据写入数据库。</div>\n            </div>\n          </div>\n          <div class=\"status\" id=\"statusBox\">已加载默认数据，正在检测管理端服务。</div>\n        </section>\n      </aside>\n    </section>\n  </main>\n\n  <div class=\"modal-shell\" id=\"addPlatformModal\" aria-hidden=\"true\">\n    <section class=\"modal-card\" role=\"dialog\" aria-modal=\"true\" aria-labelledby=\"addPlatformTitle\">\n      <div class=\"panel-head\">\n        <div>\n          <h2 class=\"panel-title\" id=\"addPlatformTitle\">新增平台</h2>\n          <div class=\"panel-meta\">在弹窗里补余额、倍率和兑换比例，保存后会直接加入编辑区。</div>\n        </div>\n        <button id=\"closeAddPlatformBtn\" class=\"modal-close\" type=\"button\" aria-label=\"关闭新增平台弹窗\">关闭</button>\n      </div>\n      <form class=\"form-grid\" id=\"addPlatformForm\">\n        <label class=\"field full\">\n          平台名\n          <input id=\"newName\" required placeholder=\"例如 newapi\">\n        </label>\n        <label class=\"field\">\n          账面余额 $\n          <input id=\"newBalance\" type=\"number\" step=\"0.01\" min=\"0\" value=\"0\">\n        </label>\n        <label class=\"field\">\n          计算余额 $\n          <input id=\"newCalcBalance\" type=\"number\" step=\"0.01\" min=\"0\" placeholder=\"空=账面余额\">\n        </label>\n        <label class=\"field\">\n          r 数\n          <input id=\"newRateR\" type=\"number\" step=\"0.0001\" min=\"0\" value=\"1\">\n        </label>\n        <label class=\"field\">\n          换得 $\n          <input id=\"newRateUsd\" type=\"number\" step=\"0.0001\" min=\"0.0001\" value=\"1\">\n        </label>\n        <label class=\"field\">\n          plus 倍率\n          <input id=\"newPlus\" type=\"number\" step=\"0.0001\" min=\"0\" placeholder=\"可空\">\n        </label>\n        <label class=\"field\">\n          pro 倍率\n          <input id=\"newProMin\" type=\"number\" step=\"0.0001\" min=\"0\" placeholder=\"可空\">\n        </label>\n        <label class=\"field\">\n          pro 上限\n          <input id=\"newProMax\" type=\"number\" step=\"0.0001\" min=\"0\" placeholder=\"区间可填\">\n        </label>\n        <label class=\"field full\">\n          备注\n          <input id=\"newNote\" placeholder=\"例如 tokeness 折算口径\">\n        </label>\n        <button class=\"primary\" type=\"submit\">添加平台</button>\n      </form>\n    </section>\n  </div>"

export function mountTokenCostLegacyTool(scope: LegacyToolScope): LegacyToolCleanup {
  const document = scope.document
  const window = scope.window
  const localStorage = scope.localStorage
  const fetch = scope.fetch
  const Headers = scope.Headers
  const CSS = scope.CSS
  const structuredClone = scope.structuredClone
  const console = scope.console

    const STORAGE_KEY = "token-api-cost-live-calculator:v1";
    const REPORT_FILE = "token-api-cost-live-calculator.html";
    const AUTH_REFRESH_PATH = "/api/v1/auth/refresh";
    const DEFAULT_PERSONAL_RECHARGE_R = 333;
    const PLATFORM_PAGE_SIZE = 6;

    const defaultState = {
      version: 1,
      updatedAt: "2026-06-28T00:00:00+08:00",
      rankMode: "plus",
      personalRechargeR: DEFAULT_PERSONAL_RECHARGE_R,
      platforms: [
        { id: "torchai", name: "torchai", balanceUsd: 180, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: null, proMin: 0.3, proMax: null, note: "当前 pro 口径，已涨到 0.3" },
        { id: "okcodex", name: "okcodex", balanceUsd: 58.82, calcBalanceUsd: null, rateR: 7, rateUsd: 100, plus: 0.1, proMin: 0.21, proMax: null, note: "余额 58.82$，plus 0.1 / pro 0.21" },
        { id: "encore", name: "encore", balanceUsd: 1055, calcBalanceUsd: null, rateR: 1, rateUsd: 10, plus: 1.15, proMin: 2.38, proMax: null, note: "encore plus/pro 双池" },
        { id: "qingflow", name: "qingflow", balanceUsd: 12, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.09, proMin: 0.11, proMax: null, note: "余额已降到 12$" },
        { id: "devpool", name: "devpool", balanceUsd: 143.57, calcBalanceUsd: null, rateR: 1, rateUsd: 10, plus: null, proMin: 1.5, proMax: null, note: "" },
        { id: "zz1cc", name: "zz1cc", balanceUsd: 397, calcBalanceUsd: null, rateR: 36, rateUsd: 550, plus: null, proMin: 2.8, proMax: null, note: "涨价到 2.8 倍" },
        { id: "aisz", name: "aisz", balanceUsd: 64, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.1, proMin: 0.2, proMax: null, note: "plus/pro 双池" },
        { id: "dawclaude", name: "dawclaude", balanceUsd: 5.49, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.3, proMin: 0.35, proMax: 0.5, note: "pro 为区间倍率" },
        { id: "eirouter", name: "eirouter", balanceUsd: 60, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: null, proMin: 0.4, proMax: null, note: "" },
        { id: "tuling", name: "tuling", balanceUsd: 10.4, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.35, proMin: 0.45, proMax: null, note: "新增平台" },
        { id: "qianxing", name: "乾行", balanceUsd: 45, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.16, proMin: 0.3, proMax: null, note: "plus 0.16 / pro 0.3" },
        { id: "5yuan", name: "5yuan", balanceUsd: 10.91, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.08, proMin: 0.16, proMax: null, note: "" },
        { id: "xiaobai-code", name: "小白code", balanceUsd: 14.6, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.13, proMin: 0.18, proMax: null, note: "" },
        { id: "mikuapi", name: "mikuapi", balanceUsd: 20.17, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.3, proMin: 0.5, proMax: null, note: "" },
        { id: "superapi", name: "superapi", balanceUsd: 28, calcBalanceUsd: null, rateR: 1, rateUsd: 1, plus: 0.08, proMin: 0.22, proMax: null, note: "新增平台" },
        { id: "tokeness", name: "tokeness", balanceUsd: 75, calcBalanceUsd: 10.7142857143, rateR: 1, rateUsd: 1, plus: 0.03, proMin: 0.1, proMax: null, note: "75$ 原始余额；5m 输入或 30m 输出折算为 10.71$ 后计算购买力" }
      ],
      history: [
        { at: "2026-06-24", summary: "初始核算", totalBalance: 2758.64, proMin: 3600.27, proMax: 3600.27, plus: null },
        { at: "2026-06-24", summary: "torchai 调到 0.2 倍率", totalBalance: 2758.64, proMin: 3985.98, proMax: 3985.98, plus: null },
        { at: "2026-06-25", summary: "zz1cc 调到 2.8 倍率，torchai 恢复 0.35 倍率", totalBalance: 2758.64, proMin: 3493.93, proMax: 3493.93, plus: null },
        { at: "2026-06-26", summary: "encore/okcodex 涨倍率，qingflow 余额调整，foyeapi/funny 用完，eirouter/aisz 余额更新，乾行到 24.71$", totalBalance: 2755.45, proMin: 2874.02, proMax: 2874.02, plus: null },
        { at: "2026-06-26", summary: "aisz 调到 0.2 倍率，5yuan 调到 0.16 倍率", totalBalance: 2755.45, proMin: 2803.93, proMax: 2803.93, plus: null },
        { at: "2026-06-26", summary: "乾行调到 0.16 倍率且余额到 38$，torchai 降到 0.28 倍率", totalBalance: 2768.74, proMin: 2964.09, proMax: 2964.09, plus: null },
        { at: "2026-06-26", summary: "加入 plus/pro 双池口径，okcodex/qingflow/aisz 区分 plus，乾行 45$，eirouter 60$，新增 tuling", totalBalance: 2782.14, proMin: 3063.37, proMax: 3063.37, plus: null },
        { at: "2026-06-27", summary: "encore 改为 plus=1.15/pro=2.38，新增 superapi 与 tokeness，删除余额为 0 的平台展示", totalBalance: 2810.14, proMin: 2852.44, proMax: 2852.44, plus: null },
        { at: "2026-06-27", summary: "tokeness 加入 plus/pro 口径，更新 dawcode、mikuapi、乾行、5yuan、小白 的 plus/pro 倍率", totalBalance: 2885.14, proMin: 3408.39, proMax: 3413.1, plus: null },
        { at: "2026-06-27", summary: "tokeness 改为先按 5m/30m 折算 10.71$ 再套 plus/pro 倍率", totalBalance: 2885.14, proMin: 2765.53, proMax: 2770.24, plus: 3630.74 },
        { at: "2026-07-03", summary: "okcodex 余额调整为 58.82$，plus=0.1，pro=0.21", totalBalance: 2179.96, proMin: 2790.96, proMax: 2795.67, plus: 3631.25 },
        { at: "2026-07-03", summary: "torchai pro 倍率涨到 0.3", totalBalance: 2294.01, proMin: 2991.54, proMax: 3073.43, plus: 4030.52 }
      ],
      events: [
        { at: "2026/7/3 09:11:35", title: "编辑平台", detail: "okcodex：余额 764 -> 58.82，plus 1.3 -> 0.1，pro 3 -> 0.21" },
        { at: "2026/7/3 12:11:12", title: "编辑平台", detail: "torchai 的 proMin：0.25 -> 0.3" }
      ]
    };

    let state = loadState();
    let filterText = "";
    let visibility = "active";
    let serverAvailable = false;
    let remoteSaveTimer = null;
    let platformPage = 1;

    const elements = {
      statsGrid: document.querySelector("#statsGrid"),
      platformTableBody: document.querySelector("#platformTableBody"),
      platformPager: document.querySelector("#platformPager"),
      rankList: document.querySelector("#rankList"),
      personalRechargeInput: document.querySelector("#personalRechargeInput"),
      rankPlusBtn: document.querySelector("#rankPlusBtn"),
      rankProBtn: document.querySelector("#rankProBtn"),
      statusBox: document.querySelector("#statusBox"),
      historyList: document.querySelector("#historyList"),
      eventList: document.querySelector("#eventList"),
      searchInput: document.querySelector("#searchInput"),
      visibilitySelect: document.querySelector("#visibilitySelect"),
      tableMeta: document.querySelector("#tableMeta"),
      tgbGrid: document.querySelector("#tgbGrid"),
      historySummaryInput: document.querySelector("#historySummaryInput"),
      historyBalanceInput: document.querySelector("#historyBalanceInput"),
      historyProInput: document.querySelector("#historyProInput"),
      historyPlusInput: document.querySelector("#historyPlusInput"),
      historyAtInput: document.querySelector("#historyAtInput"),
      viewTBtn: document.querySelector("#viewTBtn"),
      viewGBtn: document.querySelector("#viewGBtn"),
      viewBBtn: document.querySelector("#viewBBtn"),
      addPlatformModal: document.querySelector("#addPlatformModal"),
      openAddPlatformBtn: document.querySelector("#openAddPlatformBtn"),
      closeAddPlatformBtn: document.querySelector("#closeAddPlatformBtn"),
      addPlatformForm: document.querySelector("#addPlatformForm")
    };

    let activeView = "t";

    function loadState() {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) {
        return structuredClone(defaultState);
      }
      try {
        const parsed = JSON.parse(raw);
        return normalizeState(parsed);
      } catch (error) {
        console.warn("读取本地数据失败，改用默认值", error);
        return structuredClone(defaultState);
      }
    }

    function normalizeState(source) {
      const merged = {
        ...structuredClone(defaultState),
        ...source
      };
      merged.platforms = Array.isArray(source.platforms) ? source.platforms.map(normalizePlatform) : structuredClone(defaultState.platforms);
      merged.history = Array.isArray(source.history) ? source.history : structuredClone(defaultState.history);
      merged.events = Array.isArray(source.events) ? source.events : [];
      merged.personalRechargeR = toPositive(source.personalRechargeR, DEFAULT_PERSONAL_RECHARGE_R);
      return merged;
    }

    function normalizePlatform(platform) {
      return {
        id: platform.id || makeId(platform.name || "platform"),
        name: platform.name || "未命名平台",
        balanceUsd: toNumber(platform.balanceUsd),
        calcBalanceUsd: isBlank(platform.calcBalanceUsd) ? null : toNumber(platform.calcBalanceUsd),
        rateR: toPositive(platform.rateR, 1),
        rateUsd: toPositive(platform.rateUsd, 1),
        plus: isBlank(platform.plus) ? null : toPositive(platform.plus, null),
        proMin: isBlank(platform.proMin) ? null : toPositive(platform.proMin, null),
        proMax: isBlank(platform.proMax) ? null : toPositive(platform.proMax, null),
        note: platform.note || ""
      };
    }

    function saveState(message, type = "ok", syncRemote = false) {
      state.updatedAt = new Date().toISOString();
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
      setStatus(message, type);
      if (syncRemote) {
        scheduleRemoteSave();
      }
    }

    function setStatus(message, type = "") {
      elements.statusBox.className = `status ${type}`.trim();
      elements.statusBox.textContent = message;
    }

    async function checkServer() {
      if (window.location.protocol === "file:") {
        setStatus("当前是 file:// 打开，只能本地计算；请用 scripts/token_cost_editor_server.py 启动动态服务后再保存数据。", "warn");
        return;
      }
      try {
        serverAvailable = await probeServer();
        if (serverAvailable) {
          await loadStateFromServer();
        } else {
          setStatus("管理端服务未就绪。", "warn");
        }
      } catch (error) {
        serverAvailable = false;
        setStatus("管理端服务未连接，页面仍可计算但不会保存到数据库。", "warn");
      }
    }

    async function loadStateFromServer() {
      const response = await requestJson("/api/v1/admin/token-cost/state?view=page", { method: "GET" });
      if (!response.ok) {
        throw new Error("读取数据库状态失败");
      }
      if (response.data) {
        state = normalizeState(response.data);
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
        render();
        setStatus("已从数据库载入数据，后续编辑会自动保存。", "ok");
        return;
      }
      setStatus("管理端服务已连接；当前数据库暂无状态，首次修改后会自动创建。", "ok");
    }

    function scheduleRemoteSave() {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = window.setTimeout(() => saveDataToServer(false), 600);
    }

    async function saveDataToServer(manual) {
      try {
        const response = await requestJson("/api/v1/admin/token-cost/state", {
          method: "POST",
          body: { state }
        });
        if (!response.ok || !response.data.ok) {
          throw new Error(response.data.error || "保存失败");
        }
        serverAvailable = true;
        setStatus(manual ? "已保存到数据库。" : "已自动保存到数据库。", "ok");
        pulseStats();
      } catch (error) {
        serverAvailable = false;
        setStatus(`保存失败：${error.message}`, "warn");
      }
    }

    async function probeServer() {
      const response = await requestJson("/api/v1/admin/token-cost/health", { method: "GET" });
      return Boolean(response.ok && response.data && response.data.ok);
    }

    async function requestJson(path, options = {}) {
      const method = options.method || "GET";
      const body = options.body === undefined ? null : options.body;
      const init = buildRequestInit(method, body);
      let response = await fetch(path, init);
      if (response.status === 401 && await refreshAuthToken()) {
        response = await fetch(path, buildRequestInit(method, body));
      }
      const data = await readResponseData(response);
      return { ok: response.ok, status: response.status, data };
    }

    function buildRequestInit(method, body) {
      const init = { method, headers: {} };
      const token = localStorage.getItem("auth_token");
      if (token) {
        init.headers["Authorization"] = `Bearer ${token}`;
      }
      if (method !== "GET" && body !== null) {
        init.headers["Content-Type"] = "application/json";
        init.body = JSON.stringify(body);
      }
      return init;
    }

    async function refreshAuthToken() {
      const refreshToken = localStorage.getItem("refresh_token");
      if (!refreshToken) {
        return false;
      }
      try {
        const response = await fetch(AUTH_REFRESH_PATH, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refreshToken })
        });
        if (!response.ok) {
          return false;
        }
        const payload = await response.json();
        const nextToken = payload?.data?.access_token;
        const nextRefreshToken = payload?.data?.refresh_token;
        const expiresIn = Number(payload?.data?.expires_in);
        if (payload?.code !== 0 || !nextToken || !nextRefreshToken || !Number.isFinite(expiresIn)) {
          return false;
        }
        localStorage.setItem("auth_token", nextToken);
        localStorage.setItem("refresh_token", nextRefreshToken);
        localStorage.setItem("token_expires_at", String(Date.now() + expiresIn * 1000));
        return true;
      } catch (error) {
        console.warn("刷新登录态失败，保留当前本地数据", error);
        return false;
      }
    }

    async function readResponseData(response) {
      const contentType = response.headers.get("Content-Type") || "";
      let data = null;
      if (contentType.includes("application/json")) {
        data = await response.json();
      } else if (contentType.includes("text/html")) {
        const text = await response.text();
        const match = text.match(/data-state='([^']*)'/);
        if (match && match[1]) {
          const decoded = match[1]
            .replace(/&#39;/g, "'")
            .replace(/&quot;/g, '"')
            .replace(/&gt;/g, ">")
            .replace(/&lt;/g, "<")
            .replace(/&amp;/g, "&");
          data = JSON.parse(decoded);
        }
      }
      return data;
    }

    function isBlank(value) {
      return value === null || value === undefined || value === "";
    }

    function toNumber(value) {
      const numeric = Number(value);
      return Number.isFinite(numeric) ? numeric : 0;
    }

    function toPositive(value, fallback) {
      const numeric = Number(value);
      if (!Number.isFinite(numeric) || numeric <= 0) {
        return fallback;
      }
      return numeric;
    }

    function money(value) {
      return `${format(value)}$`;
    }

    function rmb(value) {
      return `${format(value)}r`;
    }

    function format(value, digits = 2) {
      if (value === null || value === undefined || Number.isNaN(value)) {
        return "-";
      }
      return Number(value).toLocaleString("zh-CN", {
        minimumFractionDigits: digits,
        maximumFractionDigits: digits
      });
    }

    function formatCompact(value) {
      if (value === null || value === undefined || Number.isNaN(value)) {
        return "-";
      }
      const abs = Math.abs(value);
      const digits = abs >= 100 ? 2 : 4;
      return Number(value).toLocaleString("zh-CN", {
        minimumFractionDigits: 0,
        maximumFractionDigits: digits
      });
    }

    function formatRange(min, max, suffix = "") {
      if (min === null || max === null) {
        return "-";
      }
      if (Math.abs(min - max) < 0.00001) {
        return `${format(min)}${suffix}`;
      }
      return `${format(min)} ~ ${format(max)}${suffix}`;
    }

    function makeId(name) {
      const base = String(name).trim().toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, "-").replace(/^-|-$/g, "");
      const seed = base || "platform";
      let candidate = seed;
      let index = 2;
      while (state && state.platforms && state.platforms.some((platform) => platform.id === candidate)) {
        candidate = `${seed}-${index}`;
        index += 1;
      }
      return candidate;
    }

    function calcBalance(platform) {
      return platform.calcBalanceUsd === null ? platform.balanceUsd : platform.calcBalanceUsd;
    }

    function rPerUsd(platform) {
      return platform.rateUsd > 0 ? platform.rateR / platform.rateUsd : 0;
    }

    function poolMetrics(platform, pool) {
      const multiplier = pool === "plus" ? platform.plus : platform.proMin;
      const multiplierMax = pool === "pro" && platform.proMax ? platform.proMax : multiplier;
      if (!multiplier || !multiplierMax || multiplier <= 0 || multiplierMax <= 0) {
        return null;
      }
      const basis = calcBalance(platform);
      const lowPower = basis / Math.max(multiplier, multiplierMax);
      const highPower = basis / Math.min(multiplier, multiplierMax);
      // 购买力使用折算余额，成本使用账面余额，避免 tokeness 这类折算平台在每 1r 排行中抵消折算价。
      const comparableCost = platform.balanceUsd * rPerUsd(platform);
      return {
        powerMin: lowPower,
        powerMax: highPower,
        comparableCost,
        perStandardMin: comparableCost / highPower,
        perStandardMax: comparableCost / lowPower,
        standardPerRMin: comparableCost > 0 ? lowPower / comparableCost : 0,
        standardPerRMax: comparableCost > 0 ? highPower / comparableCost : 0
      };
    }

    function calculateTotals() {
      const active = state.platforms.filter((platform) => platform.balanceUsd > 0);
      const plusRows = active.map((platform) => poolMetrics(platform, "plus")).filter(Boolean);
      const proRows = active.map((platform) => poolMetrics(platform, "pro")).filter(Boolean);
      const totalBalance = active.reduce((sum, platform) => sum + platform.balanceUsd, 0);
      const comparableBalance = active.reduce((sum, platform) => sum + calcBalance(platform), 0);
      const bookCost = active.reduce((sum, platform) => sum + platform.balanceUsd * rPerUsd(platform), 0);
      // 汇总性价比按真实账面成本分母计算，折算余额只进入购买力分子。
      const comparableCost = bookCost;
      const plusPower = plusRows.reduce((sum, row) => sum + row.powerMax, 0);
      const proPowerMin = proRows.reduce((sum, row) => sum + row.powerMin, 0);
      const proPowerMax = proRows.reduce((sum, row) => sum + row.powerMax, 0);
      const personalRechargeR = toPositive(state.personalRechargeR, DEFAULT_PERSONAL_RECHARGE_R);
      return {
        activeCount: active.length,
        totalBalance,
        comparableBalance,
        bookCost,
        comparableCost,
        personalRechargeR,
        plusPower,
        proPowerMin,
        proPowerMax,
        bookMultiple: personalRechargeR > 0 ? bookCost / personalRechargeR : 0,
        plusPerR: comparableCost > 0 ? plusPower / comparableCost : 0,
        proPerRMin: comparableCost > 0 ? proPowerMin / comparableCost : 0,
        proPerRMax: comparableCost > 0 ? proPowerMax / comparableCost : 0
      };
    }

    function performRender() {
      const totals = calculateTotals();
      renderStats(totals);
      renderTable();
      renderRanks();
      renderTgb();
      renderHistory();
      renderEvents();
      renderActiveView();
      syncPersonalRechargeInput();
    }

    function render() {
      if (document.startViewTransition) {
        document.startViewTransition(() => performRender());
      } else {
        performRender();
      }
    }

    function renderStats(totals) {
      const cards = [
        ["有效平台", `${totals.activeCount}`, "余额大于 0"],
        ["账面余额", money(totals.totalBalance), "原始 $ 余额合计"],
        ["计算余额", money(totals.comparableBalance), "用于购买力折算"],
        ["Plus 购买力", `${format(totals.plusPower)} 标准$`, `约 1r=${formatCompact(totals.plusPerR)} 标准$`],
        ["Pro 购买力", `${formatRange(totals.proPowerMin, totals.proPowerMax)} 标准$`, `约 1r=${formatCompact(totals.proPerRMin)}~${formatCompact(totals.proPerRMax)} 标准$`],
        ["账面价值", rmb(totals.bookCost), `个人充值 ${rmb(totals.personalRechargeR)}，约 ${format(totals.bookMultiple)} 倍`]
      ];
      elements.statsGrid.innerHTML = cards.map(([label, value, note]) => `
        <article class="stat">
          <div class="stat-label">${escapeHtml(label)}</div>
          <div class="stat-value">${escapeHtml(value)}</div>
          <div class="stat-note">${escapeHtml(note)}</div>
        </article>
      `).join("");
    }

    function visiblePlatforms() {
      const query = filterText.trim().toLowerCase();
      return state.platforms.filter((platform) => {
        if (visibility === "active" && platform.balanceUsd <= 0) {
          return false;
        }
        if (!query) {
          return true;
        }
        return `${platform.name} ${platform.note}`.toLowerCase().includes(query);
      });
    }

    // 约束平台分页页码，筛选和删除后避免停在空白页。
    function clampPlatformPage(rowCount) {
      const pageCount = Math.max(1, Math.ceil(rowCount / PLATFORM_PAGE_SIZE));
      platformPage = clamp(platformPage, 1, pageCount);
      return pageCount;
    }

    function renderTable() {
      const rows = visiblePlatforms();
      const pageCount = clampPlatformPage(rows.length);
      const pageStart = (platformPage - 1) * PLATFORM_PAGE_SIZE;
      const pageRows = rows.slice(pageStart, pageStart + PLATFORM_PAGE_SIZE);
      const rangeStart = rows.length ? pageStart + 1 : 0;
      const rangeEnd = rows.length ? pageStart + pageRows.length : 0;
      elements.tableMeta.textContent = `当前页 ${rangeStart}-${rangeEnd} / ${rows.length} 个平台，全部数据 ${state.platforms.length} 个平台。`;
      if (rows.length === 0) {
        elements.platformTableBody.innerHTML = `<div class="muted">没有匹配的平台。</div>`;
        renderPlatformPager(rows.length, pageCount, rangeStart, rangeEnd);
        return;
      }
      elements.platformTableBody.innerHTML = pageRows.map((platform) => {
        const plus = poolMetrics(platform, "plus");
        const pro = poolMetrics(platform, "pro");
        const score = platformScore(plus, pro);
        const plusBar = score.maxPerR > 0 && plus ? clamp((plus.standardPerRMax / score.maxPerR) * 100, 6, 100) : 0;
        const proBar = score.maxPerR > 0 && pro ? clamp((pro.standardPerRMax / score.maxPerR) * 100, 6, 100) : 0;
        const tier = scoreTier(score.bestPerR);
        return `
          <article class="platform-card ${tier.className}" data-id="${escapeHtml(platform.id)}">
            <span class="scanline" aria-hidden="true"></span>
            <div class="platform-card-head">
              <div class="platform-card-title">
                <div class="platform-title-line">
                  <div class="platform-card-name">${escapeHtml(platform.name)}</div>
                  <span class="score-badge">${escapeHtml(tier.label)}</span>
                </div>
                <div class="platform-card-meta">
                  账面 <strong>${money(platform.balanceUsd)}</strong>，计算 <strong>${money(calcBalance(platform))}</strong>，
                  兑换 <strong>${formatCompact(platform.rateR)}r/${formatCompact(platform.rateUsd)}$</strong>
                </div>
              </div>
              <button class="danger" data-action="delete" type="button">删除</button>
            </div>
            <div class="power-console">
              <div class="power-row">
                <span>Plus</span>
                <span class="power-track"><span class="power-fill" style="--bar-width: ${plusBar}%"></span></span>
                <strong>${plus ? `1r=${formatCompact(plus.standardPerRMax)}` : "-"}</strong>
              </div>
              <div class="power-row">
                <span>Pro</span>
                <span class="power-track"><span class="power-fill" style="--bar-width: ${proBar}%"></span></span>
                <strong>${pro ? `1r=${formatCompact(pro.standardPerRMax)}` : "-"}</strong>
              </div>
              <div class="console-split">
                <div class="console-pill">余额成本<strong>${rmb(score.cost)}</strong></div>
                <div class="console-pill">最优倍率<strong>${score.bestMultiplier ? formatCompact(score.bestMultiplier) : "-"}</strong></div>
                <div class="console-pill">备注<strong>${escapeHtml(platform.note || "无")}</strong></div>
              </div>
            </div>
            <div class="platform-metrics">
              <div class="metric-chip">
                <div class="metric-chip-label">Plus 购买力</div>
                <div class="metric-chip-value">${plus ? format(plus.powerMax) : "-"}</div>
              </div>
              <div class="metric-chip">
                <div class="metric-chip-label">Pro 购买力</div>
                <div class="metric-chip-value">${pro ? formatRange(pro.powerMin, pro.powerMax) : "-"}</div>
              </div>
              <div class="metric-chip">
                <div class="metric-chip-label">每 1r Plus</div>
                <div class="metric-chip-value">${plus ? formatCompact(plus.standardPerRMax) : "-"}</div>
              </div>
              <div class="metric-chip">
                <div class="metric-chip-label">每 1r Pro</div>
                <div class="metric-chip-value">${pro ? formatRange(pro.standardPerRMin, pro.standardPerRMax) : "-"}</div>
              </div>
            </div>
            <div class="platform-fields">
              <label class="field">
                平台名
                <input class="cell-input" data-field="name" value="${escapeAttr(platform.name)}">
              </label>
              <label class="field">
                账面余额 $
                <input class="cell-input" data-field="balanceUsd" type="number" step="0.01" min="0" value="${platform.balanceUsd}">
              </label>
              <label class="field">
                计算余额 $
                <input class="cell-input" data-field="calcBalanceUsd" type="number" step="0.01" min="0" placeholder="同余额" value="${platform.calcBalanceUsd ?? ""}">
              </label>
              <label class="field">
                r 数
                <input class="cell-input" data-field="rateR" type="number" step="0.0001" min="0" value="${platform.rateR}">
              </label>
              <label class="field">
                换得 $
                <input class="cell-input" data-field="rateUsd" type="number" step="0.0001" min="0.0001" value="${platform.rateUsd}">
              </label>
              <label class="field">
                plus 倍率
                <input class="cell-input" data-field="plus" type="number" step="0.0001" min="0" value="${platform.plus ?? ""}">
              </label>
              <label class="field">
                pro 倍率
                <input class="cell-input" data-field="proMin" type="number" step="0.0001" min="0" value="${platform.proMin ?? ""}">
              </label>
              <label class="field">
                pro 上限
                <input class="cell-input" data-field="proMax" type="number" step="0.0001" min="0" value="${platform.proMax ?? ""}">
              </label>
              <label class="field" style="grid-column: 1 / -1;">
                备注
                <input class="cell-input" data-field="note" value="${escapeAttr(platform.note)}">
              </label>
            </div>
          </article>
        `;
      }).join("");
      renderPlatformPager(rows.length, pageCount, rangeStart, rangeEnd);
    }

    // 渲染分页控制台，保留页码、上一页/下一页和当前页范围提示。
    function renderPlatformPager(rowCount, pageCount, rangeStart, rangeEnd) {
      if (!rowCount) {
        elements.platformPager.classList.add("hidden");
        elements.platformPager.innerHTML = "";
        return;
      }
      elements.platformPager.classList.remove("hidden");
      const pages = Array.from({ length: pageCount }, (_, index) => index + 1);
      elements.platformPager.innerHTML = `
        <div class="pager-meta">
          <span>每页 ${PLATFORM_PAGE_SIZE} 个平台</span>
          <strong>${rangeStart}-${rangeEnd} / ${rowCount} · 第 ${platformPage}/${pageCount} 页</strong>
        </div>
        <div class="pager-pages" aria-label="页码">
          ${pages.map((page) => `
            <button class="pager-page ${page === platformPage ? "active" : ""}" data-page="${page}" type="button" ${page === platformPage ? 'aria-current="page"' : ""}>${page}</button>
          `).join("")}
        </div>
        <div class="pager-buttons">
          <button class="pager-button" data-page="${platformPage - 1}" type="button" ${platformPage <= 1 ? "disabled" : ""}>上一页</button>
          <button class="pager-button" data-page="${platformPage + 1}" type="button" ${platformPage >= pageCount ? "disabled" : ""}>下一页</button>
        </div>
      `;
    }

    function changePlatformPage(page) {
      const rows = visiblePlatforms();
      const pageCount = Math.max(1, Math.ceil(rows.length / PLATFORM_PAGE_SIZE));
      const nextPage = clamp(page, 1, pageCount);
      if (nextPage === platformPage) {
        return;
      }
      platformPage = nextPage;
      renderTable();
      pulsePager();
      pulseVisiblePlatforms();
    }

    function platformScore(plus, pro) {
      const rows = [plus, pro].filter(Boolean);
      const best = rows.reduce((winner, item) => {
        if (!winner || item.standardPerRMax > winner.standardPerRMax) {
          return item;
        }
        return winner;
      }, null);
      const maxPerR = rows.reduce((max, item) => Math.max(max, item.standardPerRMax || 0), 0);
      return {
        bestPerR: best ? best.standardPerRMax : 0,
        bestMultiplier: best ? best.multiplierMin : null,
        maxPerR,
        cost: best ? best.comparableCost : 0
      };
    }

    function scoreTier(bestPerR) {
      if (!bestPerR) {
        return { className: "tier-unknown", label: "未定价" };
      }
      if (bestPerR >= 12) {
        return { className: "tier-super", label: "超值" };
      }
      if (bestPerR >= 7) {
        return { className: "tier-good", label: "划算" };
      }
      if (bestPerR >= 4) {
        return { className: "tier-normal", label: "正常" };
      }
      return { className: "tier-expensive", label: "偏贵" };
    }

    function renderRanks() {
      const mode = state.rankMode || "plus";
      elements.rankPlusBtn.classList.toggle("active", mode === "plus");
      elements.rankProBtn.classList.toggle("active", mode === "pro");
      const ranked = state.platforms
        .filter((platform) => platform.balanceUsd > 0)
        .map((platform) => ({ platform, metrics: poolMetrics(platform, mode) }))
        .filter((row) => row.metrics && row.metrics.comparableCost > 0)
        .sort((a, b) => b.metrics.standardPerRMax - a.metrics.standardPerRMax)
        .slice(0, 8);

      if (ranked.length === 0) {
        elements.rankList.innerHTML = `<div class="muted">暂无可排行平台。</div>`;
        return;
      }

      elements.rankList.innerHTML = ranked.map((row, index) => `
        <div class="rank-item">
          <span class="rank-index">${index + 1}</span>
          <div>
            <div class="rank-name">${escapeHtml(row.platform.name)}</div>
            <div class="muted">${mode.toUpperCase()} ${formatRange(row.metrics.powerMin, row.metrics.powerMax)} 标准$</div>
          </div>
          <div class="rank-value">1r=${formatCompact(row.metrics.standardPerRMax)}</div>
        </div>
      `).join("");
    }

    function renderTgb() {
      const buckets = [
        {
          key: "t",
          title: "T",
          desc: "总览与汇总",
          rows: [
            `有效平台 ${state.platforms.filter((platform) => platform.balanceUsd > 0).length}`,
            `历史点 ${state.history.length}`,
            `编辑事件 ${state.events.length}`
          ]
        },
        {
          key: "g",
          title: "G",
          desc: "平台编辑组",
          rows: [
            `带 plus 的平台 ${state.platforms.filter((platform) => platform.plus !== null).length}`,
            `带 pro 的平台 ${state.platforms.filter((platform) => platform.proMin !== null).length}`,
            `零余额平台 ${state.platforms.filter((platform) => platform.balanceUsd <= 0).length}`
          ]
        },
        {
          key: "b",
          title: "B",
          desc: "历史留痕组",
          rows: [
            `最新记录 ${state.history.at(-1)?.at || "-"}`,
            `最新摘要 ${state.history.at(-1)?.summary || "-"}`,
            `最新事件 ${state.events.at(-1)?.title || "-"}`
          ]
        }
      ];
      elements.tgbGrid.innerHTML = buckets.map((bucket) => `
        <article class="group-card">
          <div class="group-title">
            <span>${escapeHtml(bucket.title)}</span>
            <span class="badge">${bucket.rows.length}</span>
          </div>
          <div class="group-meta">${escapeHtml(bucket.desc)}</div>
          <div class="group-meta">${bucket.rows.map((row) => `• ${escapeHtml(row)}`).join("<br>")}</div>
        </article>
      `).join("");
    }

    function renderHistory() {
      if (!state.history.length) {
        elements.historyList.innerHTML = `<div class="muted">还没有历史点。</div>`;
        return;
      }
      elements.historyList.innerHTML = state.history.slice().reverse().map((item, reverseIndex) => {
        const index = state.history.length - reverseIndex;
        const previous = state.history[index - 2];
        const balanceDelta = previous ? item.totalBalance - previous.totalBalance : null;
        const proDelta = previous ? item.proMin - previous.proMin : null;
        const plusText = item.plus === null || item.plus === undefined ? "未记录" : format(item.plus);
        return `
          <article class="history-item">
            <div class="history-title">
              <span>#${index} ${escapeHtml(item.summary)}</span>
              <span>${escapeHtml(item.at)}</span>
            </div>
            <div class="history-body">
              总余额 ${money(item.totalBalance)}，Pro ${formatRange(item.proMin, item.proMax)}，Plus ${plusText}；
              余额涨跌 ${balanceDelta === null ? "-" : signed(balanceDelta, "$")}，Pro 涨跌 ${proDelta === null ? "-" : signed(proDelta, "")}
            </div>
          </article>
        `;
      }).join("");
    }

    function renderEvents() {
      if (!state.events.length) {
        elements.eventList.innerHTML = `<div class="muted">还没有编辑事件。</div>`;
        return;
      }
      elements.eventList.innerHTML = state.events.slice().reverse().map((event) => `
        <article class="event-item">
          <div class="event-title">
            <span>${escapeHtml(event.title)}</span>
            <span>${escapeHtml(event.at)}</span>
          </div>
          <div class="event-body">${escapeHtml(event.detail)}</div>
        </article>
      `).join("");
    }

    function formatMultiplier(min, max) {
      if (!min) {
        return "-";
      }
      if (!max || Math.abs(min - max) < 0.00001) {
        return formatCompact(min);
      }
      return `${formatCompact(min)} ~ ${formatCompact(max)}`;
    }

    function md(value) {
      return String(value).replace(/\|/g, "\\|").replace(/\n/g, " ");
    }

    function signed(value, suffix) {
      const sign = value >= 0 ? "+" : "";
      return `${sign}${format(value)}${suffix}`;
    }

    function addEvent(title, detail) {
      state.events.push({
        at: new Date().toLocaleString("zh-CN", { hour12: false }),
        title,
        detail
      });
      if (state.events.length > 200) {
        state.events = state.events.slice(-200);
      }
    }

    function syncPersonalRechargeInput() {
      if (document.activeElement === elements.personalRechargeInput) {
        return;
      }
      elements.personalRechargeInput.value = toPositive(state.personalRechargeR, DEFAULT_PERSONAL_RECHARGE_R);
    }

    function updatePersonalRecharge(value) {
      const before = toPositive(state.personalRechargeR, DEFAULT_PERSONAL_RECHARGE_R);
      const next = value === "" ? 0 : Math.max(0, toNumber(value));
      if (String(before) === String(next)) {
        return;
      }
      state.personalRechargeR = next;
      addEvent("编辑个人充值", `个人充值总价：${before}r -> ${next}r`);
      saveState("已更新个人充值总价并重新计算。", "ok", true);
      render();
      pulseStats();
    }

    function updatePlatform(id, field, value) {
      const platform = state.platforms.find((item) => item.id === id);
      if (!platform) {
        return;
      }
      const before = platform[field];
      if (field === "name" || field === "note") {
        platform[field] = value;
      } else {
        platform[field] = value === "" ? null : toNumber(value);
        if (field === "balanceUsd") {
          platform[field] = Math.max(0, platform[field]);
        }
      }
      if (String(before ?? "") !== String(platform[field] ?? "")) {
        addEvent("编辑平台", `${platform.name} 的 ${field}：${before ?? "空"} -> ${platform[field] ?? "空"}`);
        saveState("已自动保存并重新计算。", "ok", true);
        render();
        pulsePlatformCard(id, "flash");
      }
    }

    function deletePlatform(id) {
      const platform = state.platforms.find((item) => item.id === id);
      if (!platform) {
        return;
      }
      const beforeTotals = calculateTotals();
      state.platforms = state.platforms.filter((item) => item.id !== id);
      const afterTotals = calculateTotals();
      const summary = `删除 ${platform.name}（${money(platform.balanceUsd)}）`;
      const at = new Date().toLocaleDateString("zh-CN").replaceAll("/", "-");
      state.history.push({
        at,
        summary,
        totalBalance: Number(afterTotals.totalBalance.toFixed(2)),
        proMin: Number(afterTotals.proPowerMin.toFixed(2)),
        proMax: Number(afterTotals.proPowerMax.toFixed(2)),
        plus: Number(afterTotals.plusPower.toFixed(2))
      });
      addEvent(
        "删除平台",
        `${platform.name} 已从动态表移除；总余额 ${signed(afterTotals.totalBalance - beforeTotals.totalBalance, "$")}，Plus ${signed(afterTotals.plusPower - beforeTotals.plusPower, "")}，Pro ${signed(afterTotals.proPowerMin - beforeTotals.proPowerMin, "")}`
      );
      addEvent("记录历史点", `${at} / ${summary}`);
      saveState("已删除平台、保存并记录历史点。", "warn", true);
      render();
      pulsePager();
      pulseStats();
    }

    function addPlatformFromForm(event) {
      event.preventDefault();
      const name = document.querySelector("#newName").value.trim();
      if (!name) {
        setStatus("平台名不能为空。", "warn");
        return;
      }
      const platform = normalizePlatform({
        id: makeId(name),
        name,
        balanceUsd: document.querySelector("#newBalance").value,
        calcBalanceUsd: document.querySelector("#newCalcBalance").value,
        rateR: document.querySelector("#newRateR").value,
        rateUsd: document.querySelector("#newRateUsd").value,
        plus: document.querySelector("#newPlus").value,
        proMin: document.querySelector("#newProMin").value,
        proMax: document.querySelector("#newProMax").value,
        note: document.querySelector("#newNote").value
      });
      state.platforms.push(platform);
      addEvent("新增平台", `${platform.name}，余额 ${money(platform.balanceUsd)}，plus=${platform.plus ?? "空"}，pro=${platform.proMin ?? "空"}`);
      event.target.reset();
      document.querySelector("#newRateR").value = 1;
      document.querySelector("#newRateUsd").value = 1;
      closeAddPlatformModal();
      saveState("已新增平台并重新计算。", "ok", true);
      const rows = visiblePlatforms();
      platformPage = Math.max(1, Math.ceil(rows.length / PLATFORM_PAGE_SIZE));
      render();
      pulsePager();
      pulsePlatformCard(platform.id, "flash");
    }

    function openAddPlatformModal() {
      elements.addPlatformModal.classList.add("open");
      elements.addPlatformModal.setAttribute("aria-hidden", "false");
      window.setTimeout(() => document.querySelector("#newName")?.focus(), 20);
    }

    function closeAddPlatformModal() {
      elements.addPlatformModal.classList.remove("open");
      elements.addPlatformModal.setAttribute("aria-hidden", "true");
      elements.openAddPlatformBtn.focus();
    }

    function recordHistory() {
      const summary = elements.historySummaryInput.value.trim() || "手动更新";
      const at = elements.historyAtInput.value.trim() || new Date().toLocaleDateString("zh-CN").replaceAll("/", "-");
      const totalBalance = toNumber(elements.historyBalanceInput.value);
      const proValue = toNumber(elements.historyProInput.value);
      const plusValue = toNumber(elements.historyPlusInput.value);
      const totals = calculateTotals();
      state.history.push({
        at,
        summary,
        totalBalance: totalBalance > 0 ? Number(totalBalance.toFixed(2)) : Number(totals.totalBalance.toFixed(2)),
        proMin: proValue > 0 ? Number(proValue.toFixed(2)) : Number(totals.proPowerMin.toFixed(2)),
        proMax: proValue > 0 ? Number(proValue.toFixed(2)) : Number(totals.proPowerMax.toFixed(2)),
        plus: plusValue > 0 ? Number(plusValue.toFixed(2)) : Number(totals.plusPower.toFixed(2))
      });
      addEvent("记录历史点", `${at} / ${summary}`);
      saveState("已记录历史点。", "ok", true);
      elements.historySummaryInput.value = "";
      elements.historyBalanceInput.value = "";
      elements.historyProInput.value = "";
      elements.historyPlusInput.value = "";
      elements.historyAtInput.value = "";
      render();
      pulseStats();
      pulseVisiblePlatforms();
    }

    async function copyMarkdown() {
      setStatus("当前版本不再提供 Markdown 复制。", "warn");
    }

    function resetToDefault() {
      if (!window.confirm("确认恢复默认数据？当前浏览器里的动态改动会被覆盖。")) {
        return;
      }
      state = structuredClone(defaultState);
      addEvent("恢复默认", "已恢复为 2026-06-28 初始动态页数据");
      saveState("已恢复默认数据。", "warn", true);
      render();
    }

    function escapeHtml(value) {
      return String(value).replace(/[&<>"']/g, (char) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      }[char]));
    }

    function escapeAttr(value) {
      return escapeHtml(value).replace(/`/g, "&#96;");
    }

    function clamp(value, min, max) {
      return Math.min(max, Math.max(min, value));
    }

    function pulsePlatformCard(id, mode = "pulse") {
      window.requestAnimationFrame(() => {
        const card = elements.platformTableBody.querySelector(`[data-id="${CSS.escape(id)}"]`);
        if (!card) {
          return;
        }
        card.classList.remove("flash", "pulse");
        void card.offsetWidth;
        card.classList.add(mode);
      });
    }

    function pulseVisiblePlatforms() {
      window.requestAnimationFrame(() => {
        const cards = Array.from(elements.platformTableBody.querySelectorAll(".platform-card"));
        cards.forEach((card, index) => {
          window.setTimeout(() => {
            card.classList.remove("pulse");
            void card.offsetWidth;
            card.classList.add("pulse");
          }, index * 35);
        });
      });
    }

    function pulseStats() {
      elements.statsGrid.classList.remove("pulse");
      void elements.statsGrid.offsetWidth;
      elements.statsGrid.classList.add("pulse");
    }

    function pulsePager() {
      elements.platformPager.classList.remove("pulse");
      void elements.platformPager.offsetWidth;
      elements.platformPager.classList.add("pulse");
    }

    elements.addPlatformForm.addEventListener("submit", addPlatformFromForm);
    elements.personalRechargeInput.addEventListener("change", (event) => updatePersonalRecharge(event.target.value));
    elements.personalRechargeInput.addEventListener("blur", () => syncPersonalRechargeInput());
    document.querySelector("#recordHistoryBtn").addEventListener("click", recordHistory);
    document.querySelector("#saveDataBtn").addEventListener("click", () => saveDataToServer(true));
    document.querySelector("#openReportBtn").addEventListener("click", () => {
      window.location.href = REPORT_FILE;
    });
    document.querySelector("#resetBtn").addEventListener("click", resetToDefault);
    document.querySelector("#copyMarkdownBtn").addEventListener("click", copyMarkdown);
    elements.openAddPlatformBtn.addEventListener("click", openAddPlatformModal);
    elements.closeAddPlatformBtn.addEventListener("click", closeAddPlatformModal);
    elements.addPlatformModal.addEventListener("click", (event) => {
      if (event.target === elements.addPlatformModal) {
        closeAddPlatformModal();
      }
    });
    document.querySelector("#clearEventsBtn").addEventListener("click", () => {
      state.events = [];
      saveState("已清空编辑事件。", "warn", true);
      render();
    });
    elements.searchInput.addEventListener("input", (event) => {
      filterText = event.target.value;
      platformPage = 1;
      renderTable();
      pulsePager();
    });
    elements.visibilitySelect.addEventListener("change", (event) => {
      visibility = event.target.value;
      platformPage = 1;
      renderTable();
      pulsePager();
    });
    elements.platformPager.addEventListener("click", (event) => {
      const button = event.target.closest("[data-page]");
      if (!button || button.disabled) {
        return;
      }
      changePlatformPage(Number(button.dataset.page));
    });
    elements.rankPlusBtn.addEventListener("click", () => {
      state.rankMode = "plus";
      saveState("已切换到 Plus 排行。", "ok", true);
      renderRanks();
      renderTgb();
    });
    elements.rankProBtn.addEventListener("click", () => {
      state.rankMode = "pro";
      saveState("已切换到 Pro 排行。", "ok", true);
      renderRanks();
      renderTgb();
    });
    elements.viewTBtn.addEventListener("click", () => setActiveView("t"));
    elements.viewGBtn.addEventListener("click", () => setActiveView("g"));
    elements.viewBBtn.addEventListener("click", () => setActiveView("b"));
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && elements.addPlatformModal.classList.contains("open")) {
        closeAddPlatformModal();
      }
    });
    elements.platformTableBody.addEventListener("change", (event) => {
      const row = event.target.closest("[data-id]");
      if (!row) {
        return;
      }
      const action = event.target.dataset.action;
      if (action === "delete") {
        deletePlatform(row.dataset.id);
        return;
      }
      const field = event.target.dataset.field;
      if (field) {
        updatePlatform(row.dataset.id, field, event.target.value);
      }
    });
    elements.platformTableBody.addEventListener("click", (event) => {
      const row = event.target.closest("[data-id]");
      if (row && event.target.dataset.action === "delete") {
        deletePlatform(row.dataset.id);
      }
    });

    render();
    checkServer();

    function setActiveView(view) {
      activeView = view;
      document.querySelectorAll(".section-view").forEach((section) => {
        section.classList.toggle("active", section.dataset.view === view);
      });
      elements.viewTBtn.classList.toggle("active", view === "t");
      elements.viewGBtn.classList.toggle("active", view === "g");
      elements.viewBBtn.classList.toggle("active", view === "b");
      renderTgb();
    }

    function renderActiveView() {
      document.querySelectorAll(".section-view").forEach((section) => {
        section.classList.toggle("active", section.dataset.view === activeView);
      });
    }

  return scope.cleanup
}
