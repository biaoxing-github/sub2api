package service

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var openCodeGoUsageLimitResetPattern = regexp.MustCompile(`(?i)\bresets\s+in\s+`)
var openCodeGoUsageLimitDurationPartPattern = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*(s|sec|secs|second|seconds|m|min|mins|minute|minutes|h|hr|hrs|hour|hours|d|day|days|w|week|weeks)\b`)

// parseOpenCodeGoUsageLimitResetDuration 解析 OpenCode Go 明确返回的用量重置时长。
func parseOpenCodeGoUsageLimitResetDuration(message string) time.Duration {
	resetPrefix := openCodeGoUsageLimitResetPattern.FindStringIndex(message)
	if resetPrefix == nil {
		return 0
	}

	remainder := message[resetPrefix[1]:]
	var total time.Duration
	for {
		remainder = strings.TrimSpace(remainder)
		matches := openCodeGoUsageLimitDurationPartPattern.FindStringSubmatchIndex(remainder)
		if matches == nil {
			break
		}

		value, err := strconv.ParseFloat(remainder[matches[2]:matches[3]], 64)
		if err != nil || value <= 0 {
			return 0
		}

		unit := openCodeGoUsageLimitDurationUnit(remainder[matches[4]:matches[5]])
		if unit <= 0 {
			return 0
		}

		const maxDuration = time.Duration(1<<63 - 1)
		if value >= float64(maxDuration)/float64(unit) {
			return 0
		}
		part := time.Duration(value * float64(unit))
		if part <= 0 || total > maxDuration-part {
			return 0
		}
		total += part
		remainder = remainder[matches[1]:]
	}

	return total
}

func openCodeGoUsageLimitDurationUnit(raw string) time.Duration {
	switch strings.ToLower(raw) {
	case "s", "sec", "secs", "second", "seconds":
		return time.Second
	case "m", "min", "mins", "minute", "minutes":
		return time.Minute
	case "h", "hr", "hrs", "hour", "hours":
		return time.Hour
	case "d", "day", "days":
		return 24 * time.Hour
	case "w", "week", "weeks":
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}
