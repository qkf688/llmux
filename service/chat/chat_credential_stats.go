package chat

import (
	"context"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
)

// 凭据终身统计（号池页「请求/失败/最近使用」数据源，2026-09-12 定案）：
// 口径 = attempt 级——上游 200 被接受即记成功（流式中途断流不回拨，与冷却/判停
// 状态机同点）；失败域 = classifyCredentialFailure（401/403/429/5xx）+ 网络/超时。
// 组织级出口（能力 skip、转换失败、其余 4xx）不记账——客户端坏请求不该让共享 key 背锅。
// 每请求必写且与请求结果无关紧要，统一走 credwrite 异步队列（#6-4），请求路径不等待 DB。
// 探针/探活/健康检查走诊断口径不进 attempt，天然零记账（#24「只报不改」）。

// recordCredentialStatsSuccess 记一次成功尝试：total_requests+1 且 last_used_at=now。
func recordCredentialStatsSuccess(ctx context.Context, cred models.Credential) {
	if cred.ID == 0 {
		// Selection 里凭据为空（测试直调 attempt 的构造形态），与 applyCredentialCooldown 同款防御壳。
		return
	}
	credwrite.EnqueueRecordSuccess(cred.ID, time.Now())
}

// recordCredentialStatsError 记一次凭据级失败：total_errors+1（终身累计，
// 失败不刷新 last_used_at——「最近使用」= 上次成功时刻，坏 key 停格可诊断）。
func recordCredentialStatsError(ctx context.Context, cred models.Credential) {
	if cred.ID == 0 {
		return
	}
	credwrite.EnqueueRecordError(cred.ID)
}
