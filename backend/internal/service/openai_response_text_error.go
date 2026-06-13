package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIResponseTextErrorCode          = "openai_response_text_error"
	openAIResponseTextErrorClientMessage = "Upstream response matched configured error text; no fallback account was available"
	openAIResponseTextErrorTailRunes     = 2048
)

// IsOpenAIResponseTextErrorEnabled 返回账号是否启用 OpenAI 响应正文异常关键词。
func (a *Account) IsOpenAIResponseTextErrorEnabled() bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Credentials == nil {
		return false
	}
	enabled, ok := a.Credentials["openai_response_text_error_enabled"].(bool)
	return ok && enabled
}

// GetOpenAIResponseTextErrorKeywords 返回账号配置的响应正文异常关键词。
func (a *Account) GetOpenAIResponseTextErrorKeywords() []string {
	if a == nil || a.Credentials == nil {
		return nil
	}
	return parseOpenAIResponseTextErrorKeywords(a.Credentials["openai_response_text_error_keywords"])
}

func parseOpenAIResponseTextErrorKeywords(raw any) []string {
	add := func(out []string, value string) []string {
		value = strings.TrimSpace(value)
		if value == "" {
			return out
		}
		return append(out, value)
	}

	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = add(out, s)
			}
		}
		return dedupeOpenAIResponseTextErrorKeywords(out)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = add(out, item)
		}
		return dedupeOpenAIResponseTextErrorKeywords(out)
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ','
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			out = add(out, part)
		}
		return dedupeOpenAIResponseTextErrorKeywords(out)
	default:
		return nil
	}
}

