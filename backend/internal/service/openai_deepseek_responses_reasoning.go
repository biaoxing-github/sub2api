package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type deepSeekResponsesReasoningMode uint8

const (
	deepSeekResponsesReasoningUnknown deepSeekResponsesReasoningMode = iota
	deepSeekResponsesReasoningNativeSummary
	deepSeekResponsesReasoningLegacyText
)

// deepSeekResponsesReasoningItemState 记录单个 reasoning item 的 summary 生命周期。
type deepSeekResponsesReasoningItemState struct {
	itemID       string          // itemID 是上游 reasoning output item 的稳定标识。
	outputIndex  int             // outputIndex 是 reasoning item 在 response.output 中的位置。
	text         strings.Builder // text 累积流式 reasoning_text.delta 的正文。
	terminalText string          // terminalText 保存 done/item 事件提供的权威终态正文。
	partAdded    bool            // partAdded 标记 summary part added 是否已经发送。
	textDone     bool            // textDone 标记 summary text done 是否已经发送。
	partDone     bool            // partDone 标记 summary part done 是否已经发送。
}

// finalText 优先返回终态事件的完整文本，没有终态文本时使用已累积的 delta。
func (s *deepSeekResponsesReasoningItemState) finalText() string {
	if s == nil {
		return ""
	}
	if s.terminalText != "" {
		return s.terminalText
	}
	return s.text.String()
}

// deepSeekResponsesReasoningNormalizer 只把 DeepSeek 的 legacy reasoning_text
// 形态改写为原生 Responses reasoning summary，不参与请求、工具调用或 usage 处理。
type deepSeekResponsesReasoningNormalizer struct {
	enabled        bool                                            // enabled 限定是否允许执行 DeepSeek 兼容转换。
	mode           deepSeekResponsesReasoningMode                  // mode 区分未知、原生 summary 和 legacy reasoning_text。
	sequenceOffset int64                                           // sequenceOffset 补偿新增事件带来的 sequence_number 偏移。
	changed        bool                                            // changed 标记当前流是否实际发生过转换。
	items          map[string]*deepSeekResponsesReasoningItemState // items 按 item_id 保存 reasoning 生命周期状态。
	itemOrder      []string                                        // itemOrder 保持终态补事件时的原始输出顺序。
}

// newDeepSeekResponsesReasoningNormalizer 创建单个响应范围内使用的状态转换器。
func newDeepSeekResponsesReasoningNormalizer(enabled bool) *deepSeekResponsesReasoningNormalizer {
	return &deepSeekResponsesReasoningNormalizer{
		enabled: enabled,
		items:   make(map[string]*deepSeekResponsesReasoningItemState),
	}
}

