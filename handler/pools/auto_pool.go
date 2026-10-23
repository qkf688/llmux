package pools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// autoPoolNameMaxAttempts 自动池重名兜底的最大尝试次数（同名加 " (n)" 后缀）。
const autoPoolNameMaxAttempts = 20

// errAutoPoolNameTaken 池名变体用尽（同名太多）：业务条件，报 400 而不是内部故障。
var errAutoPoolNameTaken = errors.New("pools: auto pool name variants exhausted")

// autoPoolIdentity 由供应商名与分组名生成专属池的「名 + 备注」。
//
// 为什么口径在服务端：命名规则只有一处实现，前端不再拼名字——同一规则两处实现必然漂移
// （前端拼「供应商名·分组名」、后端拼「供应商名_分组名」这类漂移，用户会看到两个口径）。
//
// 为什么空名不报错而是兜底：用户可能还没填完供应商名/分组名就点了「添加 Key」，
// 这时拦住他等于把正常流程挡下来；池名是用户在号池页唯一能认出自己那批凭据的地方，
// 空串在列表里无法区分，留可读占位比留空好。真正的归属关系由 key_groups.pool_id 承载，
// 池名只是给人看的标识。
func autoPoolIdentity(providerName, groupName string) (name, note string) {
	provider := strings.TrimSpace(providerName)
	if provider == "" {
		provider = "未命名供应商"
	}
	group := strings.TrimSpace(groupName)
	if group == "" {
		group = "未命名分组"
	}
	return provider + "·" + group, fmt.Sprintf("供应商「%s」的分组「%s」自动创建", provider, group)
}

// createAutoPoolWithUniqueName 建一个专属号池并保证池名唯一：
// 重名是常态而非异常（删过供应商再建同名、同名供应商的不同分组），直接报错会把用户的
// 正常操作挡下来；加序号后缀既能建池，又不会把用户建到别的池上去
// （复用已存在的池 = 往别人的池里塞凭据，绝不能做）。
//
// 为什么不先查名再建池（与 api_keys 建 key 同口径）：查名与建池之间有窗口——两个并发
// 请求（双击「添加 Key」、两个标签页同时建同名供应商）会同时查到「名字可用」，后者撞
// pools.name 唯一索引 → 报 500，报错方向全错。uniqueIndex 才是单一真相：直接建池，
// 撞了就换下一个后缀重试。顺带收敛「软删行仍占名」——预检看不见软删行，但唯一索引看得见。
func createAutoPoolWithUniqueName(ctx context.Context, base, note string) (*models.Pool, error) {
	candidate := base
	for attempt := 1; attempt <= autoPoolNameMaxAttempts; attempt++ {
		pool := &models.Pool{Name: candidate, Note: note, AutoCreated: true}
		err := repos().Pool.Create(ctx, pool)
		if err == nil {
			return pool, nil
		}
		if !errors.Is(err, repository.ErrPoolNameTaken) {
			return nil, fmt.Errorf("create auto pool: %w", err)
		}
		candidate = fmt.Sprintf("%s (%d)", base, attempt+1)
	}
	return nil, fmt.Errorf("%w: pool name %q has no free variant within %d attempts",
		errAutoPoolNameTaken, base, autoPoolNameMaxAttempts)
}

// CreateAutoPoolWithCredentials 「供应商分组 → 添加 Key」的一步式落库：
// 建一个专属号池（auto_created=true）并立刻把粘贴的 Key 导进去，返回池标识与逐行回显。
//
// 为什么留在号池域（handler/pools）：入参是「池 + 一批 key」，产出是「池 + 它的凭据」，
// 与既有的「往这个池里灌 N 条 key」（BatchImportCredentials）同族；行级语义（批内/池内
// 去重、逐行 imported/skipped/failed）直接复用 importCredentialRows 这一份实现，不另写一套。
//
// 为什么建池与导凭据放一个请求：供应商弹窗里「加 Key」是一步动作，用户粘贴完就期待凭据
// 已经落库；拆成「先建池再导」两跳会让中间态（空池）暴露给用户，也让前端要自己兜一半失败。
// 池已建好后的追加导入走既有端点 POST /pools/:id/credentials/batch/import。
//
// POST /api/pools/auto/import
// 请求体 {"provider_name": "供应商名", "group_name": "分组名", "keys": ["sk-1", ...]}
// 池名与备注由服务端按「供应商名·分组名」生成（autoPoolIdentity），前端不参与命名。
// 响应 {"pool_id": 12, "pool_name": "...", "auto_created": true, "key_count": 2,
// "total": n, "imported": n, "skipped": n,
//
//	"failed": n, "rows": [...]}
func CreateAutoPoolWithCredentials(c *gin.Context) {
	var req AutoPoolImportRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if !validateImportKeys(c, req.Keys) {
		return
	}

	ctx := c.Request.Context()
	cipher, ok := importCipher(c)
	if !ok {
		return
	}

	name, note := autoPoolIdentity(req.ProviderName, req.GroupName)
	pool, err := createAutoPoolWithUniqueName(ctx, name, note)
	if err != nil {
		// 变体名用尽是业务条件（同名太多），不是内部故障。
		if errors.Is(err, errAutoPoolNameTaken) {
			httpresp.BadRequest(c, "Pool name is taken and no free variant available")
			return
		}
		httpresp.InternalErrorMessage(c, "Failed to create pool", err)
		return
	}

	// 行级失败（单行加密/落库异常）不回滚已建的池：逐行隔离语义与既有批量导入一致
	// （部分成功即部分落库），失败行由前端回显原因，用户可在号池页补加。
	//
	// 整批失败（importCredentialRows 只在查重查询异常时返回 error，此时一行都没写）
	// 则把刚建的空池删掉：留着它等于给用户塞一个没凭据的专属池，重试还会再撞同名。
	data, err := importCredentialRows(ctx, repos().Credential, pool.ID, req.Keys, cipher)
	if err != nil {
		if _, delErr := repos().Pool.Delete(ctx, pool.ID); delErr != nil {
			slog.Warn("auto pool cleanup after batch import failure",
				"pool_id", pool.ID, "pool_name", pool.Name, "err", delErr)
		}
		httpresp.InternalErrorMessage(c, "Failed to import credentials", err)
		return
	}
	httpresp.Success(c, AutoPoolImportData{
		PoolID:          pool.ID,
		PoolName:        pool.Name,
		AutoCreated:     true,
		KeyCount:        data.Imported,
		BatchImportData: data,
	})
}
