package apicompat

import "testing"

func TestAnthropicEventToResponses_TextEmitsContentPart(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var types []string
	feed := func(evt *AnthropicStreamEvent) {
		for _, out := range AnthropicEventToResponsesEvents(evt, state) {
			types = append(types, out.Type)
		}
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1", Model: state.Model}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hel"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "lo"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	position := func(target string) int {
		for i, eventType := range types {
			if eventType == target {
				return i
			}
		}
		return -1
	}
	partAdded := position("response.content_part.added")
	firstDelta := position("response.output_text.delta")
	if partAdded < 0 || firstDelta < 0 || partAdded > firstDelta {
		t.Fatalf("content_part.added 顺序错误: %v", types)
	}
	if position("response.content_part.done") < 0 {
		t.Fatalf("缺少 response.content_part.done: %v", types)
	}
}

func TestAnthropicEventToResponses_DoneEventsCarryFullText(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hello "}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "world"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})

	const want = "Hello world"
	var textDone, partDone bool
	for _, event := range events {
		switch event.Type {
		case "response.output_text.done":
			textDone = event.Text == want
		case "response.content_part.done":
			partDone = event.Part != nil && event.Part.Text == want
		}
	}
	if !textDone || !partDone {
		t.Fatalf("done 事件未携带完整文本: text=%v part=%v", textDone, partDone)
	}
}

func TestAnthropicEventToResponses_CompletedCarriesOutput(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "4826"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	for i := range events {
		if events[i].Type != "response.completed" || events[i].Response == nil {
			continue
		}
		output := events[i].Response.Output
		if len(output) != 1 || len(output[0].Content) != 1 || output[0].Content[0].Text != "4826" {
			t.Fatalf("终态输出不完整: %+v", output)
		}
		return
	}
	t.Fatal("缺少 response.completed")
}

func TestAnthropicEventToResponses_ToolCallCompletedCarriesArguments(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "tool_use", ID: "toolu_1", Name: "get_weather"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: `{"city":`}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: `"SH"}`}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	for i := range events {
		if events[i].Type != "response.completed" || events[i].Response == nil {
			continue
		}
		output := events[i].Response.Output
		if len(output) != 1 || output[0].Type != "function_call" || output[0].Name != "get_weather" || output[0].Arguments != `{"city":"SH"}` {
			t.Fatalf("工具调用终态不完整: %+v", output)
		}
		return
	}
	t.Fatal("缺少 response.completed")
}
