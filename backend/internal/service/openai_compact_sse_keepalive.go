package service

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// openAICompactSSEKeepaliveKey stores the keepalive state for a body-signal
// compact request.
const openAICompactSSEKeepaliveKey = "openai_compact_sse_keepalive"

// openAICompactSSEKeepalive sends ignorable SSE comments while a unary compact
// upstream call is pending. It prevents proxy idle timeouts without presenting
// a semantic protocol event to the client.
type openAICompactSSEKeepalive struct {
	mu      sync.Mutex
	writer  gin.ResponseWriter
	started bool
	stopped bool
	bytes   int
	stop    chan struct{}
}

// StartOpenAICompactSSEKeepalive starts a delayed heartbeat only for marked
// compact client streams. The wrapped writer stops the heartbeat before any
// request-side response construction, preventing concurrent writes.
func StartOpenAICompactSSEKeepalive(c *gin.Context, interval time.Duration) func() {
	if c == nil || c.Writer == nil || interval <= 0 || !openAICompactClientWantsStream(c) {
		return func() {}
	}

	// 同一请求可因 failover 进入多次 Forward。前一轮可能已把 SSE 200
	// 提交给客户端，所以新一轮必须复用该提交状态和底层 writer，不能用新的
	// context value 覆盖它，否则后续本地错误会错误地退回 JSON。
	writer := c.Writer
	committed := false
	heartbeatBytes := 0
	if previous, ok := openAICompactSSEKeepaliveFromContext(c); ok {
		previous.Stop()
		previous.mu.Lock()
		writer = previous.writer
		committed = previous.started
		heartbeatBytes = previous.bytes
		previous.mu.Unlock()
	}
	k := &openAICompactSSEKeepalive{
		writer:  writer,
		started: committed,
		bytes:   heartbeatBytes,
		stop:    make(chan struct{}),
	}
	c.Set(openAICompactSSEKeepaliveKey, k)
	originalWriter := writer
	wrappedWriter := &openAICompactKeepaliveWriter{ResponseWriter: writer, keepalive: k}
	c.Writer = wrappedWriter

	var requestDone <-chan struct{}
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-k.stop:
				return
			case <-requestDone:
				return
			case <-timer.C:
			}
			if !k.beat() {
				return
			}
			timer.Reset(interval)
		}
	}()
	return func() {
		k.Stop()
		// Do not leave a pooled middleware writer reachable through the compact
		// wrapper after the request finishes.
		if current, ok := c.Writer.(*openAICompactKeepaliveWriter); ok && current == wrappedWriter {
			c.Writer = originalWriter
		}
	}
}

func openAICompactSSEKeepaliveFromContext(c *gin.Context) (*openAICompactSSEKeepalive, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(openAICompactSSEKeepaliveKey)
	if !ok {
		return nil, false
	}
	k, ok := value.(*openAICompactSSEKeepalive)
	return k, ok && k != nil
}

// beat commits an SSE response and writes one comment while holding the same
// lock used by request-side response writes.
func (k *openAICompactSSEKeepalive) beat() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stopped {
		return false
	}
	if !k.started {
		header := k.writer.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
		k.writer.WriteHeader(http.StatusOK)
		k.started = true
	}
	n, err := k.writer.Write([]byte(": keepalive\n\n"))
	k.bytes += n
	if err != nil {
		k.stopped = true
		return false
	}
	k.writer.Flush()
	return true
}

// Stop ends the heartbeat. It is safe to call repeatedly or concurrently.
func (k *openAICompactSSEKeepalive) Stop() {
	k.mu.Lock()
	k.markStoppedLocked()
	k.mu.Unlock()
}

func (k *openAICompactSSEKeepalive) markStoppedLocked() {
	if k.stopped {
		return
	}
	k.stopped = true
	close(k.stop)
}

// StopOpenAICompactSSEKeepaliveCommitted stops the heartbeat and reports
// whether it had already committed the response as an SSE 200 stream.
func StopOpenAICompactSSEKeepaliveCommitted(c *gin.Context) bool {
	k, ok := openAICompactSSEKeepaliveFromContext(c)
	if !ok {
		return false
	}
	k.mu.Lock()
	k.markStoppedLocked()
	committed := k.started
	k.mu.Unlock()
	return committed
}

// OpenAICompactKeepaliveAdjustedWrittenSize removes heartbeat bytes from the
// writer-size signal used to decide whether a real response was emitted.
func OpenAICompactKeepaliveAdjustedWrittenSize(c *gin.Context) int {
	if c == nil || c.Writer == nil {
		return -1
	}
	k, ok := openAICompactSSEKeepaliveFromContext(c)
	if !ok {
		return c.Writer.Size()
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	size := k.writer.Size()
	if size < 0 {
		return size
	}
	if real := size - k.bytes; real > 0 {
		return real
	}
	return -1
}

// openAICompactKeepaliveWriter serializes request-side writes with heartbeat
// beats while allowing read-only writer state queries to remain non-destructive.
type openAICompactKeepaliveWriter struct {
	gin.ResponseWriter
	keepalive *openAICompactSSEKeepalive
}

func (w *openAICompactKeepaliveWriter) suspend() {
	w.keepalive.Stop()
}

func (w *openAICompactKeepaliveWriter) Header() http.Header {
	w.suspend()
	return w.ResponseWriter.Header()
}

func (w *openAICompactKeepaliveWriter) Write(data []byte) (int, error) {
	w.suspend()
	return w.ResponseWriter.Write(data)
}

func (w *openAICompactKeepaliveWriter) WriteString(value string) (int, error) {
	w.suspend()
	return w.ResponseWriter.WriteString(value)
}

func (w *openAICompactKeepaliveWriter) WriteHeader(code int) {
	w.suspend()
	w.ResponseWriter.WriteHeader(code)
}

func (w *openAICompactKeepaliveWriter) WriteHeaderNow() {
	w.suspend()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *openAICompactKeepaliveWriter) Flush() {
	w.suspend()
	w.ResponseWriter.Flush()
}

func (w *openAICompactKeepaliveWriter) Status() int {
	w.keepalive.mu.Lock()
	defer w.keepalive.mu.Unlock()
	return w.ResponseWriter.Status()
}

func (w *openAICompactKeepaliveWriter) Size() int {
	w.keepalive.mu.Lock()
	defer w.keepalive.mu.Unlock()
	return w.ResponseWriter.Size()
}

func (w *openAICompactKeepaliveWriter) Written() bool {
	w.keepalive.mu.Lock()
	defer w.keepalive.mu.Unlock()
	return w.ResponseWriter.Written()
}
