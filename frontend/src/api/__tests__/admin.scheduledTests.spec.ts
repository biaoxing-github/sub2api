import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { listRunnerSnapshots } from '@/api/admin/scheduledTests'

describe('admin scheduled tests api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads scheduled test runner snapshots', async () => {
    const response = [
      {
        name: 'scheduled_account_probe',
        interval_ms: 60000,
        initial_delay_ms: 10000,
        running: true,
        last_started_at: '2026-06-03T10:00:00Z',
        last_finished_at: null,
        last_success_at: null,
        last_error_at: null,
        last_error: null,
        last_duration_ms: 0,
        max_duration_ms: 0,
        run_count: 2,
        success_count: 1,
        failure_count: 0,
        skipped_count: 1,
      },
    ]
    get.mockResolvedValue({ data: response })

    const result = await listRunnerSnapshots({ signal: expect.any(AbortSignal) as AbortSignal })

    expect(get).toHaveBeenCalledWith('/admin/scheduled-test-runner/snapshots', {
      signal: expect.any(AbortSignal),
    })
    expect(result).toEqual(response)
  })
})
