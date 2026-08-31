package chat

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
)

// cooldownReasonAuthFail 鉴权失败判停 reason 机器码：判停时写入 CooldownReason 字段
// （字段复用：temp_unsched 在 UI 归入「错误」显示，详情行靠此机器码区分——UI 方案
// 预留，S6 接真实凭据 API 时落地；与人工 error 的区分见 CredentialStatusTempUnsched 注释）。
// 与 cooldownReason429 / cooldownReasonNetwork 并列的 reason 单源常量，
// 探活恢复路径（#6-3）消费同一常量。
const cooldownReasonAuthFail = "auth_fail"

// classifyAuthFailure 判定是否鉴权失败（401/403）：凭据级失败中走判停路径的子类
// （#6-2）——不走冷却，改为连续失败计数，达阈值判停 temp_unsched。
// 密钥失效重试无意义：冷却循环只会推迟判停（且到期恢复后照样 401）。
func classifyAuthFailure(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// applyCredentialAuthFailure 处理一次鉴权失败（401/403）：FailCount 经单条条件 UPDATE
// 原子自增并在达阈值 N（可配设置项）时判停 temp_unsched（#6-4-1 收敛读回再写）。
// 非 active（含人工 disabled/error/已判停 temp_unsched）只自增不判停。
// 穿插的 429/5xx 冷却不清零计数——只有成功请求重置（见成功出口）。
// 落库经 credwrite 有界队列异步执行，请求路径不等待 DB。
func applyCredentialAuthFailure(ctx context.Context, cred models.Credential) {
	if cred.ID == 0 {
		// Selection 里凭据为空（测试直调 attempt 的构造形态），无行可计数，
		// 与 applyCredentialCooldown 同款防御壳。
		return
	}
	credwrite.EnqueueAuthFail(cred.ID)
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
