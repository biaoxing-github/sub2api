import type {
  AccountAvailabilitySchedule,
  AccountAvailabilityScheduleDateRange,
  AccountAvailabilityScheduleException,
  AccountAvailabilityScheduleExceptionWindow
} from '@/types'

export const ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY = 'availability_schedule'

export interface AccountScheduleWindowFormRow {
  key: string
  daysOfWeek: number[]
  start: string
  end: string
}

export interface AccountScheduleExceptionWindowFormRow {
  key: string
  start: string
  end: string
}

export interface AccountScheduleExceptionFormRow {
  key: string
  date: string
  action: 'allow' | 'deny'
  windows: AccountScheduleExceptionWindowFormRow[]
}

export interface AccountAvailabilityScheduleForm {
  enabled: boolean
  timezone: string
  windows: AccountScheduleWindowFormRow[]
  dateRange?: AccountAvailabilityScheduleDateRange
  exceptions: AccountScheduleExceptionFormRow[]
}

let scheduleRowKeySeed = 0

export const weekdayOptions = [
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 },
  { label: '周日', value: 7 }
]

export function createAccountAvailabilityScheduleForm(
  schedule?: AccountAvailabilitySchedule | null
): AccountAvailabilityScheduleForm {
  if (!schedule?.enabled) {
    return {
      enabled: false,
      timezone: defaultScheduleTimezone(),
      windows: [createAccountScheduleWindowFormRow()],
      exceptions: []
    }
  }
  assertAccountAvailabilitySchedule(schedule)
  return {
    enabled: true,
    timezone: schedule.timezone,
    windows: schedule.windows.map((window) =>
      createAccountScheduleWindowFormRow(window.daysOfWeek, window.start, window.end)
    ),
    dateRange: cloneScheduleDateRange(schedule.dateRange),
    exceptions: cloneScheduleExceptionsToForm(schedule.exceptions)
  }
}

export function cloneAccountAvailabilityScheduleForm(
  schedule: AccountAvailabilityScheduleForm
): AccountAvailabilityScheduleForm {
  return {
    enabled: schedule.enabled,
    timezone: schedule.timezone,
    windows: schedule.windows.map((window) =>
      createAccountScheduleWindowFormRow(window.daysOfWeek, window.start, window.end)
    ),
    dateRange: cloneScheduleDateRange(schedule.dateRange),
    exceptions: schedule.exceptions.map((exception) =>
      createAccountScheduleExceptionFormRow(
        exception.date,
        exception.action,
        exception.windows.map((window) => ({ start: window.start, end: window.end }))
      )
    )
  }
}

export function createAccountScheduleWindowFormRow(
  daysOfWeek = [1, 2, 3, 4, 5, 6, 7],
  start = '09:00',
  end = '18:00'
): AccountScheduleWindowFormRow {
  return {
    key: nextScheduleFormKey('window'),
    daysOfWeek: [...daysOfWeek],
    start,
    end
  }
}

export function createAccountScheduleExceptionWindowFormRow(
  start = '09:00',
  end = '18:00'
): AccountScheduleExceptionWindowFormRow {
  return {
    key: nextScheduleFormKey('exception_window'),
    start,
    end
  }
}

export function createAccountScheduleExceptionFormRow(
  date = '',
  action: 'allow' | 'deny' = 'deny',
  windows?: AccountAvailabilityScheduleExceptionWindow[]
): AccountScheduleExceptionFormRow {
  return {
    key: nextScheduleFormKey('exception'),
    date,
    action,
    windows: action === 'allow'
      ? (windows?.length
          ? windows.map((window) => createAccountScheduleExceptionWindowFormRow(window.start, window.end))
          : [createAccountScheduleExceptionWindowFormRow()])
      : []
  }
}

export function readAccountAvailabilityScheduleFromExtra(
  extra?: Record<string, unknown> | null
): AccountAvailabilitySchedule | undefined {
  const raw = extra?.[ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY]
  if (!raw || typeof raw !== 'object') return undefined
  try {
    assertAccountAvailabilitySchedule(raw as AccountAvailabilitySchedule)
    return raw as AccountAvailabilitySchedule
  } catch {
    return undefined
  }
}

