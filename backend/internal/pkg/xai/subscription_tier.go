package xai

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
)

// DecodeJWTClaims 解码 JWT payload；仅用于读取上游声明，不做签名校验。
func DecodeJWTClaims(token string) map[string]any {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var claims map[string]any
	if err := decoder.Decode(&claims); err != nil {
		return nil
	}
	return claims
}

// JWTClaimString 读取并清理 JWT 字符串 claim。
func JWTClaimString(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return strings.TrimSpace(value)
}

// MapJWTSubscriptionTier 将 xAI 数字档位转换为稳定的内部标识。
func MapJWTSubscriptionTier(tier uint64) string {
	switch tier {
	case 0:
		return "free"
	case 1:
		return "supergrok"
	case 2:
		return "x_basic"
	case 3:
		return "x_premium"
	case 4:
		return "x_premium_plus"
	case 5:
		return "supergrok_heavy"
	case 6:
		return "supergrok_lite"
	case 7:
		return "supergrok_plus"
	default:
		return strconv.FormatUint(tier, 10)
	}
}

// NormalizeSubscriptionTier 统一管理端、响应头和 JWT 中的档位别名。
func NormalizeSubscriptionTier(raw string) string {
	tier := strings.ToLower(strings.TrimSpace(raw))
	tier = strings.ReplaceAll(tier, "-", "_")
	tier = strings.Join(strings.Fields(tier), "_")
	switch tier {
	case "free", "grok_free", "grokfree", "free_tier", "freetier", "grok_basic", "grokbasic":
		return "free"
	case "supergrok", "grokpro":
		return "supergrok"
	case "supergrok_lite", "supergroklite":
		return "supergrok_lite"
	case "supergrok_heavy", "supergrokheavy":
		return "supergrok_heavy"
	case "supergrok_pro", "supergrokpro":
		return "supergrok_pro"
	case "supergrok_plus", "supergrokplus":
		return "supergrok_plus"
	case "x_basic", "xbasic", "basic":
		return "x_basic"
	case "x_premium", "xpremium":
		return "x_premium"
	case "x_premium_plus", "xpremiumplus", "x_premium+":
		return "x_premium_plus"
	default:
		return tier
	}
}

// SubscriptionTierFromJWT 从 access token 的 tier claim 读取当前订阅档位。
func SubscriptionTierFromJWT(token string) string {
	claims := DecodeJWTClaims(token)
	if claims == nil {
		return ""
	}
	raw, ok := claims["tier"]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case json.Number:
		number, err := value.Int64()
		if err != nil || number < 0 {
			return NormalizeSubscriptionTier(value.String())
		}
		return MapJWTSubscriptionTier(uint64(number))
	case float64:
		if value < 0 {
			return ""
		}
		return MapJWTSubscriptionTier(uint64(value))
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return ""
		}
		if number, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
			return MapJWTSubscriptionTier(number)
		}
		return NormalizeSubscriptionTier(trimmed)
	default:
		return ""
	}
}
