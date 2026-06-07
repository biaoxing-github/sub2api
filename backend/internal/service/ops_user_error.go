package service

import "time"

// UserErrorRequest 是面向终端用户的错误请求精简脱敏视图。
// 严禁包含 client_ip、user_agent、account、upstream_endpoint 等内部字段。
type UserErrorRequest struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model"`
	InboundEndpoint string    `json:"inbound_endpoint"`
	StatusCode      int       `json:"status_code"`
	Category        string    `json:"category"`
	Platform        string    `json:"platform"`
	Message         string    `json:"message"`
	KeyName         string    `json:"key_name"`
	KeyDeleted      bool      `json:"key_deleted"`
}

// UserErrorRequestList 是用户错误请求分页结果。
type UserErrorRequestList struct {
	Items    []*UserErrorRequest `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// MapUserErrorCategory 将内部错误阶段和类型映射为前端稳定分类码。
func MapUserErrorCategory(phase, errType string) string {
	switch phase {
	case "auth":
		return "auth"
	case "routing":
		return "service_unavailable"
	case "upstream", "network":
		return "upstream"
	case "internal":
		return "internal"
	case "request":
		switch errType {
		case "rate_limit_error":
			return "rate_limit"
		case "billing_error", "subscription_error":
			return "quota"
		case "invalid_request_error":
			return "invalid_request"
		}
	}
	return "other"
}

// CategoryToFilter 将用户侧分类码反向映射为 ops 查询条件。
func CategoryToFilter(category string) (phases []string, errorTypes []string) {
	switch category {
	case "auth":
		return []string{"auth"}, nil
	case "service_unavailable":
		return []string{"routing"}, nil
	case "upstream":
		return []string{"upstream", "network"}, nil
	case "internal":
		return []string{"internal"}, nil
	case "rate_limit":
		return nil, []string{"rate_limit_error"}
	case "quota":
		return nil, []string{"billing_error", "subscription_error"}
	case "invalid_request":
		return nil, []string{"invalid_request_error"}
	default:
		return nil, nil
	}
}

// ToUserErrorRequest 将内部 OpsErrorLog 裁剪为用户安全视图。
func ToUserErrorRequest(e *OpsErrorLog) *UserErrorRequest {
	if e == nil {
		return nil
	}
	model := e.RequestedModel
	if model == "" {
		model = e.Model
	}
	return &UserErrorRequest{
		ID:              e.ID,
		CreatedAt:       e.CreatedAt,
		Model:           model,
		InboundEndpoint: e.InboundEndpoint,
		StatusCode:      e.StatusCode,
		Category:        MapUserErrorCategory(e.Phase, e.Type),
		Platform:        e.Platform,
		Message:         e.Message,
		KeyName:         e.APIKeyName,
		KeyDeleted:      e.APIKeyDeleted,
	}
}

// UserErrorRequestDetail 是错误请求详情的脱敏视图。
type UserErrorRequestDetail struct {
	UserErrorRequest
	ErrorBody          string `json:"error_body"`
	UpstreamStatusCode *int   `json:"upstream_status_code,omitempty"`
}

// ToUserErrorRequestDetail 将内部 OpsErrorLogDetail 裁剪为用户安全详情视图。
func ToUserErrorRequestDetail(e *OpsErrorLogDetail) *UserErrorRequestDetail {
	if e == nil {
		return nil
	}
	base := ToUserErrorRequest(&e.OpsErrorLog)
	if base == nil {
		return nil
	}
	return &UserErrorRequestDetail{
		UserErrorRequest:   *base,
		ErrorBody:          e.ErrorBody,
		UpstreamStatusCode: e.UpstreamStatusCode,
	}
}
