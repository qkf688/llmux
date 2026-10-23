package pools

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// defaultAutoPoolNote 自动池默认备注（前端未指定时用）。
const defaultAutoPoolNote = "供应商分组自动创建"

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
// 请求体 {"name": "供应商名·分组名", "note": "可选", "keys": ["sk-1", ...]}
// 响应 {"pool_id": 12, "pool_name": "...", "total": n, "imported": n, "skipped": n,
//
//	"failed": n, "rows": [...]}
func CreateAutoPoolWithCredentials(c *gin.Context) {
	var req AutoPoolImportRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpresp.BadRequest(c, "Pool name is required")
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

	poolName, err := uniquePoolName(ctx, name)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		note = defaultAutoPoolNote
	}
	pool := &models.Pool{Name: poolName, Note: note, AutoCreated: true}
	if err := repos().Pool.Create(ctx, pool); err != nil {
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
			slog.Warn("auto pool cleanup after batch import failure", "pool_id", pool.ID, "err", delErr)
		}
		httpresp.InternalErrorMessage(c, "Failed to import credentials", err)
		return
	}
	httpresp.Success(c, AutoPoolImportData{PoolID: pool.ID, PoolName: pool.Name, BatchImportData: data})
}
