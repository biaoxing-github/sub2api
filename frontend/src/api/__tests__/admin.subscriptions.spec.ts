import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

import type { UserSubscription } from '@/types'
import { restore, revoke } from '@/api/admin/subscriptions'

const suspendedSubscription: UserSubscription = {
  id: 42,
  user_id: 7,
  group_id: 9,
  status: 'suspended',
  starts_at: '2026-07-01T00:00:00Z',
  expires_at: '2026-08-01T00:00:00Z',
  revoked_at: null,
  daily_usage_usd: 0,
  weekly_usage_usd: 0,
  monthly_usage_usd: 0,
  daily_window_start: null,
  weekly_window_start: null,
  monthly_window_start: null,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
}

describe('admin subscriptions api', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('posts revoke and restore requests to the explicit subscription routes', async () => {
    post
      .mockResolvedValueOnce({ data: { message: 'Subscription revoked successfully' } })
      .mockResolvedValueOnce({ data: suspendedSubscription })

    await expect(revoke(42)).resolves.toEqual({ message: 'Subscription revoked successfully' })
    await expect(restore(42)).resolves.toEqual(suspendedSubscription)

    expect(post).toHaveBeenNthCalledWith(1, '/admin/subscriptions/42/revoke')
    expect(post).toHaveBeenNthCalledWith(2, '/admin/subscriptions/42/restore')
  })

  it('accepts suspended subscriptions and their optional revocation timestamp', () => {
    expect(suspendedSubscription.status).toBe('suspended')
    expect(suspendedSubscription.revoked_at).toBeNull()
  })
})
