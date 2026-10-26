package autoassoc

import (
	"context"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service"
)

// ProviderWriteState 是「provider 写操作前后的状态快照」，判定要不要重算关联的输入。
// 经 service 门面别名自 service/autoassoc：调用方（provider CRUD）不必为拿类型再引子包。
type ProviderWriteState = service.ProviderWriteState

// SnapshotProviderWriteState 取提供商此刻的状态快照（可提供的上游模型 ∪ 是否拉黑）。
func SnapshotProviderWriteState(ctx context.Context, provider models.Provider) (ProviderWriteState, error) {
	return service.GetAutoAssocService().SnapshotProviderWriteState(ctx, provider)
}

// TriggerProviderWrite 按写前/写后快照判定这次 provider 写要不要重算关联表，
// 需要重算的部分按全局开关在后台跑（Trigger*IfEnabled 自身检查开关）。
//
// 为什么不是「provider 写一律触发」：重算是全库模型 × 全库提供商的一趟扫。
// 只有「可提供的模型增减 / 从拉黑恢复」才会改关联表，其余改动（改名、改地址、
// 改类型、改端点、改分组权重、动各开关）触发重算是白跑。
func TriggerProviderWrite(ctx context.Context, before, after ProviderWriteState) {
	effect := service.DetectProviderWriteEffect(before, after)
	if effect.Associate {
		bgtask.Go(TriggerAutoAssociate)
	}
	if effect.Clean {
		bgtask.Go(TriggerAutoClean)
	}
}
