package service

import (
	"context"
	"strings"
)

// HTTPUpstreamProfile 标记上游 HTTP 请求需要使用的专用传输策略。
type HTTPUpstreamProfile string

const (
	// HTTPUpstreamProfileDefault 表示沿用通用 HTTP 上游策略。
	HTTPUpstreamProfileDefault HTTPUpstreamProfile = ""
	// HTTPUpstreamProfileOpenAI 表示 OpenAI/Codex 请求，使用独立的 HTTP/2 与超时策略。
	HTTPUpstreamProfileOpenAI HTTPUpstreamProfile = "openai"
)

const (
	// HTTPUpstreamProtocolModeDefault 表示通用上游 transport 协议模式。
	HTTPUpstreamProtocolModeDefault = "default"
	// HTTPUpstreamProtocolModeOpenAIH1 表示 OpenAI HTTP 明确关闭 HTTP/2。
	HTTPUpstreamProtocolModeOpenAIH1 = "openai_h1"
	// HTTPUpstreamProtocolModeOpenAIH2 表示 OpenAI HTTP 明确优先使用 HTTP/2。
	HTTPUpstreamProtocolModeOpenAIH2 = "openai_h2"
	// HTTPUpstreamProtocolModeOpenAIH1Fallback 表示 OpenAI 因 HTTP/2 兼容问题临时回退 HTTP/1.1。
	HTTPUpstreamProtocolModeOpenAIH1Fallback = "openai_h1_fallback"
)

type httpUpstreamProfileContextKey struct{}
type httpUpstreamAttemptInfoContextKey struct{}

// HTTPUpstreamAttemptInfo 保存一次上游请求在 transport 层解析出来的诊断信息。
type HTTPUpstreamAttemptInfo struct {
	// RequestBaseURL 是当前 OpenAI 请求实际使用的 request_base_url，用于区分同账号下的多线路。
	RequestBaseURL string
	// ProtocolMode 是实际选择的协议模式，例如 openai_h2 或 openai_h1_fallback。
	ProtocolMode string
	// FallbackKey 是 HTTP/2 兼容性回退状态的隔离键，包含账号、代理和 request_base_url。
	FallbackKey string
}

// WithHTTPUpstreamProfile 将上游传输策略写入 context。
func WithHTTPUpstreamProfile(ctx context.Context, profile HTTPUpstreamProfile) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if profile == HTTPUpstreamProfileDefault {
		return ctx
	}
	return context.WithValue(ctx, httpUpstreamProfileContextKey{}, profile)
}

// HTTPUpstreamProfileFromContext 从 context 解析上游传输策略。
func HTTPUpstreamProfileFromContext(ctx context.Context) HTTPUpstreamProfile {
	if ctx == nil {
		return HTTPUpstreamProfileDefault
	}
	profile, ok := ctx.Value(httpUpstreamProfileContextKey{}).(HTTPUpstreamProfile)
	if !ok {
		return HTTPUpstreamProfileDefault
	}
	switch profile {
	case HTTPUpstreamProfileOpenAI:
		return profile
	default:
		return HTTPUpstreamProfileDefault
	}
}

// WithHTTPUpstreamAttemptInfo 将请求级诊断信息指针写入 context，供 repository 回填。
func WithHTTPUpstreamAttemptInfo(ctx context.Context, info *HTTPUpstreamAttemptInfo) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if info == nil {
		return ctx
	}
	info.RequestBaseURL = strings.TrimSpace(info.RequestBaseURL)
	return context.WithValue(ctx, httpUpstreamAttemptInfoContextKey{}, info)
}

// HTTPUpstreamAttemptInfoFromContext 从 context 读取请求级诊断信息。
func HTTPUpstreamAttemptInfoFromContext(ctx context.Context) *HTTPUpstreamAttemptInfo {
	if ctx == nil {
		return nil
	}
	info, _ := ctx.Value(httpUpstreamAttemptInfoContextKey{}).(*HTTPUpstreamAttemptInfo)
	return info
}
