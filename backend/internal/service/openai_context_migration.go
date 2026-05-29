package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	OpenAIContextMigrationPortableFull       = "portable_full"
	OpenAIContextMigrationPortableSummary    = "portable_summary"
	OpenAIContextMigrationUpstreamBound      = "upstream_bound"
	OpenAIContextMigrationSnapshotReplayable = "snapshot_replayable"
	OpenAIContextMigrationUnsafeAfterStream  = "unsafe_after_stream"
	OpenAIContextMigrationUnknown            = "unknown"
)

type OpenAIContextMigrationObservation struct {
	Class                   string         `json:"class"`
	Reason                  string         `json:"reason,omitempty"`
	HasPreviousResponseID   bool           `json:"has_previous_response_id"`
	PreviousResponseIDKind  string         `json:"previous_response_id_kind"`
	PreviousResponseIDLen   int            `json:"previous_response_id_len,omitempty"`
	HasFullInput            bool           `json:"has_full_input"`
	InputItemCount          int            `json:"input_item_count,omitempty"`
	MessageItemCount        int            `json:"message_item_count,omitempty"`
	FunctionCallOutputCount int            `json:"function_call_output_count,omitempty"`
	CustomToolOutputCount   int            `json:"custom_tool_output_count,omitempty"`
	ReasoningItemCount      int            `json:"reasoning_item_count,omitempty"`
	RequestBodyBytes        int            `json:"request_body_bytes,omitempty"`
	InputTypeCounts         map[string]int `json:"input_type_counts,omitempty"`
	InputTypeBytes          map[string]int `json:"input_type_bytes,omitempty"`
	LargestInputItemType    string         `json:"largest_input_item_type,omitempty"`
	LargestInputItemBytes   int            `json:"largest_input_item_bytes,omitempty"`
	SnapshotAvailable       bool           `json:"snapshot_available,omitempty"`
	SnapshotReplayable      bool           `json:"snapshot_replayable,omitempty"`
	AlreadyStreamedToClient bool           `json:"already_streamed_to_client,omitempty"`
}

const (
	OpenAIRequestBodyDiagnosisLevelOK       = "ok"
	OpenAIRequestBodyDiagnosisLevel8MB      = "warn_8mb"
	OpenAIRequestBodyDiagnosisLevel16MB     = "warn_16mb"
	OpenAIRequestBodyDiagnosisLevel24MB     = "critical_24mb"
	OpenAIRequestBodyDiagnosisLevelReject   = "reject_32mb"
	openAIRequestBodyDiagnosisThreshold8MB  = 8 * 1024 * 1024
	openAIRequestBodyDiagnosisThreshold16MB = 16 * 1024 * 1024
	openAIRequestBodyDiagnosisThreshold24MB = 24 * 1024 * 1024
	openAIRequestBodyDiagnosisThreshold32MB = 32 * 1024 * 1024
)

type OpenAIRequestBodyInputTypeSize struct {
	Type    string  `json:"type"`
	Bytes   int     `json:"bytes"`
	Count   int     `json:"count,omitempty"`
	Percent float64 `json:"percent,omitempty"`
}

type OpenAIRequestBodyDiagnosis struct {
	Level              string                         `json:"level"`
	TotalBytes         int                            `json:"total_bytes"`
	ThresholdBytes     int                            `json:"threshold_bytes,omitempty"`
	InputTypeBytes     map[string]int                 `json:"input_type_bytes,omitempty"`
	InputTypeCounts    map[string]int                 `json:"input_type_counts,omitempty"`
	TopInputTypes      []OpenAIRequestBodyInputTypeSize `json:"top_input_types,omitempty"`
	LargestInputType   string                         `json:"largest_input_type,omitempty"`
	LargestInputBytes  int                            `json:"largest_input_bytes,omitempty"`
}

func (o OpenAIContextMigrationObservation) IsPortable() bool {
	switch strings.TrimSpace(o.Class) {
	case OpenAIContextMigrationPortableFull, OpenAIContextMigrationPortableSummary, OpenAIContextMigrationSnapshotReplayable:
		return true
	default:
		return false
	}
}

