package service

import (
	"encoding/json"
	"net/http"

	"github.com/tidwall/gjson"
)

// Anthropic passthrough/native 流式响应在"客户端尚未收到任何真实输出"时，
// 若上游异常结束（空流、纯 error 事件、零 token 终止），网关应直接 failover 换号，
// 而不是把半截/无效响应透传给客户端。客户端已经开始接收输出后则不能再换号，
// 只能补发一个 error 事件告知客户端流被截断。
const (
	anthropicStreamIncompleteErrType = "stream_incomplete"
	anthropicStreamInvalidErrType    = "stream_invalid"
	anthropicStreamIncompleteMessage = "upstream stream ended before a terminal event"
	anthropicStreamInvalidMessage    = "upstream stream terminated without producing any usable output"
)

// anthropicStreamDataErrorPayload 判断一条 SSE data 是否为 Anthropic 上游错误事件（type:error）。
// 命中时返回原始 payload，用于写前 failover 透传上游错误体（保留 rate_limit_error 等信息）。
func anthropicStreamDataErrorPayload(data string) ([]byte, bool) {
	if data == "" || data == "[DONE]" || !gjson.Valid(data) {
		return nil, false
	}
	if gjson.Get(data, "type").String() == "error" {
		return []byte(data), true
	}
	return nil, false
}

// anthropicStreamLineStartsRealOutput 判断一条 SSE data 是否代表真实模型输出已开始
// （content_block_delta 内容增量）。message_start / content_block_start 等前导帧不算。
func anthropicStreamLineStartsRealOutput(data string) bool {
	if data == "" || data == "[DONE]" || !gjson.Valid(data) {
		return false
	}
	return gjson.Get(data, "type").String() == "content_block_delta"
}

func anthropicStreamFailoverBody(errType, message string) []byte {
	body, err := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": message,
		},
	})
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"` + errType + `","message":"` + message + `"}}`)
	}
	return body
}

// anthropicStreamClientErrorEvent 构造写后阶段补发给客户端的 SSE error 事件。
func anthropicStreamClientErrorEvent(errType, message string) string {
	return "event: error\ndata: " + string(anthropicStreamFailoverBody(errType, message)) + "\n\n"
}

// newAnthropicStreamFailoverError 构造流式写前阶段的 failover 错误。statusCode 固定 502。
func newAnthropicStreamFailoverError(body []byte, retryableSameAccount bool) *UpstreamFailoverError {
	return &UpstreamFailoverError{
		StatusCode:             http.StatusBadGateway,
		ResponseBody:           body,
		RetryableOnSameAccount: retryableSameAccount,
	}
}
