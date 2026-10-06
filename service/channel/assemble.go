package channel

import (
	"context"
	"fmt"
	"sort"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// Assembler 从 repository 装配某个供应商的选路快照。
// 依赖注入：构造时传入 *repository.Repositories（同 autoassoc.NewService 模式），
// 不取包级默认——便于单测与未来 healthcheck/modelsync 复用。
type Assembler struct {
	repos *repository.Repositories
}

// NewAssembler 创建装配器。
func NewAssembler(repos *repository.Repositories) *Assembler {
	return &Assembler{repos: repos}
}

// Assemble 读取供应商的端点/分组/凭据并归并成 Snapshot：
//
//   - endpoints：EndpointRepo.ListByProvider
//   - groups：KeyGroupRepo.ListByProvider
//   - credentials：CredentialRepo.ListByPools（按本供应商分组引用的号池 ID 集合收敛，
//     SQL 侧过滤并 id ASC 排序），再按分组归并——组 ID → 其 PoolID 池内凭据
//
// 候选顺序稳定性是轮询取模正确的前提（分组档内轮询 / 组内凭据轮询），
// 三路数据统一在此按 ID ASC 收敛，不依赖 SQLite 的隐性行序。
func (a *Assembler) Assemble(ctx context.Context, provider models.Provider) (*Snapshot, error) {
	endpoints, err := a.repos.Endpoint.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("channel: list endpoints: %w", err)
	}
	groups, err := a.repos.KeyGroup.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("channel: list key groups: %w", err)
	}

	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ID < endpoints[j].ID })
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })

	poolIDs := make([]uint, 0, len(groups))
	for _, g := range groups {
		if g.PoolID != nil {
			poolIDs = append(poolIDs, *g.PoolID)
		}
	}
	allCreds, err := a.repos.Credential.ListByPools(ctx, poolIDs)
	if err != nil {
		return nil, fmt.Errorf("channel: list credentials: %w", err)
	}

	byGroup := make(map[uint][]models.Credential, len(groups))
	for _, g := range groups {
		byGroup[g.ID] = groupCredentials(allCreds, g.PoolID)
	}
	return &Snapshot{
		Provider:           provider,
		Endpoints:          endpoints,
		Groups:             groups,
		CredentialsByGroup: byGroup,
	}, nil
}

// groupCredentials 取分组所引用号池内的凭据。组未选池（PoolID 为 nil）或池内无凭据时
// 返回 v=nil；快照 map 键仍存在，选择路径据此走 ErrNoCredentialAvailable
// （与「组键缺失」语义一致，统一收口）。
func groupCredentials(all []models.Credential, poolID *uint) []models.Credential {
	if poolID == nil {
		return nil
	}
	var out []models.Credential
	for _, c := range all {
		if c.PoolID == *poolID {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = nil
	}
	return out
}
