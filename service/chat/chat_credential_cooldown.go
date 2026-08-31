package chat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/models"
)

// minimalCredentialCooldown 凭据最小冷却窗口（设计定案第 4/5 节的 S3 先行版）：
// 429/5xx/网络/鉴权失败后冷却该凭据，窗口内选路剔除（channel 的 credentialUsable
// 判定「CooldownUntil > now 即冷却中」），到期自动恢复、不占状态位。
// S3 先取统一常量；S4 完整状态机替换为「可配基数」设置项并按失败类型分窗。
const minimalCredentialCooldown = 1 * time.Minute

// classifyCredentialFailure 判定一次失败是否属于「凭据级」——该凭据自身的问题
// （限流/服务端错误/鉴权），换一条 key 可能成功，组内故障转移（#13）以此换 key；
// 其余 4xx（请求体被拒等）是请求本身的问题，换 key 无意义，走组织级语义。
func classifyCredentialFailure(statusCode int) bool {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return true
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return true
	case statusCode >= 500:
		return true
	}
	return false
}

// cooldownReasonForStatus 冷却原因机器码（供 S4 状态机/前端文案映射消费；
// 5xx 直接带状态码便于排查，不细分 5xx 子类——S4 如需按类分窗再拆）。
func cooldownReasonForStatus(statusCode int) string {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return "http_429"
	case statusCode == http.StatusUnauthorized:
		return "http_401"
	case statusCode == http.StatusForbidden:
		return "http_403"
	default:
		return fmt.Sprintf("http_%d", statusCode)
	}
}

// applyCredentialCooldown 写单条凭据的冷却状态（CooldownUntil 到期自动恢复）。
// 写库失败仅告警不阻断：选路指针「选择即推进」保证同请求内即使冷却未生效，
// 也不会反复选出同一条 key（配合 retry loop 的组内换 key 上限收敛）。
func applyCredentialCooldown(ctx context.Context, cred models.Credential, reason string) {
	if cred.ID == 0 {
		// Selection 里凭据为空（测试直调 attempt 的构造形态），无行可冷却。
		// 生产路径 Select 成功必有 Credential，此处只是防御壳非兼容分支。
		return
	}
	if _, err := repos().Credential.UpdateFields(ctx, cred.ID, map[string]any{
		"cooldown_until":  time.Now().Add(minimalCredentialCooldown),
		"cooldown_reason": reason,
	}); err != nil {
		slog.Warn("failed to apply credential cooldown", "credential_id", cred.ID, "reason", reason, "error", err)
	}
}
