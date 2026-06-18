import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPaymentStatusPoller } from '@/components/payment/PaymentStatusPolling'

describe('createPaymentStatusPoller', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('waits for each poll to finish before scheduling the next backoff interval', async () => {
    let releaseFirstPoll!: () => void
    const poll = vi.fn()
      .mockImplementationOnce(() => new Promise<void>((resolve) => {
        releaseFirstPoll = resolve
      }))
      .mockResolvedValue(undefined)

    const poller = createPaymentStatusPoller(poll)
    poller.start()

    expect(poll).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(3000)
    expect(poll).toHaveBeenCalledTimes(1)

    releaseFirstPoll()
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(2999)
    expect(poll).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1)
    expect(poll).toHaveBeenCalledTimes(2)

    poller.stop()
  })

  it('uses 3 seconds for the first 20 completed attempts and 10 seconds afterwards', async () => {
    const poll = vi.fn().mockResolvedValue(undefined)
    const poller = createPaymentStatusPoller(poll)
    poller.start()

    for (let attempt = 1; attempt < 20; attempt += 1) {
      await Promise.resolve()
      await vi.advanceTimersByTimeAsync(3000)
    }
    expect(poll).toHaveBeenCalledTimes(20)

    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(9999)
    expect(poll).toHaveBeenCalledTimes(20)

    await vi.advanceTimersByTimeAsync(1)
    expect(poll).toHaveBeenCalledTimes(21)

    poller.stop()
  })

  it('stops scheduling when the poll function returns false or stop is called', async () => {
    const terminalPoll = vi.fn()
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(false)

    const terminalPoller = createPaymentStatusPoller(terminalPoll)
    terminalPoller.start()
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(3000)
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(3000)
    expect(terminalPoll).toHaveBeenCalledTimes(2)

    const manualPoll = vi.fn().mockResolvedValue(undefined)
    const manualPoller = createPaymentStatusPoller(manualPoll)
    manualPoller.start()
    manualPoller.stop()
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(3000)
    expect(manualPoll).toHaveBeenCalledTimes(1)
  })
})