// shouldNormalizeDeepSeekResponsesReasoning 以请求模型、映射模型和官方上游地址
// 识别 DeepSeek；普通 OpenAI Responses 及其他兼容上游保持原样。
func shouldNormalizeDeepSeekResponsesReasoning(account *Account, models ...string) bool {
	for _, model := range models {
		if strings.Contains(strings.ToLower(strings.TrimSpace(model)), "deepseek") {
			return true
		}
	}
	if account == nil || !account.IsOpenAIApiKey() {
		return false
	}
	for _, rawBaseURL := range account.GetOpenAIRequestBaseURLs() {
		parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
		if err != nil {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		if host == "deepseek.com" || strings.HasSuffix(host, ".deepseek.com") {
			return true
		}
	}
	return false
}

// normalizeDeepSeekResponsesReasoningResponse 归一化非流式 response，或流式
// response.completed 事件中的 response.output。只要检测到原生 summary 就完全旁路。
func normalizeDeepSeekResponsesReasoningResponse(body []byte, enabled bool) ([]byte, bool) {
	if !enabled || len(body) == 0 {
		return body, false
	}
	outputPath := "output"
	output := gjson.GetBytes(body, outputPath)
	if !output.Exists() || !output.IsArray() {
		outputPath = "response.output"
		output = gjson.GetBytes(body, outputPath)
	}
	if !output.Exists() || !output.IsArray() {
		return body, false
	}

	items := output.Array()
	for _, item := range items {
		if reasoningItemHasNativeSummary(item) {
			return body, false
		}
	}

	normalizedItems := make([]json.RawMessage, 0, len(items))
	changed := false
	for _, item := range items {
		raw := []byte(item.Raw)
		if normalized, _, itemChanged := normalizeDeepSeekResponsesReasoningItem(raw); itemChanged {
			raw = normalized
			changed = true
		}
		normalizedItems = append(normalizedItems, json.RawMessage(raw))
	}
	if !changed {
		return body, false
	}
	rawOutput, err := json.Marshal(normalizedItems)
	if err != nil {
		return body, false
	}
	updated, err := sjson.SetRawBytes(body, outputPath, rawOutput)
	if err != nil {
		return body, false
	}
	return updated, true
}

// reasoningItemHasNativeSummary 判断 reasoning item 是否已经携带上游原生 summary。
func reasoningItemHasNativeSummary(item gjson.Result) bool {
	if strings.TrimSpace(item.Get("type").String()) != "reasoning" {
		return false
	}
	summary := item.Get("summary")
	return summary.Exists() && summary.IsArray() && len(summary.Array()) > 0
}

// normalizeDeepSeekResponsesReasoningItem 将 reasoning content 中的 reasoning_text
// 移入 summary，同时保留 item 的 id、status、encrypted_content 及未知扩展字段。
func normalizeDeepSeekResponsesReasoningItem(item []byte) ([]byte, string, bool) {
	if strings.TrimSpace(gjson.GetBytes(item, "type").String()) != "reasoning" ||
		reasoningItemHasNativeSummary(gjson.ParseBytes(item)) {
		return item, "", false
	}
	content := gjson.GetBytes(item, "content")
	if !content.Exists() || !content.IsArray() {
		return item, "", false
	}

	var reasoningText strings.Builder
	remaining := make([]json.RawMessage, 0, len(content.Array()))
	foundReasoningText := false
	for _, part := range content.Array() {
		if strings.TrimSpace(part.Get("type").String()) == "reasoning_text" {
			foundReasoningText = true
			_, _ = reasoningText.WriteString(part.Get("text").String())
			continue
		}
		remaining = append(remaining, json.RawMessage(part.Raw))
	}
	if !foundReasoningText {
		return item, "", false
	}

	text := reasoningText.String()
	summary, err := json.Marshal([]map[string]string{{
		"type": "summary_text",
		"text": text,
	}})
	if err != nil {
		return item, "", false
	}
	updated, err := sjson.SetRawBytes(item, "summary", summary)
	if err != nil {
		return item, "", false
	}
	if len(remaining) == 0 {
		updated, err = sjson.DeleteBytes(updated, "content")
	} else {
		rawRemaining, marshalErr := json.Marshal(remaining)
		if marshalErr != nil {
			return item, "", false
		}
		updated, err = sjson.SetRawBytes(updated, "content", rawRemaining)
	}
	if err != nil {
		return item, "", false
	}
	return updated, text, true
}

// transformEvent 将一个 Responses data 载荷转换为零个或多个兼容事件。
func (n *deepSeekResponsesReasoningNormalizer) transformEvent(data []byte) ([][]byte, bool) {
	if n == nil || !n.enabled || len(data) == 0 {
		return [][]byte{data}, false
	}
	eventType := strings.TrimSpace(gjson.GetBytes(data, "type").String())
	if eventType == "" {
		return [][]byte{data}, false
	}

	if strings.HasPrefix(eventType, "response.reasoning_summary_") {
		if n.mode == deepSeekResponsesReasoningUnknown {
			n.mode = deepSeekResponsesReasoningNativeSummary
		}
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}

	switch eventType {
	case "response.output_item.added":
		item := gjson.GetBytes(data, "item")
		if n.mode == deepSeekResponsesReasoningUnknown && reasoningItemHasNativeSummary(item) {
			n.mode = deepSeekResponsesReasoningNativeSummary
		}
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	case "response.reasoning_text.delta":
		if n.mode == deepSeekResponsesReasoningNativeSummary {
			return n.applySequenceNumbers(data, [][]byte{data}, false)
		}
		n.mode = deepSeekResponsesReasoningLegacyText
		state := n.stateForEvent(data, nil)
		delta := gjson.GetBytes(data, "delta").String()
		_, _ = state.text.WriteString(delta)
		events := make([][]byte, 0, 2)
		if !state.partAdded {
			events = append(events, makeDeepSeekReasoningPartEvent("response.reasoning_summary_part.added", state, ""))
			state.partAdded = true
		}
		events = append(events, convertDeepSeekReasoningTextEvent(data, "response.reasoning_summary_text.delta"))
		return n.applySequenceNumbers(data, events, true)
	case "response.reasoning_text.done":
		if n.mode == deepSeekResponsesReasoningNativeSummary {
			return n.applySequenceNumbers(data, [][]byte{data}, false)
		}
		n.mode = deepSeekResponsesReasoningLegacyText
		state := n.stateForEvent(data, nil)
		if text := gjson.GetBytes(data, "text").String(); text != "" {
			state.terminalText = text
		}
		events := make([][]byte, 0, 2)
		if !state.partAdded {
			events = append(events, makeDeepSeekReasoningPartEvent("response.reasoning_summary_part.added", state, ""))
			state.partAdded = true
		}
		events = append(events, convertDeepSeekReasoningTextEvent(data, "response.reasoning_summary_text.done"))
		state.textDone = true
		return n.applySequenceNumbers(data, events, true)
	case "response.output_item.done":
		return n.transformOutputItemDone(data)
	case "response.completed", "response.done", "response.incomplete", "response.cancelled", "response.canceled":
		return n.transformTerminalResponse(data)
	default:
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
}

// transformOutputItemDone 补齐 reasoning summary 终止事件并改写 item 终态。
func (n *deepSeekResponsesReasoningNormalizer) transformOutputItemDone(data []byte) ([][]byte, bool) {
	if n.mode == deepSeekResponsesReasoningNativeSummary {
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
	item := gjson.GetBytes(data, "item")
	if reasoningItemHasNativeSummary(item) {
		if n.mode == deepSeekResponsesReasoningUnknown {
			n.mode = deepSeekResponsesReasoningNativeSummary
		}
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
	normalizedItem, text, changed := normalizeDeepSeekResponsesReasoningItem([]byte(item.Raw))
	if !changed && n.mode != deepSeekResponsesReasoningLegacyText {
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
	n.mode = deepSeekResponsesReasoningLegacyText
	state := n.stateForEvent(data, normalizedItem)
	if text != "" {
		state.terminalText = text
	}
	events := n.closeStateEvents(state)
	updated := data
	if changed {
		if next, err := sjson.SetRawBytes(data, "item", normalizedItem); err == nil {
			updated = next
		} else {
			return n.applySequenceNumbers(data, [][]byte{data}, false)
		}
	}
	events = append(events, updated)
	return n.applySequenceNumbers(data, events, changed || len(events) > 1)
}

// transformTerminalResponse 同步 response.completed 等终态事件中的 response.output。
func (n *deepSeekResponsesReasoningNormalizer) transformTerminalResponse(data []byte) ([][]byte, bool) {
	if n.mode == deepSeekResponsesReasoningNativeSummary {
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
	updated, changed := normalizeDeepSeekResponsesReasoningResponse(data, true)
	if changed {
		n.mode = deepSeekResponsesReasoningLegacyText
		n.captureTerminalResponseStates(updated)
	}
	if n.mode != deepSeekResponsesReasoningLegacyText {
		return n.applySequenceNumbers(data, [][]byte{data}, false)
	}
	events := make([][]byte, 0, len(n.itemOrder)*3+1)
	for _, key := range n.itemOrder {
		events = append(events, n.closeStateEvents(n.items[key])...)
	}
	events = append(events, updated)
	return n.applySequenceNumbers(data, events, changed || len(events) > 1)
}

// stateForEvent 按 item_id 或 output_index 获取并初始化 reasoning item 状态。
func (n *deepSeekResponsesReasoningNormalizer) stateForEvent(data []byte, item []byte) *deepSeekResponsesReasoningItemState {
	itemID := strings.TrimSpace(gjson.GetBytes(data, "item_id").String())
	outputIndex := int(gjson.GetBytes(data, "output_index").Int())
	if len(item) > 0 {
		if value := strings.TrimSpace(gjson.GetBytes(item, "id").String()); value != "" {
			itemID = value
		}
	}
	key := itemID
	if key == "" {
		key = "output:" + strconv.Itoa(outputIndex)
	}
	state, ok := n.items[key]
	if !ok {
		state = &deepSeekResponsesReasoningItemState{itemID: itemID, outputIndex: outputIndex}
		n.items[key] = state
		n.itemOrder = append(n.itemOrder, key)
	}
	if state.itemID == "" {
		state.itemID = itemID
	}
	state.outputIndex = outputIndex
	return state
}

// captureTerminalResponseStates 从已归一化的 completed output 补充缺失的 item 状态。
func (n *deepSeekResponsesReasoningNormalizer) captureTerminalResponseStates(data []byte) {
	for outputIndex, item := range gjson.GetBytes(data, "response.output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "reasoning" {
			continue
		}
		itemID := strings.TrimSpace(item.Get("id").String())
		envelope := []byte(`{"output_index":` + strconv.Itoa(outputIndex) + `,"item_id":` + strconv.Quote(itemID) + `}`)
		state := n.stateForEvent(envelope, []byte(item.Raw))
		if summary := item.Get("summary.0.text").String(); summary != "" {
			state.terminalText = summary
		}
	}
}

// closeStateEvents 为尚未闭合的 reasoning item 依次生成 text done 和 part done。
func (n *deepSeekResponsesReasoningNormalizer) closeStateEvents(state *deepSeekResponsesReasoningItemState) [][]byte {
	if state == nil || state.partDone {
		return nil
	}
	text := state.finalText()
	events := make([][]byte, 0, 3)
	if !state.partAdded {
		events = append(events, makeDeepSeekReasoningPartEvent("response.reasoning_summary_part.added", state, ""))
		state.partAdded = true
	}
	if !state.textDone {
		events = append(events, makeDeepSeekReasoningTextDoneEvent(state, text))
		state.textDone = true
	}
	events = append(events, makeDeepSeekReasoningPartEvent("response.reasoning_summary_part.done", state, text))
	state.partDone = true
	return events
}

// convertDeepSeekReasoningTextEvent 把 reasoning_text 的索引字段改为 summary_text 形态。
func convertDeepSeekReasoningTextEvent(data []byte, eventType string) []byte {
	updated, err := sjson.SetBytes(data, "type", eventType)
	if err != nil {
		return data
	}
	updated, _ = sjson.DeleteBytes(updated, "content_index")
	updated, _ = sjson.SetBytes(updated, "summary_index", 0)
	return updated
}

// makeDeepSeekReasoningPartEvent 构造 summary part added/done 的标准载荷。
func makeDeepSeekReasoningPartEvent(eventType string, state *deepSeekResponsesReasoningItemState, text string) []byte {
	payload := map[string]any{
		"type":          eventType,
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"part": map[string]any{
			"type": "summary_text",
			"text": text,
		},
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

// makeDeepSeekReasoningTextDoneEvent 构造缺失的 summary text done 载荷。
func makeDeepSeekReasoningTextDoneEvent(state *deepSeekResponsesReasoningItemState, text string) []byte {
	payload := map[string]any{
		"type":          "response.reasoning_summary_text.done",
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"text":          text,
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

// applySequenceNumbers 为新增事件顺延 sequence_number，并同步后续事件偏移。
func (n *deepSeekResponsesReasoningNormalizer) applySequenceNumbers(source []byte, events [][]byte, changed bool) ([][]byte, bool) {
	sequence := gjson.GetBytes(source, "sequence_number")
	if sequence.Exists() {
		base := sequence.Int() + n.sequenceOffset
		for index, event := range events {
			if updated, err := sjson.SetBytes(event, "sequence_number", base+int64(index)); err == nil {
				events[index] = updated
			}
		}
		if len(events) > 1 {
			n.sequenceOffset += int64(len(events) - 1)
		}
		if base != sequence.Int() {
			changed = true
		}
	}
	if changed {
		n.changed = true
	}
	return events, changed
}

type openAISSELineScanner interface {
	Scan() bool
	Text() string
	Err() error
}

// deepSeekResponsesReasoningSSEScanner 按完整 SSE event 缓冲一小段文本，确保
// event 名、data.type 和新增的 summary 生命周期始终同步输出。
type deepSeekResponsesReasoningSSEScanner struct {
	source     openAISSELineScanner                  // source 是原始 Responses SSE 行扫描器。
	normalizer *deepSeekResponsesReasoningNormalizer // normalizer 保存当前响应的 reasoning 转换状态。
	pending    []string                              // pending 保存一个上游事件转换出的待发送行。
	current    string                                // current 是最近一次 Scan 返回的行。
	eventLines []string                              // eventLines 暂存当前完整 SSE event 的原始行。
	sourceDone bool                                  // sourceDone 标记底层扫描器已到达 EOF。
}

// newDeepSeekResponsesReasoningSSEScanner 创建保持 SSE 事件边界的转换扫描器。
func newDeepSeekResponsesReasoningSSEScanner(source openAISSELineScanner, enabled bool) *deepSeekResponsesReasoningSSEScanner {
	return &deepSeekResponsesReasoningSSEScanner{
		source:     source,
		normalizer: newDeepSeekResponsesReasoningNormalizer(enabled),
	}
}

// Scan 返回下一条归一化后的 SSE 行，并在需要时展开新增 summary 事件。
func (s *deepSeekResponsesReasoningSSEScanner) Scan() bool {
	for {
		if len(s.pending) > 0 {
			s.current = s.pending[0]
			s.pending = s.pending[1:]
			return true
		}
		if s.sourceDone {
			return false
		}
		if s.source != nil && s.source.Scan() {
			line := s.source.Text()
			if line == "" {
				s.flushEventLines(true)
				continue
			}
			if isNewSSEEventBoundary(s.eventLines, line) {
				s.flushEventLines(true)
				s.eventLines = append(s.eventLines, line)
				continue
			}
			s.eventLines = append(s.eventLines, line)
			continue
		}
		s.sourceDone = true
		s.flushEventLines(false)
	}
}

// Text 返回最近一次 Scan 生成的 SSE 行。
func (s *deepSeekResponsesReasoningSSEScanner) Text() string { return s.current }

// Err 返回底层扫描器的读取错误。
func (s *deepSeekResponsesReasoningSSEScanner) Err() error {
	if s == nil || s.source == nil {
		return nil
	}
	return s.source.Err()
}

// isNewSSEEventBoundary 兼容缺少空行分隔、但连续出现 event/data 的上游流。
func isNewSSEEventBoundary(lines []string, next string) bool {
	if len(lines) == 0 {
		return false
	}
	hasData := false
	for _, line := range lines {
		if _, ok := extractOpenAISSEDataLine(line); ok {
			hasData = true
			break
		}
	}
	if !hasData {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(next), "event:") {
		return true
	}
	_, nextIsData := extractOpenAISSEDataLine(next)
	return nextIsData
}

// flushEventLines 转换当前完整 SSE event，并把结果展开到 pending 队列。
func (s *deepSeekResponsesReasoningSSEScanner) flushEventLines(terminated bool) {
	if len(s.eventLines) == 0 {
		if terminated {
			s.pending = append(s.pending, "")
		}
		return
	}
	lines := append([]string(nil), s.eventLines...)
	s.eventLines = s.eventLines[:0]

	dataIndex := -1
	data := ""
	hasEventLine := false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "event:") {
			hasEventLine = true
		}
		if payload, ok := extractOpenAISSEDataLine(line); ok {
			dataIndex = index
			data = payload
			break
		}
	}
	if dataIndex < 0 || strings.TrimSpace(data) == "[DONE]" {
		s.pending = append(s.pending, lines...)
		if terminated {
			s.pending = append(s.pending, "")
		}
		return
	}

	events, changed := s.normalizer.transformEvent([]byte(data))
	if !changed {
		s.pending = append(s.pending, lines...)
		if terminated {
			s.pending = append(s.pending, "")
		}
		return
	}
	for eventIndex, event := range events {
		eventType := strings.TrimSpace(gjson.GetBytes(event, "type").String())
		if eventIndex == 0 {
			for index, line := range lines {
				switch {
				case index == dataIndex:
					s.pending = append(s.pending, "data: "+string(event))
				case strings.HasPrefix(strings.TrimSpace(line), "event:"):
					s.pending = append(s.pending, "event: "+eventType)
				default:
					s.pending = append(s.pending, line)
				}
			}
		} else {
			if hasEventLine {
				s.pending = append(s.pending, "event: "+eventType)
			}
			s.pending = append(s.pending, "data: "+string(event))
		}
		s.pending = append(s.pending, "")
	}
}

// normalizeDeepSeekResponsesReasoningSSEBody 复用流式转换器处理已完整读取的
// SSE，供 stream=false 但上游仍返回 event-stream 的路径使用。
func normalizeDeepSeekResponsesReasoningSSEBody(body []byte, enabled bool) ([]byte, bool) {
	if !enabled || len(body) == 0 {
		return body, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanBuf := make([]byte, 64*1024)
	scanner.Buffer(scanBuf, defaultMaxLineSize)
	stream := newDeepSeekResponsesReasoningSSEScanner(newOpenAISSEJSONDocumentScanner(scanner), true)
	var normalized strings.Builder
	for stream.Scan() {
		normalized.WriteString(stream.Text())
		normalized.WriteByte('\n')
	}
	if stream.Err() != nil || !stream.normalizer.changed {
		return body, false
	}
	return []byte(normalized.String()), true
}
