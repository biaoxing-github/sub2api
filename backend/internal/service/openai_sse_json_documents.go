package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

const (
	maxOpenAIConcatenatedJSONDocuments = 16
	maxOpenAIConcatenatedJSONBytes     = 16 * 1024 * 1024
)

// splitOpenAIConcatenatedJSONDocuments 仅拆分多个完整 Responses 事件被拼进同一载荷的异常形态。
// 其他损坏 JSON 保持原样，继续交给既有错误路径处理。
func splitOpenAIConcatenatedJSONDocuments(payload []byte) ([][]byte, bool) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || len(payload) > maxOpenAIConcatenatedJSONBytes || json.Valid(payload) {
		return nil, false
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	documents := make([][]byte, 0, 2)
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF && len(documents) > 1 {
				return documents, true
			}
			return nil, false
		}
		raw = bytes.TrimSpace(raw)
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, false
		}
		eventType := strings.TrimSpace(envelope.Type)
		if eventType == "" || strings.ContainsAny(eventType, "\r\n") {
			return nil, false
		}
		if len(documents) == maxOpenAIConcatenatedJSONDocuments {
			return nil, false
		}
		documents = append(documents, raw)
	}
}

// openAISSEJSONDocumentScanner 将单个 data 行中的多个 JSON 文档恢复成独立 SSE 事件。
type openAISSEJSONDocumentScanner struct {
	scanner *bufio.Scanner
	pending []string
	current string
}

// newOpenAISSEJSONDocumentScanner 包装现有 Scanner，并保留其错误和缓冲限制语义。
func newOpenAISSEJSONDocumentScanner(scanner *bufio.Scanner) *openAISSEJSONDocumentScanner {
	return &openAISSEJSONDocumentScanner{scanner: scanner}
}

// Scan 返回下一条原始 SSE 行，或从拼接载荷中恢复出的事件行。
func (s *openAISSEJSONDocumentScanner) Scan() bool {
	if len(s.pending) > 0 {
		s.current = s.pending[0]
		s.pending = s.pending[1:]
		return true
	}
	if s.scanner == nil || !s.scanner.Scan() {
		return false
	}

	line := s.scanner.Text()
	data, ok := extractOpenAISSEDataLine(line)
	if !ok || len(data) > maxOpenAIConcatenatedJSONBytes {
		s.current = line
		return true
	}
	documents, repaired := splitOpenAIConcatenatedJSONDocuments([]byte(data))
	if !repaired {
		s.current = line
		return true
	}

	expanded := make([]string, 0, len(documents)*3)
	for i, document := range documents {
		if i > 0 {
			var envelope struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(document, &envelope)
			expanded = append(expanded, "event: "+strings.TrimSpace(envelope.Type))
		}
		expanded = append(expanded, "data: "+string(document), "")
	}
	s.current = expanded[0]
	s.pending = expanded[1:]
	return true
}

// Text 返回最近一次 Scan 产生的 SSE 行。
func (s *openAISSEJSONDocumentScanner) Text() string { return s.current }

// Err 返回底层 Scanner 的读取错误。
func (s *openAISSEJSONDocumentScanner) Err() error {
	if s.scanner == nil {
		return nil
	}
	return s.scanner.Err()
}
