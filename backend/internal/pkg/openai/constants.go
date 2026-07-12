// Package openai provides helpers and types for OpenAI API integration.
package openai

import (
	_ "embed"
	"strings"
)

// Model represents an OpenAI model
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// DefaultModels OpenAI models list
var DefaultModels = []Model{
	{ID: "gpt-5.6", Object: "model", Created: 1783382400, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 (Sol)"},
	{ID: "gpt-5.6-sol", Object: "model", Created: 1783382400, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Sol"},
	{ID: "gpt-5.6-terra", Object: "model", Created: 1783382400, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Terra"},
	{ID: "gpt-5.6-luna", Object: "model", Created: 1783382400, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Luna"},
	{ID: "gpt-5.5", Object: "model", Created: 1776873600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.5"},
	{ID: "gpt-5.4", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4"},
	{ID: "gpt-5.4-mini", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4 Mini"},
	{ID: "gpt-5.3-codex", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.3 Codex"},
	{ID: "gpt-5.3-codex-spark", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.3 Codex Spark"},
	{ID: "codex-auto-review", Object: "model", Created: 1776902400, OwnedBy: "openai", Type: "model", DisplayName: "Codex Auto Review"},
	{ID: "gpt-5.2", Object: "model", Created: 1733875200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.2"},
	{ID: "gpt-image-1", Object: "model", Created: 1733875200, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 1"},
	{ID: "gpt-image-1.5", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 1.5"},
	{ID: "gpt-image-2", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 2"},
}

// DefaultModelIDs returns the default model ID list
func DefaultModelIDs() []string {
	ids := make([]string, len(DefaultModels))
	for i, m := range DefaultModels {
		ids[i] = m.ID
	}
	return ids
}

// DefaultTestModel 是 OpenAI 账号测试、体检和快照探测使用的默认模型。
const DefaultTestModel = "gpt-5.6-terra"

// DefaultPricingFallbackModel 仅用于未知 OpenAI 模型的计费兜底，不能跟随
// 探测默认模型变更，否则会意外改变正常请求的费用计算。
const DefaultPricingFallbackModel = "gpt-5.5"

// DefaultInstructions default instructions for non-Codex CLI requests.
// 内容为 Codex 系模型默认使用的真实 Codex base prompt。
//
//go:embed instructions.txt
var DefaultInstructions string

// instructionsGPT51 / instructionsGPT52 / instructionsGPT55 保存非 codex GPT-5.x 模型对应的真实 Codex CLI base prompt。
//
//go:embed instructions_gpt5_1.txt
var instructionsGPT51 string

//go:embed instructions_gpt5_2.txt
var instructionsGPT52 string

//go:embed instructions_gpt5_5.txt
var instructionsGPT55 string

// latestCodexInstructions 返回当前已知最新版本的 Codex base instructions。
func latestCodexInstructions() string {
	if strings.TrimSpace(instructionsGPT55) != "" {
		return instructionsGPT55
	}
	return DefaultInstructions
}

// CodexBaseInstructionsForModel 按模型选择最贴近真实 Codex CLI 的 base prompt。
func CodexBaseInstructionsForModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "codex"):
		return DefaultInstructions
	case strings.HasPrefix(m, "gpt-5.6"):
		return latestCodexInstructions()
	case strings.HasPrefix(m, "gpt-5.5"):
		return latestCodexInstructions()
	case strings.HasPrefix(m, "gpt-5.2"):
		if strings.TrimSpace(instructionsGPT52) != "" {
			return instructionsGPT52
		}
	case strings.HasPrefix(m, "gpt-5.1"):
		if strings.TrimSpace(instructionsGPT51) != "" {
			return instructionsGPT51
		}
	}
	return latestCodexInstructions()
}
