package openai_ws_v2

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// TestRelay_CloseBeforeTerminal 验证传输 EOF 不会把未完成轮次报告为成功。
func TestRelay_CloseBeforeTerminal(t *testing.T) {
	client := newPassthroughTestFrameConn(nil, false)
	upstream := newPassthroughTestFrameConn([]passthroughTestFrame{
		{msgType: coderws.MessageText, payload: []byte(`{"type":"response.created","response":{"id":"resp_pending","status":"in_progress"}}`)},
	}, true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, exit := Relay(ctx, client, upstream, []byte(`{"type":"response.create","model":"gpt-5","input":[]}`), RelayOptions{})
	require.NotNil(t, exit)
	require.Equal(t, "read_upstream", exit.Stage)
	require.ErrorContains(t, exit.Err, "closed before terminal event")
	require.Empty(t, result.TerminalEventType)
}

// TestRelay_PendingTurnEOF 覆盖后续轮次首事件前关闭以及完整终态后的正常关闭。
func TestRelay_PendingTurnEOF(t *testing.T) {
	for _, secondTurn := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "second_pending"}[secondTurn], func(t *testing.T) {
			state := &relayState{}
			request := []byte(`{"type":"response.create","model":"gpt-5","input":[]}`)
			observeClientTurnRequest(state, coderws.MessageText, request)
			observeUpstreamMessage(state, []byte(`{"type":"response.completed","response":{"id":"r1"}}`), time.Now(), time.Now, nil)
			require.Zero(t, state.pendingTurns.Load())
			if secondTurn {
				observeClientTurnRequest(state, coderws.MessageText, request)
			}
			exits := make(chan relayExitSignal, 1)
			runUpstreamToClient(context.Background(), newPassthroughTestFrameConn(nil, true),
				func(coderws.MessageType, []byte) error { return nil }, time.Now(), time.Now, state,
				nil, nil, &atomic.Bool{}, &atomic.Int64{}, &atomic.Int64{}, func() {}, nil, exits)
			exit := <-exits
			require.Equal(t, !secondTurn, exit.graceful)
			if secondTurn {
				require.ErrorContains(t, exit.err, "closed before terminal event")
			}
		})
	}
}
