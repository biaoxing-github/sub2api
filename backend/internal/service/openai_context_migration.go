package service

import (
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
	SnapshotAvailable       bool           `json:"snapshot_available,omitempty"`
	SnapshotReplayable      bool           `json:"snapshot_replayable,omitempty"`
	AlreadyStreamedToClient bool           `json:"already_streamed_to_client,omitempty"`
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
	if len(o.InputTypeCounts) > 0 {
		detail["input_type_counts"] = o.InputTypeCounts
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
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
	countInputItem := func(item gjson.Result) {
		o.InputItemCount++
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "" {
			itemType = "unknown"
		}
		o.InputTypeCounts[itemType]++
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
			return true
		})
		o.HasFullInput = o.InputItemCount > 0
	}
}
