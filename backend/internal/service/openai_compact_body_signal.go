package service

import "github.com/tidwall/gjson"

// hasCompactionTriggerInInput 检测 type="compaction_trigger" 输入项。
// 公共包装层会供 handler 结合请求路径、stream 标志和 Codex beta feature
// 请求头，区分原生 remote compaction v2 与 /responses/compact 桥接链路。
func hasCompactionTriggerInInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "compaction_trigger" {
			found = true
			return false
		}
		return true
	})
	return found
}
