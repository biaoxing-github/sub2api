export type PaymentStatusPollResult = void | boolean
export type PaymentStatusPollTask = () => PaymentStatusPollResult | Promise<PaymentStatusPollResult>

export interface PaymentStatusPoller {
  start: () => void
  stop: () => void
}

const FAST_POLL_INTERVAL_MS = 3000
const SLOW_POLL_INTERVAL_MS = 10000
const FAST_POLL_ATTEMPTS = 20

function getNextPollDelay(completedAttempts: number): number {
  return completedAttempts < FAST_POLL_ATTEMPTS
    ? FAST_POLL_INTERVAL_MS
    : SLOW_POLL_INTERVAL_MS
}

export function createPaymentStatusPoller(poll: PaymentStatusPollTask): PaymentStatusPoller {
  let timer: ReturnType<typeof setTimeout> | null = null
  let stopped = true
  let running = false
  let completedAttempts = 0

  function clearScheduledPoll() {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  function stop() {
    stopped = true
    clearScheduledPoll()
  }

  async function runOnce() {
    if (stopped || running) return
    running = true
    try {
      const shouldContinue = await poll()
      completedAttempts += 1
      if (shouldContinue === false) {
        stop()
        return
      }
    } finally {
      running = false
    }

    if (!stopped) {
      timer = setTimeout(runOnce, getNextPollDelay(completedAttempts))
    }
  }

  function start() {
    stop()
    stopped = false
    completedAttempts = 0
    void runOnce()
  }

  return { start, stop }
}
