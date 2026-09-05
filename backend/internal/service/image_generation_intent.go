package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	openAIResponsesEndpoint          = "/v1/responses"
	openAIResponsesCompactEndpoint   = "/v1/responses/compact"
	responsesLiteHeader              = "X-OpenAI-Internal-Codex-Responses-Lite"
	responsesLiteHeaderKey           = "x-openai-internal-codex-responses-lite"
	responsesLiteWSMetadataKey       = "ws_request_header_x_openai_internal_codex_responses_lite"
	imageGenerationPermissionMessage = "Image generation is not enabled for this group"
)

func isOpenAIResponsesLiteHeader(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func isOpenAIResponsesLiteWebSocketPayload(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	return isOpenAIResponsesLiteHeader(gjson.GetBytes(body, "client_metadata."+responsesLiteWSMetadataKey).String())
}

// ImageGenerationPermissionMessage returns the stable end-user error text for disabled groups.
func ImageGenerationPermissionMessage() string {
	return imageGenerationPermissionMessage
}

func IsOpenAIImageGenerationNotEnabledError(statusCode int, message string, body []byte) bool {
	if statusCode != http.StatusForbidden {
		return false
	}
	combined := strings.ToLower(strings.TrimSpace(message + " " + extractUpstreamErrorMessage(body) + " " + string(body)))
	return strings.Contains(combined, strings.ToLower(imageGenerationPermissionMessage))
}

// GroupAllowsImageGeneration preserves ungrouped-key behavior and enforces the flag when a group is present.
func GroupAllowsImageGeneration(group *Group) bool {
	return group == nil || group.AllowImageGeneration
}

// IsImageGenerationIntent classifies requests that can produce generated images.
func IsImageGenerationIntent(endpoint string, requestedModel string, body []byte) bool {
	if IsImageGenerationEndpoint(endpoint) {
		return true
	}
	if isOpenAIImageGenerationModel(requestedModel) {
		return true
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); isOpenAIImageGenerationModel(model) {
		return true
	}
	if openAIJSONToolsContainImageGeneration(gjson.GetBytes(body, "tools")) {
		return true
	}
	if openAIJSONInputContainsImageGenTool(gjson.GetBytes(body, "input")) {
		return true
	}
	return openAIJSONToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice"))
}

// IsExplicitImageGenerationIntent 仅将实际调用图片能力的信号视为生图意图。
// Codex 会在普通请求中被动声明 image_gen namespace；该声明只能作为工具目录，
// 不应触发生图权限、并发或 Responses-only 账号筛选。
func IsExplicitImageGenerationIntent(endpoint string, requestedModel string, body []byte) bool {
	if IsImageGenerationEndpoint(endpoint) || isOpenAIImageGenerationModel(requestedModel) {
		return true
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); isOpenAIImageGenerationModel(model) {
		return true
	}
	if openAIJSONToolsContainNativeImageGeneration(gjson.GetBytes(body, "tools")) {
		return true
	}
	return openAIJSONToolChoiceSelectsExplicitImageGeneration(gjson.GetBytes(body, "tool_choice"))
}

// IsImageGenerationIntentMap is the map-backed variant used after service-side request mutation.
func IsImageGenerationIntentMap(endpoint string, requestedModel string, reqBody map[string]any) bool {
	if IsImageGenerationEndpoint(endpoint) {
		return true
	}
	if isOpenAIImageGenerationModel(requestedModel) {
		return true
	}
	if reqBody == nil {
		return false
	}
	if isOpenAIImageGenerationModel(firstNonEmptyString(reqBody["model"])) {
		return true
	}
	if hasOpenAIImageGenerationTool(reqBody) {
		return true
	}
	return openAIAnyToolChoiceSelectsImageGeneration(reqBody["tool_choice"])
}

// IsImageGenerationEndpoint identifies dedicated generated-image endpoints.
func IsImageGenerationEndpoint(endpoint string) bool {
	switch normalizeImageGenerationEndpoint(endpoint) {
	case "/v1/images/generations", "/v1/images/edits", "/images/generations", "/images/edits":
		return true
	default:
		return false
	}
}

func normalizeImageGenerationEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(strings.ToLower(endpoint))
	if endpoint == "" {
		return ""
	}
	endpoint = strings.TrimPrefix(endpoint, "https://api.openai.com")
	if idx := strings.IndexByte(endpoint, '?'); idx >= 0 {
		endpoint = endpoint[:idx]
	}
	return strings.TrimRight(endpoint, "/")
}

func openAIJSONToolsContainImageGeneration(tools gjson.Result) bool {
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, item gjson.Result) bool {
		if isOpenAIImageGenerationType(strings.TrimSpace(item.Get("type").String())) {
			found = true
			return false
		}
		if isImageGenNamespaceTool(item) {
			found = true
			return false
		}
		return true
	})
	return found
}

