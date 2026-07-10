<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountModelProbes.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" data-test="open-bazaarlink-probe-dialog" class="btn btn-primary px-3" @click="openBazaarLinkDialog">
              <Icon name="externalLink" size="sm" />
              <span class="ml-1.5">{{ t('admin.accountModelProbes.bazaarLinkProbe') }}</span>
            </button>
            <button type="button" data-test="open-batch-model-probe-dialog" class="btn btn-secondary px-3" @click="openBatchDialog">
              <Icon name="sparkles" size="sm" />
              <span class="ml-1.5">{{ t('admin.accountModelProbes.batchModelProbe') }}</span>
            </button>
          </div>
        </div>
        <form data-test="run-model-probe" class="grid gap-3 md:grid-cols-[160px_minmax(220px,1fr)_180px_180px_auto]" @submit.prevent="submitProbe">
          <input
            v-model.number="form.account_id"
            data-test="model-probe-account-id"
            type="number"
            min="1"
            class="input"
            :placeholder="t('admin.accountModelProbes.accountId')"
          />
          <input
            v-model="form.model"
            data-test="model-probe-model"
            type="text"
            class="input"
            :placeholder="t('admin.accountModelProbes.modelPlaceholder')"
          />
          <select
            v-model="form.request_mode"
            data-test="model-probe-request-mode"
            class="input"
          >
            <option value="non_stream">{{ t('admin.accountModelProbes.requestModes.non_stream') }}</option>
            <option value="stream">{{ t('admin.accountModelProbes.requestModes.stream') }}</option>
          </select>
          <input
            v-model.number="form.trusted_comparison_account_id"
            data-test="model-probe-trusted-account-id"
            type="number"
            min="1"
            class="input"
            :placeholder="t('admin.accountModelProbes.trustedComparisonAccountId')"
          />
          <button type="submit" class="btn btn-primary justify-center" :disabled="submitting">
            <Icon name="beaker" size="sm" :class="submitting ? 'animate-pulse' : ''" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.run') }}</span>
          </button>
        </form>
        <div v-if="message" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200">
          {{ message }}
        </div>
        <div v-if="error" class="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
          {{ error }}
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accountModelProbes.recentRuns') }}</div>
          <div class="flex flex-wrap items-center justify-end gap-2">
            <form data-test="model-probe-search" class="flex items-center gap-2" @submit.prevent="applyFilters">
              <input
                v-model="filters.keyword"
                data-test="model-probe-keyword"
                type="search"
                class="input h-9 w-56 text-sm"
                :placeholder="t('admin.accountModelProbes.keywordPlaceholder')"
              />
              <button type="submit" class="btn btn-secondary px-3 py-1.5 text-sm">
                {{ t('common.search') }}
              </button>
            </form>
            <button
              type="button"
              data-test="delete-selected-model-probe-runs"
              class="btn btn-danger px-3 py-1.5 text-sm"
              :disabled="selectedRunIds.length === 0 || deletingRuns"
              @click="deleteSelectedRuns"
            >
              <Icon name="trash" size="sm" :class="deletingRuns ? 'animate-pulse' : ''" />
              <span class="ml-1.5">{{ t('admin.accountModelProbes.deleteSelected', { count: selectedRunIds.length }) }}</span>
            </button>
            <button type="button" class="btn btn-ghost px-2 py-1 text-sm" :disabled="loading" @click="loadRuns">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[980px]">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="w-10 px-4 py-3 text-left">
                  <input
                    data-test="select-visible-model-probe-runs"
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                    :checked="allVisibleRunsSelected"
                    :disabled="selectableRuns.length === 0"
                    :aria-label="t('admin.accountModelProbes.selectAllRuns')"
                    @change="handleToggleAllVisibleRuns"
                  />
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.account') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.model') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.requestMode') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.probeSource') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.status') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.time') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && runs.length === 0">
                <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="runs.length === 0">
                <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.empty') }}</td>
              </tr>
              <tr v-for="run in runs" :key="run.id" class="border-t border-gray-100 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3">
                  <input
                    data-test="model-probe-run-select"
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                    :checked="selectedRunIdSet.has(run.id)"
                    :disabled="run.status === 'running'"
                    :aria-label="t('admin.accountModelProbes.selectRun', { id: run.id })"
                    @change="handleToggleRun(run.id, $event)"
                  />
                </td>
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ run.account_name || `#${run.account_id}` }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">#{{ run.account_id }}</div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ run.model || '-' }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatRequestMode(run.request_mode) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatProbeSource(run.probe_source) }}</td>
                <td class="px-4 py-3">
                  <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatRunScore(run) }}</div>
                  <div v-if="run.grade_label" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ run.grade_label }}</div>
                </td>
                <td class="px-4 py-3">
                  <span :class="statusClass(run.status)" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                    {{ formatStatus(run.status) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(run.created_at) }}</td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap items-center gap-2">
                    <button type="button" class="btn btn-ghost px-2 py-1 text-sm" :data-test="`model-probe-detail-${run.id}`" @click="loadDetail(run.id)">
                      <Icon name="eye" size="sm" />
                      <span class="ml-1">{{ t('common.view') }}</span>
                    </button>
                    <button
                      type="button"
                      class="btn btn-ghost px-2 py-1 text-sm text-rose-600 hover:bg-rose-50 dark:text-rose-300 dark:hover:bg-rose-950/30"
                      :data-test="`model-probe-delete-${run.id}`"
                      :disabled="run.status === 'running' || deletingRuns"
                      @click="deleteModelProbeRuns([run.id])"
                    >
                      <Icon name="trash" size="sm" :class="deletingRuns ? 'animate-pulse' : ''" />
                      <span class="ml-1">{{ t('admin.accountModelProbes.deleteRun') }}</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>

      <BaseDialog
        :show="detailDialogOpen"
        :title="t('admin.accountModelProbes.detailTitle')"
        width="extra-wide"
        @close="clearDetail"
      >
        <div v-if="detailLoading" class="text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
        <div v-else-if="detailError" class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">{{ detailError }}</div>
        <div v-else-if="detailRun" class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-4">
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.status') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatStatus(displayDetailRunStatus(detailRun)) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.probeSource') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatProbeSource(detailRun.probe_source) }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatRunScore(detailRun) }}</div>
              <div v-if="detailRun.grade_label" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ detailRun.grade_label }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.time') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatDateTime(detailRun.created_at) }}</div>
            </div>
          </div>

          <div v-for="sample in detailSamples" :key="sample.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ sample.label || sample.type || `#${sample.request_index}` }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ sample.model || detailRun.model || '-' }}</div>
              </div>
              <span :class="statusClass(displaySampleStatus(sample))" class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                {{ formatStatus(displaySampleStatus(sample)) }}
              </span>
            </div>
            <dl class="mt-3 grid gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-2 lg:grid-cols-4">
              <div v-if="sample.upstream_endpoint">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.upstreamEndpoint') }}:</dt>
                <dd class="inline break-all"> {{ sample.upstream_endpoint }}</dd>
              </div>
              <div v-if="sample.http_status">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.httpStatus') }}:</dt>
                <dd class="inline"> {{ sample.http_status }}</dd>
              </div>
              <div v-if="typeof sample.latency_ms === 'number'">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.latency') }}:</dt>
                <dd class="inline"> {{ formatDuration(sample.latency_ms) }}</dd>
              </div>
              <div v-if="typeof sample.first_token_ms === 'number'">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.firstToken') }}:</dt>
                <dd class="inline"> {{ formatDuration(sample.first_token_ms) }}</dd>
              </div>
              <div v-if="!isBazaarLinkSample(sample) && (sample.api_key_masked || sample.api_key_fingerprint)">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.apiKey') }}:</dt>
                <dd class="inline break-all"> {{ sample.api_key_masked || sample.api_key_fingerprint }}</dd>
              </div>
              <div v-if="typeof sample.input_tokens === 'number' || typeof sample.output_tokens === 'number' || typeof sample.tokens === 'number'" class="sm:col-span-2">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.tokens') }}:</dt>
                <dd class="inline"> {{ formatSampleTokens(sample) }}</dd>
              </div>
              <div v-if="shouldShowSampleError(sample)" class="sm:col-span-2">
                <dt class="inline font-medium">{{ t('admin.accountModelProbes.error') }}:</dt>
                <dd class="inline break-all"> {{ sample.error_code || '' }} {{ sample.error || sample.error_message || '' }}</dd>
              </div>
            </dl>
            <div v-if="hasBazaarLinkResult(sample)" class="mt-3 space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-950/30">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="text-sm font-semibold text-slate-900 dark:text-gray-100">{{ t('admin.accountModelProbes.bazaarLinkResult') }}</div>
                <span class="rounded-full bg-white px-2 py-0.5 text-xs font-semibold text-slate-700 dark:bg-dark-900 dark:text-gray-200">
                  {{ t('admin.accountModelProbes.runId') }} {{ bazaarLinkRunId(sample) }}
                </span>
              </div>
              <dl class="grid gap-3 text-xs text-slate-800 dark:text-gray-100 sm:grid-cols-2 lg:grid-cols-4">
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</dt>
                  <dd class="mt-1 text-sm font-semibold">{{ bazaarLinkScore(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.identityStatus') }}</dt>
                  <dd class="mt-1 text-sm font-semibold">{{ bazaarLinkIdentityStatus(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.confidence') }}</dt>
                  <dd class="mt-1 text-sm font-semibold">{{ bazaarLinkConfidence(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.status') }}</dt>
                  <dd class="mt-1 text-sm font-semibold">{{ bazaarLinkStatus(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.claimedModel') }}</dt>
                  <dd class="mt-1 break-all text-sm font-semibold">{{ bazaarLinkClaimedModel(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.predictedFamily') }}</dt>
                  <dd class="mt-1 text-sm font-semibold">{{ bazaarLinkPredictedFamily(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.v3fModel') }}</dt>
                  <dd class="mt-1 break-all text-sm font-semibold">{{ bazaarLinkV3F(sample) }}</dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.riskFlags') }}</dt>
                  <dd class="mt-1 break-all text-sm font-semibold">{{ bazaarLinkRiskFlags(sample) }}</dd>
                </div>
              </dl>
              <div v-if="bazaarLinkV3Candidates(sample).length" data-test="bazaarlink-v3-candidates" class="overflow-x-auto">
                <div class="mb-1 text-xs font-semibold text-slate-800 dark:text-gray-100">{{ t('admin.accountModelProbes.v3Candidates') }}</div>
                <table class="w-full min-w-[640px]">
                  <thead>
                    <tr>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.candidateName') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.candidateModel') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.candidateFamily') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.candidateScore') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="candidate in bazaarLinkV3Candidates(sample)" :key="bazaarLinkCandidateKey(candidate)" class="border-t border-gray-200 dark:border-dark-700">
                      <td class="px-2 py-2 text-sm font-semibold text-slate-900 dark:text-gray-100">{{ candidate.displayName || '-' }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ candidate.modelId || '-' }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ candidate.family || '-' }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ formatBazaarLinkCandidateScore(candidate) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-if="bazaarLinkItems(sample).length" class="overflow-x-auto">
                <div class="mb-1 text-xs font-semibold text-slate-800 dark:text-gray-100">{{ t('admin.accountModelProbes.probeItems') }}</div>
                <table class="w-full min-w-[760px]">
                  <thead>
                    <tr>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.probeId') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.evidence') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.group') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.passed') }}</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">TTFT</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">TPS</th>
                      <th class="px-2 py-1 text-left text-xs font-medium text-slate-500 dark:text-gray-400">{{ t('admin.accountModelProbes.response') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="item in bazaarLinkItems(sample)" :key="item.probeId || item.label" class="border-t border-gray-200 dark:border-dark-700">
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ item.probeId || '-' }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ item.label || '-' }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ item.group || '-' }}</td>
                      <td class="px-2 py-2 text-sm font-semibold text-slate-900 dark:text-gray-100">{{ formatBazaarLinkPassed(item.passed) }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ formatDuration(item.ttftMs) }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100">{{ formatNumber(item.tps) }}</td>
                      <td class="px-2 py-2 text-sm text-slate-900 dark:text-gray-100"><div class="max-h-20 overflow-auto whitespace-pre-wrap break-words">{{ item.response || '-' }}</div></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <pre v-if="!isBazaarLinkSample(sample) && sample.output_text && !sampleFailureResultDetails(sample).length" class="mt-3 max-h-40 overflow-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100">{{ sample.output_text }}</pre>
            <div v-if="sampleFailureReasons(sample).length" class="mt-3 rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200">
              <div class="text-xs font-semibold uppercase text-rose-700 dark:text-rose-200">{{ t('admin.accountModelProbes.failureReason') }}</div>
              <ul class="mt-2 space-y-1">
                <li v-for="reason in sampleFailureReasons(sample)" :key="reason" class="break-words">{{ reason }}</li>
              </ul>
            </div>
            <div v-if="sampleFailureResultDetails(sample).length" class="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100">
              <div class="text-xs font-semibold uppercase text-amber-800 dark:text-amber-100">{{ t('admin.accountModelProbes.modelResult') }}</div>
              <dl class="mt-2 space-y-2">
                <div v-for="detail in sampleFailureResultDetails(sample)" :key="detail.key">
                  <dt class="mb-1 text-xs font-medium text-amber-700 dark:text-amber-200">{{ detail.label }}</dt>
                  <dd>
                    <pre class="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-md bg-white/70 p-2 font-mono text-xs text-amber-900 dark:bg-dark-900/70 dark:text-amber-100">{{ detail.value }}</pre>
                  </dd>
                </div>
              </dl>
            </div>
            <div v-if="!isBazaarLinkSample(sample) && (sample.request_prompt || sample.request_body)" class="mt-3 space-y-3">
              <div v-if="sample.request_prompt">
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.accountModelProbes.requestPrompt') }}</div>
                <pre class="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-gray-50 p-3 font-mono text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-200">{{ sample.request_prompt }}</pre>
              </div>
              <div v-if="sample.request_body">
                <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.accountModelProbes.requestBody') }}</div>
                <pre class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-gray-50 p-3 font-mono text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-200">{{ sample.request_body }}</pre>
              </div>
            </div>
            <div v-if="sample.validation_evidence?.length" class="mt-3 overflow-x-auto">
              <table class="w-full min-w-[820px]">
                <thead>
                  <tr>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.evidence') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.expected') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.observed') }}</th>
                    <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.score') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="evidence in sample.validation_evidence" :key="evidence.key" class="border-t border-gray-100 dark:border-dark-700">
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">
                      <span :class="displayEvidencePassed(sample, evidence) ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'" class="font-medium">{{ evidence.label }}</span>
                      <dl v-if="hasEvidenceProbeDetails(evidence)" class="mt-1 grid gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-2">
                        <div v-if="evidence.category">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.evidenceCategory') }}:</dt>
                          <dd class="inline break-all"> {{ evidence.category }}</dd>
                        </div>
                        <div v-if="evidence.severity">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.evidenceSeverity') }}:</dt>
                          <dd class="inline break-all"> {{ evidence.severity }}</dd>
                        </div>
                        <div v-if="evidence.response_model">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.responseModel') }}:</dt>
                          <dd class="inline break-all"> {{ evidence.response_model }}</dd>
                        </div>
                        <div v-if="evidence.expected_model">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.expectedModel') }}:</dt>
                          <dd class="inline break-all"> {{ evidence.expected_model }}</dd>
                        </div>
                        <div v-if="typeof evidence.attempt_count === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.attemptCount') }}:</dt>
                          <dd class="inline"> {{ evidence.attempt_count }}</dd>
                        </div>
                        <div v-if="typeof evidence.retry_attempt_count === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.retryAttemptCount') }}:</dt>
                          <dd class="inline"> {{ evidence.retry_attempt_count }}</dd>
                        </div>
                        <div v-if="evidence.attempt_status_codes?.length" class="sm:col-span-2">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.attemptStatusCodes') }}:</dt>
                          <dd class="inline"> {{ formatEvidenceStatusCodes(evidence) }}</dd>
                        </div>
                        <div v-if="evidence.trusted_account_id">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.trustedAccount') }}:</dt>
                          <dd class="inline"> #{{ evidence.trusted_account_id }}</dd>
                        </div>
                        <div v-if="typeof evidence.similarity_percent === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.similarity') }}:</dt>
                          <dd class="inline"> {{ evidence.similarity_percent }}%</dd>
                        </div>
                        <div v-if="typeof evidence.pair_coverage_percent === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.pairCoverage') }}:</dt>
                          <dd class="inline"> {{ evidence.pair_coverage_percent }}%</dd>
                        </div>
                        <div v-if="typeof evidence.target_pass_rate_percent === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.targetPassRate') }}:</dt>
                          <dd class="inline"> {{ evidence.target_pass_rate_percent }}%</dd>
                        </div>
                        <div v-if="typeof evidence.trusted_pass_rate_percent === 'number'">
                          <dt class="inline font-medium">{{ t('admin.accountModelProbes.trustedPassRate') }}:</dt>
                          <dd class="inline"> {{ evidence.trusted_pass_rate_percent }}%</dd>
                        </div>
                      </dl>
                      <p v-if="displayEvidenceMessage(sample, evidence)" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ displayEvidenceMessage(sample, evidence) }}</p>
                    </td>
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">{{ evidence.expected || '-' }}</td>
                    <td class="px-2 py-2 text-sm text-gray-700 dark:text-gray-300">{{ evidence.observed || '-' }}</td>
                    <td class="px-2 py-2 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatSampleEvidenceScore(sample, evidence) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <template #footer>
          <button type="button" class="btn btn-secondary" @click="clearDetail">{{ t('common.close') }}</button>
        </template>
      </BaseDialog>

      <BaseDialog
        :show="bazaarLinkDialogOpen"
        :title="t('admin.accountModelProbes.bazaarLinkDialogTitle')"
        width="wide"
        @close="closeBazaarLinkDialog"
      >
        <form id="bazaarlink-probe-form" data-test="bazaarlink-probe-form" class="space-y-4" @submit.prevent="submitBazaarLinkProbe">
          <div class="grid gap-3 sm:grid-cols-[160px_minmax(220px,1fr)]">
            <input
              v-model.number="bazaarLinkForm.account_id"
              data-test="bazaarlink-probe-account-id"
              type="number"
              min="1"
              class="input"
              :placeholder="t('admin.accountModelProbes.accountId')"
            />
            <input
              v-model="bazaarLinkForm.model"
              data-test="bazaarlink-probe-model"
              type="text"
              class="input"
              :placeholder="t('admin.accountModelProbes.modelPlaceholder')"
            />
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-300">
              {{ t('admin.accountModelProbes.bazaarLinkMode') }}
            </div>
            <div data-test="bazaarlink-probe-mode" class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
              <button
                type="button"
                data-test="bazaarlink-probe-mode-quick"
                :class="bazaarLinkForm.mode === 'quick' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-300' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100'"
                class="rounded-md px-3 py-2 text-sm font-medium transition"
                @click="bazaarLinkForm.mode = 'quick'"
              >
                {{ t('admin.accountModelProbes.bazaarLinkModes.quick') }}
              </button>
              <button
                type="button"
                data-test="bazaarlink-probe-mode-full"
                :class="bazaarLinkForm.mode === 'full' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-300' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100'"
                class="rounded-md px-3 py-2 text-sm font-medium transition"
                @click="bazaarLinkForm.mode = 'full'"
              >
                {{ t('admin.accountModelProbes.bazaarLinkModes.full') }}
              </button>
            </div>
          </div>
          <div v-if="error" class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
            {{ error }}
          </div>
        </form>
        <template #footer>
          <button type="button" class="btn btn-secondary" @click="closeBazaarLinkDialog">{{ t('common.cancel') }}</button>
          <button
            type="button"
            data-test="bazaarlink-probe-submit"
            class="btn btn-primary"
            :disabled="bazaarLinkSubmitting"
            @click="submitBazaarLinkProbe"
          >
            <Icon name="externalLink" size="sm" :class="bazaarLinkSubmitting ? 'animate-pulse' : ''" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.bazaarLinkProbe') }}</span>
          </button>
        </template>
      </BaseDialog>

      <BaseDialog
        :show="batchDialogOpen"
        :title="t('admin.accountModelProbes.batchModelDialogTitle')"
        width="extra-wide"
        @close="closeBatchDialog"
      >
        <div class="space-y-4">
          <div class="grid gap-3 md:grid-cols-[minmax(220px,1fr)_180px_240px]">
            <input
              v-model="batchForm.model"
              data-test="batch-model-probe-model"
              type="text"
              class="input"
              :placeholder="t('admin.accountModelProbes.modelPlaceholder')"
            />
            <select
              v-model="batchForm.request_mode"
              data-test="batch-model-probe-request-mode"
              class="input"
            >
              <option value="non_stream">{{ t('admin.accountModelProbes.requestModes.non_stream') }}</option>
              <option value="stream">{{ t('admin.accountModelProbes.requestModes.stream') }}</option>
            </select>
            <select
              v-model.number="batchForm.trusted_comparison_account_id"
              data-test="batch-model-probe-trusted-account"
              class="input"
            >
              <option value="">{{ t('admin.accountModelProbes.noTrustedComparison') }}</option>
              <option v-for="account in batchAccounts" :key="account.id" :value="account.id">
                {{ account.name || `#${account.id}` }}
              </option>
            </select>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <input
              v-model="batchAccountSearch"
              data-test="batch-model-account-search"
              type="search"
              class="input max-w-xs"
              :placeholder="t('admin.accountModelProbes.batchAccountSearchPlaceholder')"
              @input="onBatchAccountSearch"
            />
            <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
              <button type="button" class="btn btn-ghost px-2 py-1 text-sm" @click="selectAllBatchAccounts">
                {{ t('admin.accountModelProbes.selectAll') }}
              </button>
              <button type="button" class="btn btn-ghost px-2 py-1 text-sm" @click="clearBatchAccountSelection">
                {{ t('admin.accountModelProbes.clearSelection') }}
              </button>
              <span>{{ t('admin.accountModelProbes.selectedAccounts', { count: selectedAccountIds.length }) }}</span>
            </div>
          </div>

          <div v-if="batchError" class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-950/30 dark:text-rose-200">
            {{ batchError }}
          </div>
          <div v-if="batchMessage" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200">
            {{ batchMessage }}
          </div>

          <div class="max-h-[420px] overflow-auto rounded-lg border border-gray-200 dark:border-dark-700">
            <div v-if="batchAccountsLoading" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
            <div v-else-if="batchAccounts.length === 0" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountModelProbes.batchAccountsEmpty') }}</div>
            <template v-else>
              <label
                v-for="account in batchAccounts"
                :key="account.id"
                class="flex items-start gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-dark-700"
              >
                <input
                  v-model="selectedAccountIds"
                  data-test="batch-account-select"
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600"
                  :value="account.id"
                />
                <span class="min-w-0 flex-1">
                  <span class="block font-medium text-gray-900 dark:text-gray-100">{{ account.name || `#${account.id}` }}</span>
                  <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">#{{ account.id }} · {{ account.platform }} · {{ account.type }}</span>
                </span>
              </label>
            </template>
          </div>
        </div>
        <template #footer>
          <button type="button" class="btn btn-secondary" @click="closeBatchDialog">{{ t('common.cancel') }}</button>
          <button
            type="button"
            data-test="batch-model-probe-submit"
            class="btn btn-primary"
            :disabled="selectedAccountIds.length === 0 || batchSubmitting"
            @click="submitBatchModelProbe"
          >
            <Icon name="sparkles" size="sm" :class="batchSubmitting ? 'animate-pulse' : ''" />
            <span class="ml-1.5">{{ t('admin.accountModelProbes.batchModelProbe') }}</span>
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { batchAccountModelProbeRuns, createAccountModelProbeRun, createBazaarLinkModelProbeRun, deleteAccountProbeRuns, getAccountProbeRun, list as listAccounts, listAccountProbeRuns } from '@/api/admin/accounts'
import type { Account, AccountProbeRequestMode, AccountProbeRun, AccountProbeSample, AccountProbeValidationEvidence } from '@/types'

const { t } = useI18n()

interface SampleFailureResultDetail {
  key: string
  label: string
  value: string
}

interface BazaarLinkModelCandidate {
  displayName?: string
  modelId?: string
  family?: string
  score?: number
}

interface BazaarLinkCandidateGroup {
  candidates?: BazaarLinkModelCandidate[]
  topCandidates?: BazaarLinkModelCandidate[]
  matches?: BazaarLinkModelCandidate[]
}

interface BazaarLinkIdentityAssessment {
  status?: string
  confidence?: number
  claimedModel?: string
  predictedFamily?: string
  subModelMatchV3F?: {
    modelId?: string
    score?: number
  } | null
  riskFlags?: string[]
  v3?: BazaarLinkCandidateGroup | BazaarLinkModelCandidate[] | null
  v3Candidates?: BazaarLinkModelCandidate[]
  candidates?: BazaarLinkModelCandidate[]
}

interface BazaarLinkProbeItem {
  probeId?: string
  label?: string
  group?: string
  passed?: boolean | 'warning' | null
  response?: string
  ttftMs?: number
  tps?: number
}

interface BazaarLinkProbeResult {
  runId?: string
  status?: string
  score?: number
  identityAssessment?: BazaarLinkIdentityAssessment
  items?: BazaarLinkProbeItem[]
  totalInputTokens?: number | null
  totalOutputTokens?: number | null
}

interface BazaarLinkObservedFields {
  status?: string
  confidence?: number
  family?: string
  v3f?: string
  flags?: string[]
}

const runs = ref<AccountProbeRun[]>([])
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const message = ref('')
const detailRun = ref<AccountProbeRun | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailDialogOpen = ref(false)
const deletingRuns = ref(false)
const batchDialogOpen = ref(false)
const bazaarLinkDialogOpen = ref(false)
const bazaarLinkSubmitting = ref(false)
const batchSubmitting = ref(false)
const batchAccountsLoading = ref(false)
const batchError = ref('')
const batchMessage = ref('')
const batchAccounts = ref<Account[]>([])
const selectedRunIds = ref<number[]>([])
const selectedAccountIds = ref<number[]>([])
const batchAccountSearch = ref('')
const defaultOpenAIAccountTestModelID = 'gpt-5.6-terra'

const filters = reactive({
  keyword: '',
})

const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

const form = reactive({
  account_id: undefined as number | undefined,
  model: defaultOpenAIAccountTestModelID,
  request_mode: 'non_stream' as AccountProbeRequestMode,
  trusted_comparison_account_id: undefined as number | undefined,
})

const batchForm = reactive({
  model: defaultOpenAIAccountTestModelID,
  request_mode: 'non_stream' as AccountProbeRequestMode,
  trusted_comparison_account_id: undefined as number | undefined,
})

const bazaarLinkForm = reactive({
  account_id: undefined as number | undefined,
  model: defaultOpenAIAccountTestModelID,
  mode: 'quick' as 'quick' | 'full',
})

const detailSamples = computed<AccountProbeSample[]>(() => detailRun.value?.samples || [])
const selectedRunIdSet = computed(() => new Set(selectedRunIds.value))
const selectableRuns = computed(() => runs.value.filter(run => run.status !== 'running'))
const allVisibleRunsSelected = computed(() => selectableRuns.value.length > 0 && selectableRuns.value.every(run => selectedRunIdSet.value.has(run.id)))

let listAbortController: AbortController | null = null
let submitAbortController: AbortController | null = null
let detailAbortController: AbortController | null = null
let deleteAbortController: AbortController | null = null
let bazaarLinkSubmitAbortController: AbortController | null = null
let batchAccountsAbortController: AbortController | null = null
let batchSubmitAbortController: AbortController | null = null
let batchSearchTimer: ReturnType<typeof setTimeout> | null = null

async function loadRuns() {
  listAbortController?.abort()
  const controller = new AbortController()
  listAbortController = controller
  loading.value = true
  error.value = ''
  try {
    const response = await listAccountProbeRuns(pagination.page, pagination.page_size, {
      mode: 'model_validation',
      keyword: filters.keyword.trim() || undefined,
      sort_by: 'created_at',
      sort_order: 'desc',
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    runs.value = response.items || []
    pruneSelectedRuns()
    pagination.total = response.total || 0
    pagination.page = response.page || pagination.page
    pagination.page_size = response.page_size || pagination.page_size
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoad')
    runs.value = []
    selectedRunIds.value = []
    pagination.total = 0
  } finally {
    if (listAbortController === controller) {
      loading.value = false
      listAbortController = null
    }
  }
}

function applyFilters() {
  pagination.page = 1
  loadRuns()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRuns()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRuns()
}

function pruneSelectedRuns() {
  const selectableIDs = new Set(selectableRuns.value.map(run => run.id))
  selectedRunIds.value = selectedRunIds.value.filter(id => selectableIDs.has(id))
}

function toggleRun(runId: number, checked: boolean) {
  const ids = new Set(selectedRunIds.value)
  if (checked) {
    ids.add(runId)
  } else {
    ids.delete(runId)
  }
  selectedRunIds.value = Array.from(ids)
}

function handleToggleRun(runId: number, event: Event) {
  toggleRun(runId, (event.target as HTMLInputElement).checked)
}

function toggleAllVisibleRuns(checked: boolean) {
  const ids = new Set(selectedRunIds.value)
  for (const run of selectableRuns.value) {
    if (checked) {
      ids.add(run.id)
    } else {
      ids.delete(run.id)
    }
  }
  selectedRunIds.value = Array.from(ids)
}

function handleToggleAllVisibleRuns(event: Event) {
  toggleAllVisibleRuns((event.target as HTMLInputElement).checked)
}

function deleteSelectedRuns() {
  deleteModelProbeRuns(selectedRunIds.value)
}

async function deleteModelProbeRuns(runIds: number[]) {
  const ids = Array.from(new Set(runIds.filter(id => id > 0)))
  if (ids.length === 0 || deletingRuns.value) return
  if (!window.confirm(t('admin.accountModelProbes.deleteConfirm'))) return

  deleteAbortController?.abort()
  const controller = new AbortController()
  deleteAbortController = controller
  deletingRuns.value = true
  error.value = ''
  message.value = ''
  try {
    const result = await deleteAccountProbeRuns(ids, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    selectedRunIds.value = selectedRunIds.value.filter(id => !ids.includes(id))
    if (detailRun.value && ids.includes(detailRun.value.id)) {
      clearDetail()
    }
    message.value = t('admin.accountModelProbes.deleteSucceeded', {
      count: result.deleted_count,
      skipped: result.skipped_running_count,
    })
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.deleteFailed')
  } finally {
    if (deleteAbortController === controller) {
      deletingRuns.value = false
      deleteAbortController = null
    }
  }
}

async function submitProbe() {
  if (submitting.value) return
  if (!form.account_id || form.account_id <= 0) {
    error.value = t('admin.accountModelProbes.accountRequired')
    return
  }
  submitAbortController?.abort()
  const controller = new AbortController()
  submitAbortController = controller
  submitting.value = true
  error.value = ''
  message.value = ''
  try {
    const payload = {
      account_id: form.account_id,
      model: form.model.trim() || undefined,
      request_mode: form.request_mode,
    } as {
      account_id: number
      model?: string
      request_mode?: AccountProbeRequestMode
      trusted_comparison_account_id?: number
    }
    const trustedAccountId = positiveAccountId(form.trusted_comparison_account_id)
    if (trustedAccountId) {
      payload.trusted_comparison_account_id = trustedAccountId
    }
    await createAccountModelProbeRun(payload, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    message.value = t('admin.accountModelProbes.started')
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToSubmit')
  } finally {
    if (submitAbortController === controller) {
      submitting.value = false
      submitAbortController = null
    }
  }
}

async function loadBatchAccounts() {
  batchAccountsAbortController?.abort()
  const controller = new AbortController()
  batchAccountsAbortController = controller
  batchAccountsLoading.value = true
  batchError.value = ''
  try {
    const response = await listAccounts(1, 100, {
      platform: 'openai',
      type: 'apikey',
      search: batchAccountSearch.value.trim() || undefined,
      sort_by: 'name',
      sort_order: 'asc',
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    batchAccounts.value = response.items || []
    selectedAccountIds.value = batchAccounts.value.map(account => account.id)
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    batchError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoadAccounts')
    batchAccounts.value = []
    selectedAccountIds.value = []
  } finally {
    if (batchAccountsAbortController === controller) {
      batchAccountsLoading.value = false
      batchAccountsAbortController = null
    }
  }
}

async function submitBatchModelProbe() {
  if (selectedAccountIds.value.length === 0 || batchSubmitting.value) return
  batchSubmitAbortController?.abort()
  const controller = new AbortController()
  batchSubmitAbortController = controller
  batchSubmitting.value = true
  batchError.value = ''
  batchMessage.value = ''
  try {
    const payload = {
      account_ids: selectedAccountIds.value,
      model: batchForm.model.trim() || undefined,
      request_mode: batchForm.request_mode,
    } as {
      account_ids: number[]
      model?: string
      request_mode?: AccountProbeRequestMode
      trusted_comparison_account_id?: number
    }
    const trustedAccountId = positiveAccountId(batchForm.trusted_comparison_account_id)
    if (trustedAccountId) {
      payload.trusted_comparison_account_id = trustedAccountId
    }
    const response = await batchAccountModelProbeRuns(payload, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    const skippedCount = response.skipped_count || 0
    batchMessage.value = skippedCount > 0
      ? t('admin.accountModelProbes.batchModelAcceptedWithSkipped', {
        count: response.accepted_count,
        skipped: skippedCount,
      })
      : t('admin.accountModelProbes.batchModelAccepted', { count: response.accepted_count })
    if (skippedCount === 0) {
      batchDialogOpen.value = false
    }
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    batchError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.batchModelFailed')
  } finally {
    if (batchSubmitAbortController === controller) {
      batchSubmitting.value = false
      batchSubmitAbortController = null
    }
  }
}

async function submitBazaarLinkProbe() {
  if (bazaarLinkSubmitting.value) return
  if (!bazaarLinkForm.account_id || bazaarLinkForm.account_id <= 0) {
    error.value = t('admin.accountModelProbes.accountRequired')
    return
  }
  bazaarLinkSubmitAbortController?.abort()
  const controller = new AbortController()
  bazaarLinkSubmitAbortController = controller
  bazaarLinkSubmitting.value = true
  error.value = ''
  message.value = ''
  try {
    await createBazaarLinkModelProbeRun({
      account_id: bazaarLinkForm.account_id,
      model: bazaarLinkForm.model.trim() || undefined,
      mode: bazaarLinkForm.mode,
    }, {
      signal: controller.signal,
    })
    if (controller.signal.aborted) return
    message.value = t('admin.accountModelProbes.bazaarLinkStarted')
    bazaarLinkDialogOpen.value = false
    await loadRuns()
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    error.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.bazaarLinkFailed')
  } finally {
    if (bazaarLinkSubmitAbortController === controller) {
      bazaarLinkSubmitting.value = false
      bazaarLinkSubmitAbortController = null
    }
  }
}

function openBazaarLinkDialog() {
  bazaarLinkDialogOpen.value = true
  error.value = ''
  message.value = ''
}

function closeBazaarLinkDialog() {
  bazaarLinkDialogOpen.value = false
  bazaarLinkSubmitAbortController?.abort()
}

function openBatchDialog() {
  batchDialogOpen.value = true
  batchError.value = ''
  batchMessage.value = ''
  if (batchAccounts.value.length === 0) {
    loadBatchAccounts()
  } else {
    selectedAccountIds.value = batchAccounts.value.map(account => account.id)
  }
}

function closeBatchDialog() {
  batchDialogOpen.value = false
  batchAccountsAbortController?.abort()
  batchSubmitAbortController?.abort()
}

function selectAllBatchAccounts() {
  selectedAccountIds.value = batchAccounts.value.map(account => account.id)
}

function clearBatchAccountSelection() {
  selectedAccountIds.value = []
}

function positiveAccountId(value: number | string | undefined): number | undefined {
  const numeric = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : undefined
}

function onBatchAccountSearch() {
  if (batchSearchTimer) {
    clearTimeout(batchSearchTimer)
  }
  batchSearchTimer = setTimeout(() => {
    loadBatchAccounts()
  }, 250)
}

async function loadDetail(runId: number) {
  detailAbortController?.abort()
  const controller = new AbortController()
  detailAbortController = controller
  detailDialogOpen.value = true
  detailLoading.value = true
  detailError.value = ''
  try {
    detailRun.value = await getAccountProbeRun(runId, {
      signal: controller.signal,
    })
  } catch (err: any) {
    if (controller.signal.aborted || err?.code === 'ERR_CANCELED') return
    detailError.value = err?.response?.data?.error || err?.message || t('admin.accountModelProbes.failedToLoadDetail')
  } finally {
    if (detailAbortController === controller) {
      detailLoading.value = false
      detailAbortController = null
    }
  }
}

function clearDetail() {
  detailAbortController?.abort()
  detailDialogOpen.value = false
  detailRun.value = null
  detailLoading.value = false
  detailError.value = ''
}

function formatNumber(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(value % 1 === 0 ? 0 : 1) : '-'
}

function formatRunScore(run: AccountProbeRun): string {
  const displayScore = run.probe_source === 'bazaarlink_api' ? run.display_score : undefined
  return formatNumber(typeof displayScore === 'number' && Number.isFinite(displayScore) ? displayScore : run.score)
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatRequestMode(value: string | undefined): string {
  if (!value) return '-'
  if (value === 'quick' || value === 'full') {
    return t(`admin.accountModelProbes.bazaarLinkModes.${value}`)
  }
  return t(`admin.accountModelProbes.requestModes.${value}`)
}

function formatProbeSource(value: string | undefined): string {
  if (value === 'bazaarlink_api') return t('admin.accountModelProbes.probeSources.bazaarlink_api')
  return t('admin.accountModelProbes.probeSources.self_validation')
}

function formatStatus(value: string | undefined): string {
  if (!value) return '-'
  return t(`admin.accountModelProbes.statuses.${value}`)
}

function formatEvidenceScore(evidence: AccountProbeValidationEvidence): string {
  return `${formatNumber(evidence.score)} / ${formatNumber(evidence.max_score)}`
}

function formatSampleEvidenceScore(sample: AccountProbeSample, evidence: AccountProbeValidationEvidence): string {
  if (isBazaarLinkSample(sample) && evidence.key === 'bazaarlink_identity') {
    const displayScore = bazaarLinkDisplayScoreValue(sample, evidence)
    if (typeof displayScore === 'number') {
      return `${formatNumber(displayScore)} / ${formatNumber(evidence.max_score)}`
    }
  }
  return formatEvidenceScore(evidence)
}

function formatEvidenceStatusCodes(evidence: AccountProbeValidationEvidence): string {
  return evidence.attempt_status_codes?.join(', ') || '-'
}

function formatDuration(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${formatNumber(value)} ms` : '-'
}

function formatSampleTokens(sample: AccountProbeSample): string {
  const parts = [
    `${t('admin.accountModelProbes.inputTokens')} ${formatNumber(sample.input_tokens)}`,
    `${t('admin.accountModelProbes.outputTokens')} ${formatNumber(sample.output_tokens)}`,
    `${t('admin.accountModelProbes.totalTokens')} ${formatNumber(sample.tokens)}`,
  ]
  return parts.join(' / ')
}

function parseBazaarLinkResult(sample: AccountProbeSample): BazaarLinkProbeResult | null {
  if (!isBazaarLinkSample(sample) || !sample.response_body) return null
  try {
    const parsed = JSON.parse(sample.response_body) as BazaarLinkProbeResult
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function numberValue(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number.parseFloat(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function findJsonPropertyIndex(source: string, property: string, startIndex = 0): number {
  return source.indexOf(`"${property}"`, Math.max(0, startIndex))
}

function parseJsonStringAfterProperty(source: string | null | undefined, property: string, startIndex = 0): string {
  if (!source) return ''
  const propertyIndex = findJsonPropertyIndex(source, property, startIndex)
  if (propertyIndex < 0) return ''
  const colonIndex = source.indexOf(':', propertyIndex + property.length + 2)
  if (colonIndex < 0) return ''
  let valueIndex = colonIndex + 1
  while (valueIndex < source.length && /\s/.test(source[valueIndex])) valueIndex += 1
  if (source[valueIndex] !== '"') return ''

  let escaped = false
  for (let index = valueIndex + 1; index < source.length; index += 1) {
    const char = source[index]
    if (escaped) {
      escaped = false
      continue
    }
    if (char === '\\') {
      escaped = true
      continue
    }
    if (char === '"') {
      try {
        const parsed = JSON.parse(source.slice(valueIndex, index + 1))
        return typeof parsed === 'string' ? parsed.trim() : ''
      } catch {
        return ''
      }
    }
  }
  return ''
}

function parseJsonNumberAfterProperty(source: string | null | undefined, property: string, startIndex = 0): number | undefined {
  if (!source) return undefined
  const propertyIndex = findJsonPropertyIndex(source, property, startIndex)
  if (propertyIndex < 0) return undefined
  const colonIndex = source.indexOf(':', propertyIndex + property.length + 2)
  if (colonIndex < 0) return undefined
  const match = source.slice(colonIndex + 1).match(/^\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/)
  if (!match) return undefined
  const parsed = Number.parseFloat(match[1])
  return Number.isFinite(parsed) ? parsed : undefined
}

function extractBalancedJsonArray(source: string, arrayStart: number): string | null {
  let depth = 0
  let inString = false
  let escaped = false
  for (let index = arrayStart; index < source.length; index += 1) {
    const char = source[index]
    if (inString) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === '"') {
        inString = false
      }
      continue
    }
    if (char === '"') {
      inString = true
    } else if (char === '[') {
      depth += 1
    } else if (char === ']') {
      depth -= 1
      if (depth === 0) return source.slice(arrayStart, index + 1)
    }
  }
  return null
}

function parseJsonArrayAfterProperty(source: string | null | undefined, property: string, startIndex = 0): unknown[] {
  if (!source) return []
  const propertyIndex = findJsonPropertyIndex(source, property, startIndex)
  if (propertyIndex < 0) return []
  const colonIndex = source.indexOf(':', propertyIndex + property.length + 2)
  if (colonIndex < 0) return []
  const arrayStart = source.indexOf('[', colonIndex + 1)
  if (arrayStart < 0) return []
  const rawArray = extractBalancedJsonArray(source, arrayStart)
  if (!rawArray) return []
  try {
    const parsed = JSON.parse(rawArray)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function isBazaarLinkSample(sample: AccountProbeSample): boolean {
  return sample.type === 'bazaarlink_api' || detailRun.value?.probe_source === 'bazaarlink_api'
}

function hasBazaarLinkResult(sample: AccountProbeSample): boolean {
  return Boolean(parseBazaarLinkResult(sample) || bazaarLinkIdentityEvidence(sample) || bazaarLinkPartialIdentityStatus(sample))
}

function bazaarLinkIdentityEvidence(sample: AccountProbeSample): AccountProbeValidationEvidence | null {
  if (!isBazaarLinkSample(sample)) return null
  return sample.validation_evidence?.find(evidence => evidence.key === 'bazaarlink_identity') || null
}

function bazaarLinkIdentityStart(sample: AccountProbeSample): number {
  const index = sample.response_body?.indexOf('"identityAssessment"') ?? -1
  return index >= 0 ? index : 0
}

function bazaarLinkPartialRunId(sample: AccountProbeSample): string {
  return parseJsonStringAfterProperty(sample.response_body, 'runId')
}

function bazaarLinkPartialTopLevelScore(sample: AccountProbeSample): number | undefined {
  const body = sample.response_body
  if (!body) return undefined
  const scoreIndex = findJsonPropertyIndex(body, 'score')
  if (scoreIndex < 0) return undefined
  const nestedBoundaryIndexes = [
    findJsonPropertyIndex(body, 'identityAssessment'),
    findJsonPropertyIndex(body, 'items'),
  ].filter(index => index >= 0)
  const firstNestedBoundary = nestedBoundaryIndexes.length ? Math.min(...nestedBoundaryIndexes) : -1
  if (firstNestedBoundary >= 0 && scoreIndex > firstNestedBoundary) return undefined
  return parseJsonNumberAfterProperty(body, 'score')
}

function bazaarLinkPartialIdentityStatus(sample: AccountProbeSample): string {
  return parseJsonStringAfterProperty(sample.response_body, 'status', bazaarLinkIdentityStart(sample))
}

function bazaarLinkPartialIdentityConfidence(sample: AccountProbeSample): number | undefined {
  return parseJsonNumberAfterProperty(sample.response_body, 'confidence', bazaarLinkIdentityStart(sample))
}

function bazaarLinkPartialClaimedModel(sample: AccountProbeSample): string {
  return parseJsonStringAfterProperty(sample.response_body, 'claimedModel', bazaarLinkIdentityStart(sample))
}

function bazaarLinkPartialPredictedFamily(sample: AccountProbeSample): string {
  return parseJsonStringAfterProperty(sample.response_body, 'predictedFamily', bazaarLinkIdentityStart(sample))
}

// BazaarLink response_body 会被后端截断用于安全展示；截断后优先使用已落库的 evidence 摘要。
function parseBazaarLinkObservedFields(value: string | null | undefined): BazaarLinkObservedFields {
  const fields: BazaarLinkObservedFields = {}
  if (!value) return fields

  for (const part of value.split(';')) {
    const trimmed = part.trim()
    const separator = trimmed.indexOf('=')
    if (separator <= 0) continue

    const key = trimmed.slice(0, separator).trim().toLowerCase()
    const fieldValue = trimmed.slice(separator + 1).trim()
    if (!fieldValue) continue

    if (key === 'status') {
      fields.status = fieldValue
    } else if (key === 'confidence') {
      const confidence = Number.parseFloat(fieldValue)
      if (Number.isFinite(confidence)) fields.confidence = confidence
    } else if (key === 'family') {
      fields.family = fieldValue
    } else if (key === 'v3f') {
      fields.v3f = fieldValue
    } else if (key === 'flags') {
      fields.flags = fieldValue.split(',').map(flag => flag.trim()).filter(Boolean)
    }
  }

  return fields
}

function bazaarLinkObservedFields(sample: AccountProbeSample): BazaarLinkObservedFields {
  return parseBazaarLinkObservedFields(bazaarLinkIdentityEvidence(sample)?.observed)
}

function bazaarLinkRunId(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  return result?.runId || bazaarLinkPartialRunId(sample) || (sample.run_id ? `#${sample.run_id}` : '-')
}

function bazaarLinkReturnedScore(sample: AccountProbeSample): number | undefined {
  const result = parseBazaarLinkResult(sample)
  if (typeof result?.score === 'number' && Number.isFinite(result.score)) return result.score
  const partialScore = bazaarLinkPartialTopLevelScore(sample)
  return typeof partialScore === 'number' && Number.isFinite(partialScore) ? partialScore : undefined
}

function isBazaarLinkQuickSample(sample: AccountProbeSample): boolean {
  return detailRun.value?.request_mode === 'quick' || /快速|quick/i.test(sample.label || '')
}

function bazaarLinkDisplayScoreValue(sample: AccountProbeSample, evidence = bazaarLinkIdentityEvidence(sample)): number | undefined {
  if (isBazaarLinkQuickSample(sample)) {
    const candidateScore = bazaarLinkClaimedCandidateScore(sample)
    if (typeof candidateScore === 'number') return candidateScore
    if (typeof evidence?.display_score === 'number' && Number.isFinite(evidence.display_score)) return evidence.display_score
  }
  const returnedScore = bazaarLinkReturnedScore(sample)
  if (typeof returnedScore === 'number') return returnedScore
  if (typeof evidence?.score === 'number' && Number.isFinite(evidence.score)) return evidence.score
  const runDisplayScore = detailRun.value?.display_score
  if (typeof runDisplayScore === 'number' && Number.isFinite(runDisplayScore)) return runDisplayScore
  const runScore = detailRun.value?.score
  return typeof runScore === 'number' && Number.isFinite(runScore) ? runScore : undefined
}

function bazaarLinkScore(sample: AccountProbeSample): string {
  return formatNumber(bazaarLinkDisplayScoreValue(sample))
}

function bazaarLinkIdentityStatus(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  if (result?.identityAssessment?.status) return result.identityAssessment.status
  const fields = bazaarLinkObservedFields(sample)
  if (fields.status) return fields.status
  const partialStatus = bazaarLinkPartialIdentityStatus(sample)
  if (partialStatus) return partialStatus
  const evidence = bazaarLinkIdentityEvidence(sample)
  return evidence?.passed ? t('admin.accountModelProbes.passed') : '-'
}

function bazaarLinkStatus(sample: AccountProbeSample): string {
  return parseBazaarLinkResult(sample)?.status || sample.status || '-'
}

function bazaarLinkClaimedModel(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  const evidence = bazaarLinkIdentityEvidence(sample)
  return result?.identityAssessment?.claimedModel || bazaarLinkPartialClaimedModel(sample) || evidence?.expected_model || evidence?.expected || sample.model || '-'
}

function bazaarLinkPredictedFamily(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  return result?.identityAssessment?.predictedFamily || bazaarLinkObservedFields(sample).family || bazaarLinkPartialPredictedFamily(sample) || '-'
}

function formatBazaarLinkConfidenceValue(confidence: number | null | undefined): string {
  const score = bazaarLinkConfidenceScore(confidence)
  return typeof score === 'number' ? `${formatNumber(score)}%` : '-'
}

function bazaarLinkConfidenceScore(confidence: number | null | undefined): number | null {
  if (typeof confidence !== 'number' || !Number.isFinite(confidence) || confidence <= 0) return null
  return Math.min(100, Math.max(0, confidence <= 1 ? confidence * 100 : confidence))
}

function bazaarLinkConfidence(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  const confidence = result?.identityAssessment?.confidence
  if (typeof confidence === 'number' && Number.isFinite(confidence)) {
    return formatBazaarLinkConfidenceValue(confidence)
  }
  return formatBazaarLinkConfidenceValue(bazaarLinkObservedFields(sample).confidence ?? bazaarLinkPartialIdentityConfidence(sample))
}

function bazaarLinkV3F(sample: AccountProbeSample): string {
  const result = parseBazaarLinkResult(sample)
  const match = result?.identityAssessment?.subModelMatchV3F
  if (match?.modelId && typeof match.score === 'number' && Number.isFinite(match.score)) {
    return `${match.modelId} (${formatNumber(match.score * 100)}%)`
  }
  return match?.modelId || bazaarLinkObservedFields(sample).v3f || bazaarLinkIdentityEvidence(sample)?.response_model || '-'
}

function bazaarLinkIdentityStatusConfirmed(status: string | undefined): boolean {
  return ['confirmed', 'match', 'matched'].includes((status || '').trim().toLowerCase())
}

function bazaarLinkIdentityPassed(sample: AccountProbeSample): boolean {
  const result = parseBazaarLinkResult(sample)
  const status = result?.identityAssessment?.status || bazaarLinkObservedFields(sample).status || bazaarLinkPartialIdentityStatus(sample)
  if (bazaarLinkIdentityStatusConfirmed(status)) return true
  return Boolean(bazaarLinkIdentityEvidence(sample)?.passed)
}

function displayEvidencePassed(sample: AccountProbeSample, evidence: AccountProbeValidationEvidence): boolean {
  if (isBazaarLinkSample(sample) && evidence.key === 'bazaarlink_identity' && bazaarLinkIdentityPassed(sample)) return true
  return evidence.passed
}

function displayEvidenceMessage(sample: AccountProbeSample, evidence: AccountProbeValidationEvidence): string {
  if (isBazaarLinkSample(sample) && evidence.key === 'bazaarlink_identity' && bazaarLinkIdentityPassed(sample) && !evidence.passed) {
    const flags = bazaarLinkRiskFlagList(sample)
    return flags.length ? `BazaarLink 身份验证通过，存在风险提示：${flags.join(', ')}` : 'BazaarLink 身份验证通过'
  }
  return evidence.message || ''
}

function displaySampleStatus(sample: AccountProbeSample): string | undefined {
  if (isBazaarLinkSample(sample) && bazaarLinkIdentityPassed(sample)) return 'success'
  return sample.status
}

function displayDetailRunStatus(run: AccountProbeRun): string | undefined {
  if (run.probe_source === 'bazaarlink_api' && run.samples?.some(sample => isBazaarLinkSample(sample) && bazaarLinkIdentityPassed(sample))) {
    return 'success'
  }
  return run.status
}

function shouldShowSampleError(sample: AccountProbeSample): boolean {
  if (isBazaarLinkSample(sample) && bazaarLinkIdentityPassed(sample)) return false
  return Boolean(sample.error_code || sample.error || sample.error_message)
}

function bazaarLinkRiskFlagList(sample: AccountProbeSample): string[] {
  const result = parseBazaarLinkResult(sample)
  const flags = result?.identityAssessment?.riskFlags?.map(flag => flag.trim()).filter(Boolean) || []
  if (flags.length) return flags
  const observedFlags = bazaarLinkObservedFields(sample).flags || []
  if (observedFlags.length) return observedFlags
  return parseJsonArrayAfterProperty(sample.response_body, 'riskFlags', bazaarLinkIdentityStart(sample))
    .map(flag => stringValue(flag))
    .filter(Boolean)
}

function bazaarLinkRiskFlags(sample: AccountProbeSample): string {
  const flags = bazaarLinkRiskFlagList(sample)
  return flags.length ? flags.join(', ') : t('admin.accountModelProbes.noRiskFlags')
}

function normalizeBazaarLinkCandidate(value: unknown): BazaarLinkModelCandidate | null {
  if (!isPlainObject(value)) return null
  const displayName = stringValue(value.displayName) || stringValue(value.display_name) || stringValue(value.name)
  const modelId = stringValue(value.modelId) || stringValue(value.model_id) || stringValue(value.id) || stringValue(value.model)
  const family = stringValue(value.family) || stringValue(value.modelFamily) || stringValue(value.predictedFamily)
  const score = numberValue(value.score) ?? numberValue(value.confidence)
  if (!displayName && !modelId && !family && typeof score !== 'number') return null
  return {
    displayName: displayName || undefined,
    modelId: modelId || undefined,
    family: family || undefined,
    score,
  }
}

function normalizeBazaarLinkCandidates(value: unknown): BazaarLinkModelCandidate[] {
  if (!Array.isArray(value)) return []
  return value.map(candidate => normalizeBazaarLinkCandidate(candidate)).filter((candidate): candidate is BazaarLinkModelCandidate => Boolean(candidate))
}

function bazaarLinkCandidatesFromContainer(value: unknown): BazaarLinkModelCandidate[] {
  if (Array.isArray(value)) return normalizeBazaarLinkCandidates(value)
  if (!isPlainObject(value)) return []
  for (const key of ['candidates', 'topCandidates', 'matches']) {
    const candidates = normalizeBazaarLinkCandidates(value[key])
    if (candidates.length) return candidates
  }
  return []
}

function bazaarLinkV3CandidatesFromBody(sample: AccountProbeSample): BazaarLinkModelCandidate[] {
  const identityStart = bazaarLinkIdentityStart(sample)
  const body = sample.response_body
  if (!body) return []
  const v3Index = body.indexOf('"v3"', identityStart)
  const candidatesStart = v3Index >= 0 ? v3Index : identityStart
  const candidates = normalizeBazaarLinkCandidates(parseJsonArrayAfterProperty(body, 'candidates', candidatesStart))
  if (candidates.length) return candidates
  return normalizeBazaarLinkCandidates(parseJsonArrayAfterProperty(body, 'v3Candidates', identityStart))
}

function bazaarLinkV3Candidates(sample: AccountProbeSample): BazaarLinkModelCandidate[] {
  const identity = parseBazaarLinkResult(sample)?.identityAssessment
  const containers: unknown[] = [
    identity?.v3,
    identity?.v3Candidates,
    identity?.candidates,
  ]
  for (const container of containers) {
    const candidates = bazaarLinkCandidatesFromContainer(container)
    if (candidates.length) return candidates
  }
  return bazaarLinkV3CandidatesFromBody(sample)
}

function bazaarLinkCandidateKey(candidate: BazaarLinkModelCandidate): string {
  return [candidate.modelId, candidate.displayName, candidate.family, String(candidate.score ?? '')].filter(Boolean).join(':')
}

function formatBazaarLinkCandidateScore(candidate: BazaarLinkModelCandidate): string {
  const score = candidate.score
  if (typeof score !== 'number' || !Number.isFinite(score)) return '-'
  return `${formatNumber(score <= 1 ? score * 100 : score)}%`
}

function bazaarLinkClaimedCandidateScore(sample: AccountProbeSample): number | undefined {
  const targets = [
    bazaarLinkClaimedModel(sample),
    sample.model,
    bazaarLinkIdentityEvidence(sample)?.expected_model,
    bazaarLinkIdentityEvidence(sample)?.expected,
    detailRun.value?.model,
  ]
  for (const target of targets) {
    if (!target || target === '-') continue
    for (const candidate of bazaarLinkV3Candidates(sample)) {
      if (!bazaarLinkCandidateMatchesModel(candidate, target)) continue
      const score = normalizeBazaarLinkCandidateScore(candidate.score)
      if (typeof score === 'number') return score
    }
  }
  return undefined
}

function bazaarLinkCandidateMatchesModel(candidate: BazaarLinkModelCandidate, expectedModel: string): boolean {
  return bazaarLinkModelNameMatches(candidate.modelId, expectedModel) || bazaarLinkModelNameMatches(candidate.displayName, expectedModel)
}

function bazaarLinkModelNameMatches(left: string | undefined, right: string | undefined): boolean {
  const normalizedLeft = normalizeBazaarLinkModelName(left)
  const normalizedRight = normalizeBazaarLinkModelName(right)
  if (!normalizedLeft || !normalizedRight) return false
  return normalizedLeft === normalizedRight || normalizedLeft.endsWith(normalizedRight) || normalizedRight.endsWith(normalizedLeft)
}

function normalizeBazaarLinkModelName(value: string | undefined): string {
  const normalized = (value || '').trim().toLowerCase()
  const withoutProvider = normalized.includes('/') ? normalized.slice(normalized.lastIndexOf('/') + 1) : normalized
  return withoutProvider.replace(/[\s_-]+/g, '')
}

function normalizeBazaarLinkCandidateScore(score: number | undefined): number | undefined {
  if (typeof score !== 'number' || !Number.isFinite(score)) return undefined
  const percent = score <= 1 ? score * 100 : score
  return Math.min(100, Math.max(0, percent))
}

function bazaarLinkItems(sample: AccountProbeSample): BazaarLinkProbeItem[] {
  return parseBazaarLinkResult(sample)?.items || []
}

function formatBazaarLinkPassed(value: BazaarLinkProbeItem['passed']): string {
  if (value === true) return t('admin.accountModelProbes.passed')
  if (value === false) return t('admin.accountModelProbes.notPassed')
  if (value === 'warning') return t('admin.accountModelProbes.warning')
  return '-'
}

function normalizeReasonText(value: string | null | undefined): string {
  return typeof value === 'string' ? value.trim() : ''
}

function sampleFailureReasons(sample: AccountProbeSample): string[] {
  if (isBazaarLinkSample(sample) && bazaarLinkIdentityPassed(sample)) return []

  const reasons: string[] = []
  const errorParts = [
    normalizeReasonText(sample.error_code),
    normalizeReasonText(sample.error || sample.error_message),
  ].filter(Boolean)

  if (errorParts.length) {
    reasons.push(errorParts.join(' '))
  }

  for (const evidence of sample.validation_evidence || []) {
    if (evidence.passed) continue

    const label = normalizeReasonText(evidence.label || evidence.key) || t('admin.accountModelProbes.evidence')
    const details = [
      normalizeReasonText(evidence.message),
      normalizeReasonText(evidence.observed) ? `${t('admin.accountModelProbes.observed')}: ${normalizeReasonText(evidence.observed)}` : '',
      normalizeReasonText(evidence.expected) ? `${t('admin.accountModelProbes.expected')}: ${normalizeReasonText(evidence.expected)}` : '',
    ].filter(Boolean)

    reasons.push(details.length ? `${label}: ${details.join(' / ')}` : label)
  }

  return Array.from(new Set(reasons))
}

function sampleFailureResultDetails(sample: AccountProbeSample): SampleFailureResultDetail[] {
  if (sampleFailureReasons(sample).length === 0) return []

  const details: SampleFailureResultDetail[] = []
  const outputText = normalizeReasonText(sample.output_text)
  if (outputText) {
    details.push({
      key: 'output_text',
      label: t('admin.accountModelProbes.modelOutput'),
      value: outputText,
    })
  }

  for (const evidence of sample.validation_evidence || []) {
    if (evidence.passed) continue

    const label = normalizeReasonText(evidence.label || evidence.key) || t('admin.accountModelProbes.evidence')
    const observed = normalizeReasonText(evidence.observed)
    const expected = normalizeReasonText(evidence.expected)
    if (observed) {
      details.push({
        key: `${evidence.key}:observed`,
        label: `${label} ${t('admin.accountModelProbes.parsedObserved')}`,
        value: observed,
      })
    }
    if (expected) {
      details.push({
        key: `${evidence.key}:expected`,
        label: `${label} ${t('admin.accountModelProbes.expected')}`,
        value: expected,
      })
    }
  }

  return details
}

function hasEvidenceProbeDetails(evidence: AccountProbeValidationEvidence): boolean {
  return Boolean(
    evidence.category ||
    evidence.severity ||
    evidence.response_model ||
    evidence.expected_model ||
    typeof evidence.attempt_count === 'number' ||
    typeof evidence.retry_attempt_count === 'number' ||
    evidence.attempt_status_codes?.length ||
    evidence.trusted_account_id ||
    typeof evidence.similarity_percent === 'number' ||
    typeof evidence.pair_coverage_percent === 'number' ||
    typeof evidence.target_pass_rate_percent === 'number' ||
    typeof evidence.trusted_pass_rate_percent === 'number'
  )
}

function statusClass(status: string | undefined): string {
  if (status === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (status === 'partial' || status === 'warning' || status === 'running' || status === 'pending') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  if (status === 'failed') return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

onMounted(() => {
  loadRuns()
})

onUnmounted(() => {
  listAbortController?.abort()
  submitAbortController?.abort()
  detailAbortController?.abort()
  deleteAbortController?.abort()
  bazaarLinkSubmitAbortController?.abort()
  batchAccountsAbortController?.abort()
  batchSubmitAbortController?.abort()
  if (batchSearchTimer) {
    clearTimeout(batchSearchTimer)
  }
})
</script>