export function validateAccountAvailabilityScheduleForm(
  schedule: AccountAvailabilityScheduleForm
): string | undefined {
  if (!schedule.enabled) return undefined
  if (!isValidScheduleTimezone(schedule.timezone)) return '请填写有效的可用时段时区'
  const windows = normalizedScheduleWindows(schedule)
  const invalidWindowIndex = windows.findIndex((window) =>
    hasInvalidScheduleDays(window.daysOfWeek) || !validScheduleWindow(window.start, window.end)
  )
  if (invalidWindowIndex >= 0) return `请完整填写第 ${invalidWindowIndex + 1} 个可用时段`
  if (schedule.dateRange && !validScheduleDateRange(schedule.dateRange)) {
    return '请填写有效的可用日期范围'
  }
  const invalidExceptionIndex = schedule.exceptions.findIndex((exception) => {
    if (!validScheduleDate(exception.date)) return true
    if (exception.action === 'deny') return false
    if (exception.action !== 'allow' || exception.windows.length === 0) return true
    return exception.windows.some((window) => !validScheduleWindow(window.start, window.end))
  })
  if (invalidExceptionIndex >= 0) return `请完整填写第 ${invalidExceptionIndex + 1} 个例外日期`
  return undefined
}

export function buildAccountAvailabilitySchedulePayload(
  schedule: AccountAvailabilityScheduleForm
): AccountAvailabilitySchedule | null {
  if (!schedule.enabled) return null
  const dateRange = normalizeScheduleDateRange(schedule.dateRange)
  const exceptions = normalizeScheduleExceptions(schedule.exceptions)
  return {
    enabled: true,
    timezone: schedule.timezone.trim(),
    mode: 'allow_windows',
    windows: normalizedScheduleWindows(schedule).map((window) => ({
      daysOfWeek: window.daysOfWeek,
      start: window.start,
      end: window.end
    })),
    ...(dateRange ? { dateRange } : {}),
    ...(exceptions.length ? { exceptions } : {})
  }
}

export function writeAccountAvailabilityScheduleToExtra(
  extra: Record<string, unknown>,
  schedule: AccountAvailabilityScheduleForm
): void {
  const payload = buildAccountAvailabilitySchedulePayload(schedule)
  if (payload) {
    extra[ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY] = payload
  } else {
    delete extra[ACCOUNT_AVAILABILITY_SCHEDULE_EXTRA_KEY]
  }
}

export function accountAvailabilityScheduleFormFingerprint(
  schedule: AccountAvailabilityScheduleForm
): string {
  if (!schedule.enabled) return JSON.stringify({ enabled: false })
  return JSON.stringify(buildAccountAvailabilitySchedulePayload(schedule))
}

export function accountScheduleSummary(
  schedule?: AccountAvailabilitySchedule | null,
  now = new Date()
): string {
  if (!schedule?.enabled || schedule.windows.length === 0) return '未设置'
  try {
    assertAccountAvailabilitySchedule(schedule)
  } catch {
    return '计划数据异常'
  }
  const windows = schedule.windows
    .slice(0, 2)
    .map((window) => `${daysOfWeekText(window.daysOfWeek)} ${scheduleWindowText(window.start, window.end)}`)
  const suffix = schedule.windows.length > 2 ? ` 等 ${schedule.windows.length} 段` : ''
  const current = isScheduleCurrentlyAllowed(schedule, now) ? '当前可用' : '计划停用'
  return `${current}：${windows.join(' / ')}${suffix}`
}

