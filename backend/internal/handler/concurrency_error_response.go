package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const statusClientClosedRequest = 499

// concurrencyErrorResponse 将并发槽位获取错误映射为客户端可理解的状态码。
func concurrencyErrorResponse(err error, slotType string) (int, string, string) {
	var concurrencyErr *ConcurrencyError
	if errors.As(err, &concurrencyErr) {
		if concurrencyErr.SlotType != "" {
			slotType = concurrencyErr.SlotType
		}
		return http.StatusTooManyRequests, "rate_limit_error",
			fmt.Sprintf("Concurrency limit exceeded for %s, please retry later", slotType)
	}

	if errors.Is(err, context.Canceled) {
		return statusClientClosedRequest, "api_error", "context canceled"
	}

	return http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable, please retry later"
}