func (o OpenAIContextMigrationObservation) DetailMap() map[string]any {
	detail := map[string]any{}
	addString := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			detail[key] = value
		}
	}
	addBool := func(key string, value bool) {
		if value {
			detail[key] = value
		}
	}
	addInt := func(key string, value int) {
		if value > 0 {
			detail[key] = value
		}
	}
	addString("context_migration_class", o.Class)
	addString("context_migration_reason", o.Reason)
	addString("previous_response_id_kind", o.PreviousResponseIDKind)
	addBool("has_previous_response_id", o.HasPreviousResponseID)
	addBool("has_full_input", o.HasFullInput)
	addBool("snapshot_available", o.SnapshotAvailable)
	addBool("snapshot_replayable", o.SnapshotReplayable)
	addBool("already_streamed_to_client", o.AlreadyStreamedToClient)
	addInt("previous_response_id_len", o.PreviousResponseIDLen)
	addInt("input_item_count", o.InputItemCount)
	addInt("message_item_count", o.MessageItemCount)
	addInt("function_call_output_count", o.FunctionCallOutputCount)
	addInt("custom_tool_call_output_count", o.CustomToolOutputCount)
	addInt("reasoning_item_count", o.ReasoningItemCount)
	addInt("request_body_bytes", o.RequestBodyBytes)
	addString("largest_input_item_type", o.LargestInputItemType)
	addInt("largest_input_item_bytes", o.LargestInputItemBytes)
	if len(o.InputTypeCounts) > 0 {
		detail["input_type_counts"] = o.InputTypeCounts
	}
	if len(o.InputTypeBytes) > 0 {
		detail["input_type_bytes"] = o.InputTypeBytes
	}
	if diagnosis := diagnoseOpenAIRequestBodyObservation(o); diagnosis.Level != OpenAIRequestBodyDiagnosisLevelOK {
		for key, value := range diagnosis.DetailMap() {
			detail[key] = value
		}
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
}

func DiagnoseOpenAIRequestBody(body []byte) OpenAIRequestBodyDiagnosis {
	obs := ClassifyOpenAIContextMigration(body, false)
	return diagnoseOpenAIRequestBodyObservation(obs)
}

func diagnoseOpenAIRequestBodyObservation(obs OpenAIContextMigrationObservation) OpenAIRequestBodyDiagnosis {
	diagnosis := OpenAIRequestBodyDiagnosis{
		Level:             openAIRequestBodyDiagnosisLevel(obs.RequestBodyBytes),
		TotalBytes:        obs.RequestBodyBytes,
		ThresholdBytes:    openAIRequestBodyDiagnosisThreshold(obs.RequestBodyBytes),
		InputTypeBytes:    cloneIntMap(obs.InputTypeBytes),
		InputTypeCounts:   cloneIntMap(obs.InputTypeCounts),
		LargestInputType:  obs.LargestInputItemType,
		LargestInputBytes: obs.LargestInputItemBytes,
	}
	diagnosis.TopInputTypes = buildOpenAIRequestBodyTopInputTypes(diagnosis.InputTypeBytes, diagnosis.InputTypeCounts, diagnosis.TotalBytes, 5)
	return diagnosis
}

func (d OpenAIRequestBodyDiagnosis) DetailMap() map[string]any {
	if d.Level == "" {
		d.Level = OpenAIRequestBodyDiagnosisLevelOK
	}
	detail := map[string]any{
		"request_body_diagnosis_level": d.Level,
		"request_body_bytes":           d.TotalBytes,
	}
	if d.ThresholdBytes > 0 {
		detail["request_body_threshold_bytes"] = d.ThresholdBytes
	}
	if len(d.InputTypeBytes) > 0 {
		detail["request_body_input_type_bytes"] = d.InputTypeBytes
	}
	if len(d.TopInputTypes) > 0 {
		detail["request_body_top_input_types"] = d.TopInputTypes
	}
	if strings.TrimSpace(d.LargestInputType) != "" {
		detail["request_body_largest_input_type"] = d.LargestInputType
		detail["request_body_largest_input_bytes"] = d.LargestInputBytes
	}
	return detail
}

