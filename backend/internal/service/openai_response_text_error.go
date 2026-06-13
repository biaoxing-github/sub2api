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
	openAIResponseTextErrorRulesKey      = "openai_response_text_error_rules"
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

// GetOpenAIResponseTextErrorRules 返回账号配置的结构化响应正文规则；未配置时兼容旧关键词。
func (a *Account) GetOpenAIResponseTextErrorRules() []openAIResponseTextRule {
	if a == nil || a.Credentials == nil {
		return nil
	}
	if rules := parseOpenAIResponseTextRules(a.Credentials[openAIResponseTextErrorRulesKey]); len(rules) > 0 {
		return rules
	}
	return openAIResponseTextRulesFromLegacyKeywords(a.GetOpenAIResponseTextErrorKeywords())
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

type openAIResponseTextRuleAction string

const (
	openAIResponseTextRuleActionObserve  openAIResponseTextRuleAction = "observe"
	openAIResponseTextRuleActionDrop     openAIResponseTextRuleAction = "drop"
	openAIResponseTextRuleActionFail     openAIResponseTextRuleAction = "fail"
	openAIResponseTextRuleActionRetry    openAIResponseTextRuleAction = "retry"
	openAIResponseTextRuleActionAvoidTTL openAIResponseTextRuleAction = "avoid_ttl"
)

type openAIResponseTextRuleMatch struct {
	TextIncludes []string
	TextExcludes []string
	ErrorCodes   []string
}

type openAIResponseTextRule struct {
	ID     string
	Match  openAIResponseTextRuleMatch
	Action openAIResponseTextRuleAction
}

type openAIResponseTextErrorMatch struct {
	RuleID     string
	Keyword    string
	MatchField string
	Action     openAIResponseTextRuleAction
}

func (m openAIResponseTextErrorMatch) normalized() openAIResponseTextErrorMatch {
	if strings.TrimSpace(m.RuleID) == "" {
		m.RuleID = openAIResponseTextErrorCode
	}
	if strings.TrimSpace(m.MatchField) == "" {
		m.MatchField = "response_text"
	}
	if strings.TrimSpace(string(m.Action)) == "" {
		m.Action = openAIResponseTextRuleActionAvoidTTL
	}
	return m
}

func (m openAIResponseTextErrorMatch) actionable() bool {
	return m.normalized().Action != openAIResponseTextRuleActionObserve
}

func parseOpenAIResponseTextRules(raw any) []openAIResponseTextRule {
	switch v := raw.(type) {
	case []any:
		return parseOpenAIResponseTextRuleItems(v)
	case []map[string]any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
		return parseOpenAIResponseTextRuleItems(items)
	case string:
		var items []any
		if err := json.Unmarshal([]byte(strings.TrimSpace(v)), &items); err != nil {
			return nil
		}
		return parseOpenAIResponseTextRuleItems(items)
	default:
		return nil
	}
}

func parseOpenAIResponseTextRuleItems(items []any) []openAIResponseTextRule {
	rules := make([]openAIResponseTextRule, 0, len(items))
	for index, item := range items {
		rawRule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		matchRaw, _ := rawRule["match"].(map[string]any)
		match := openAIResponseTextRuleMatch{
			TextIncludes: parseOpenAIResponseTextRuleStrings(matchRaw, "textIncludes", "text_includes"),
			TextExcludes: parseOpenAIResponseTextRuleStrings(matchRaw, "textExcludes", "text_excludes"),
			ErrorCodes:   parseOpenAIResponseTextRuleStrings(matchRaw, "errorCodes", "error_codes"),
		}
		if len(match.TextIncludes) == 0 && len(match.ErrorCodes) == 0 {
			continue
		}
		action, ok := parseOpenAIResponseTextRuleAction(rawRule["action"])
		if !ok {
			continue
		}
		ruleID := strings.TrimSpace(fmt.Sprint(rawRule["id"]))
		if ruleID == "" || ruleID == "<nil>" {
			ruleID = fmt.Sprintf("%s_%d", openAIResponseTextErrorCode, index+1)
		}
		rules = append(rules, openAIResponseTextRule{
			ID:     ruleID,
			Match:  match,
			Action: action,
		})
	}
	return rules
}

func parseOpenAIResponseTextRuleStrings(raw map[string]any, keys ...string) []string {
	if len(raw) == 0 {
		return nil
	}
	for _, key := range keys {
		if values := parseOpenAIResponseTextErrorKeywords(raw[key]); len(values) > 0 {
			return values
		}
	}
	return nil
}

func parseOpenAIResponseTextRuleAction(raw any) (openAIResponseTextRuleAction, bool) {
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
	if value == "" || value == "<nil>" {
		return openAIResponseTextRuleActionAvoidTTL, true
	}
	switch value {
	case string(openAIResponseTextRuleActionObserve):
		return openAIResponseTextRuleActionObserve, true
	case string(openAIResponseTextRuleActionDrop):
		return openAIResponseTextRuleActionDrop, true
	case string(openAIResponseTextRuleActionFail):
		return openAIResponseTextRuleActionFail, true
	case string(openAIResponseTextRuleActionRetry):
		return openAIResponseTextRuleActionRetry, true
	case string(openAIResponseTextRuleActionAvoidTTL), "avoid_account_ttl":
		return openAIResponseTextRuleActionAvoidTTL, true
	default:
		return "", false
	}
}

func openAIResponseTextRulesFromLegacyKeywords(keywords []string) []openAIResponseTextRule {
	if len(keywords) == 0 {
		return nil
	}
	return []openAIResponseTextRule{{
		ID: openAIResponseTextErrorCode,
		Match: openAIResponseTextRuleMatch{
			TextIncludes: keywords,
		},
		Action: openAIResponseTextRuleActionAvoidTTL,
	}}
}

type openAIResponseTextErrorDetector struct {
	enabled         bool
	rules           []openAIResponseTextRule
	observedRuleIDs map[string]struct{}
	tail            string
}

func newOpenAIResponseTextErrorDetector(account *Account) *openAIResponseTextErrorDetector {
	if account == nil || !account.IsOpenAIResponseTextErrorEnabled() {
		return &openAIResponseTextErrorDetector{}
	}
	rules := account.GetOpenAIResponseTextErrorRules()
	return &openAIResponseTextErrorDetector{
		enabled:         len(rules) > 0,
		rules:           rules,
		observedRuleIDs: map[string]struct{}{},
	}
}

func (d *openAIResponseTextErrorDetector) Enabled() bool {
	return d != nil && d.enabled
}

func (d *openAIResponseTextErrorDetector) ObserveJSONBytes(body []byte) (string, bool) {
	match, ok := d.ObserveJSONBytesMatch(body)
	if !ok || !match.actionable() {
		return "", false
	}
	return match.Keyword, true
}

func (d *openAIResponseTextErrorDetector) ObserveJSONBytesMatch(body []byte) (openAIResponseTextErrorMatch, bool) {
	if !d.Enabled() || len(body) == 0 || !gjson.ValidBytes(body) {
		return openAIResponseTextErrorMatch{}, false
	}
	return d.observeJSONResultMatch(gjson.ParseBytes(body))
}

func (d *openAIResponseTextErrorDetector) ObserveSSEPayload(payload []byte) (string, bool) {
	match, ok := d.ObserveSSEPayloadMatch(payload)
	if !ok || !match.actionable() {
		return "", false
	}
	return match.Keyword, true
}

func (d *openAIResponseTextErrorDetector) ObserveSSEPayloadMatch(payload []byte) (openAIResponseTextErrorMatch, bool) {
	if !d.Enabled() || len(payload) == 0 {
		return openAIResponseTextErrorMatch{}, false
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "[DONE]" || !gjson.Valid(trimmed) {
		return openAIResponseTextErrorMatch{}, false
	}
	return d.observeJSONResultMatch(gjson.Parse(trimmed))
}

func (d *openAIResponseTextErrorDetector) observeJSONResultMatch(root gjson.Result) (openAIResponseTextErrorMatch, bool) {
	if !root.Exists() {
		return openAIResponseTextErrorMatch{}, false
	}
	if match, ok := d.observeErrorCodeMatch(root); ok {
		return match, true
	}
	var matched openAIResponseTextErrorMatch
	forEachOpenAIResponseTextInJSON(root, func(text string) bool {
		if match, ok := d.ObserveTextMatch(text); ok {
			matched = match
			return false
		}
		return true
	})
	if matched.Keyword != "" {
		return matched, true
	}
	return openAIResponseTextErrorMatch{}, false
}

func (d *openAIResponseTextErrorDetector) observeErrorCodeMatch(root gjson.Result) (openAIResponseTextErrorMatch, bool) {
	for _, path := range []string{"error.code", "response.error.code", "code"} {
		code := strings.TrimSpace(root.Get(path).String())
		if code == "" {
			continue
		}
		if match, ok := d.matchErrorCode(code, path); ok {
			return d.acceptMatch(match)
		}
	}
	return openAIResponseTextErrorMatch{}, false
}

func (d *openAIResponseTextErrorDetector) ObserveSSEBody(body string) (string, bool) {
	match, ok := d.ObserveSSEBodyMatch(body)
	if !ok || !match.actionable() {
		return "", false
	}
	return match.Keyword, true
}

func (d *openAIResponseTextErrorDetector) ObserveSSEBodyMatch(body string) (openAIResponseTextErrorMatch, bool) {
	if !d.Enabled() || strings.TrimSpace(body) == "" {
		return openAIResponseTextErrorMatch{}, false
	}
	var matched openAIResponseTextErrorMatch
	forEachOpenAISSEDataPayload(body, func(data []byte) {
		if matched.Keyword != "" {
			return
		}
		if match, ok := d.ObserveSSEPayloadMatch(data); ok {
			matched = match
		}
	})
	if matched.Keyword != "" {
		return matched, true
	}
	return openAIResponseTextErrorMatch{}, false
}

func (d *openAIResponseTextErrorDetector) ObserveText(text string) (string, bool) {
	match, ok := d.ObserveTextMatch(text)
	if !ok || !match.actionable() {
		return "", false
	}
	return match.Keyword, true
}

func (d *openAIResponseTextErrorDetector) ObserveTextMatch(text string) (openAIResponseTextErrorMatch, bool) {
	if !d.Enabled() {
		return openAIResponseTextErrorMatch{}, false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return openAIResponseTextErrorMatch{}, false
	}
	combined := d.tail + text
	for _, rule := range d.rules {
		if match, ok := rule.matchText(combined); ok {
			d.tail = trailingRunes(combined, openAIResponseTextErrorTailRunes)
			return d.acceptMatch(match)
		}
	}
	d.tail = trailingRunes(combined, openAIResponseTextErrorTailRunes)
	return openAIResponseTextErrorMatch{}, false
}

func (d *openAIResponseTextErrorDetector) acceptMatch(match openAIResponseTextErrorMatch) (openAIResponseTextErrorMatch, bool) {
	match = match.normalized()
	if match.Action != openAIResponseTextRuleActionObserve {
		return match, true
	}
	if d.observedRuleIDs == nil {
		d.observedRuleIDs = map[string]struct{}{}
	}
	if _, exists := d.observedRuleIDs[match.RuleID]; exists {
		return openAIResponseTextErrorMatch{}, false
	}
	d.observedRuleIDs[match.RuleID] = struct{}{}
	return match, true
}

func (d *openAIResponseTextErrorDetector) matchErrorCode(code string, path string) (openAIResponseTextErrorMatch, bool) {
	for _, rule := range d.rules {
		for _, want := range rule.Match.ErrorCodes {
			if strings.EqualFold(strings.TrimSpace(code), strings.TrimSpace(want)) {
				return openAIResponseTextErrorMatch{
					RuleID:     rule.ID,
					Keyword:    strings.TrimSpace(want),
					MatchField: strings.TrimSpace(path),
					Action:     rule.Action,
				}, true
			}
		}
	}
	return openAIResponseTextErrorMatch{}, false
}

func (r openAIResponseTextRule) matchText(text string) (openAIResponseTextErrorMatch, bool) {
	for _, exclude := range r.Match.TextExcludes {
		if containsOpenAIResponseTextKeyword(text, exclude) {
			return openAIResponseTextErrorMatch{}, false
		}
	}
	for _, include := range r.Match.TextIncludes {
		if containsOpenAIResponseTextKeyword(text, include) {
			return openAIResponseTextErrorMatch{
				RuleID:     r.ID,
				Keyword:    strings.TrimSpace(include),
				MatchField: "response_text",
				Action:     r.Action,
			}, true
		}
	}
	return openAIResponseTextErrorMatch{}, false
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
