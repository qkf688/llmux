package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credprobe"
)

// probeRecoverInGroup 组内凭据全不可用时的惰性探活入口（#6-3）：
// 对给定组的凭据候选轻探一次（频控在 credprobe 内），成功恢复则返回 true。
// 调用方负责刷新快照并重试选路。每候选每请求应最多调用一次（由 retry loop 旗标约束）。
//
// endpoint/creds 必须来自「刚失败的那次选路」已锁定的端点与分组——外层用 Select
// 凭据失败时带回的 Endpoint/Group，内层用 selection 锁定字段；禁止为探活再跑
// SelectGroup（会二次推进 RR，多组时探错组）。
func probeRecoverInGroup(
	ctx context.Context,
	provider models.Provider,
	endpoint models.Endpoint,
	creds []models.Credential,
) bool {
	interval := time.Duration(getCredHealthProbeIntervalSec(ctx)) * time.Second
	out := credprobe.TryRecover(credprobe.Request{
		Ctx:         ctx,
		Provider:    provider,
		Endpoint:    endpoint,
		Credentials: creds,
		Interval:    interval,
		Now:         time.Now(),
	})
	if out.Recovered {
		slog.Info("credential recovered by probe",
			"provider", provider.Name,
			"credential_id", out.CredentialID,
			"endpoint", endpoint.URL,
		)
	}
	return out.Recovered
}
