package service

import (
	"encoding/json"
	"time"
)

const (
	// AccountAvailabilityScheduleExtraKey 是账号 extra 中保存可用时段计划的统一字段。
	AccountAvailabilityScheduleExtraKey = "availability_schedule"
)

type accountAvailabilitySchedule struct {
	// Enabled 表示该计划是否参与调度判断；未启用时账号沿用原调度状态。
	Enabled bool `json:"enabled"`
	// Timezone 是判断日期、星期和分钟的 IANA 时区名称。
	Timezone string `json:"timezone"`
	// Mode 当前只支持 allow_windows，表示仅允许配置的时间窗调度。
	Mode string `json:"mode"`
	// Windows 是常规每周允许调度的时间窗列表。
	Windows []accountAvailabilityScheduleWindow `json:"windows"`
	// DateRange 限制计划生效日期范围。
	DateRange *accountAvailabilityScheduleDateRange `json:"dateRange"`
	// Exceptions 是指定日期的允许或拒绝例外。
	Exceptions []accountAvailabilityScheduleException `json:"exceptions"`
}

type accountAvailabilityScheduleWindow struct {
	// DaysOfWeek 使用 1-7 表示周一到周日。
	DaysOfWeek []int `json:"daysOfWeek"`
	// Start 是 HH:mm 格式的开始分钟，包含该分钟。
	Start string `json:"start"`
	// End 是 HH:mm 格式的结束分钟，不包含该分钟；小于 Start 表示跨午夜。
	End string `json:"end"`
}

type accountAvailabilityScheduleDateRange struct {
	// StartDate 是可选的本地日期下限，格式 YYYY-MM-DD。
	StartDate string `json:"startDate"`
	// EndDate 是可选的本地日期上限，格式 YYYY-MM-DD。
	EndDate string `json:"endDate"`
}

type accountAvailabilityScheduleException struct {
	// Date 是例外生效的本地日期，格式 YYYY-MM-DD。
	Date string `json:"date"`
	// Action 为 allow 时只允许 Windows；为 deny 时整日禁止调度。
	Action string `json:"action"`
	// Windows 是 allow 例外日期上的允许时间窗。
	Windows []accountAvailabilityScheduleExceptionWindow `json:"windows"`
}

type accountAvailabilityScheduleExceptionWindow struct {
	// Start 是 HH:mm 格式的开始分钟，包含该分钟。
	Start string `json:"start"`
	// End 是 HH:mm 格式的结束分钟，不包含该分钟；小于 Start 表示跨午夜。
	End string `json:"end"`
}

type accountAvailabilityScheduleParts struct {
	dateKey     string
	dayOfWeek   int
	minuteOfDay int
}

// IsAvailabilityScheduleAllowedAt 判断账号当前是否落在管理员配置的可用时段内。
func (a *Account) IsAvailabilityScheduleAllowedAt(now time.Time) bool {
	schedule, ok := a.availabilitySchedule()
	if !ok || !schedule.Enabled {
		return true
	}
	return schedule.allowedAt(now)
}

func (a *Account) availabilitySchedule() (accountAvailabilitySchedule, bool) {
	if a == nil || len(a.Extra) == 0 {
		return accountAvailabilitySchedule{}, false
	}
	raw, ok := a.Extra[AccountAvailabilityScheduleExtraKey]
	if !ok || raw == nil {
		return accountAvailabilitySchedule{}, false
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return accountAvailabilitySchedule{Enabled: true}, true
	}
	var schedule accountAvailabilitySchedule
	if err := json.Unmarshal(payload, &schedule); err != nil {
		return accountAvailabilitySchedule{Enabled: true}, true
	}
	return schedule, true
}

func (s accountAvailabilitySchedule) allowedAt(now time.Time) bool {
	if !s.Enabled {
		return true
	}
	location, ok := s.location()
	if !ok || !s.valid() {
		return false
	}
	parts := availabilityScheduleParts(now, location)
	if s.DateRange != nil {
		if s.DateRange.StartDate != "" && parts.dateKey < s.DateRange.StartDate {
			return false
		}
		if s.DateRange.EndDate != "" && parts.dateKey > s.DateRange.EndDate {
			return false
		}
	}
	for _, exception := range s.Exceptions {
		if exception.Date != parts.dateKey {
			continue
		}
		if exception.Action == "deny" {
			return false
		}
		if exception.Action == "allow" {
			for _, window := range exception.Windows {
				if scheduleWindowContains(parts, []int{parts.dayOfWeek}, window.Start, window.End) {
					return true
				}
			}
			return false
		}
	}
	for _, window := range s.Windows {
		if scheduleWindowContains(parts, window.DaysOfWeek, window.Start, window.End) {
			return true
		}
	}
	return false
}

