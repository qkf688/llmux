package channel

import (
	"fmt"
	"time"

	"github.com/qkf688/llmux/models"
)

// SelectCredential 在选中分组下选择凭据（设计定案第 4 节第 2 级）：
//
//   - 过滤：仅 active 状态（disabled/error 排除）+ 非冷却中
//   - 组内轮询：过滤后的候选按 Selector 指针推进（选择即推进）
//   - 全部不可用 → ErrNoCredentialAvailable，由上层（#13 故障转移）决定
//     组内换下一层还是向上失败
//
// now 由调用方注入（可测）：冷却判定用「CooldownUntil > now 即冷却中」，
// 到期自动恢复、不占状态位（设计定案第 5 节，S4 完整状态机前的最小冷却语义）。
func SelectCredential(s *Selector, snapshot *Snapshot, groupID uint, now time.Time) (models.Credential, error) {
	candidates, ok := snapshot.CredentialsByGroup[groupID]
	if !ok || len(candidates) == 0 {
		return models.Credential{}, fmt.Errorf("%w: group %d has no credentials", ErrNoCredentialAvailable, groupID)
	}

	usable := make([]models.Credential, 0, len(candidates))
	for _, c := range candidates {
		if credentialUsable(c, now) {
			usable = append(usable, c)
		}
	}
	if len(usable) == 0 {
		return models.Credential{}, fmt.Errorf("%w: group %d all credentials unusable", ErrNoCredentialAvailable, groupID)
	}

	idx, _ := s.nextCredentialInRR(snapshot.Provider.ID, groupID, len(usable))
	return usable[idx], nil
}

// credentialUsable 凭据是否可调度：active（空值按 GORM default 'active' 语义
// 宽容）且不在冷却窗口内。
func credentialUsable(c models.Credential, now time.Time) bool {
	if c.Status != "" && c.Status != models.CredentialStatusActive {
		return false
	}
	if c.CooldownUntil != nil && c.CooldownUntil.After(now) {
		return false
	}
	return true
}
