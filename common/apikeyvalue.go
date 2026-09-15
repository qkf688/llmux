package common

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// 调用方 API key（api_keys 表）的值规则。多 api-key 方案 §5 定案：
// 缺前缀自动补、最终值最短 16 字符（防手滑）、可整体自定义。
// 放 common 是因为 handler/apikeys（管理创建）与 service/auth（bootstrap 首把
// default key）两个域共用同一规则（AGENTS 4.4：跨 2 域复用下沉）。
const (
	// CallerKeyPrefix 自动补全与随机生成统一使用的 key 前缀。
	CallerKeyPrefix = "sk-llmux-"
	// MinCallerKeyValueLen 是补前缀后最终值的最小长度。
	MinCallerKeyValueLen = 16
	// callerKeyRandomHexLen 随机生成体的 hex 字符数（16 随机字节）。
	callerKeyRandomHexLen = 32
)

// NormalizeCallerKeyValue 规范化用户提交的 key 明文：裁首尾空白 → 缺前缀补
// CallerKeyPrefix → 校验最终长度 ≥MinCallerKeyValueLen。
// ok=false 表示应拒绝（空白值或补前缀后仍过短）。空输入返回 ("", false)，
// 「省略值→随机生成」是调用方的另一分支，不在本函数语义内。
func NormalizeCallerKeyValue(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if !strings.HasPrefix(value, CallerKeyPrefix) {
		value = CallerKeyPrefix + value
	}
	if len(value) < MinCallerKeyValueLen {
		return "", false
	}
	return value, true
}

// GenerateCallerKeyValue 生成一把合规随机 key：CallerKeyPrefix + 32 位 hex。
// 产物恒满足 NormalizeCallerKeyValue 的契约。
func GenerateCallerKeyValue() (string, error) {
	buf := make([]byte, callerKeyRandomHexLen/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return CallerKeyPrefix + hex.EncodeToString(buf), nil
}