// openAIJSONToolsContainNativeImageGeneration 只识别上游实际执行的原生生图工具。
func openAIJSONToolsContainNativeImageGeneration(tools gjson.Result) bool {
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, item gjson.Result) bool {
		found = isOpenAIImageGenerationType(strings.TrimSpace(item.Get("type").String()))
		return !found
	})
	return found
}

func isOpenAIImageGenerationType(value string) bool {
	return strings.TrimSpace(value) == "image_generation"
}

func isOpenAIImageGenNamespaceName(value string) bool {
	return strings.TrimSpace(value) == "image_gen"
}

// isImageGenNamespaceTool 识别 Codex 使用的 image_gen namespace 图片工具声明。
func isImageGenNamespaceTool(tool gjson.Result) bool {
	return strings.TrimSpace(tool.Get("type").String()) == "namespace" &&
		isOpenAIImageGenNamespaceName(tool.Get("name").String())
}

// openAIJSONInputContainsImageGenTool 扫描 Responses Lite input.additional_tools 中的图片 namespace。
func openAIJSONInputContainsImageGenTool(input gjson.Result) bool {
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if strings.TrimSpace(item.Get("type").String()) != "additional_tools" {
			return true
		}
		found = openAIJSONToolsContainImageGeneration(item.Get("tools"))
		return !found
	})
	return found
}

func openAIRequestBodyHasImageGenerationDeclaration(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	return openAIJSONToolsContainImageGeneration(gjson.GetBytes(body, "tools")) ||
		openAIJSONInputContainsImageGenTool(gjson.GetBytes(body, "input")) ||
		openAIJSONToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice"))
}

func openAIRequestBodyImageGenerationToolNeedsNormalization(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	needsNormalization := false
	tools.ForEach(func(_, item gjson.Result) bool {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation" {
			return true
		}
		if item.Get("format").Exists() || item.Get("compression").Exists() {
			needsNormalization = true
			return false
		}
		return true
	})
	return needsNormalization
}

func openAIJSONToolChoiceSelectsImageGeneration(choice gjson.Result) bool {
	if !choice.Exists() {
		return false
	}
	if choice.Type == gjson.String {
		return isOpenAIImageGenerationType(choice.String())
	}
	if !choice.IsObject() {
		return false
	}
	choiceType := strings.TrimSpace(choice.Get("type").String())
	if isOpenAIImageGenerationType(choiceType) {
		return true
	}
	if choiceType == "namespace" && (isOpenAIImageGenNamespaceName(choice.Get("name").String()) || isOpenAIImageGenNamespaceName(choice.Get("namespace").String())) {
		return true
	}
	if tool := choice.Get("tool"); tool.IsObject() && openAIJSONToolChoiceSelectsImageGeneration(tool) {
		return true
	}
	if isOpenAIImageGenerationType(choice.Get("function.name").String()) {
		return true
	}
	return false
}

// openAIJSONToolChoiceSelectsExplicitImageGeneration 识别客户端对图片工具的显式选择。
// 除标准 image_generation 外，也兼容 Codex 对 image_gen.imagegen 的函数引用形式。
func openAIJSONToolChoiceSelectsExplicitImageGeneration(choice gjson.Result) bool {
	if openAIJSONToolChoiceSelectsImageGeneration(choice) {
		return true
	}
	if !choice.IsObject() {
		return false
	}
	if tool := choice.Get("tool"); tool.IsObject() && openAIJSONToolChoiceSelectsExplicitImageGeneration(tool) {
		return true
	}
	if isOpenAIImageGenFunctionReference(choice.Get("namespace").String(), choice.Get("name").String()) {
		return true
	}
	if function := choice.Get("function"); function.IsObject() {
		return isOpenAIImageGenFunctionReference(function.Get("namespace").String(), function.Get("name").String())
	}
	return false
}

// isOpenAIImageGenFunctionReference 判断函数式 tool_choice 是否指定 Codex 图片工具。
func isOpenAIImageGenFunctionReference(namespace string, name string) bool {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "image_gen" && name == "imagegen" {
		return true
	}
	switch name {
	case "image_gen.imagegen", "image_gen__imagegen":
		return true
	default:
		return false
	}
}

func openAIAnyToolChoiceSelectsImageGeneration(choice any) bool {
	switch v := choice.(type) {
	case string:
		return isOpenAIImageGenerationType(v)
	case map[string]any:
		choiceType := strings.TrimSpace(firstNonEmptyString(v["type"]))
		if isOpenAIImageGenerationType(choiceType) {
			return true
		}
		if choiceType == "namespace" && (isOpenAIImageGenNamespaceName(firstNonEmptyString(v["name"])) || isOpenAIImageGenNamespaceName(firstNonEmptyString(v["namespace"]))) {
			return true
		}
		if tool, ok := v["tool"].(map[string]any); ok && openAIAnyToolChoiceSelectsImageGeneration(tool) {
			return true
		}
		if fn, ok := v["function"].(map[string]any); ok && isOpenAIImageGenerationType(firstNonEmptyString(fn["name"])) {
			return true
		}
	}
	return false
}

