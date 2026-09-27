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
	return selectCredentialWithPrefer(s, snapshot, groupID, 0, now, nil)
}

// selectCredentialWithPrefer 带钉定偏好选凭据：钉定凭据须在可调度候选内（与
// SelectCredential 同一套过滤），命中不推进组内轮询指针（同 selectGroupWithPrefer
// 理由）；失效则回退常规轮询。precomputed 是分组层已算好的可调度候选（钉定命中
// 路径向下传，防同组重复过滤，Stage 30），nil 时在这里算一次——过滤判据单源
// usableCredentials 不变，miss 回退不再重算第二遍。
func selectCredentialWithPrefer(s *Selector, snapshot *Snapshot, groupID uint, preferID uint, now time.Time, precomputed []models.Credential) (models.Credential, error) {
	usable := precomputed
	if len(usable) == 0 {
		var err error
		usable, err = usableCredentials(snapshot, groupID, now)
		if err != nil {
			return models.Credential{}, err
		}
	}
	if preferID != 0 {
		for _, c := range usable {
			if c.ID == preferID {
				return c, nil
			}
		}
	}
	idx, _ := s.nextCredentialInRR(snapshot.Provider.ID, groupID, len(usable))
	return usable[idx], nil
}

// FirstUsableCredential 取组内**首个**可用凭据（候选按 ID ASC，跳过 disabled/error/
// temp_unsched 与冷却未到期者）——探活/同步/管理端诊断的确定性凭据口径。
//
// 为什么不走 SelectCredential：那条路径组内轮询且「选择即推进」，而诊断流量必须可复现
// （配置没动时前后两次测的是同一把 key），也不该把管理员一次点击计入生产压力分摊。
// 历史上这一口径靠「新建 &Selector{}、指针恒为 0」隐式达成，这里把它显式化，不再依赖
// Selector 零值的巧合（选中的凭据也不会推进 DefaultSelector 的指针）。
//
// 已知并接受代价：只验组内第一条可用凭据，坏在后面的 key 测不出来——逐 key 健康是
// credprobe 与号池详情「按凭据测试」(#24) 的职责，不该由模型弹窗的测试按钮隐式承担。
//
// 组内无可用凭据 → 包装 ErrNoCredentialAvailable 的错误（errors.Is 可判），调用方按
// 「跳过该组、继续其余组」处理，与 SelectCredential 同一分层错误语义。
func FirstUsableCredential(snapshot *Snapshot, groupID uint, now time.Time) (models.Credential, error) {
	usable, err := usableCredentials(snapshot, groupID, now)
	if err != nil {
		return models.Credential{}, err
	}
	return usable[0], nil
}

// usableCredentials 过滤出组内可调度凭据（保持装配顺序 = ID ASC），并把「本组无凭据」
// 与「有凭据但全不可用」两种情形分成两条 sentinel 文案——两条路径对调用方是不同事实
// （前者多半是分组配置缺失，后者是 key 全挂了），不能合并成一条。
// 返回空切片时 err 非 nil，故两个选择器都可先判 err 再直接下标取值。
func usableCredentials(snapshot *Snapshot, groupID uint, now time.Time) ([]models.Credential, error) {
	candidates, ok := snapshot.CredentialsByGroup[groupID]
	if !ok || len(candidates) == 0 {
		return nil, fmt.Errorf("%w: group %d has no credentials", ErrNoCredentialAvailable, groupID)
	}

	usable := make([]models.Credential, 0, len(candidates))
	for _, c := range candidates {
		if credentialUsable(c, now) {
			usable = append(usable, c)
		}
	}
	if len(usable) == 0 {
		return nil, fmt.Errorf("%w: group %d all credentials unusable", ErrNoCredentialAvailable, groupID)
	}
	return usable, nil
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
