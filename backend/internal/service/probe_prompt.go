package service

import "math/rand/v2"

// quickValidationPrompts 保存低 token、答案明确且无需外部知识的快速验证题。
var quickValidationPrompts = []string{
	"只回答 7+8 的结果，不要解释。",
	"只回答英文单词 blue 的大写形式。",
	"将 9、3、6 从小到大排列，只用逗号分隔。",
	"只回答一年有多少个月，不要解释。",
	"只回答字符串 abc 的反转结果。",
	"只回答 20 除以 4 的结果，不要解释。",
	"只回答星期一之后是哪一天，不要解释。",
	"只回答 5、8、2 中最大的数字。",
}

// RandomQuickValidationPrompt 随机返回一道适合连通性和可用性探测的快速验证题。
func RandomQuickValidationPrompt() string {
	return quickValidationPromptAt(rand.IntN(len(quickValidationPrompts)))
}

// quickValidationPromptAt 提供可重复的题库索引读取，供测试验证完整题库。
func quickValidationPromptAt(index int) string {
	return quickValidationPrompts[index]
}
