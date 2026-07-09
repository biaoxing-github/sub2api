/**
 * 管理端 Grok/xAI OAuth API。
 */

import { apiClient } from '../client'

export interface GrokAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
}

export interface GrokAuthUrlRequest {
  proxy_id?: number
  redirect_uri?: string
}

export interface GrokExchangeCodeRequest {
  session_id: string
  state: string
  code: string
  proxy_id?: number
  redirect_uri?: string
}

export interface GrokTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  id_token?: string
  expires_at?: number | string
  expires_in?: number
  scope?: string
  client_id?: string
  email?: string
  subscription_tier?: string
  entitlement_status?: string
  [key: string]: unknown
}

// generateAuthUrl 生成 Grok OAuth 授权链接和临时会话。
export async function generateAuthUrl(payload: GrokAuthUrlRequest): Promise<GrokAuthUrlResponse> {
  const { data } = await apiClient.post<GrokAuthUrlResponse>('/admin/grok/oauth/auth-url', payload)
  return data
}

// exchangeCode 使用授权码换取 Grok token 信息。
export async function exchangeCode(payload: GrokExchangeCodeRequest): Promise<GrokTokenInfo> {
  const { data } = await apiClient.post<GrokTokenInfo>('/admin/grok/oauth/exchange-code', payload)
  return data
}

// refreshGrokToken 验证手动粘贴的 Grok refresh_token。
export async function refreshGrokToken(
  refreshToken: string,
  proxyId?: number | null
): Promise<GrokTokenInfo> {
  const payload: Record<string, unknown> = { refresh_token: refreshToken }
  if (proxyId) payload.proxy_id = proxyId

  const { data } = await apiClient.post<GrokTokenInfo>('/admin/grok/oauth/refresh-token', payload)
  return data
}

export default { generateAuthUrl, exchangeCode, refreshGrokToken }