func getAPIKeyFromContext(c interface{ Get(string) (any, bool) }) *APIKey {
	if c == nil {
		return nil
	}
	v, exists := c.Get("api_key")
	if !exists {
		return nil
	}
	apiKey, _ := v.(*APIKey)
	return apiKey
}

func apiKeyGroup(apiKey *APIKey) *Group {
	if apiKey == nil {
		return nil
	}
	return apiKey.Group
}

func cloneRequestMapForImageIntent(body []byte) map[string]any {
	if len(body) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil
	}
	return out
}

type OpenAIResponsesImageBillingConfig struct {
	Model     string
	SizeTier  string
	InputSize string
}

func resolveOpenAIResponsesImageBillingConfigDetailed(reqBody map[string]any, fallbackModel string) (OpenAIResponsesImageBillingConfig, error) {
	imageModel := ""
	imageSize := ""
	hasImageTool := false
	if reqBody != nil {
		rawTools, _ := reqBody["tools"].([]any)
		for _, rawTool := range rawTools {
			toolMap, ok := rawTool.(map[string]any)
			if !ok || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "image_generation" {
				continue
			}
			hasImageTool = true
			imageModel = strings.TrimSpace(firstNonEmptyString(toolMap["model"]))
			imageSize = strings.TrimSpace(firstNonEmptyString(toolMap["size"]))
			break
		}
		if imageSize == "" {
			imageSize = strings.TrimSpace(firstNonEmptyString(reqBody["size"]))
		}
	}
	if imageModel == "" && reqBody != nil {
		bodyModel := strings.TrimSpace(firstNonEmptyString(reqBody["model"]))
		if isOpenAIImageBillingModelAlias(bodyModel) || !hasImageTool {
			imageModel = bodyModel
		}
	}
	if imageModel == "" && hasImageTool {
		imageModel = "gpt-image-2"
	}
	if imageModel == "" {
		imageModel = strings.TrimSpace(fallbackModel)
	}
	sizeTier := normalizeOpenAIImageSizeTier(imageSize)
	return OpenAIResponsesImageBillingConfig{
		Model:     imageModel,
		SizeTier:  sizeTier,
		InputSize: imageSize,
	}, nil
}

func resolveOpenAIResponsesImageBillingConfigFromBody(body []byte, fallbackModel string) (string, string, error) {
	cfg, err := resolveOpenAIResponsesImageBillingConfigDetailedFromBody(body, fallbackModel)
	if err != nil {
		return "", "", err
	}
	return cfg.Model, cfg.SizeTier, nil
}

func resolveOpenAIResponsesImageBillingConfigDetailedFromBody(body []byte, fallbackModel string) (OpenAIResponsesImageBillingConfig, error) {
	reqBody := cloneRequestMapForImageIntent(body)
	if reqBody != nil {
		return resolveOpenAIResponsesImageBillingConfigDetailed(reqBody, fallbackModel)
	}
	return resolveOpenAIResponsesImageBillingConfigDetailedFromRawBody(body, fallbackModel), nil
}

func resolveOpenAIResponsesImageBillingConfigDetailedFromRawBody(body []byte, fallbackModel string) OpenAIResponsesImageBillingConfig {
	imageModel := ""
	imageSize := ""
	hasImageTool := false
	if len(body) > 0 && gjson.ValidBytes(body) {
		root := gjson.ParseBytes(body)
		root.Get("tools").ForEach(func(_, item gjson.Result) bool {
			if strings.TrimSpace(item.Get("type").String()) != "image_generation" {
				return true
			}
			hasImageTool = true
			imageModel = strings.TrimSpace(item.Get("model").String())
			imageSize = strings.TrimSpace(item.Get("size").String())
			return false
		})
		if imageSize == "" {
			imageSize = strings.TrimSpace(root.Get("size").String())
		}
		if imageModel == "" {
			bodyModel := strings.TrimSpace(root.Get("model").String())
			if isOpenAIImageBillingModelAlias(bodyModel) || !hasImageTool {
				imageModel = bodyModel
			}
		}
	}
	if imageModel == "" && hasImageTool {
		imageModel = "gpt-image-2"
	}
	if imageModel == "" {
		imageModel = strings.TrimSpace(fallbackModel)
	}
	return OpenAIResponsesImageBillingConfig{
		Model:     imageModel,
		SizeTier:  normalizeOpenAIImageSizeTier(imageSize),
		InputSize: imageSize,
	}
}

func isOpenAIImageBillingModelAlias(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return false
	}
	return isOpenAIImageGenerationModel(normalized) || strings.Contains(normalized, "image")
}
