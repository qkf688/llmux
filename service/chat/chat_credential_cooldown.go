package chat

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
)

// cooldownReason429 429 限流冷却 reason 机器码：分窗判定（cooldownWindowForReason）
// 与产出侧（cooldownReasonForStatus）共用的唯一来源，改动必须同步两处，
// 否则分窗判定会静默失效（429 掉进服务端窗口）。
const cooldownReason429 = "http_429"

// cooldownReasonNetwork 网络/超时失败 reason 机器码（调用点唯一来源，
// 与 cooldownReasonForStatus 产出的 http_<code> 序列并列）。
const cooldownReasonNetwork = "network"

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

// cooldownReasonForStatus 冷却原因机器码（供状态机/前端文案映射消费；
// 5xx 直接带状态码便于排查，不细分 5xx 子类——#6-1 分窗按 reason 前缀分派，
// 5xx 与网络都落在 server 窗，不依赖子类细分。401/403 由判停路径接管
// （chat_credential_auth_fail.go，reason=auth_fail），不经本函数）。
func cooldownReasonForStatus(statusCode int) string {
	switch statusCode {
	case http.StatusTooManyRequests:
		return cooldownReason429 // 单源：与 cooldownWindowForReason 分窗判定共用
	default:
		return fmt.Sprintf("http_%d", statusCode)
	}
}

// cooldownWindowForReason 按失败类型分配冷却窗口（#6-1 可配基数 + 分窗）：
// 429/过载独立读「429 窗口」；其余凭据级失败（5xx/超时/网络）共用「服务端窗口」；
// 鉴权失败（401/403）不走冷却，走判停路径（#6-2，见 chat_credential_auth_fail.go）。
// 分窗判定集中此一处（OCP）：复用既有窗口的新失败类型此判定即覆盖（多半连这都不用改）；
// 但「新窗口」类型需同步设置项全链 6 处（键常量/schema/DTO/前端 interface/卡片 intFields/本函数）。
func cooldownWindowForReason(ctx context.Context, reason string) time.Duration {
	if reason == cooldownReason429 {
		return time.Duration(getCredHealthCooldown429Sec(ctx)) * time.Second
	}
	return time.Duration(getCredHealthCooldownServerSec(ctx)) * time.Second
}

// applyCredentialCooldown 写单条凭据的冷却状态（CooldownUntil 到期自动恢复）。
// 窗口 = 按失败类型分窗读设置项（见 cooldownWindowForReason），非固定常量；
// 行为经逐请求读库即时生效，修改设置项无需重载。
// 落库经 credwrite 有界队列异步执行（#6-4-1），请求路径不等待 DB。
func applyCredentialCooldown(ctx context.Context, cred models.Credential, reason string) {
	if cred.ID == 0 {
		// Selection 里凭据为空（测试直调 attempt 的构造形态），无行可冷却。
		// 生产路径 Select 成功必有 Credential，此处只是防御壳非兼容分支。
		return
	}
	credwrite.EnqueueCooldown(cred.ID, reason)
}
