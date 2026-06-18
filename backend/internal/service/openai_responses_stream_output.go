package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// observeOpenAIResponsesVisibleText 从 Responses SSE 事件里提取可见文本。
// addedText 只返回本次新增到缓冲区的文本，避免 output_text.done / output_item.done
// 把前面已通过 delta 发出的正文再重复追加一遍。
func observeOpenAIResponsesVisibleText(output *strings.Builder, event *apicompat.ResponsesStreamEvent) (addedText string, sawOutput bool) {
	if output == nil || event == nil {
		return "", false
	}

	switch strings.TrimSpace(event.Type) {
	case "response.output_text.delta":
		if event.Delta == "" {
			return "", false
		}
		return appendOpenAIResponsesVisibleText(output, event.Delta), true
	case "response.output_text.done":
		if event.Text == "" {
			return "", false
		}
		return appendOpenAIResponsesVisibleText(output, event.Text), true
	case "response.output_item.added", "response.output_item.done":
		text := extractOpenAIResponsesVisibleItemText(event.Item)
		if text == "" {
			return "", false
		}
		return appendOpenAIResponsesVisibleText(output, text), true
	default:
		return "", false
	}
}

// appendOpenAIResponsesVisibleText 以“尽量不重复”的方式把可见文本补到缓冲区。
func appendOpenAIResponsesVisibleText(output *strings.Builder, text string) string {
	if output == nil || text == "" {
		return ""
	}

	current := output.String()
	switch {
	case current == "":
		_, _ = output.WriteString(text)
		return text
	case text == current:
		return ""
	case strings.HasPrefix(text, current):
		added := text[len(current):]
		if added != "" {
			_, _ = output.WriteString(added)
		}
		return added
	case strings.HasSuffix(current, text):
		return ""
	default:
		_, _ = output.WriteString(text)
		return text
	}
}

// extractOpenAIResponsesVisibleItemText 仅提取消息项中的可见 output_text，避免把工具调用等
// 非正文事件误判为“已有可见输出”。
func extractOpenAIResponsesVisibleItemText(item *apicompat.ResponsesOutput) string {
	if item == nil || strings.TrimSpace(item.Type) != "message" {
		return ""
	}

	var text strings.Builder
	for _, part := range item.Content {
		partType := strings.TrimSpace(part.Type)
		if (partType == "output_text" || partType == "text") && part.Text != "" {
			_, _ = text.WriteString(part.Text)
		}
	}
	return text.String()
}
