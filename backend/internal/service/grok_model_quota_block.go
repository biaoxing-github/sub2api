package service

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// grokModelQuotaBlock 保存单个 Grok 账号在某模型上的短期不可用时间。
type grokModelQuotaBlock struct {
	until time.Time
}

var globalGrokModelQuotaBlocks = struct {
	sync.Mutex
	items map[string]grokModelQuotaBlock
}{items: make(map[string]grokModelQuotaBlock)}

const grokModelCapacityBlockTTL = 3 * time.Minute

// markGrokModelCapacityBlocked 只阻止指定账号的指定模型，避免容量抖动下线同账号其他模型。
func markGrokModelCapacityBlocked(accountID int64, model string, now time.Time) {
	model = strings.ToLower(strings.TrimSpace(model))
	if accountID <= 0 || model == "" {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	globalGrokModelQuotaBlocks.Lock()
	defer globalGrokModelQuotaBlocks.Unlock()
	globalGrokModelQuotaBlocks.items[grokModelQuotaBlockKey(accountID, model)] = grokModelQuotaBlock{until: now.Add(grokModelCapacityBlockTTL)}
	for key, block := range globalGrokModelQuotaBlocks.items {
		if !block.until.After(now) {
			delete(globalGrokModelQuotaBlocks.items, key)
		}
	}
}

// isGrokModelCapacityBlocked 返回账号是否正在该模型的容量冷却窗口内。
func isGrokModelCapacityBlocked(accountID int64, model string, now time.Time) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if accountID <= 0 || model == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	globalGrokModelQuotaBlocks.Lock()
	defer globalGrokModelQuotaBlocks.Unlock()
	key := grokModelQuotaBlockKey(accountID, model)
	block, ok := globalGrokModelQuotaBlocks.items[key]
	if !ok {
		return false
	}
	if !block.until.After(now) {
		delete(globalGrokModelQuotaBlocks.items, key)
		return false
	}
	return true
}

// grokModelQuotaBlockKey 生成稳定的账号和模型隔离键。
func grokModelQuotaBlockKey(accountID int64, model string) string {
	return model + "|" + strconv.FormatInt(accountID, 10)
}
