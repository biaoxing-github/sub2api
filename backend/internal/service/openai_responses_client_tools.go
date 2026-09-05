package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const openAIResponsesClientToolMappingContextKey = "openai_responses_client_tool_mapping"

// adaptOpenAIResponsesClientTools 将 OpenAI API-key 上游不接受的客户端工具协议降为 function 工具。
func adaptOpenAIResponsesClientTools(body []byte) ([]byte, apicompat.ResponsesClientToolMapping, error) {
	if !needsOpenAIResponsesClientToolAdaptation(body) {
		return body, apicompat.ResponsesClientToolMapping{}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var requestBody map[string]any
	if err := decoder.Decode(&requestBody); err != nil {
		return body, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("decode OpenAI Responses client tools: %w", err)
	}
	var trailingValue any
	if err := decoder.Decode(&trailingValue); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return body, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("decode OpenAI Responses client tools trailing data: %w", err)
	}

	mapping, changed, err := apicompat.AdaptResponsesClientTools(requestBody)
	if err != nil || !changed {
		return body, mapping, err
	}
	rebuilt, err := marshalOpenAIUpstreamJSON(requestBody)
	if err != nil {
		return body, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("encode OpenAI Responses client tools: %w", err)
	}
	return rebuilt, mapping, nil
}

// needsOpenAIResponsesClientToolAdaptation 检查请求及历史输入中是否包含需降级的客户端工具项。
func needsOpenAIResponsesClientToolAdaptation(body []byte) bool {
	needsAdaptation := false
	var visit func(gjson.Result) bool
	visit = func(value gjson.Result) bool {
		if value.IsObject() {
			switch strings.TrimSpace(value.Get("type").String()) {
			case "custom", "custom_tool_call", "custom_tool_call_output",
				"tool_search", "tool_search_call", "tool_search_output":
				needsAdaptation = true
				return false
			}
		}
		if value.IsObject() || value.IsArray() {
			value.ForEach(func(_, child gjson.Result) bool {
				return visit(child)
			})
		}
		return !needsAdaptation
	}
	visit(gjson.ParseBytes(body))
	return needsAdaptation
}

// openAIResponsesClientToolMapping 返回本次转发需要用于恢复响应协议的映射。
func openAIResponsesClientToolMapping(c *gin.Context) (apicompat.ResponsesClientToolMapping, bool) {
	if c == nil {
		return apicompat.ResponsesClientToolMapping{}, false
	}
	value, ok := c.Get(openAIResponsesClientToolMappingContextKey)
	mapping, typed := value.(apicompat.ResponsesClientToolMapping)
	hasMapping := len(mapping.CustomTools) > 0 || mapping.ToolSearch || len(mapping.NamespaceTools) > 0
	return mapping, ok && typed && hasMapping
}

// clearOpenAIResponsesClientToolMapping 清除同一 Gin 上下文中上一次账号尝试留下的映射。
func clearOpenAIResponsesClientToolMapping(c *gin.Context) {
	if c == nil {
		return
	}
	if _, exists := c.Get(openAIResponsesClientToolMappingContextKey); exists {
		c.Set(openAIResponsesClientToolMappingContextKey, apicompat.ResponsesClientToolMapping{})
	}
}

// restoreOpenAIResponsesClientToolPayload 将 function 兼容响应恢复为客户端声明的工具协议。
func restoreOpenAIResponsesClientToolPayload(c *gin.Context, payload []byte) ([]byte, error) {
	mapping, ok := openAIResponsesClientToolMapping(c)
	if !ok || !json.Valid(payload) {
		return payload, nil
	}
	restored, _, err := apicompat.RestoreResponsesClientToolPayload(payload, mapping)
	return restored, err
}
