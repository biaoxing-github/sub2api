package service

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var openAIResponsesToolCallItemTypes = map[string]bool{
	"function_call":    true,
	"tool_call":        true,
	"custom_tool_call": true,
	"mcp_tool_call":    true,
}

// shouldStripOpenAIResponsesInputNamespaces 判断 HTTP 转发前是否需要移除 input item 的 namespace。
// 原生 WSv2 支持 namespace，其他 OpenAI OAuth/API Key 转发路径按官方 HTTP 请求格式清理。
func shouldStripOpenAIResponsesInputNamespaces(account *Account, transport OpenAIUpstreamTransport, passthroughEnabled bool) bool {
	if account == nil || (!account.IsOpenAIOAuth() && !account.IsOpenAIApiKey()) {
		return false
	}
	if transport == OpenAIUpstreamTransportResponsesWebsocketV2 && !passthroughEnabled {
		return false
	}
	return true
}

// stripOpenAIResponsesInputNamespaces 只移除 input 数组直属对象的 namespace 字段。
// 该实现单次重建 input，保留嵌套字段和 JSON 数字的原始表示。
func stripOpenAIResponsesInputNamespaces(body []byte, keepToolCallNamespaces bool) ([]byte, error) {
	if !bytes.Contains(body, []byte(`"namespace"`)) {
		return body, nil
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, nil
	}

	var rebuilt bytes.Buffer
	rebuilt.Grow(len(input.Raw))
	rebuilt.WriteByte('[')
	changed := false
	first := true
	var stripErr error
	input.ForEach(func(_, item gjson.Result) bool {
		if !first {
			rebuilt.WriteByte(',')
		}
		first = false
		itemBody := []byte(item.Raw)
		itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		if item.IsObject() && item.Get("namespace").Exists() &&
			(!keepToolCallNamespaces || !openAIResponsesToolCallItemTypes[itemType]) {
			itemBody, stripErr = sjson.DeleteBytes(itemBody, "namespace")
			if stripErr != nil {
				return false
			}
			changed = true
		}
		rebuilt.Write(itemBody)
		return true
	})
	if stripErr != nil {
		return body, fmt.Errorf("delete OpenAI input namespace: %w", stripErr)
	}
	if !changed {
		return body, nil
	}
	rebuilt.WriteByte(']')
	stripped, err := sjson.SetRawBytes(body, "input", rebuilt.Bytes())
	if err != nil {
		return body, fmt.Errorf("replace OpenAI input after namespace deletion: %w", err)
	}
	return stripped, nil
}

// shouldKeepOpenAIResponsesToolCallNamespaces 仅为声明 namespace 工具的 API-key 请求保留调用项 namespace。
func shouldKeepOpenAIResponsesToolCallNamespaces(account *Account, compactPath bool, body []byte) bool {
	if account == nil || compactPath || !account.IsOpenAIApiKey() {
		return false
	}
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, tool gjson.Result) bool {
		if strings.EqualFold(strings.TrimSpace(tool.Get("type").String()), "namespace") {
			found = true
			return false
		}
		return true
	})
	return found
}
