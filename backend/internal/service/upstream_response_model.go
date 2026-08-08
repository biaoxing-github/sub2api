package service

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	upstreamResponseModelObserverContextKey = "upstream_response_model_observer"
	upstreamResponseModelMaxLength          = 200
)

// upstreamResponseModelObserver 记录单次上游尝试声明的实际模型；终态声明优先。
type upstreamResponseModelObserver struct {
	first    string
	terminal string
	conflict bool
}

func (o *upstreamResponseModelObserver) Observe(model string, terminal bool) {
	model = normalizeObservedUpstreamResponseModel(model)
	if o == nil || model == "" {
		return
	}
	if current := o.Model(); current != "" && !strings.EqualFold(current, model) {
		o.conflict = true
	}
	if terminal {
		o.terminal = model
		return
	}
	if o.first == "" {
		o.first = model
	}
}

func normalizeObservedUpstreamResponseModel(model string) string {
	model = strings.TrimSpace(model)
	runes := []rune(model)
	if len(runes) > upstreamResponseModelMaxLength {
		model = string(runes[:upstreamResponseModelMaxLength])
	}
	return model
}

func (o *upstreamResponseModelObserver) ObserveOpenAI(payload []byte, eventType string) {
	if o == nil || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return
	}
	model := firstTrimmedGJSONModel(gjson.GetBytes(payload, "response.model"), gjson.GetBytes(payload, "model"))
	o.Observe(model, isUpstreamResponseModelTerminalEvent(eventType))
}

func (o *upstreamResponseModelObserver) ObserveAnthropic(payload []byte) {
	if o == nil || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return
	}
	o.Observe(firstTrimmedGJSONModel(gjson.GetBytes(payload, "message.model"), gjson.GetBytes(payload, "model")), false)
}

func (o *upstreamResponseModelObserver) ObserveGemini(payload []byte) {
	if o == nil || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return
	}
	o.Observe(firstTrimmedGJSONModel(gjson.GetBytes(payload, "modelVersion"), gjson.GetBytes(payload, "response.modelVersion")), true)
}

func (o *upstreamResponseModelObserver) Model() string {
	if o == nil {
		return ""
	}
	if o.terminal != "" {
		return o.terminal
	}
	return o.first
}

func (o *upstreamResponseModelObserver) Conflict() bool { return o != nil && o.conflict }

func beginUpstreamResponseModelObservation(c *gin.Context) *upstreamResponseModelObserver {
	observer := &upstreamResponseModelObserver{}
	if c != nil {
		c.Set(upstreamResponseModelObserverContextKey, observer)
	}
	return observer
}

func upstreamResponseModelObserverFromContext(c *gin.Context) *upstreamResponseModelObserver {
	if c == nil {
		return nil
	}
	value, ok := c.Get(upstreamResponseModelObserverContextKey)
	if !ok {
		return nil
	}
	observer, _ := value.(*upstreamResponseModelObserver)
	return observer
}

func observedUpstreamResponseModel(c *gin.Context) string {
	return upstreamResponseModelObserverFromContext(c).Model()
}

func observedUpstreamResponseModelConflict(c *gin.Context) bool {
	return upstreamResponseModelObserverFromContext(c).Conflict()
}

func firstTrimmedGJSONModel(values ...gjson.Result) string {
	for _, value := range values {
		if value.Exists() && value.Type == gjson.String {
			if model := strings.TrimSpace(value.String()); model != "" {
				return model
			}
		}
	}
	return ""
}

func isUpstreamResponseModelTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func upstreamModelMismatch(sentModel, responseModel string) *bool {
	responseModel = strings.TrimSpace(responseModel)
	if responseModel == "" {
		return nil
	}
	mismatch := !strings.EqualFold(strings.TrimSpace(sentModel), responseModel)
	return &mismatch
}

func upstreamSentModel(requestedModel, upstreamModel string) string {
	if model := strings.TrimSpace(upstreamModel); model != "" {
		return model
	}
	return strings.TrimSpace(requestedModel)
}
