package credprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
	"github.com/qkf688/llmux/service/credwrite"
)

// DefaultTimeout 探活上游 /models 的默认超时（短于普通聊天客户端超时，
// 避免组耗尽自愈把整请求拖死）。
const DefaultTimeout = 5 * time.Second

// Request 一次惰性探活输入：组内凭据候选 + 选中端点 + 频控间隔。
// Interval/Now 由调用方注入（chat 读设置项、可测时钟）。
type Request struct {
	Ctx         context.Context
	Provider    models.Provider
	Endpoint    models.Endpoint
	Credentials []models.Credential
	Interval    time.Duration
	Now         time.Time
	// Timeout 探活 HTTP 超时；0 则用 DefaultTimeout。
	Timeout time.Duration
}

// Outcome 探活结果：Attempted=是否真的打了上游；Recovered=是否写回 active。
type Outcome struct {
	Attempted    bool
	Recovered    bool
	CredentialID uint
}

// TryRecover 组内全不可用时的惰性探活（#6-3 / #6-4-2）：
//
//  1. 在候选中按 ID ASC 取第一条 temp_unsched 且已过探活间隔的凭据（本轮最多 1 条）
//  2. CAS claim LastProbeAt（防双请求双打上游）
//  3. 用选中端点组装动态 config，调 Models() 轻探
//  4. 成败写路径经 credwrite 队列：失败 touch；成功条件恢复 active
//
// 无候选/频控全跳过/未抢到 claim → Attempted=false，调用方保持原失败语义。
func TryRecover(req Request) Outcome {
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	cand := pickProbeCandidate(req.Credentials, req.Now, req.Interval)
	if cand == nil {
		return Outcome{}
	}

	cutoff := req.Now.Add(-req.Interval)
	n, err := repos().Credential.ClaimLastProbeAt(req.Ctx, cand.ID, req.Now, cutoff)
	if err != nil {
		slog.Warn("credential probe claim last_probe_at failed",
			"credential_id", cand.ID, "error", err)
		return Outcome{}
	}
	if n == 0 {
		// 他路已 claim 或间隔内——不得打上游。
		return Outcome{}
	}

	out := Outcome{Attempted: true, CredentialID: cand.ID}
	probeErr := probeModels(req.Ctx, timeout, req.Provider, req.Endpoint, *cand)

	if probeErr != nil {
		slog.Warn("credential probe failed",
			"credential_id", cand.ID, "provider", req.Provider.Name, "error", probeErr)
		touchLastProbeAt(req.Ctx, cand.ID, req.Now)
		return out
	}

	if recoverCredential(req.Ctx, cand.ID, req.Now) {
		out.Recovered = true
	}
	return out
}

// pickProbeCandidate 选本轮唯一探活目标：temp_unsched + 过间隔，ID ASC 稳定首选。
func pickProbeCandidate(creds []models.Credential, now time.Time, interval time.Duration) *models.Credential {
	sorted := append([]models.Credential(nil), creds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for i := range sorted {
		c := &sorted[i]
		if c.Status != models.CredentialStatusTempUnsched {
			continue
		}
		if c.LastProbeAt != nil && !now.After(c.LastProbeAt.Add(interval)) {
			continue
		}
		return c
	}
	return nil
}

func probeModels(ctx context.Context, timeout time.Duration, provider models.Provider, endpoint models.Endpoint, cred models.Credential) error {
	plainKey, err := channel.DecryptCredentialKey(cred)
	if err != nil {
		return err
	}
	cfg, _, err := channel.BuildConfig(provider, endpoint, plainKey)
	if err != nil {
		return err
	}
	// 剥 custom_models：否则 openai/anthropic Models() 本地短路返回，失效 key
	// 会被当成探活成功拉回 active（与 modelsync 拉上游前 dropCustomModels 同因）。
	cfg, err = stripCustomModels(cfg)
	if err != nil {
		return err
	}
	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		return fmt.Errorf("credprobe: unknown endpoint protocol %q", endpoint.Protocol)
	}
	chatModel, err := providers.New(providerType, cfg, provider.Proxy)
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err = chatModel.Models(probeCtx)
	return err
}

// stripCustomModels 从动态 config JSON 移除 custom_models，强制 Models() 打上游。
func stripCustomModels(config string) (string, error) {
	if config == "" {
		return config, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", fmt.Errorf("credprobe: parse config for strip custom_models: %w", err)
	}
	if _, ok := parsed["custom_models"]; !ok {
		return config, nil
	}
	delete(parsed, "custom_models")
	out, err := json.Marshal(parsed)
	if err != nil {
		return "", fmt.Errorf("credprobe: marshal config after strip custom_models: %w", err)
	}
	return string(out), nil
}

// touchLastProbeAt 探活失败路径：只记账探活时间，不动 Status/FailCount/CooldownReason。
// CAS 已 stamp 过 LastProbeAt；仍入队保证队列化路径与恢复失败触点一致（#6-4-2）。
func touchLastProbeAt(_ context.Context, id uint, now time.Time) {
	credwrite.EnqueueTouchProbe(id, now)
}

// recoverCredential 探活成功写回 active：同步落库（与随后 Assemble/Select happens-before）；
// 条件更新与失败 touch 在 Recover handler 内完成。
func recoverCredential(_ context.Context, id uint, now time.Time) bool {
	return credwrite.EnqueueRecover(id, now)
}
