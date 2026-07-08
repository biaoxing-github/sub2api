package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	tokenCostDefaultVersion           = 1
	tokenCostDefaultRankMode          = "plus"
	tokenCostDefaultPersonalRechargeR = 400
)

// TokenCostOptions 描述 token 成本计算器服务的运行依赖。
type TokenCostOptions struct {
	// Repository 是 token 成本计算器的 SQL 持久化端口。
	Repository TokenCostRepository
	// Now 返回当前时间，测试中用于固定 updatedAt。
	Now func() time.Time
}

// TokenCostService 承载 token 成本计算器的状态读写与汇总计算。
type TokenCostService struct {
	repo TokenCostRepository
	now  func() time.Time
}

// TokenCostRepository 定义 token 成本计算器的持久化端口。
type TokenCostRepository interface {
	// LoadState 读取当前计算器状态；found=false 表示数据库尚未初始化。
	LoadState(ctx context.Context) (TokenCostState, bool, error)
	// SaveState 保存当前计算器完整状态。
	SaveState(ctx context.Context, state TokenCostState) error
	// StorageLabel 返回页面健康检查中展示的存储位置说明。
	StorageLabel() string
}

// TokenCostState 是前端 live calculator 的完整可编辑状态。
type TokenCostState struct {
	// Version 是状态结构版本。
	Version int `json:"version"`
	// UpdatedAt 是最后一次页面或服务端更新状态的 ISO 时间。
	UpdatedAt string `json:"updatedAt"`
	// RankMode 是页面排行榜当前视图，取值通常为 plus 或 pro。
	RankMode string `json:"rankMode"`
	// PersonalRechargeR 是个人充值总价，单位 r。
	PersonalRechargeR float64 `json:"personalRechargeR"`
	// Platforms 是每个平台的余额、成本和倍率配置。
	Platforms []TokenCostPlatform `json:"platforms"`
	// History 是用户主动记录的余额与购买力历史点。
	History []TokenCostHistoryEntry `json:"history"`
	// Events 是平台编辑、删除、历史记录等可见事件流水。
	Events []TokenCostEvent `json:"events"`
}

// TokenCostPlatform 是单个平台的成本与购买力配置。
type TokenCostPlatform struct {
	// ID 是前端行编辑和删除使用的稳定标识。
	ID string `json:"id"`
	// Name 是平台展示名称。
	Name string `json:"name"`
	// BalanceUSD 是平台账面美元余额。
	BalanceUSD float64 `json:"balanceUsd"`
	// CalcBalanceUSD 是特殊平台的折算美元余额；为空时使用 BalanceUSD。
	CalcBalanceUSD *float64 `json:"calcBalanceUsd"`
	// RateR 是兑换成本分子，表示 rateR r。
	RateR float64 `json:"rateR"`
	// RateUSD 是兑换成本分母，表示 rateUSD 美元。
	RateUSD float64 `json:"rateUsd"`
	// Plus 是 plus 池倍率；为空表示不参与 plus 购买力。
	Plus *float64 `json:"plus"`
	// ProMin 是 pro 池低口径倍率；为空表示不参与 pro 购买力。
	ProMin *float64 `json:"proMin"`
	// ProMax 是 pro 池高口径倍率；为空时等同 ProMin。
	ProMax *float64 `json:"proMax"`
	// Note 是页面中展示的人工说明。
	Note string `json:"note"`
}

// TokenCostHistoryEntry 是历史表中的一个汇总点。
type TokenCostHistoryEntry struct {
	// At 是历史点日期或时间文本。
	At string `json:"at"`
	// Summary 是这次更新的用户可读摘要。
	Summary string `json:"summary"`
	// TotalBalance 是该历史点的总余额。
	TotalBalance float64 `json:"totalBalance"`
	// ProMin 是该历史点的 pro 购买力下界。
	ProMin *float64 `json:"proMin"`
	// ProMax 是该历史点的 pro 购买力上界。
	ProMax *float64 `json:"proMax"`
	// Plus 是该历史点的 plus 购买力。
	Plus *float64 `json:"plus"`
}