func (d OpenAIRequestBodyDiagnosis) UserMessage() string {
	if d.Level == "" || d.Level == OpenAIRequestBodyDiagnosisLevelOK {
		return ""
	}
	parts := make([]string, 0, min(len(d.TopInputTypes), 3))
	for _, item := range d.TopInputTypes {
		if item.Type == "" || item.Bytes <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", item.Type, formatOpenAIRequestBodyBytes(item.Bytes)))
		if len(parts) >= 3 {
			break
		}
	}
	where := "未能识别具体 input 类型"
	if len(parts) > 0 {
		where = strings.Join(parts, "、")
	}
	return fmt.Sprintf("请求体 %s 已达到 %s 档位，主要由 %s 撑大；建议裁剪历史上下文、图片生成结果或工具输出后重试。",
		formatOpenAIRequestBodyBytes(d.TotalBytes),
		d.Level,
		where,
	)
}

func openAIRequestBodyDiagnosisLevel(totalBytes int) string {
	switch {
	case totalBytes >= openAIRequestBodyDiagnosisThreshold32MB:
		return OpenAIRequestBodyDiagnosisLevelReject
	case totalBytes >= openAIRequestBodyDiagnosisThreshold24MB:
		return OpenAIRequestBodyDiagnosisLevel24MB
	case totalBytes >= openAIRequestBodyDiagnosisThreshold16MB:
		return OpenAIRequestBodyDiagnosisLevel16MB
	case totalBytes >= openAIRequestBodyDiagnosisThreshold8MB:
		return OpenAIRequestBodyDiagnosisLevel8MB
	default:
		return OpenAIRequestBodyDiagnosisLevelOK
	}
}

func openAIRequestBodyDiagnosisThreshold(totalBytes int) int {
	switch openAIRequestBodyDiagnosisLevel(totalBytes) {
	case OpenAIRequestBodyDiagnosisLevelReject:
		return openAIRequestBodyDiagnosisThreshold32MB
	case OpenAIRequestBodyDiagnosisLevel24MB:
		return openAIRequestBodyDiagnosisThreshold24MB
	case OpenAIRequestBodyDiagnosisLevel16MB:
		return openAIRequestBodyDiagnosisThreshold16MB
	case OpenAIRequestBodyDiagnosisLevel8MB:
		return openAIRequestBodyDiagnosisThreshold8MB
	default:
		return 0
	}
}

