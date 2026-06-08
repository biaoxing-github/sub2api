//go:build unit

package service

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

// TestClassifyOpenAITransportError 固定 OpenAI 上游传输层错误分类：
// 持久故障会临时摘除账号，瞬时故障只触发 failover。
func TestClassifyOpenAITransportError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		persistent bool
	}{
		{"socks5 proxy credential rejected", errors.New(`Post "https://chatgpt.com/backend-api/codex/responses": socks connect tcp 85.255.176.68:12324->chatgpt.com:443: username/password authentication failed`), true},
		{"proxy connection refused", errors.New(`proxyconnect tcp: dial tcp 1.2.3.4:1080: connect: connection refused`), true},
		{"no route to host", errors.New(`dial tcp 1.2.3.4:443: connect: no route to host`), true},
		{"dns resolution failure", errors.New(`dial tcp: lookup proxy.example.com: no such host`), true},
		{"network unreachable", errors.New(`dial tcp 1.2.3.4:443: connect: network is unreachable`), true},
		{"client timeout", errors.New(`Post "https://chatgpt.com/...": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), false},
		{"io timeout", errors.New(`dial tcp 1.2.3.4:443: i/o timeout`), false},
		{"connection reset by peer", errors.New(`read tcp 10.0.0.1:5->2.2.2.2:443: read: connection reset by peer`), false},
		{"unexpected eof", errors.New(`unexpected EOF`), false},
		{"broken pipe", errors.New(`write tcp 10.0.0.1:5->2.2.2.2:443: write: broken pipe`), false},
		{"nil error", nil, false},
		{
			"ECONNREFUSED via net.OpError",
			&net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
			},
			true,
		},
		{"ECONNREFUSED bare", syscall.ECONNREFUSED, true},
		{"EHOSTUNREACH bare", syscall.EHOSTUNREACH, true},
		{"ENETUNREACH bare", syscall.ENETUNREACH, true},
		{"DNS not found", &net.DNSError{Err: "no such host", Name: "proxy.example.com", IsNotFound: true}, true},
		{"DNS timeout", &net.DNSError{Err: "i/o timeout", Name: "proxy.example.com", IsTimeout: true}, false},
		{"context canceled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyOpenAITransportError(tc.err).Persistent
			if got != tc.persistent {
				t.Fatalf("classifyOpenAITransportError(%q).Persistent = %v, want %v", openAITransportErrString(tc.err), got, tc.persistent)
			}
		})
	}
}

func openAITransportErrString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