export function isScheduleCurrentlyAllowed(
  schedule: AccountAvailabilitySchedule,
  now = new Date()
): boolean {
  if (!schedule.enabled) return true
  assertAccountAvailabilitySchedule(schedule)
  const current = zonedScheduleParts(now, schedule.timezone)
  if (schedule.dateRange?.startDate && current.dateKey < schedule.dateRange.startDate) return false
  if (schedule.dateRange?.endDate && current.dateKey > schedule.dateRange.endDate) return false
  const exception = schedule.exceptions?.find((item) => item.date === current.dateKey)
  if (exception?.action === 'deny') return false
  if (exception?.action === 'allow') {
    return (exception.windows ?? []).some((window) =>
      isCurrentMinuteInScheduleWindow(current, {
        daysOfWeek: [current.dayOfWeek],
        start: window.start,
        end: window.end
      })
    )
  }
  return schedule.windows.some((window) => isCurrentMinuteInScheduleWindow(current, window))
}

function assertAccountAvailabilitySchedule(schedule: AccountAvailabilitySchedule): void {
  if (!schedule || typeof schedule !== 'object') throw new Error('账户可用时段必须是对象')
  if (schedule.enabled !== true) throw new Error('账户可用时段启用状态异常')
  if (schedule.mode !== 'allow_windows') throw new Error('账户可用时段模式异常')
  if (!isValidScheduleTimezone(schedule.timezone)) throw new Error('账户可用时段时区异常')
  if (!Array.isArray(schedule.windows) || schedule.windows.length === 0) throw new Error('账户可用时段为空')
  for (const window of schedule.windows) {
    if (hasInvalidScheduleDays(window.daysOfWeek) || !validScheduleWindow(window.start, window.end)) {
      throw new Error('账户可用时段异常')
    }
  }
  if (schedule.dateRange && !validScheduleDateRange(schedule.dateRange)) {
    throw new Error('账户可用日期范围异常')
  }
  if (schedule.exceptions !== undefined) {
    if (!Array.isArray(schedule.exceptions)) throw new Error('账户可用时段例外异常')
    for (const exception of schedule.exceptions) {
      if (!validScheduleDate(exception.date)) throw new Error('账户可用时段例外日期异常')
      if (exception.action === 'deny') {
        if (exception.windows?.length) throw new Error('账户可用时段拒绝例外不能带允许时段')
      } else if (exception.action === 'allow') {
        if (!exception.windows?.length) throw new Error('账户可用时段允许例外缺少时段')
        for (const window of exception.windows) {
          if (!validScheduleWindow(window.start, window.end)) throw new Error('账户可用时段例外窗口异常')
        }
      } else {
        throw new Error('账户可用时段例外动作异常')
      }
    }
  }
}

function normalizedScheduleWindows(
  schedule: AccountAvailabilityScheduleForm
): Array<{ daysOfWeek: number[]; start: string; end: string }> {
  return schedule.windows.map((window) => ({
    daysOfWeek: normalizedScheduleDays(window.daysOfWeek),
    start: window.start,
    end: window.end
  }))
}

function normalizedScheduleDays(days: number[]): number[] {
  return [...new Set(days.map((day) => Number(day)))].sort((left, right) => left - right)
}

function normalizeScheduleDateRange(
  dateRange?: AccountAvailabilityScheduleDateRange
): AccountAvailabilityScheduleDateRange | undefined {
  if (!dateRange?.startDate && !dateRange?.endDate) return undefined
  return {
    ...(dateRange.startDate ? { startDate: dateRange.startDate } : {}),
    ...(dateRange.endDate ? { endDate: dateRange.endDate } : {})
  }
}

function normalizeScheduleExceptions(
  exceptions: AccountScheduleExceptionFormRow[]
): AccountAvailabilityScheduleException[] {
  return exceptions.map((exception) => {
    if (exception.action === 'allow') {
      return {
        date: exception.date,
        action: 'allow',
        windows: exception.windows.map((window) => ({ start: window.start, end: window.end }))
      }
    }
    return {
      date: exception.date,
      action: 'deny'
    }
  })
}

function cloneScheduleDateRange(
  dateRange?: AccountAvailabilityScheduleDateRange
): AccountAvailabilityScheduleDateRange | undefined {
  return dateRange ? { ...dateRange } : undefined
}

