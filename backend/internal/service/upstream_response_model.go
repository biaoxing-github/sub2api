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
	first             string
	terminal          string
	conflict          bool
	firstTier         string // Chat 响应首次声明的实际服务档位。
	firstTierConflict bool   // 非终态档位发生冲突时不用于计费。
	terminalTier      string // Responses 终态声明优先，忽略前导事件的请求回显。
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
	if o == nil {
		return
	}
	model := firstValidTrimmedGJSONModel(payload, "response.model", "model")
	terminal := isUpstreamResponseModelTerminalEvent(eventType)
	o.Observe(model, terminal)
	// 档位可以独立于模型出现在 usage chunk 或终态响应中。
	if terminal || strings.TrimSpace(eventType) == "" {
		o.ObserveServiceTier(normalizeObservedOpenAIServiceTier(firstValidTrimmedGJSONModel(payload, "response.service_tier", "service_tier")), terminal)
	}
}

// ObserveServiceTier 只保留一致的非终态声明或上游终态明确给出的实际档位。
func (o *upstreamResponseModelObserver) ObserveServiceTier(tier string, terminal bool) {
	if o == nil || tier == "" {
		return
	}
	if terminal {
		o.terminalTier = tier
		return
	}
	if o.firstTier == "" {
		o.firstTier = tier
		return
	}
	if o.firstTier != tier {
		o.firstTierConflict = true
	}
}

// ServiceTier 返回可用于结算的实际档位；不明确时返回空值。
func (o *upstreamResponseModelObserver) ServiceTier() string {
	if o == nil {
		return ""
	}
	if o.terminalTier != "" {
		return o.terminalTier
	}
	if o.firstTierConflict {
		return ""
	}
	return o.firstTier
}

// normalizeObservedOpenAIServiceTier 不把 auto 或未知值当作实际处理档位。
func normalizeObservedOpenAIServiceTier(raw string) string {
	switch value := strings.ToLower(strings.TrimSpace(raw)); value {
	case "priority", "fast":
		return "priority"
	case "default", "flex", "scale":
		return value
	default:
		return ""
	}
}

func observedUpstreamResponseServiceTier(c *gin.Context) string {
	return upstreamResponseModelObserverFromContext(c).ServiceTier()
}

func (o *upstreamResponseModelObserver) ObserveAnthropic(payload []byte) {
	if o == nil {
		return
	}
	o.Observe(firstValidTrimmedGJSONModel(payload, "message.model", "model"), false)
}

func (o *upstreamResponseModelObserver) ObserveGemini(payload []byte) {
	if o == nil {
		return
	}
	o.Observe(firstValidTrimmedGJSONModel(
		payload,
		"modelVersion",
		"response.modelVersion",
		"response.response.modelVersion",
	), true)
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

func firstValidTrimmedGJSONModel(payload []byte, paths ...string) string {
	if len(payload) == 0 {
		return ""
	}
	for _, path := range paths {
		value := gjson.GetBytes(payload, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		if model := strings.TrimSpace(value.String()); model != "" {
			// 仅在发现候选模型字段后校验完整 JSON，避免为不含模型的流式增量重复扫描。
			if !gjson.ValidBytes(payload) {
				return ""
			}
			return model
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
	sentModel = strings.TrimSpace(sentModel)
	mismatch := sentModel == "" || !upstreamModelsMatchForAudit(sentModel, responseModel)
	return &mismatch
}

// upstreamModelsMatchForAudit 仅在审计中归一化 Grok 公共别名，保留原始计费模型。
func upstreamModelsMatchForAudit(sentModel, responseModel string) bool {
	if strings.EqualFold(sentModel, responseModel) {
		return true
	}
	model := canonicalGrokBuildRuntimeModel(sentModel)
	return model != "" && model == canonicalGrokBuildRuntimeModel(responseModel)
}

// canonicalGrokBuildRuntimeModel 映射已知 xAI 模型别名与运行时 build 标识。
func canonicalGrokBuildRuntimeModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "grok-4.5", "grok-4.5-latest", "grok-4.5-build":
		return "grok-4.5-build"
	case "grok-4.6", "grok-4.6-latest", "grok-4.6-build":
		return "grok-4.6-build"
	default:
		return ""
	}
}

func upstreamSentModel(requestedModel, upstreamModel string) string {
	if model := strings.TrimSpace(upstreamModel); model != "" {
		return model
	}
	return strings.TrimSpace(requestedModel)
}