// TokenCostEvent 是页面右侧事件流水的一条记录。
type TokenCostEvent struct {
	// At 是事件发生时间文本。
	At string `json:"at"`
	// Title 是事件类型标题。
	Title string `json:"title"`
	// Detail 是事件详情。
	Detail string `json:"detail"`
}

// TokenCostTotals 是页面顶部卡片和性价比排行使用的汇总结果。
type TokenCostTotals struct {
	// ActiveCount 是余额大于 0 的平台数。
	ActiveCount int `json:"activeCount"`
	// TotalBalance 是活跃平台账面余额合计。
	TotalBalance float64 `json:"totalBalance"`
	// ComparableBalance 是折算后余额合计。
	ComparableBalance float64 `json:"comparableBalance"`
	// BookCost 是按各平台兑换比例计算出的账面 r 成本。
	BookCost float64 `json:"bookCost"`
	// ComparableCost 当前与源页面一致，使用账面成本作为性价比分母。
	ComparableCost float64 `json:"comparableCost"`
	// PersonalRechargeR 是个人充值总价。
	PersonalRechargeR float64 `json:"personalRechargeR"`
	// PlusPower 是 plus 池标准美元购买力。
	PlusPower float64 `json:"plusPower"`
	// ProPowerMin 是 pro 池购买力下界。
	ProPowerMin float64 `json:"proPowerMin"`
	// ProPowerMax 是 pro 池购买力上界。
	ProPowerMax float64 `json:"proPowerMax"`
	// BookMultiple 是账面成本相对个人充值的倍数。
	BookMultiple float64 `json:"bookMultiple"`
	// PlusPerR 是每 1r 对应的 plus 标准美元购买力。
	PlusPerR float64 `json:"plusPerR"`
	// ProPerRMin 是每 1r 对应的 pro 标准美元购买力下界。
	ProPerRMin float64 `json:"proPerRMin"`
	// ProPerRMax 是每 1r 对应的 pro 标准美元购买力上界。
	ProPerRMax float64 `json:"proPerRMax"`
}

// TokenCostHealth 是计算器健康检查响应。
type TokenCostHealth struct {
	// OK 表示服务与存储端口可用。
	OK bool `json:"ok"`
	// State 是兼容旧页面的存储位置说明。
	State string `json:"state"`
	// Storage 是当前使用的持久化后端。
	Storage string `json:"storage"`
}

// NewTokenCostService 创建 token 成本计算器服务。
func NewTokenCostService(options TokenCostOptions) *TokenCostService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &TokenCostService{
		repo: options.Repository,
		now:  now,
	}
}

// ProvideTokenCostService 使用 SQL 仓储创建 token 成本计算器服务。
func ProvideTokenCostService(repo TokenCostRepository) *TokenCostService {
	return NewTokenCostService(TokenCostOptions{Repository: repo})
}

// GetState 读取当前状态，数据库为空时返回页面可用的默认状态。
func (s *TokenCostService) GetState(ctx context.Context) (TokenCostState, error) {
	if s == nil || s.repo == nil {
		return TokenCostState{}, errors.New("token cost repository is not configured")
	}
	state, found, err := s.repo.LoadState(ctx)
	if err != nil {
		return TokenCostState{}, err
	}
	if !found {
		return normalizeTokenCostState(TokenCostState{}), nil
	}
	return normalizeTokenCostState(state), nil
}

// SaveState 规范化并保存页面提交的完整状态。
func (s *TokenCostService) SaveState(ctx context.Context, state TokenCostState) (TokenCostState, error) {
	if s == nil || s.repo == nil {
		return TokenCostState{}, errors.New("token cost repository is not configured")
	}
	normalized := normalizeTokenCostState(state)
	if strings.TrimSpace(normalized.UpdatedAt) == "" {
		normalized.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	}
	if err := s.repo.SaveState(ctx, normalized); err != nil {
		return TokenCostState{}, err
	}
	return normalized, nil
}

// Health 返回兼容旧计算器页面的健康检查信息。
func (s *TokenCostService) Health() TokenCostHealth {
	storage := ""
	if s != nil && s.repo != nil {
		storage = s.repo.StorageLabel()
	}
	return TokenCostHealth{OK: storage != "", State: storage, Storage: storage}
}

