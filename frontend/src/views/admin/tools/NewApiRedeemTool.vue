<template>
  <section class="space-y-6 p-4 sm:p-6" data-test="newapi-redeem-tool">
    <header class="flex flex-col gap-3 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <div class="flex items-center gap-2">
          <Icon name="gift" size="md" class="text-primary-600 dark:text-primary-400" />
          <h1 class="text-lg font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.title') }}</h1>
        </div>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.newapiRedeem.description') }}</p>
      </div>
      <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <Icon name="globe" size="sm" />
        <span>{{ t('admin.newapiRedeem.endpoint') }}: {{ overview.base_url || '-' }}</span>
      </div>
    </header>

    <div
      v-if="notice"
      :class="notice.type === 'error'
        ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200'
        : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'"
      class="flex items-start gap-2 rounded-lg border px-3 py-2 text-sm"
      role="status"
    >
      <Icon :name="notice.type === 'error' ? 'exclamationCircle' : 'checkCircle'" size="sm" class="mt-0.5 shrink-0" />
      <span>{{ notice.text }}</span>
    </div>

    <section class="grid gap-6 border-b border-gray-200 pb-6 dark:border-dark-700 xl:grid-cols-[minmax(0,1.25fr)_minmax(19rem,0.75fr)]">
      <div class="min-w-0">
        <div class="mb-3 flex items-center gap-2">
          <Icon name="users" size="sm" class="text-gray-500 dark:text-gray-400" />
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.accountImport') }}</h2>
        </div>
        <label for="newapi-redeem-account-import" class="sr-only">{{ t('admin.newapiRedeem.accountImport') }}</label>
        <textarea
          id="newapi-redeem-account-import"
          v-model="importText"
          class="input min-h-32 w-full resize-y px-3 py-2 font-mono text-sm"
          :placeholder="t('admin.newapiRedeem.accountImportPlaceholder')"
          data-test="account-import-input"
        />
        <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
          <p class="max-w-2xl text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.accountImportHint') }}</p>
          <button
            type="button"
            class="btn btn-primary shrink-0"
            :disabled="!importText.trim() || actionID === 'import'"
            data-test="import-accounts"
            @click="importAccountBatch"
          >
            <Icon name="upload" size="sm" :class="actionID === 'import' ? 'animate-spin' : ''" />
            <span class="ml-1.5">{{ t('admin.newapiRedeem.importAccounts') }}</span>
          </button>
        </div>
      </div>

      <div class="min-w-0 border-t border-gray-200 pt-5 dark:border-dark-700 xl:border-l xl:border-t-0 xl:pl-6 xl:pt-0">
        <div class="mb-3 flex items-center gap-2">
          <Icon name="play" size="sm" class="text-gray-500 dark:text-gray-400" />
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.selection') }}</h2>
        </div>
        <div class="space-y-1 text-sm text-gray-600 dark:text-gray-300">
          <p>{{ t('admin.newapiRedeem.selectedAccounts', { count: selectedAccountIDs.length }) }}</p>
          <p>{{ t('admin.newapiRedeem.selectedFiles', { count: selectedFileIDs.length }) }}</p>
        </div>
        <button
          v-if="activeRun"
          type="button"
          class="btn btn-danger mt-4 w-full"
          :disabled="actionID === `cancel-${activeRun.id}` || activeRun.status === 'cancelling'"
          data-test="cancel-redemption"
          @click="cancelActiveRun"
        >
          <Icon name="x" size="sm" :class="actionID === `cancel-${activeRun.id}` ? 'animate-pulse' : ''" />
          <span class="ml-1.5">{{ t('admin.newapiRedeem.stop') }}</span>
        </button>
        <button
          v-else
          type="button"
          class="btn btn-primary mt-4 w-full"
          :disabled="!canStart || actionID === 'start'"
          data-test="start-redemption"
          @click="startRedemption"
        >
          <Icon name="play" size="sm" :class="actionID === 'start' ? 'animate-pulse' : ''" />
          <span class="ml-1.5">{{ t('admin.newapiRedeem.start') }}</span>
        </button>
        <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
          {{ activeRun ? activeRun.message || statusLabel(activeRun.status) : t('admin.newapiRedeem.noActiveRun') }}
        </p>
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <Icon name="users" size="sm" class="text-gray-500 dark:text-gray-400" />
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.accounts') }}</h2>
          <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ overview.accounts.length }}
          </span>
          <span data-test="total-balance" class="text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.newapiRedeem.totalBalance') }}
            <span class="font-semibold text-gray-900 dark:text-gray-100">{{ formatUSD(totalQuota) }}</span>
          </span>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary px-3"
            :disabled="overview.accounts.length === 0 || actionID === 'refresh-all'"
            data-test="refresh-all-accounts"
            @click="refreshAllAccounts"
          >
            <Icon name="refresh" size="sm" :class="actionID === 'refresh-all' ? 'animate-spin' : ''" />
            <span class="ml-1.5">{{ t('admin.newapiRedeem.refreshAll') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary px-3"
            :disabled="loading"
            :title="t('admin.newapiRedeem.refresh')"
            :aria-label="t('admin.newapiRedeem.refresh')"
            data-test="refresh-overview"
            @click="loadOverview"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <div v-if="loading && overview.accounts.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="overview.accounts.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.newapiRedeem.noAccounts') }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[68rem] text-left text-sm">
          <thead class="border-y border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
            <tr>
              <th class="w-10 px-3 py-2 font-medium">
                <label class="inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="allAccountsSelected"
                    :aria-label="t('admin.newapiRedeem.selectAllAccounts')"
                    :title="t('admin.newapiRedeem.selectAllAccounts')"
                    @change="toggleAllAccounts"
                  />
                </label>
              </th>
              <th class="px-3 py-2 font-medium">ID</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.username') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.email') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.group') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.quota') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.apiKeys') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.redeemedCodes') }}</th>
              <th class="px-3 py-2 text-right font-medium"><span class="sr-only">{{ t('admin.newapiRedeem.actions') }}</span></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <template v-for="account in overview.accounts" :key="account.id">
              <tr data-test="account-row">
                <td class="px-3 py-3 align-top">
                  <input v-model="selectedAccountIDs" :value="account.id" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                </td>
                <td class="px-3 py-3 align-top font-mono text-xs text-gray-800 dark:text-gray-200">
                  <div>{{ account.user_id }}</div>
                  <div class="mt-1 text-gray-500 dark:text-gray-400">{{ account.access_key_masked }}</div>
                  <div class="mt-1 font-sans text-gray-500 dark:text-gray-400">{{ account.browser_fingerprint }}</div>
                </td>
                <td class="px-3 py-3 align-top text-gray-900 dark:text-gray-100">{{ account.username || '-' }}</td>
                <td class="px-3 py-3 align-top text-gray-700 dark:text-gray-300">{{ account.email || '-' }}</td>
                <td class="px-3 py-3 align-top text-gray-700 dark:text-gray-300">{{ account.group || '-' }}</td>
                <td class="px-3 py-3 align-top text-gray-700 dark:text-gray-300">
                  <div>{{ formatUSD(account.quota) }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.usedQuota') }} {{ formatUSD(account.used_quota) }}</div>
                </td>
                <td class="px-3 py-3 align-top text-gray-700 dark:text-gray-300">{{ account.api_keys.length }}</td>
                <td class="px-3 py-3 align-top text-gray-700 dark:text-gray-300">
                  <div v-if="account.redeemed_codes.length > 0">
                    <div class="font-medium text-emerald-700 dark:text-emerald-300">
                      {{ t('admin.newapiRedeem.redeemedCodeCount', { count: account.redeemed_codes.length }) }}
                    </div>
                    <div class="mt-1 max-w-48 truncate text-xs text-gray-500 dark:text-gray-400" :title="account.redeemed_codes[0].file_name">
                      {{ account.redeemed_codes[0].file_name || '-' }}
                    </div>
                  </div>
                  <span v-else>-</span>
                </td>
                <td class="px-3 py-3 align-top text-right">
                  <div class="inline-flex items-center gap-1">
                    <button
                      type="button"
                      class="inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                      :disabled="actionID === `refresh-${account.id}` || actionID === 'refresh-all'"
                      :title="t('admin.newapiRedeem.refresh')"
                      :aria-label="t('admin.newapiRedeem.refresh')"
                      data-test="refresh-account"
                      @click="refreshAccount(account.id)"
                    >
                      <Icon name="refresh" size="sm" :class="actionID === `refresh-${account.id}` ? 'animate-spin' : ''" />
                    </button>
                    <button
                      type="button"
                      class="inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-rose-50 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-rose-950/40 dark:hover:text-rose-300"
                      :disabled="actionID === `delete-account-${account.id}` || Boolean(activeRun)"
                      :title="t('admin.newapiRedeem.deleteAccount')"
                      :aria-label="t('admin.newapiRedeem.deleteAccount')"
                      data-test="delete-account"
                      @click="deleteAccount(account.id, account.user_id)"
                    >
                      <Icon name="trash" size="sm" :class="actionID === `delete-account-${account.id}` ? 'animate-pulse' : ''" />
                    </button>
                  </div>
                </td>
              </tr>
              <tr>
                <td colspan="9" class="bg-gray-50/70 px-3 py-3 dark:bg-dark-900/35">
                  <details>
                    <summary class="cursor-pointer select-none text-sm font-medium text-gray-700 marker:text-gray-400 dark:text-gray-200">
                      {{ t('admin.newapiRedeem.apiKeys') }} ({{ account.api_keys.length }})
                    </summary>
                    <div class="mt-3 space-y-3">
                      <div v-if="account.api_keys.length === 0" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.noApiKeys') }}</div>
                      <div v-for="apiKey in account.api_keys" :key="apiKey.id" class="grid gap-2 border-t border-gray-200 pt-3 first:border-t-0 first:pt-0 lg:grid-cols-[minmax(8rem,0.7fr)_minmax(12rem,1fr)_minmax(10rem,0.8fr)_auto] lg:items-center dark:border-dark-700">
                        <div class="min-w-0 font-mono text-xs text-gray-700 dark:text-gray-300">
                          <div class="truncate" :title="apiKey.name">{{ apiKey.name || `#${apiKey.id}` }}</div>
                          <div class="mt-1 truncate text-gray-500 dark:text-gray-400" :title="apiKey.masked_key">{{ visibleKey(account.id, apiKey.id) || apiKey.masked_key }}</div>
                        </div>
                        <div class="flex min-w-0 items-center gap-2">
                          <label :for="`newapi-redeem-group-${account.id}-${apiKey.id}`" class="sr-only">{{ t('admin.newapiRedeem.group') }}</label>
                          <select
                            :id="`newapi-redeem-group-${account.id}-${apiKey.id}`"
                            :value="keyGroup(account.id, apiKey.id, apiKey.group)"
                            class="input h-9 min-w-0 flex-1 px-2 py-1 text-xs"
                            @change="setKeyGroup(account.id, apiKey.id, $event)"
                          >
                            <option v-for="group in availableGroups(account)" :key="group" :value="group">{{ group }}</option>
                            <option v-if="!availableGroups(account).includes(keyGroup(account.id, apiKey.id, apiKey.group))" :value="keyGroup(account.id, apiKey.id, apiKey.group)">
                              {{ keyGroup(account.id, apiKey.id, apiKey.group) || '-' }}
                            </option>
                          </select>
                          <button
                            type="button"
                            class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-white hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                            :disabled="actionID === `group-${account.id}-${apiKey.id}`"
                            :title="t('admin.newapiRedeem.saveGroup')"
                            :aria-label="t('admin.newapiRedeem.saveGroup')"
                            @click="saveKeyGroup(account.id, apiKey.id, apiKey.group)"
                          >
                            <Icon name="check" size="sm" :class="actionID === `group-${account.id}-${apiKey.id}` ? 'animate-pulse' : ''" />
                          </button>
                        </div>
                        <button
                          type="button"
                          class="btn btn-secondary h-9 justify-center px-3 text-xs"
                          :disabled="actionID === `reveal-${account.id}-${apiKey.id}`"
                          @click="toggleAPIKeyVisibility(account.id, apiKey.id)"
                        >
                          <Icon :name="visibleKey(account.id, apiKey.id) ? 'eyeOff' : 'eye'" size="sm" />
                          <span class="ml-1.5">{{ visibleKey(account.id, apiKey.id) ? t('admin.newapiRedeem.hideKey') : t('admin.newapiRedeem.revealKey') }}</span>
                        </button>
                      </div>
                      <template v-for="apiKey in account.api_keys" :key="`${apiKey.id}-references`">
                        <div
                          v-if="databaseReferences(apiKey).length > 0 || apiKey.target_accounts?.length"
                          class="border-t border-gray-200 pt-3 text-xs dark:border-dark-700"
                          data-test="api-key-references"
                        >
                        <div v-if="databaseReferences(apiKey).length > 0" class="text-gray-600 dark:text-gray-300">
                          <span class="font-medium">{{ t('admin.newapiRedeem.databaseReferences') }}:</span>
                          <span v-for="reference in databaseReferences(apiKey)" :key="`database-${reference.id}`" class="ml-2 text-emerald-700 dark:text-emerald-300">
                            {{ reference.name || `#${reference.id}` }}
                          </span>
                        </div>
                        <div v-if="apiKey.target_accounts?.length" class="mt-2 space-y-2">
                          <div class="font-medium text-gray-600 dark:text-gray-300">{{ t('admin.newapiRedeem.matchingAccounts') }}</div>
                          <div v-for="target in apiKey.target_accounts" :key="`target-${target.id}`" class="flex flex-wrap items-center gap-2">
                            <span :class="target.referenced ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-600 dark:text-gray-300'">
                              {{ target.referenced ? t('admin.newapiRedeem.referencedAccount') : t('admin.newapiRedeem.availableAccount') }}: {{ target.name || `#${target.id}` }}
                            </span>
                            <button
                              type="button"
                              class="btn btn-secondary h-8 px-2 text-xs"
                              :disabled="Boolean(actionID)"
                              data-test="append-key"
                              @click="linkAPIKey(account.id, apiKey.id, target.id, 'append', target.name)"
                            >
                              <Icon name="plus" size="sm" />
                              <span class="ml-1">{{ t('admin.newapiRedeem.appendKey') }}</span>
                            </button>
                            <button
                              type="button"
                              class="btn btn-secondary h-8 px-2 text-xs"
                              :disabled="Boolean(actionID)"
                              data-test="replace-key"
                              @click="linkAPIKey(account.id, apiKey.id, target.id, 'replace', target.name)"
                            >
                              <Icon name="refresh" size="sm" />
                              <span class="ml-1">{{ t('admin.newapiRedeem.replaceKey') }}</span>
                            </button>
                          </div>
                        </div>
                        </div>
                      </template>
                      <div class="grid gap-2 border-t border-gray-200 pt-3 sm:grid-cols-[minmax(12rem,1fr)_minmax(10rem,0.7fr)_auto] sm:items-center dark:border-dark-700">
                        <label :for="`newapi-redeem-create-name-${account.id}`" class="sr-only">{{ t('admin.newapiRedeem.keyNamePlaceholder') }}</label>
                        <input
                          :id="`newapi-redeem-create-name-${account.id}`"
                          v-model="newKeyNames[account.id]"
                          class="input h-9 px-2 py-1 text-sm"
                          :placeholder="t('admin.newapiRedeem.keyNamePlaceholder')"
                        />
                        <select v-model="newKeyGroups[account.id]" class="input h-9 px-2 py-1 text-sm">
                          <option value="">{{ t('admin.newapiRedeem.group') }}</option>
                          <option v-for="group in availableGroups(account)" :key="group" :value="group">{{ group }}</option>
                        </select>
                        <button
                          type="button"
                          class="btn btn-secondary h-9 justify-center px-3 text-sm"
                          :disabled="!newKeyNames[account.id]?.trim() || actionID === `create-${account.id}`"
                          @click="createAPIKey(account.id)"
                        >
                          <Icon name="plus" size="sm" :class="actionID === `create-${account.id}` ? 'animate-pulse' : ''" />
                          <span class="ml-1.5">{{ t('admin.newapiRedeem.createKey') }}</span>
                        </button>
                      </div>
                    </div>
                  </details>
                  <details class="mt-3 border-t border-gray-200 pt-3 dark:border-dark-700">
                    <summary class="cursor-pointer select-none text-sm font-medium text-gray-700 marker:text-gray-400 dark:text-gray-200">
                      {{ t('admin.newapiRedeem.redeemedCodes') }} ({{ account.redeemed_codes.length }})
                    </summary>
                    <div v-if="account.redeemed_codes.length === 0" class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.newapiRedeem.noRedeemedCodes') }}
                    </div>
                    <div v-else class="mt-3 overflow-x-auto">
                      <table class="w-full min-w-[32rem] text-left text-xs">
                        <thead class="border-y border-gray-200 text-gray-500 dark:border-dark-700 dark:text-gray-400">
                          <tr>
                            <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.files') }}</th>
                            <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.requestResult') }}</th>
                            <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.time') }}</th>
                          </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                          <tr v-for="item in account.redeemed_codes" :key="`${item.file_id}-${item.redeemed_at}`">
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ item.file_name || '-' }}</td>
                            <td class="px-3 py-2">
                              <span class="rounded-full px-2 py-0.5 font-medium" :class="redeemLogResultClass(item.result)">
                                {{ redeemLogResultLabel(item.result) }}
                              </span>
                            </td>
                            <td class="px-3 py-2 text-gray-500 dark:text-gray-400">{{ formatDate(item.redeemed_at) }}</td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </details>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <Icon name="document" size="sm" class="text-gray-500 dark:text-gray-400" />
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.files') }}</h2>
          <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ overview.files.length }}
          </span>
          <label class="ml-1 inline-flex cursor-pointer items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-gray-300">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allFilesSelected"
              @change="toggleAllFiles"
            />
            <span>{{ t('admin.newapiRedeem.selectAllFiles') }}</span>
          </label>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <input ref="fileInput" type="file" multiple class="sr-only" data-test="voucher-file-input" @change="selectUploadFiles" />
          <button type="button" class="btn btn-secondary px-3" @click="fileInput?.click()">
            <Icon name="upload" size="sm" />
            <span class="ml-1.5">{{ t('admin.newapiRedeem.chooseFiles') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-primary px-3"
            :disabled="uploadFiles.length === 0 || actionID === 'upload'"
            data-test="upload-voucher-files"
            @click="uploadSelectedFiles"
          >
            <Icon name="upload" size="sm" :class="actionID === 'upload' ? 'animate-spin' : ''" />
            <span class="ml-1.5">{{ t('admin.newapiRedeem.uploadFiles') }}</span>
          </button>
        </div>
      </div>
      <p v-if="uploadFiles.length" class="mb-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.selectedUploadFiles', { count: uploadFiles.length }) }}</p>
      <div v-if="overview.files.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.noFiles') }}</div>
      <div v-else class="grid gap-x-6 divide-y divide-gray-100 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700 lg:grid-cols-2 lg:divide-y-0">
        <div v-for="file in overview.files" :key="file.id" class="flex min-w-0 items-center gap-3 px-3 py-3 lg:border-b lg:border-gray-100 lg:odd:border-r dark:lg:border-dark-700">
          <label class="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
            <input v-model="selectedFileIDs" :value="file.id" type="checkbox" class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            <Icon name="document" size="sm" class="shrink-0 text-gray-400 dark:text-gray-500" />
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium text-gray-900 dark:text-gray-100" :title="file.name">{{ file.name }}</span>
              <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.codeCount', { count: file.code_count }) }} · {{ t('admin.newapiRedeem.uploadedAt', { time: formatDate(file.uploaded_at) }) }}</span>
            </span>
          </label>
          <button
            type="button"
            class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-rose-50 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-rose-950/40 dark:hover:text-rose-300"
            :disabled="actionID === `delete-file-${file.id}` || Boolean(activeRun)"
            :title="t('admin.newapiRedeem.deleteFile')"
            :aria-label="t('admin.newapiRedeem.deleteFile')"
            data-test="delete-file"
            @click="deleteVoucherFile(file.id, file.name)"
          >
            <Icon name="trash" size="sm" :class="actionID === `delete-file-${file.id}` ? 'animate-pulse' : ''" />
          </button>
        </div>
      </div>
    </section>

    <section>
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <Icon name="clock" size="sm" class="text-gray-500 dark:text-gray-400" />
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.logs') }}</h2>
        </div>
        <button
          v-if="logTargetRun"
          type="button"
          class="btn btn-secondary h-9 px-3 text-xs"
          data-test="view-run-logs"
          :disabled="logsLoading"
          @click="loadRunLogs"
        >
          <Icon name="refresh" size="sm" :class="logsLoading ? 'animate-spin' : ''" />
          <span class="ml-1.5">{{ t('admin.newapiRedeem.viewFirstLogs') }}</span>
        </button>
      </div>
      <div v-if="activeRun" class="mb-4 grid gap-3 border-y border-gray-200 py-3 text-sm sm:grid-cols-2 lg:grid-cols-6 dark:border-dark-700">
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.task') }}</div>
          <div class="mt-1 font-medium text-gray-900 dark:text-gray-100">{{ statusLabel(activeRun.status) }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.taskProgress', { completed: activeRun.completed_pairs, total: activeRun.total_pairs }) }}</div>
          <div class="mt-1 font-medium text-gray-900 dark:text-gray-100">{{ activeRun.message || '-' }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.attempts', { count: activeRun.attempts }) }}</div>
          <div class="mt-1 font-medium text-gray-900 dark:text-gray-100">{{ t('admin.newapiRedeem.successes', { count: activeRun.successes }) }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">ID</div>
          <div class="mt-1 truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="activeRun.id">{{ activeRun.id }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.currentFile') }}</div>
          <div class="mt-1 truncate font-medium text-gray-900 dark:text-gray-100" :title="activeRun.current_file_name || ''">{{ activeRun.current_file_name || '-' }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.currentNetwork') }}</div>
          <div class="mt-1 truncate font-medium text-gray-900 dark:text-gray-100" :title="activeRun.current_node || ''">{{ activeRun.current_node || '-' }}</div>
          <div class="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{{ activeRun.current_exit_ip || '-' }} · {{ t('admin.newapiRedeem.switches', { count: activeRun.switch_count || 0 }) }}</div>
        </div>
      </div>
      <div v-if="requestLogs.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.newapiRedeem.noLogs') }}</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[76rem] text-left text-sm">
          <thead class="border-y border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.accounts') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.files') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.redeemCode') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.requestResult') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.network') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.httpStatus') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.response') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.newapiRedeem.time') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(log, logIndex) in requestLogs" :key="`${log.at}-${log.account_id}-${log.file_id}-${log.code}-${logIndex}`">
              <td class="px-3 py-3 text-gray-900 dark:text-gray-100">
                <div>{{ log.user_id }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ log.browser_fingerprint || '-' }}</div>
              </td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ log.file_name }}</td>
              <td class="px-3 py-3 font-mono text-xs break-all text-gray-700 dark:text-gray-300" :title="log.code">{{ log.code }}</td>
              <td class="px-3 py-3">
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="redeemLogResultClass(log.result)">
                  {{ redeemLogResultLabel(log.result) }}
                </span>
                <div v-if="log.removed_from_file" class="mt-1 text-xs text-rose-600 dark:text-rose-300">{{ t('admin.newapiRedeem.codeRemoved') }}</div>
              </td>
              <td class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">
                <div class="max-w-44 truncate font-medium text-gray-700 dark:text-gray-200" :title="log.node || ''">{{ log.node || '-' }}</div>
                <div class="mt-1 font-mono">{{ log.exit_ip || '-' }}</div>
                <div v-if="log.switched_node || log.switched_exit_ip" class="mt-2 border-t border-gray-100 pt-2 dark:border-dark-700">
                  <div class="max-w-44 truncate text-emerald-700 dark:text-emerald-300" :title="log.switched_node || ''">→ {{ log.switched_node || '-' }}</div>
                  <div class="mt-1 font-mono text-emerald-700 dark:text-emerald-300">{{ log.switched_exit_ip || '-' }}</div>
                </div>
              </td>
              <td class="px-3 py-3 font-mono text-xs text-gray-500 dark:text-gray-400">{{ log.status_code || '-' }}</td>
              <td class="max-w-md px-3 py-3 text-xs break-words text-gray-500 dark:text-gray-400">{{ log.message || '-' }}</td>
              <td class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(log.at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { newapiRedeemAPI } from '@/api/admin/newapiRedeem'
import type {
  NewAPIRedeemAccount,
  NewAPIRedeemAccountImportInput,
  NewAPIRedeemOverview,
  NewAPIRedeemRequestLog
} from '@/types'

interface ToolNotice {
  type: 'success' | 'error'
  text: string
}

const { t } = useI18n()
const fileInput = ref<HTMLInputElement | null>(null)
const overview = ref<NewAPIRedeemOverview>({ base_url: '', accounts: [], files: [], runs: [] })
const loading = ref(false)
const actionID = ref('')
const importText = ref('')
const uploadFiles = ref<File[]>([])
const selectedAccountIDs = ref<string[]>([])
const selectedFileIDs = ref<string[]>([])
const visibleKeys = ref<Record<string, string>>({})
const newKeyNames = reactive<Record<string, string>>({})
const newKeyGroups = reactive<Record<string, string>>({})
const keyGroups = reactive<Record<string, string>>({})
const notice = ref<ToolNotice | null>(null)
const requestLogs = ref<NewAPIRedeemRequestLog[]>([])
const logsLoading = ref(false)
// NewAPI 上游额度使用固定原始单位，500000 对应 US$1。
const newAPIQuotaPerUSD = 500000

let pollTimer: number | undefined

const activeRun = computed(() => overview.value.runs.find(run => run.status === 'running' || run.status === 'cancelling') || null)
const canStart = computed(() => selectedAccountIDs.value.length > 0 && selectedFileIDs.value.length > 0)
const logTargetRun = computed(() => activeRun.value || overview.value.runs[0] || null)
const allAccountsSelected = computed(() => overview.value.accounts.length > 0 && overview.value.accounts.every(account => selectedAccountIDs.value.includes(account.id)))
const allFilesSelected = computed(() => overview.value.files.length > 0 && overview.value.files.every(file => selectedFileIDs.value.includes(file.id)))
// totalQuota 保持 NewAPI 原始额度单位，展示时统一通过 formatUSD 换算。
const totalQuota = computed(() => overview.value.accounts.reduce((total, account) => {
  const quota = Number(account.quota)
  return Number.isFinite(quota) ? total + quota : total
}, 0))

function setNotice(type: ToolNotice['type'], text: string) {
  notice.value = { type, text }
}

function errorMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return String(error || t('common.error'))
}

function applyOverview(next: NewAPIRedeemOverview) {
  overview.value = {
    base_url: next.base_url || '',
    accounts: next.accounts || [],
    files: next.files || [],
    runs: next.runs || []
  }
  const accountIDs = new Set(overview.value.accounts.map(account => account.id))
  const fileIDs = new Set(overview.value.files.map(file => file.id))
  selectedAccountIDs.value = selectedAccountIDs.value.filter(id => accountIDs.has(id))
  selectedFileIDs.value = selectedFileIDs.value.filter(id => fileIDs.has(id))
  schedulePolling()
}

/** 按当前账号列表整体切换兑换任务选择状态。 */
function toggleAllAccounts(event: Event) {
  selectedAccountIDs.value = (event.target as HTMLInputElement).checked
    ? overview.value.accounts.map(account => account.id)
    : []
}

/** 按当前兑换码文件列表整体切换兑换任务选择状态。 */
function toggleAllFiles(event: Event) {
  selectedFileIDs.value = (event.target as HTMLInputElement).checked
    ? overview.value.files.map(file => file.id)
    : []
}

async function loadOverview() {
  if (loading.value) return
  loading.value = true
  try {
    applyOverview(await newapiRedeemAPI.getOverview())
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    loading.value = false
  }
}

function schedulePolling() {
  if (pollTimer !== undefined) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
  if (!activeRun.value) return
  pollTimer = window.setTimeout(() => {
    void pollRun(activeRun.value?.id)
  }, 3000)
}

async function pollRun(runID?: string) {
  if (!runID) return
  try {
    const run = await newapiRedeemAPI.getRun(runID)
    overview.value = {
      ...overview.value,
      runs: [run, ...overview.value.runs.filter(item => item.id !== run.id)]
    }
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    schedulePolling()
  }
}

/** 手动加载当前任务最近 100 条日志快照；轮询只更新无日志任务摘要。 */
async function loadRunLogs() {
  const runID = logTargetRun.value?.id
  if (!runID || logsLoading.value) return
  logsLoading.value = true
  try {
    const run = await newapiRedeemAPI.getRunLogs(runID, 100)
    requestLogs.value = [...(run.logs || [])].reverse()
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    logsLoading.value = false
  }
}

function parseImportInputs(raw: string): NewAPIRedeemAccountImportInput[] {
  const inputs: NewAPIRedeemAccountImportInput[] = []
  const seenUserIDs = new Set<string>()
  for (const [index, rawLine] of raw.split(/\r?\n/).entries()) {
    const line = rawLine.trim().replace(/^\uFEFF/, '')
    if (!line) continue
    const parts = line.split(/\s+/)
    if (parts[0]?.toLowerCase() === 'id') parts.shift()
    if (parts.length < 2) {
      throw new Error(`第 ${index + 1} 行缺少用户 ID 或访问 Key`)
    }
    const userID = parts.shift()?.trim() || ''
    const accessKey = parts.join('').trim()
    if (!userID || !accessKey) {
      throw new Error(`第 ${index + 1} 行缺少用户 ID 或访问 Key`)
    }
    if (seenUserIDs.has(userID)) {
      throw new Error(`第 ${index + 1} 行的用户 ID 重复: ${userID}`)
    }
    seenUserIDs.add(userID)
    inputs.push({ user_id: userID, access_key: accessKey })
  }
  if (inputs.length === 0) {
    throw new Error('至少需要导入一个账号')
  }
  return inputs
}

async function importAccountBatch() {
  let inputs: NewAPIRedeemAccountImportInput[]
  try {
    inputs = parseImportInputs(importText.value)
  } catch (error) {
    setNotice('error', errorMessage(error))
    return
  }
  actionID.value = 'import'
  try {
    const accounts = await newapiRedeemAPI.importAccounts(inputs)
    importText.value = ''
    const refreshResult = await refreshAccountsInBatches(accounts.map(account => account.id))
    await loadOverview()
    if (refreshResult.failed > 0) {
      setNotice(
        'error',
        t('admin.newapiRedeem.importedWithRefreshPartial', {
          count: accounts.length,
          success: refreshResult.success,
          failed: refreshResult.failed
        })
      )
      return
    }
    setNotice('success', t('admin.newapiRedeem.importedAndRefreshed', { count: accounts.length }))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

/** 导入后限并发刷新，避免一次性打爆上游。 */
async function refreshAccountsInBatches(accountIDs: string[], concurrency = 3): Promise<{ success: number; failed: number }> {
  let success = 0
  let failed = 0
  let cursor = 0
  const workers = Array.from({ length: Math.min(concurrency, accountIDs.length) }, async () => {
    while (cursor < accountIDs.length) {
      const index = cursor
      cursor += 1
      const accountID = accountIDs[index]
      try {
        await newapiRedeemAPI.refreshAccount(accountID)
        success += 1
      } catch {
        failed += 1
      }
    }
  })
  await Promise.all(workers)
  return { success, failed }
}

async function refreshAccount(accountID: string) {
  actionID.value = `refresh-${accountID}`
  try {
    await newapiRedeemAPI.refreshAccount(accountID)
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.accountRefreshed'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

/** 复用导入后的限并发路径刷新全部已导入账号。 */
async function refreshAllAccounts() {
  const accountIDs = overview.value.accounts.map(account => account.id)
  if (accountIDs.length === 0) return
  actionID.value = 'refresh-all'
  try {
    const result = await refreshAccountsInBatches(accountIDs)
    await loadOverview()
    if (result.failed > 0) {
      setNotice('error', t('admin.newapiRedeem.refreshAllPartial', result))
      return
    }
    setNotice('success', t('admin.newapiRedeem.refreshedAll', { count: result.success }))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function deleteAccount(accountID: string, userID: string) {
  if (!window.confirm(t('admin.newapiRedeem.deleteAccountConfirm', { id: userID }))) return
  actionID.value = `delete-account-${accountID}`
  try {
    await newapiRedeemAPI.deleteAccount(accountID)
    selectedAccountIDs.value = selectedAccountIDs.value.filter(id => id !== accountID)
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.accountDeleted'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function deleteVoucherFile(fileID: string, fileName: string) {
  if (!window.confirm(t('admin.newapiRedeem.deleteFileConfirm', { name: fileName }))) return
  actionID.value = `delete-file-${fileID}`
  try {
    await newapiRedeemAPI.deleteVoucherFile(fileID)
    selectedFileIDs.value = selectedFileIDs.value.filter(id => id !== fileID)
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.fileDeleted'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

function availableGroups(account: NewAPIRedeemAccount): string[] {
  return account.available_groups.length > 0 ? account.available_groups : (account.group ? [account.group] : [])
}

function keyReference(accountID: string, apiKeyID: number): string {
  return `${accountID}:${apiKeyID}`
}

function visibleKey(accountID: string, apiKeyID: number): string {
  return visibleKeys.value[keyReference(accountID, apiKeyID)] || ''
}

function keyGroup(accountID: string, apiKeyID: number, fallback: string): string {
  return keyGroups[keyReference(accountID, apiKeyID)] ?? fallback
}

function setKeyGroup(accountID: string, apiKeyID: number, event: Event) {
  keyGroups[keyReference(accountID, apiKeyID)] = (event.target as HTMLSelectElement).value
}

async function toggleAPIKeyVisibility(accountID: string, apiKeyID: number) {
  const reference = keyReference(accountID, apiKeyID)
  if (visibleKeys.value[reference]) {
    const nextKeys = { ...visibleKeys.value }
    delete nextKeys[reference]
    visibleKeys.value = nextKeys
    return
  }
  actionID.value = `reveal-${accountID}-${apiKeyID}`
  try {
    const secret = await newapiRedeemAPI.revealAPIKey(accountID, apiKeyID)
    visibleKeys.value = { ...visibleKeys.value, [reference]: secret.key }
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

/** 返回未重复展示在可关联列表中的数据库 Key 引用。 */
function databaseReferences(apiKey: NewAPIRedeemAccount['api_keys'][number]) {
  const targetIDs = new Set((apiKey.target_accounts || []).map(reference => reference.id))
  return (apiKey.referenced_accounts || []).filter(reference => !targetIDs.has(reference.id))
}

/** 将兑换工具 Key 追加或覆盖到匹配 base_url 的 Sub2API 账号。 */
async function linkAPIKey(accountID: string, apiKeyID: number, targetAccountID: number, operation: 'append' | 'replace', targetName: string) {
  const operationLabel = operation === 'replace' ? t('admin.newapiRedeem.replaceKey') : t('admin.newapiRedeem.appendKey')
  if (operation === 'replace' && !window.confirm(t('admin.newapiRedeem.replaceKeyConfirm', { name: targetName || `#${targetAccountID}` }))) return
  actionID.value = `link-${accountID}-${apiKeyID}-${targetAccountID}-${operation}`
  try {
    const result = await newapiRedeemAPI.linkAPIKey(accountID, apiKeyID, targetAccountID, operation)
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.linkedKey', { operation: operationLabel, name: result.target_account_name, count: result.key_count }))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function saveKeyGroup(accountID: string, apiKeyID: number, fallback: string) {
  const group = keyGroup(accountID, apiKeyID, fallback).trim()
  if (!group) {
    setNotice('error', t('admin.newapiRedeem.groupRequired'))
    return
  }
  actionID.value = `group-${accountID}-${apiKeyID}`
  try {
    await newapiRedeemAPI.updateAPIKeyGroup(accountID, apiKeyID, group)
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.groupUpdated'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function createAPIKey(accountID: string) {
  const name = newKeyNames[accountID]?.trim() || ''
  if (!name) return
  actionID.value = `create-${accountID}`
  try {
    await newapiRedeemAPI.createAPIKey(accountID, name, newKeyGroups[accountID] || '')
    newKeyNames[accountID] = ''
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.keyCreated'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

function selectUploadFiles(event: Event) {
  uploadFiles.value = Array.from((event.target as HTMLInputElement).files || [])
}

async function uploadSelectedFiles() {
  if (uploadFiles.value.length === 0) return
  actionID.value = 'upload'
  try {
    const saved = await newapiRedeemAPI.uploadVoucherFiles(uploadFiles.value)
    uploadFiles.value = []
    if (fileInput.value) fileInput.value.value = ''
    await loadOverview()
    setNotice('success', t('admin.newapiRedeem.uploaded', { count: saved.length }))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function startRedemption() {
  if (!canStart.value) return
  actionID.value = 'start'
  try {
    requestLogs.value = []
    const run = await newapiRedeemAPI.startRedemption({
      file_ids: selectedFileIDs.value,
      account_ids: selectedAccountIDs.value
    })
    overview.value = { ...overview.value, runs: [run, ...overview.value.runs.filter(item => item.id !== run.id)] }
    schedulePolling()
    setNotice('success', t('admin.newapiRedeem.started'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

async function cancelActiveRun() {
  if (!activeRun.value) return
  const runID = activeRun.value.id
  actionID.value = `cancel-${runID}`
  try {
    const run = await newapiRedeemAPI.cancelRun(runID)
    overview.value = { ...overview.value, runs: [run, ...overview.value.runs.filter(item => item.id !== run.id)] }
    schedulePolling()
    setNotice('success', t('admin.newapiRedeem.cancelled'))
  } catch (error) {
    setNotice('error', errorMessage(error))
  } finally {
    actionID.value = ''
  }
}

function statusLabel(status: string): string {
  const key = `admin.newapiRedeem.status.${status}`
  const translated = t(key)
  return translated === key ? status || '-' : translated
}

/** 将后端持久化的兑换请求结果转换为页面可读文本。 */
function redeemLogResultLabel(result: string): string {
  const labels: Record<string, string> = {
    redeemed: 'redeemed',
    already_redeemed: 'alreadyRedeemed',
    invalid_code: 'invalidCode',
    rate_limited: 'rateLimited',
    request_error: 'requestError',
    rejected: 'rejected'
  }
  const key = labels[result]
  return key ? t(`admin.newapiRedeem.${key}`) : result || '-'
}

/** 用颜色区分成功、限流和可观察的上游失败。 */
function redeemLogResultClass(result: string): string {
  if (result === 'redeemed' || result === 'already_redeemed') {
    return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  }
  if (result === 'rate_limited') {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  }
  return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

/** 将 NewAPI 原始额度换算成便于管理者阅读的美元金额。 */
function formatUSD(value?: string | number): string {
  if (value === undefined || value === '') return '-'
  const rawQuota = Number(value)
  if (!Number.isFinite(rawQuota)) return String(value)
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 4
  }).format(rawQuota / newAPIQuotaPerUSD)
}

onMounted(() => {
  void loadOverview()
})

onUnmounted(() => {
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
})
</script>
