package service

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// OpenAIUpstreamHTTP2StreamErrorCode 表示上游 HTTP/2 响应流在请求开始后被重置。
	OpenAIUpstreamHTTP2StreamErrorCode = "upstream_http2_stream_error"
	// OpenAIUpstreamStreamReadErrorCode 表示其他上游响应流读取中断。
	OpenAIUpstreamStreamReadErrorCode = "upstream_stream_read_error"
)

type openAIUpstreamStreamReadError struct {
	cause         error
	clientCode    string
	clientMessage string
}

func (e *openAIUpstreamStreamReadError) Error() string {
	return fmt.Sprintf("stream usage incomplete: %v", e.cause)
}

func (e *openAIUpstreamStreamReadError) Unwrap() error { return e.cause }

func newOpenAIUpstreamStreamReadError(err error) error {
	code, message := classifyOpenAIUpstreamStreamReadError(err)
	return &openAIUpstreamStreamReadError{
		cause:         err,
		clientCode:    code,
		clientMessage: message,
	}
}

// OpenAIUpstreamStreamReadErrorDetails 返回适合客户端和 Ops 使用的稳定分类，不泄露底层流 ID。
func OpenAIUpstreamStreamReadErrorDetails(err error) (code, message string, ok bool) {
	var streamErr *openAIUpstreamStreamReadError
	if !errors.As(err, &streamErr) || streamErr == nil {
		return "", "", false
	}
	return streamErr.clientCode, streamErr.clientMessage, true
}

func classifyOpenAIUpstreamStreamReadError(err error) (code, message string) {
	if err != nil {
		lower := strings.ToLower(err.Error())
		// net/http 的 HTTP/2 streamError 未导出，只匹配稳定的传输层文本特征。
		if strings.Contains(lower, "stream error: stream id ") ||
			(strings.Contains(lower, "http2:") && strings.Contains(lower, "stream")) {
			return OpenAIUpstreamHTTP2StreamErrorCode, "Upstream HTTP/2 stream failed"
		}
	}
	return OpenAIUpstreamStreamReadErrorCode, "Upstream response stream was interrupted"
}