func (s accountAvailabilitySchedule) location() (*time.Location, bool) {
	if s.Timezone == "" {
		return nil, false
	}
	location, err := time.LoadLocation(s.Timezone)
	return location, err == nil
}

func (s accountAvailabilitySchedule) valid() bool {
	if s.Mode != "allow_windows" || len(s.Windows) == 0 {
		return false
	}
	for _, window := range s.Windows {
		if !validScheduleDays(window.DaysOfWeek) || !validScheduleWindow(window.Start, window.End) {
			return false
		}
	}
	if s.DateRange != nil {
		if !validScheduleDateRange(*s.DateRange) {
			return false
		}
	}
	for _, exception := range s.Exceptions {
		if !validScheduleDate(exception.Date) {
			return false
		}
		switch exception.Action {
		case "deny":
			if len(exception.Windows) > 0 {
				return false
			}
		case "allow":
			if len(exception.Windows) == 0 {
				return false
			}
			for _, window := range exception.Windows {
				if !validScheduleWindow(window.Start, window.End) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func availabilityScheduleParts(now time.Time, location *time.Location) accountAvailabilityScheduleParts {
	local := now.In(location)
	dayOfWeek := int(local.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	return accountAvailabilityScheduleParts{
		dateKey:     local.Format("2006-01-02"),
		dayOfWeek:   dayOfWeek,
		minuteOfDay: local.Hour()*60 + local.Minute(),
	}
}

func scheduleWindowContains(parts accountAvailabilityScheduleParts, days []int, startText, endText string) bool {
	start := scheduleMinuteOfDay(startText)
	end := scheduleMinuteOfDay(endText)
	daySet := make(map[int]struct{}, len(days))
	for _, day := range days {
		daySet[day] = struct{}{}
	}
	if start < end {
		_, ok := daySet[parts.dayOfWeek]
		return ok && parts.minuteOfDay >= start && parts.minuteOfDay < end
	}
	if _, ok := daySet[parts.dayOfWeek]; ok && parts.minuteOfDay >= start {
		return true
	}
	_, ok := daySet[previousScheduleDay(parts.dayOfWeek)]
	return ok && parts.minuteOfDay < end
}

func scheduleMinuteOfDay(value string) int {
	return int(value[0]-'0')*600 + int(value[1]-'0')*60 + int(value[3]-'0')*10 + int(value[4]-'0')
}

func previousScheduleDay(day int) int {
	if day == 1 {
		return 7
	}
	return day - 1
}

func validScheduleDays(days []int) bool {
	if len(days) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(days))
	for _, day := range days {
		if day < 1 || day > 7 {
			return false
		}
		if _, ok := seen[day]; ok {
			return false
		}
		seen[day] = struct{}{}
	}
	return true
}

func validScheduleWindow(start, end string) bool {
	return validScheduleTime(start) && validScheduleTime(end) && start != end
}

func validScheduleTime(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	hourTens := value[0]
	hourOnes := value[1]
	minuteTens := value[3]
	minuteOnes := value[4]
	if hourTens < '0' || hourTens > '2' || hourOnes < '0' || hourOnes > '9' {
		return false
	}
	if minuteTens < '0' || minuteTens > '5' || minuteOnes < '0' || minuteOnes > '9' {
		return false
	}
	hour := int(hourTens-'0')*10 + int(hourOnes-'0')
	return hour <= 23
}

func validScheduleDateRange(dateRange accountAvailabilityScheduleDateRange) bool {
	if dateRange.StartDate != "" && !validScheduleDate(dateRange.StartDate) {
		return false
	}
	if dateRange.EndDate != "" && !validScheduleDate(dateRange.EndDate) {
		return false
	}
	return dateRange.StartDate == "" || dateRange.EndDate == "" || dateRange.StartDate <= dateRange.EndDate
}

func validScheduleDate(value string) bool {
	if len(value) != 10 {
		return false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}