// CalculateTokenCostTotals 按源页面公式计算余额、成本和 plus/pro 购买力。
func CalculateTokenCostTotals(state TokenCostState) TokenCostTotals {
	state = normalizeTokenCostState(state)
	totals := TokenCostTotals{PersonalRechargeR: state.PersonalRechargeR}
	for _, platform := range state.Platforms {
		if platform.BalanceUSD <= 0 {
			continue
		}
		totals.ActiveCount++
		totals.TotalBalance += platform.BalanceUSD
		totals.ComparableBalance += tokenCostCalcBalance(platform)
		totals.BookCost += platform.BalanceUSD * tokenCostRPerUSD(platform)
		if plus := tokenCostPoolMetrics(platform, platform.Plus, nil); plus.ok {
			totals.PlusPower += plus.powerMax
		}
		if pro := tokenCostPoolMetrics(platform, platform.ProMin, platform.ProMax); pro.ok {
			totals.ProPowerMin += pro.powerMin
			totals.ProPowerMax += pro.powerMax
		}
	}
	totals.ComparableCost = totals.BookCost
	if totals.PersonalRechargeR > 0 {
		totals.BookMultiple = totals.BookCost / totals.PersonalRechargeR
	}
	if totals.ComparableCost > 0 {
		totals.PlusPerR = totals.PlusPower / totals.ComparableCost
		totals.ProPerRMin = totals.ProPowerMin / totals.ComparableCost
		totals.ProPerRMax = totals.ProPowerMax / totals.ComparableCost
	}
	return totals
}

func normalizeTokenCostState(state TokenCostState) TokenCostState {
	if state.Version <= 0 {
		state.Version = tokenCostDefaultVersion
	}
	if strings.TrimSpace(state.RankMode) == "" {
		state.RankMode = tokenCostDefaultRankMode
	}
	if state.PersonalRechargeR <= 0 {
		state.PersonalRechargeR = tokenCostDefaultPersonalRechargeR
	}
	if state.Platforms == nil {
		state.Platforms = []TokenCostPlatform{}
	}
	for index := range state.Platforms {
		state.Platforms[index] = normalizeTokenCostPlatform(state.Platforms[index], index)
	}
	if state.History == nil {
		state.History = []TokenCostHistoryEntry{}
	}
	if state.Events == nil {
		state.Events = []TokenCostEvent{}
	}
	return state
}

func normalizeTokenCostPlatform(platform TokenCostPlatform, index int) TokenCostPlatform {
	platform.ID = strings.TrimSpace(platform.ID)
	platform.Name = strings.TrimSpace(platform.Name)
	if platform.Name == "" {
		platform.Name = "未命名平台"
	}
	if platform.ID == "" {
		platform.ID = fmt.Sprintf("platform-%d", index+1)
	}
	if platform.RateR <= 0 {
		platform.RateR = 1
	}
	if platform.RateUSD <= 0 {
		platform.RateUSD = 1
	}
	return platform
}

func tokenCostCalcBalance(platform TokenCostPlatform) float64 {
	if platform.CalcBalanceUSD != nil {
		return *platform.CalcBalanceUSD
	}
	return platform.BalanceUSD
}

func tokenCostRPerUSD(platform TokenCostPlatform) float64 {
	if platform.RateUSD <= 0 {
		return 0
	}
	return platform.RateR / platform.RateUSD
}

type tokenCostPoolResult struct {
	ok       bool
	powerMin float64
	powerMax float64
}

func tokenCostPoolMetrics(platform TokenCostPlatform, multiplier *float64, multiplierMax *float64) tokenCostPoolResult {
	if multiplier == nil || *multiplier <= 0 {
		return tokenCostPoolResult{}
	}
	lowMultiplier := *multiplier
	highMultiplier := *multiplier
	if multiplierMax != nil && *multiplierMax > 0 {
		highMultiplier = *multiplierMax
	}
	minMultiplier := lowMultiplier
	maxMultiplier := highMultiplier
	if minMultiplier > maxMultiplier {
		minMultiplier, maxMultiplier = maxMultiplier, minMultiplier
	}
	basis := tokenCostCalcBalance(platform)
	return tokenCostPoolResult{
		ok:       true,
		powerMin: basis / maxMultiplier,
		powerMax: basis / minMultiplier,
	}
}
