package service

import (
	"context"
	"strings"
)

// 注册邮箱别名归一化用于重复账号检测。
//
// 旧注册检查只比较小写并去除首尾空白后的邮箱字面量，同一真实收件箱仍可通过以下
// 服务商别名能力注册多个账号：
//   - 加号寻址：user+tag@gmail.com 与 user@gmail.com 投递到同一收件箱；
//   - Gmail 点号别名：u.s.e.r@gmail.com 与 user@gmail.com 等价；
//   - FQDN 根点：user@gmail.com. 是 user@gmail.com 的绝对域名形式。
//
// 域名白名单和邮件验证会把这些变体视为可投递的不同地址，因此这里将它们归并为
// 同一“收件箱身份”，供注册路径拒绝重复账号。
//
// 归一化规则：
//   - 所有域名统一小写、去除首尾空白和 FQDN 根点，并移除本地部分的“+后缀”；
//   - Gmail 家族（gmail.com / googlemail.com）额外移除本地部分点号，并统一为 gmail.com。
//
// 该规则只影响注册查重，不改变邮箱的存储、展示、登录或投递行为。

var gmailFamilyDomains = map[string]struct{}{
	"gmail.com":      {},
	"googlemail.com": {},
}

// NormalizeEmailForAliasDedup 返回邮箱对应的规范收件箱身份。格式异常时只做
// 小写和首尾空白清理，格式合法性仍由调用方负责。
func NormalizeEmailForAliasDedup(email string) string {
	local, domain, ok := splitEmailForAliasDedup(email)
	if !ok {
		return strings.ToLower(strings.TrimSpace(email))
	}
	local = stripEmailPlusSuffix(local)
	if isGmailFamilyDomain(domain) {
		local = stripEmailLocalDots(local)
		domain = "gmail.com"
	}
	return local + "@" + domain
}

// EmailAliasProbe 描述存量重复地址可能呈现的查询形态。Local 是去除加号后缀和
// 点号后的本地部分，Domain 是去除点号后的候选域名。仓储查询两侧均去点，使单个
// 域名探针同时覆盖 Gmail 点号别名和 FQDN 根点；非 Gmail 域名可能产生的过度匹配
// 会由 NormalizeEmailForAliasDedup 对候选结果再次过滤。
type EmailAliasProbe struct {
	Local  string
	Domain string
}

// EmailAliasDedupProbes 返回覆盖同一收件箱所有存量地址形态的探针。Gmail 家族域名
// 互为别名，其他域名只与自身比较；邮箱格式异常或本地部分去点后为空时返回 nil。
func EmailAliasDedupProbes(email string) []EmailAliasProbe {
	local, domain, ok := splitEmailForAliasDedup(email)
	if !ok {
		return nil
	}
	probeLocal := strings.ReplaceAll(stripEmailPlusSuffix(local), ".", "")
	if probeLocal == "" {
		return nil
	}
	domains := []string{domain}
	if isGmailFamilyDomain(domain) {
		domains = []string{"gmail.com", "googlemail.com"}
	}
	probes := make([]EmailAliasProbe, 0, len(domains))
	for _, candidate := range domains {
		probes = append(probes, EmailAliasProbe{
			Local:  probeLocal,
			Domain: strings.ReplaceAll(candidate, ".", ""),
		})
	}
	return probes
}

func splitEmailForAliasDedup(email string) (local string, domain string, ok bool) {
	local, domain, ok = splitEmailForPolicy(email)
	if !ok {
		return "", "", false
	}
	domain = strings.TrimRight(domain, ".")
	if domain == "" {
		return "", "", false
	}
	return local, domain, true
}

func stripEmailPlusSuffix(local string) string {
	// 仅处理 idx > 0；"+tag@host" 没有可保留的本地部分，若统一折叠为 "@host"，
	// 会把该域名下互不相关的地址错误归并。
	if idx := strings.IndexByte(local, '+'); idx > 0 {
		return local[:idx]
	}
	return local
}

func stripEmailLocalDots(local string) string {
	if stripped := strings.ReplaceAll(local, ".", ""); stripped != "" {
		return stripped
	}
	return local
}

func isGmailFamilyDomain(domain string) bool {
	_, ok := gmailFamilyDomains[domain]
	return ok
}

// existsByEmailOrAlias 判断邮箱或指向同一收件箱的别名是否已注册。先执行精确邮箱
// 查询，仅在未命中时查询别名；查询错误直接上抛，使注册路径失败退出，不能通过
// 人为制造查询错误绕过重复检查。
func (s *AuthService) existsByEmailOrAlias(ctx context.Context, email string) (bool, error) {
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil || exists {
		return exists, err
	}
	return s.userRepo.ExistsByEmailAlias(ctx, email)
}