function cloneScheduleExceptionsToForm(
  exceptions?: AccountAvailabilityScheduleException[]
): AccountScheduleExceptionFormRow[] {
  return (exceptions ?? []).map((exception) =>
    createAccountScheduleExceptionFormRow(exception.date, exception.action, exception.windows)
  )
}

function hasInvalidScheduleDays(days: number[]): boolean {
  return !days.length || days.some((day) => !Number.isInteger(day) || day < 1 || day > 7)
}

function validScheduleWindow(start: string, end: string): boolean {
  return validScheduleTime(start) && validScheduleTime(end) && start !== end
}

function validScheduleTime(value: string): boolean {
  return /^([01]\d|2[0-3]):([0-5]\d)$/.test(value)
}

function validScheduleDateRange(dateRange: AccountAvailabilityScheduleDateRange): boolean {
  if (dateRange.startDate && !validScheduleDate(dateRange.startDate)) return false
  if (dateRange.endDate && !validScheduleDate(dateRange.endDate)) return false
  return !dateRange.startDate || !dateRange.endDate || dateRange.startDate <= dateRange.endDate
}

function validScheduleDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value
}

function isValidScheduleTimezone(value: string): boolean {
  if (!value?.trim()) return false
  try {
    new Intl.DateTimeFormat('en-CA', { timeZone: value }).format(new Date(0))
    return true
  } catch {
    return false
  }
}

function scheduleWindowText(start: string, end: string): string {
  return start > end ? `${start}-次日 ${end}` : `${start}-${end}`
}

function daysOfWeekText(days: number[]): string {
  const normalized = [...new Set(days)].sort((left, right) => left - right).join(',')
  if (normalized === '1,2,3,4,5,6,7') return '每天'
  if (normalized === '1,2,3,4,5') return '工作日'
  if (normalized === '6,7') return '周末'
  const labels = new Map(weekdayOptions.map((item) => [item.value, item.label]))
  return [...new Set(days)]
    .sort((left, right) => left - right)
    .map((day) => labels.get(day) ?? `周${day}`)
    .join('、')
}

function isCurrentMinuteInScheduleWindow(
  current: { dayOfWeek: number; minuteOfDay: number },
  window: { daysOfWeek: number[]; start: string; end: string }
): boolean {
  const start = scheduleMinuteOfDay(window.start)
  const end = scheduleMinuteOfDay(window.end)
  const days = new Set(window.daysOfWeek)
  if (start < end) {
    return days.has(current.dayOfWeek) && current.minuteOfDay >= start && current.minuteOfDay < end
  }
  return (
    (days.has(current.dayOfWeek) && current.minuteOfDay >= start) ||
    (days.has(previousScheduleDayOfWeek(current.dayOfWeek)) && current.minuteOfDay < end)
  )
}

function scheduleMinuteOfDay(value: string): number {
  const [hour, minute] = value.split(':').map((item) => Number(item))
  return hour * 60 + minute
}

function previousScheduleDayOfWeek(dayOfWeek: number): number {
  return dayOfWeek === 1 ? 7 : dayOfWeek - 1
}

function zonedScheduleParts(date: Date, timezone: string): {
  dateKey: string
  dayOfWeek: number
  minuteOfDay: number
} {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  })
  const parts = Object.fromEntries(formatter.formatToParts(date).map((part) => [part.type, part.value]))
  const year = Number(parts.year)
  const month = Number(parts.month)
  const day = Number(parts.day)
  const hour = Number(parts.hour)
  const minute = Number(parts.minute)
  const dateKey = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
  const utcDay = new Date(Date.UTC(year, month - 1, day)).getUTCDay()
  return {
    dateKey,
    dayOfWeek: utcDay === 0 ? 7 : utcDay,
    minuteOfDay: hour * 60 + minute
  }
}

function defaultScheduleTimezone(): string {
  if (typeof Intl === 'undefined') return 'Asia/Shanghai'
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Shanghai'
}

function nextScheduleFormKey(prefix: string): string {
  scheduleRowKeySeed += 1
  return `${prefix}_${Date.now()}_${scheduleRowKeySeed}`
}
