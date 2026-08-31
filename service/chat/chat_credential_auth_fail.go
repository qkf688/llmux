package chat

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// cooldownReasonAuthFail 鉴权失败判停 reason 机器码：判停时写入 CooldownReason 字段
// （字段复用：temp_unsched 在 UI 归入「错误」显示，详情行靠此机器码区分——
// 与人工 error 的区分见 models.CredentialStatusTempUnsched 注释）。
// 与 cooldownReason429 / cooldownReasonNetwork 并列的 reason 单源常量，
// 探活恢复路径（#6-3）消费同一常量。
const cooldownReasonAuthFail = "auth_fail"

// classifyAuthFailure 判定是否鉴权失败（401/403）：凭据级失败中走判停路径的子类
// （#6-2）——不走冷却，改为连续失败计数，达阈值判停 temp_unsched。
// 密钥失效重试无意义：冷却循环只会推迟判停（且到期恢复后照样 401）。
func classifyAuthFailure(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// applyCredentialAuthFailure 处理一次鉴权失败（401/403）：FailCount 经 SQL 原子自增
// （防并发 401 丢计数——读改写在并发下会漏加），读回达阈值 N（可配设置项）则判停：
// Status=temp_unsched + reason=auth_fail + 清 cooldown_until（判停前若有冷却残留，
// temp_unsched 的排除不依赖冷却字段，但清掉避免 UI 误显示冷却态）。
// 判停凭据被 credentialUsable 排除（非 active 即剔除）；恢复走探活（#6-3）或人工改回。
// 自增与读回非原子：并发下可能第 N+1 次才判停，可接受偏差（判停不需精确到 N；
// #6-4 异步落库收敛时统一处理写路径）。穿插的 429/5xx 冷却不清零计数——只有
// 成功请求重置（见 executeSingleProviderAttempt 成功出口）。
// 写库失败仅告警不阻断（与 applyCredentialCooldown 同策略）。
func applyCredentialAuthFailure(ctx context.Context, cred models.Credential) {
	if cred.ID == 0 {
		// Selection 里凭据为空（测试直调 attempt 的构造形态），无行可计数，
		// 与 applyCredentialCooldown 同款防御壳。
		return
	}
	if _, err := repos().Credential.UpdateFields(ctx, cred.ID, map[string]any{
		"fail_count": gorm.Expr("fail_count + 1"),
	}); err != nil {
		slog.Warn("failed to increment credential fail count", "credential_id", cred.ID, "error", err)
		return
	}
	updated, err := repos().Credential.Get(ctx, cred.ID)
	if err != nil {
		slog.Warn("failed to reload credential for auth-fail threshold check", "credential_id", cred.ID, "error", err)
		return
	}
	threshold := getCredHealthAuthFailThreshold(ctx)
	if threshold < 1 {
		// 防御：设置写入侧已 Min=1 校验，此处兜底绕过 API 的脏值（0 会让首败即判停）。
		threshold = 1
	}
	if updated.FailCount < threshold {
		return
	}
	if _, err := repos().Credential.UpdateFields(ctx, cred.ID, map[string]any{
		"status":          models.CredentialStatusTempUnsched,
		"cooldown_reason": cooldownReasonAuthFail,
		"cooldown_until":  nil,
	}); err != nil {
		slog.Warn("failed to stop credential after consecutive auth failures",
			"credential_id", cred.ID, "fail_count", updated.FailCount, "error", err)
	}
}

// resetCredentialAuthFailCount 成功请求后重置凭据的鉴权失败计数（#6-2 状态机的
// 成功侧出口，与 applyCredentialAuthFailure 的失败侧对称：「连续」失败因成功而中断）。
// 仅当快照计数 > 0 时才写库：成功是热路径，FailCount=0 的常态请求不应产生任何
// 凭据写（防写放大；#6-4 异步落库前的必要约束）。只写 fail_count 单字段——
// 与判停写的 status 字段分离，成功不会误清 temp_unsched 状态。快照计数可能过期
// （并发下判停先落），漏一次重置由下次成功补上，可接受。
func resetCredentialAuthFailCount(ctx context.Context, cred models.Credential) {
	if cred.ID == 0 || cred.FailCount <= 0 {
		return
	}
	if _, err := repos().Credential.UpdateFields(ctx, cred.ID, map[string]any{
		"fail_count": 0,
	}); err != nil {
		slog.Warn("failed to reset credential fail count", "credential_id", cred.ID, "error", err)
	}
}
