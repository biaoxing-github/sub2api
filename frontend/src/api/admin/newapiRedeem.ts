/**
 * NewAPI 兑换工具管理端接口。
 * 所有账号访问凭据仅由导入请求提交，后续响应使用脱敏展示模型。
 */

import { apiClient } from '../client'
import type {
  NewAPIRedeemAccount,
  NewAPIRedeemAccountImportInput,
  NewAPIRedeemAPIKeySecret,
  NewAPIRedeemLinkAPIKeyResult,
  NewAPIRedeemOverview,
  NewAPIRedeemRun,
  NewAPIRedeemStartRunRequest,
  NewAPIRedeemVoucherFile
} from '@/types'

const basePath = '/admin/newapi-redeem'

/** 读取账号、兑换码文件和近期任务的聚合状态。 */
export async function getOverview(): Promise<NewAPIRedeemOverview> {
  const { data } = await apiClient.get<NewAPIRedeemOverview>(`${basePath}/overview`, { params: { include_logs: false } })
  return data
}

/** 批量导入或更新 Key 模式 NewAPI 账号。 */
export async function importAccounts(accounts: NewAPIRedeemAccountImportInput[]): Promise<NewAPIRedeemAccount[]> {
  const { data } = await apiClient.post<NewAPIRedeemAccount[]>(`${basePath}/accounts/import`, { accounts })
  return data
}

/** 删除本地导入的 NewAPI 账号及其成功标记。 */
export async function deleteAccount(accountID: string): Promise<void> {
  await apiClient.delete(`${basePath}/accounts/${encodeURIComponent(accountID)}`)
}

/** 刷新一个账号的上游余额、分组与 API Key 快照。 */
export async function refreshAccount(accountID: string): Promise<NewAPIRedeemAccount> {
  const { data } = await apiClient.post<NewAPIRedeemAccount>(`${basePath}/accounts/${encodeURIComponent(accountID)}/refresh`)
  return data
}

/** 为指定账号创建一个上游 API Key。 */
export async function createAPIKey(accountID: string, name: string, group: string): Promise<NewAPIRedeemAccount> {
  const { data } = await apiClient.post<NewAPIRedeemAccount>(
    `${basePath}/accounts/${encodeURIComponent(accountID)}/api-keys`,
    { name, group }
  )
  return data
}

/** 修改指定上游 API Key 的分组。 */
export async function updateAPIKeyGroup(accountID: string, apiKeyID: number, group: string): Promise<NewAPIRedeemAccount> {
  const { data } = await apiClient.put<NewAPIRedeemAccount>(
    `${basePath}/accounts/${encodeURIComponent(accountID)}/api-keys/${apiKeyID}/group`,
    { group }
  )
  return data
}

/** 显式查看一个上游 API Key 的明文值。 */
export async function revealAPIKey(accountID: string, apiKeyID: number): Promise<NewAPIRedeemAPIKeySecret> {
  const { data } = await apiClient.post<NewAPIRedeemAPIKeySecret>(
    `${basePath}/accounts/${encodeURIComponent(accountID)}/api-keys/${apiKeyID}/reveal`
  )
  return data
}

/** 将兑换工具 Key 追加或覆盖到同 base_url 的 Sub2API 账号。 */
export async function linkAPIKey(
  accountID: string,
  apiKeyID: number,
  targetAccountID: number,
  operation: 'append' | 'replace'
): Promise<NewAPIRedeemLinkAPIKeyResult> {
  const { data } = await apiClient.post<NewAPIRedeemLinkAPIKeyResult>(
    `${basePath}/accounts/${encodeURIComponent(accountID)}/api-keys/${apiKeyID}/link`,
    { target_account_id: targetAccountID, operation }
  )
  return data
}

/** 上传一个或多个按行存放兑换码的文本文件。 */
export async function uploadVoucherFiles(files: File[]): Promise<NewAPIRedeemVoucherFile[]> {
  if (files.length === 0) {
    throw new Error('请选择至少一个兑换码文件')
  }
  const form = new FormData()
  files.forEach(file => form.append('files', file))
  // 覆盖 apiClient 的 JSON 默认头，让浏览器为 FormData 写入 multipart boundary。
  const { data } = await apiClient.post<NewAPIRedeemVoucherFile[]>(`${basePath}/files`, form, {
    headers: { 'Content-Type': false }
  })
  return data
}

/** 删除本地兑换码文件及其成功标记。 */
export async function deleteVoucherFile(fileID: string): Promise<void> {
  await apiClient.delete(`${basePath}/files/${encodeURIComponent(fileID)}`)
}

/** 启动选择文件和账号组合的异步兑换任务。 */
export async function startRedemption(request: NewAPIRedeemStartRunRequest): Promise<NewAPIRedeemRun> {
  const { data } = await apiClient.post<NewAPIRedeemRun>(`${basePath}/runs`, request)
  return data
}

/** 获取一个异步兑换任务的最新状态。 */
export async function getRun(runID: string): Promise<NewAPIRedeemRun> {
  const { data } = await apiClient.get<NewAPIRedeemRun>(`${basePath}/runs/${encodeURIComponent(runID)}`, { params: { include_logs: false } })
  return data
}

/** 按需获取任务最近的日志快照，页面轮询不会调用此接口。 */
export async function getRunLogs(runID: string, limit = 100): Promise<NewAPIRedeemRun> {
  const { data } = await apiClient.get<NewAPIRedeemRun>(`${basePath}/runs/${encodeURIComponent(runID)}`, {
    params: { include_logs: true, log_limit: limit }
  })
  return data
}

/** 请求停止仍在执行的兑换任务。 */
export async function cancelRun(runID: string): Promise<NewAPIRedeemRun> {
  const { data } = await apiClient.post<NewAPIRedeemRun>(`${basePath}/runs/${encodeURIComponent(runID)}/cancel`)
  return data
}

export const newapiRedeemAPI = {
  getOverview,
  importAccounts,
  deleteAccount,
  refreshAccount,
  createAPIKey,
  updateAPIKeyGroup,
  revealAPIKey,
  linkAPIKey,
  uploadVoucherFiles,
  deleteVoucherFile,
  startRedemption,
  getRun,
  getRunLogs,
  cancelRun
}

export default newapiRedeemAPI
