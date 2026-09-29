package healthcheck

import (
	"context"
	"errors"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// errRoundSkipped 表示整轮检测未执行：本轮所需配置读不出来（Stage 36 拍板：这轮干脆不测）。
// 单列一个哨兵而不是普通错误——调用方要能把「配置读不出来」与「查关联列表失败」分开：
// 前者是已知降级（Warn 一条、下轮再看），后者是真故障（Error）。
var errRoundSkipped = errors.New("health check round skipped: settings unreadable")

// ValidateRoundConfig 在启动整轮检测前同步确认本轮所需配置可读（Stage 36 Ruling 1）。
//
// 为什么必须在启动前探：批量接口是先返回批次号、真正干活在后台，若把读失败留到后台，
// 前端只会看到一个永远不完成的批次（轮询没有超时），用户看到的是永久转圈。
func (h *HealthChecker) ValidateRoundConfig(ctx context.Context) error {
	if _, err := h.getCheckDisabledOnlyE(ctx); err != nil {
		settings.LogReadSkip("health check scope", models.SettingKeyHealthCheckCheckDisabledOnly, err)
		return errRoundSkipped
	}
	return nil
}

// checkAll 执行一轮定时检测。ctx 由 run 传入（派生自进程级 ctx），使整轮检测在
// 进程关闭时可被取消——此前自建 context.Background() 让一轮检测完全没有取消感知。
func (h *HealthChecker) checkAll(ctx context.Context) {
	modelProviders, checkDisabledOnly, err := h.listModelProvidersForCheck(ctx)
	if errors.Is(err, errRoundSkipped) {
		slog.Warn("health check round skipped: settings unreadable")
		return
	}
	if err != nil {
		slog.Error("failed to get model providers for health check", "error", err)
		return
	}

	slog.Info("starting health check", "count", len(modelProviders), "disabled_only", checkDisabledOnly)
	for _, mp := range modelProviders {
		if ctx.Err() != nil {
			slog.Info("health check interrupted", "reason", ctx.Err())
			return
		}
		h.checkOne(ctx, &mp)
	}
	slog.Info("health check completed")
}

// CheckAllWithBatch 批量检测所有模型提供商（手动触发，带 batchID）。
func (h *HealthChecker) CheckAllWithBatch(ctx context.Context, batchID string) error {
	modelProviders, checkDisabledOnly, err := h.listModelProvidersForCheck(ctx)
	if err != nil {
		if errors.Is(err, errRoundSkipped) {
			slog.Warn("batch health check skipped: settings unreadable", "batch_id", batchID)
			return err
		}
		slog.Error("failed to get model providers for batch health check", "error", err, "batch_id", batchID)
		return err
	}

	slog.Info("starting batch health check", "count", len(modelProviders), "batch_id", batchID, "disabled_only", checkDisabledOnly)
	for _, mp := range modelProviders {
		// 与 checkAll 一致：ctx 取消即中止整批，不把剩余候选继续打到上游。
		if ctx.Err() != nil {
			slog.Info("batch health check interrupted", "reason", ctx.Err(), "batch_id", batchID)
			return ctx.Err()
		}
		h.checkOneWithBatch(ctx, &mp, batchID)
	}
	slog.Info("batch health check completed", "batch_id", batchID)

	return nil
}

func (h *HealthChecker) listModelProvidersForCheck(ctx context.Context) ([]models.ModelWithProvider, bool, error) {
	checkDisabledOnly, err := h.getCheckDisabledOnlyE(ctx)
	if err != nil {
		// 读不出来就不测这一轮：按默认值继续等于替用户把「只测已停用的」关掉，
		// 对全部关联发真实探测请求（按量计费）——读不出来时不该扩大开销。
		settings.LogReadSkip("health check scope", models.SettingKeyHealthCheckCheckDisabledOnly, err)
		return nil, false, errRoundSkipped
	}

	var status *bool
	if checkDisabledOnly {
		falseVal := false
		status = &falseVal
	}

	modelProviders, err := repos().ModelWithProvider.ListByStatus(ctx, status)
	return modelProviders, checkDisabledOnly, err
}
