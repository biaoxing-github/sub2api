package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// sanitizeOpenAICapacityShedErrorCodeForClient 仅修改下发副本，使容量错误进入客户端重试；账号归因保留原码。
func sanitizeOpenAICapacityShedErrorCodeForClient(payload []byte) ([]byte, bool) {
	if !gjson.ValidBytes(payload) {
		return payload, false
	}
	eventType := gjson.GetBytes(payload, "type").String()
	if eventType != "error" && eventType != "response.failed" {
		return payload, false
	}
	capacity := false
	for _, path := range []string{"response.error", "error"} {
		code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, path+".code").String()))
		message := strings.ToLower(gjson.GetBytes(payload, path+".message").String())
		capacity = capacity || code == "server_is_overloaded" || code == "slow_down" ||
			strings.Contains(message, "server is overloaded") || strings.Contains(message, "servers are overloaded") || strings.Contains(message, "servers are currently overloaded")
	}
	if !capacity {
		return payload, false
	}
	updated := payload
	changed := false
	for _, parent := range []string{"response.error", "error"} {
		if !gjson.GetBytes(updated, parent).Exists() {
			continue
		}
		code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(updated, parent+".code").String()))
		if code != "" && code != "server_is_overloaded" && code != "slow_down" {
			continue
		}
		next, err := sjson.SetBytes(updated, parent+".code", "server_error")
		if err != nil {
			return payload, false
		}
		updated, changed = next, true
	}
	return updated, changed
}