func buildOpenAIRequestBodyTopInputTypes(bytesByType map[string]int, countsByType map[string]int, totalBytes int, limit int) []OpenAIRequestBodyInputTypeSize {
	if len(bytesByType) == 0 || limit <= 0 {
		return nil
	}
	items := make([]OpenAIRequestBodyInputTypeSize, 0, len(bytesByType))
	for itemType, size := range bytesByType {
		if size <= 0 {
			continue
		}
		item := OpenAIRequestBodyInputTypeSize{
			Type:  itemType,
			Bytes: size,
			Count: countsByType[itemType],
		}
		if totalBytes > 0 {
			item.Percent = float64(size) * 100 / float64(totalBytes)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Bytes != items[j].Bytes {
			return items[i].Bytes > items[j].Bytes
		}
		return items[i].Type < items[j].Type
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func cloneIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func formatOpenAIRequestBodyBytes(bytes int) string {
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%.2fMB", float64(bytes)/(1024*1024))
	}
	if bytes >= 1024 {
		return fmt.Sprintf("%.2fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}

func ClassifyOpenAIContextMigration(body []byte, alreadyStreamedToClient bool) OpenAIContextMigrationObservation {
	obs := OpenAIContextMigrationObservation{
		Class:                   OpenAIContextMigrationUnknown,
		Reason:                  "invalid_or_empty_request_body",
		PreviousResponseIDKind:  OpenAIPreviousResponseIDKindEmpty,
		RequestBodyBytes:        len(body),
		AlreadyStreamedToClient: alreadyStreamedToClient,
	}
	if alreadyStreamedToClient {
		obs.Class = OpenAIContextMigrationUnsafeAfterStream
		obs.Reason = "already_streamed_to_client"
		return obs
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return obs
	}

	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	obs.HasPreviousResponseID = previousResponseID != ""
	obs.PreviousResponseIDKind = ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	obs.PreviousResponseIDLen = len(previousResponseID)
	obs.collectInputShape(body)

	if obs.HasPreviousResponseID {
		obs.Class = OpenAIContextMigrationUpstreamBound
		obs.Reason = "previous_response_id_requires_original_upstream_context"
		return obs
	}

	if !obs.HasFullInput {
		obs.Class = OpenAIContextMigrationUnknown
		obs.Reason = "missing_full_input"
		return obs
	}
	if obs.FunctionCallOutputCount > 0 || obs.CustomToolOutputCount > 0 || obs.ReasoningItemCount > 0 || obs.RequestBodyBytes >= 512*1024 || obs.InputItemCount >= 200 {
		obs.Class = OpenAIContextMigrationPortableFull
		obs.Reason = "client_carries_full_context"
		return obs
	}
	obs.Class = OpenAIContextMigrationPortableSummary
	obs.Reason = "client_carries_summary_context"
	return obs
}

func markOpenAIContextMigrationSnapshotReplayable(obs OpenAIContextMigrationObservation, reason string) OpenAIContextMigrationObservation {
	obs.Class = OpenAIContextMigrationSnapshotReplayable
	if reason = strings.TrimSpace(reason); reason == "" {
		reason = "local_snapshot_replayable"
	}
	obs.Reason = reason
	obs.SnapshotAvailable = true
	obs.SnapshotReplayable = true
	return obs
}

func (o *OpenAIContextMigrationObservation) collectInputShape(body []byte) {
	if o == nil {
		return
	}
	o.InputTypeCounts = map[string]int{}
	o.InputTypeBytes = map[string]int{}
	recordInputBytes := func(itemType string, raw string, fallback string) {
		itemType = strings.TrimSpace(itemType)
		if itemType == "" {
			itemType = "unknown"
		}
		size := len(raw)
		if size == 0 {
			size = len(fallback)
		}
		if size <= 0 {
			return
		}
		o.InputTypeBytes[itemType] += size
		if size > o.LargestInputItemBytes {
			o.LargestInputItemBytes = size
			o.LargestInputItemType = itemType
		}
	}
	countInputItem := func(item gjson.Result) {
		o.InputItemCount++
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "" {
			itemType = "unknown"
		}
		o.InputTypeCounts[itemType]++
		recordInputBytes(itemType, item.Raw, item.String())
		switch itemType {
		case "message":
			o.MessageItemCount++
		case "function_call_output":
			o.FunctionCallOutputCount++
		case "custom_tool_call_output":
			o.CustomToolOutputCount++
		case "reasoning":
			o.ReasoningItemCount++
		}
	}

	input := gjson.GetBytes(body, "input")
	if input.IsArray() {
		input.ForEach(func(_, value gjson.Result) bool {
			countInputItem(value)
			return true
		})
		o.HasFullInput = o.InputItemCount > 0
		return
	}
	if strings.TrimSpace(input.String()) != "" {
		o.InputItemCount = 1
		o.HasFullInput = true
		o.InputTypeCounts["scalar"] = 1
		recordInputBytes("scalar", input.Raw, input.String())
		return
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		messages.ForEach(func(_, value gjson.Result) bool {
			o.InputItemCount++
			o.MessageItemCount++
			itemType := strings.TrimSpace(value.Get("role").String())
			if itemType == "" {
				itemType = "message"
			}
			o.InputTypeCounts[itemType]++
			recordInputBytes(itemType, value.Raw, value.String())
			return true
		})
		o.HasFullInput = o.InputItemCount > 0
	}
}
