import { describe, expect, it } from 'vitest'
import {
  ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY,
  accountAvailabilityScheduleFormFingerprint,
  accountScheduleSummary,
  buildAccountAvailabilitySchedulePayload,
  createAccountAvailabilityScheduleForm,
  createAccountScheduleExceptionFormRow,
  createAccountScheduleWindowFormRow,
  isScheduleCurrentlyAllowed,
  validateAccountAvailabilityScheduleForm,
  writeAccountAvailabilityScheduleToExtra
} from '../accountAvailabilitySchedule'

describe('accountAvailabilitySchedule', () => {
  it('构建启用的可用时段 payload 并归一化星期', () => {
    const form = createAccountAvailabilityScheduleForm()
    form.enabled = true
    form.timezone = 'Asia/Shanghai'
    form.windows = [createAccountScheduleWindowFormRow([5, 1, 1], '09:00', '17:30')]
    form.dateRange = { startDate: '2026-06-01', endDate: '2026-06-30' }
    form.exceptions = [
      createAccountScheduleExceptionFormRow('2026-06-08', 'deny'),
      createAccountScheduleExceptionFormRow('2026-06-09', 'allow', [{ start: '10:00', end: '11:00' }])
    ]

    expect(validateAccountAvailabilityScheduleForm(form)).toBeUndefined()
    expect(buildAccountAvailabilitySchedulePayload(form)).toEqual({
      enabled: true,
      timezone: 'Asia/Shanghai',
      mode: 'allow_windows',
      windows: [{ daysOfWeek: [1, 5], start: '09:00', end: '17:30' }],
      dateRange: { startDate: '2026-06-01', endDate: '2026-06-30' },
      exceptions: [
        { date: '2026-06-08', action: 'deny' },
        { date: '2026-06-09', action: 'allow', windows: [{ start: '10:00', end: '11:00' }] }
      ]
    })
  })

  it('关闭时从 extra 删除 availability_schedule', () => {
    const extra: Record<string, unknown> = {
      keep_me: true,
      [ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY]: { enabled: true }
    }
    const form = createAccountAvailabilityScheduleForm()

    writeAccountAvailabilityScheduleToExtra(extra, form)

    expect(extra).toEqual({ keep_me: true })
  })

  it('启用时写入 extra.availability_schedule 并保留其它字段', () => {
    const extra: Record<string, unknown> = { keep_me: true }
    const form = createAccountAvailabilityScheduleForm()
    form.enabled = true
    form.timezone = 'UTC'
    form.windows = [createAccountScheduleWindowFormRow([1], '09:00', '17:00')]

    writeAccountAvailabilityScheduleToExtra(extra, form)

    expect(extra.keep_me).toBe(true)
    expect(extra[ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY]).toMatchObject({
      enabled: true,
      timezone: 'UTC',
      mode: 'allow_windows',
      windows: [{ daysOfWeek: [1], start: '09:00', end: '17:00' }]
    })
  })

  it('按时区、跨午夜和例外日期判断当前是否可用', () => {
    const schedule = {
      enabled: true,
      timezone: 'UTC',
      mode: 'allow_windows',
      windows: [{ daysOfWeek: [1], start: '22:00', end: '02:00' }],
      exceptions: [{ date: '2026-06-09', action: 'deny' }]
    } as const

    expect(isScheduleCurrentlyAllowed(schedule, new Date('2026-06-08T23:30:00.000Z'))).toBe(true)
    expect(isScheduleCurrentlyAllowed(schedule, new Date('2026-06-09T01:30:00.000Z'))).toBe(false)
  })

  it('摘要和 fingerprint 只跟有效业务字段有关', () => {
    const form = createAccountAvailabilityScheduleForm()
    form.enabled = true
    form.timezone = 'UTC'
    form.windows = [createAccountScheduleWindowFormRow([1, 2, 3, 4, 5], '09:00', '18:00')]

    const fingerprint = accountAvailabilityScheduleFormFingerprint(form)
    form.windows[0].key = 'changed-key'

    expect(accountAvailabilityScheduleFormFingerprint(form)).toBe(fingerprint)
    expect(accountScheduleSummary(buildAccountAvailabilitySchedulePayload(form)!, new Date('2026-06-08T10:00:00.000Z'))).toContain('工作日')
  })

  it('拦截不完整或非法的启用配置', () => {
    const form = createAccountAvailabilityScheduleForm()
    form.enabled = true
    form.timezone = 'Invalid/Timezone'
    form.windows = [createAccountScheduleWindowFormRow([], '09:00', '09:00')]

    expect(validateAccountAvailabilityScheduleForm(form)).toBeTruthy()
  })
})
