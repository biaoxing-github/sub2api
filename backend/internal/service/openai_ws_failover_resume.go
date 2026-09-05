package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/tidwall/sjson"
)

// openAIWSCurrentTurnFailoverError 标记后续轮次，防止 handler 重放首轮。
type openAIWSCurrentTurnFailoverError struct {
	cause        error  // 原始切号错误，保留状态码和调度元数据。
	retryPayload []byte // 完整当前轮请求；空值表示上下文不足，禁止切号。
}

func (e *openAIWSCurrentTurnFailoverError) Error() string { return e.cause.Error() }
func (e *openAIWSCurrentTurnFailoverError) Unwrap() error { return e.cause }

// newOpenAIWSCurrentTurnFailoverError 隔离重试请求的所有权。
func newOpenAIWSCurrentTurnFailoverError(cause error, payload []byte) error {
	return &openAIWSCurrentTurnFailoverError{cause: cause, retryPayload: append([]byte(nil), payload...)}
}

// OpenAIWSCurrentTurnRetryPayload 返回当前轮重试副本和后续轮次标记。
func OpenAIWSCurrentTurnRetryPayload(err error) ([]byte, bool) {
	var retryErr *openAIWSCurrentTurnFailoverError
	if !errors.As(err, &retryErr) || retryErr == nil {
		return nil, false
	}
	return append([]byte(nil), retryErr.retryPayload...), true
}

// buildOpenAIWSCurrentTurnRetryPayload 用完整历史构造不依赖旧账号响应 ID 的请求。
func buildOpenAIWSCurrentTurnRetryPayload(payload []byte, fullInput []json.RawMessage, fullInputExists bool, originalModel string) ([]byte, bool, error) {
	if !fullInputExists {
		return nil, false, nil
	}
	retryPayload, err := setOpenAIWSPayloadInputSequence(payload, fullInput, true)
	if err != nil {
		return nil, false, err
	}
	retryPayload, _, err = dropPreviousResponseIDFromRawPayload(retryPayload)
	if err != nil {
		return nil, false, err
	}
	if model := strings.TrimSpace(originalModel); model != "" {
		retryPayload, err = sjson.SetBytes(retryPayload, "model", model)
		if err != nil {
			return nil, false, err
		}
	}
	if openAIWSRawItemsHasFunctionCallOutput(fullInput) && !openAIWSRawItemsHaveToolCallContextForOutputs(fullInput) {
		return nil, false, nil
	}
	return retryPayload, true, nil
}