func dedupeOpenAIResponseTextErrorKeywords(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

type openAIResponseTextErrorDetector struct {
	enabled  bool
	keywords []string
	tail     string
}

func newOpenAIResponseTextErrorDetector(account *Account) *openAIResponseTextErrorDetector {
	if account == nil || !account.IsOpenAIResponseTextErrorEnabled() {
		return &openAIResponseTextErrorDetector{}
	}
	keywords := account.GetOpenAIResponseTextErrorKeywords()
	return &openAIResponseTextErrorDetector{
		enabled:  len(keywords) > 0,
		keywords: keywords,
	}
}

func (d *openAIResponseTextErrorDetector) Enabled() bool {
	return d != nil && d.enabled
}

func (d *openAIResponseTextErrorDetector) ObserveJSONBytes(body []byte) (string, bool) {
	if !d.Enabled() || len(body) == 0 || !gjson.ValidBytes(body) {
		return "", false
	}
	matched := ""
	forEachOpenAIResponseTextInJSON(gjson.ParseBytes(body), func(text string) bool {
		if keyword, ok := d.ObserveText(text); ok {
			matched = keyword
			return false
		}
		return true
	})
	if matched != "" {
		return matched, true
	}
	return "", false
}

func (d *openAIResponseTextErrorDetector) ObserveSSEPayload(payload []byte) (string, bool) {
	if !d.Enabled() || len(payload) == 0 {
		return "", false
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "[DONE]" || !gjson.Valid(trimmed) {
		return "", false
	}
	root := gjson.Parse(trimmed)
	matched := ""
	forEachOpenAIResponseTextInJSON(root, func(text string) bool {
		if keyword, ok := d.ObserveText(text); ok {
			matched = keyword
			return false
		}
		return true
	})
	if matched != "" {
		return matched, true
	}
	return "", false
}

func (d *openAIResponseTextErrorDetector) ObserveSSEBody(body string) (string, bool) {
	if !d.Enabled() || strings.TrimSpace(body) == "" {
		return "", false
	}
	matched := ""
	forEachOpenAISSEDataPayload(body, func(data []byte) {
		if matched != "" {
			return
		}
		if keyword, ok := d.ObserveSSEPayload(data); ok {
			matched = keyword
		}
	})
	if matched != "" {
		return matched, true
	}
	return "", false
}

func (d *openAIResponseTextErrorDetector) ObserveText(text string) (string, bool) {
	if !d.Enabled() {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	combined := d.tail + text
	for _, keyword := range d.keywords {
		if containsOpenAIResponseTextKeyword(combined, keyword) {
			return keyword, true
		}
	}
	d.tail = trailingRunes(combined, openAIResponseTextErrorTailRunes)
	return "", false
}

func forEachOpenAIResponseTextInJSON(root gjson.Result, fn func(string) bool) {
	if fn == nil || !root.Exists() {
		return
	}
	visitString := func(result gjson.Result) bool {
		if result.Exists() && result.Type == gjson.String {
			return fn(result.String())
		}
		return true
	}

	if !visitString(root.Get("output_text")) ||
		!visitString(root.Get("delta")) ||
		!visitString(root.Get("text")) {
		return
	}

	if choices := root.Get("choices"); choices.Exists() && choices.IsArray() {
		for _, choice := range choices.Array() {
			if !visitString(choice.Get("delta.content")) ||
				!visitString(choice.Get("message.content")) ||
				!visitString(choice.Get("text")) {
				return
			}
		}
	}

	for _, outputRoot := range []gjson.Result{root.Get("output"), root.Get("response.output")} {
		if !outputRoot.Exists() || !outputRoot.IsArray() {
			continue
		}
		for _, item := range outputRoot.Array() {
			if !visitString(item.Get("text")) {
				return
			}
			content := item.Get("content")
			if !content.Exists() {
				continue
			}
			if content.Type == gjson.String {
				if !fn(content.String()) {
					return
				}
				continue
			}
			if !content.IsArray() {
				continue
			}
			for _, part := range content.Array() {
				if !visitString(part.Get("text")) ||
					!visitString(part.Get("delta")) {
					return
				}
			}
		}
	}
}

func containsOpenAIResponseTextKeyword(text, keyword string) bool {
	text = strings.TrimSpace(text)
	keyword = strings.TrimSpace(keyword)
	if text == "" || keyword == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func trailingRunes(text string, maxRunes int) string {
	if maxRunes <= 0 || text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[len(runes)-maxRunes:])
}

func newOpenAIResponseTextFailoverError(c *gin.Context, account *Account, upstreamRequestID string, matchedKeyword string) *UpstreamFailoverError {
	message := openAIResponseTextErrorMessage(matchedKeyword)

	accountID := int64(0)
	accountName := ""
	platform := PlatformOpenAI
	if account != nil {
		accountID = account.ID
		accountName = account.Name
		platform = account.Platform
	}

	setOpsUpstreamError(c, http.StatusBadGateway, message, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           platform,
		AccountID:          accountID,
		AccountName:        accountName,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		Kind:               "failover",
		Message:            message,
	})

	headers := http.Header{}
	if strings.TrimSpace(upstreamRequestID) != "" {
		headers.Set("x-request-id", strings.TrimSpace(upstreamRequestID))
	}
	return &UpstreamFailoverError{
		StatusCode:      http.StatusBadGateway,
		ResponseBody:    openAIResponseTextErrorBody(matchedKeyword, message),
		ResponseHeaders: headers,
		ActionLabel:     OpenAIStreamActionAvoidAccountTTL,
	}
}

func openAIResponseTextErrorMessage(matchedKeyword string) string {
	matchedKeyword = strings.TrimSpace(matchedKeyword)
	message := "OpenAI upstream response matched configured error text"
	if matchedKeyword != "" {
		message = fmt.Sprintf("%s: %s", message, matchedKeyword)
	}
	return message
}

func openAIResponseTextErrorBody(matchedKeyword string, message string) []byte {
	body, err := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":          "upstream_error",
			"code":          openAIResponseTextErrorCode,
			"message":       message,
			"matched_text":  matchedKeyword,
			"matched_scope": "openai_response_text",
		},
	})
	if err != nil {
		return []byte(`{"error":{"type":"upstream_error","code":"openai_response_text_error","message":"OpenAI upstream response matched configured error text"}}`)
	}
	return body
}

// IsOpenAIResponseTextErrorBody 判断响应体是否来自 OpenAI 响应正文关键词异常检测器。
func IsOpenAIResponseTextErrorBody(body []byte) bool {
	return strings.TrimSpace(gjson.GetBytes(body, "error.code").String()) == openAIResponseTextErrorCode
}

// OpenAIResponseTextErrorClientMessage 返回关键词异常在账号耗尽时展示给客户端的消息。
func OpenAIResponseTextErrorClientMessage() string {
	return openAIResponseTextErrorClientMessage
}
