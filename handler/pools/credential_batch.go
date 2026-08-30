package pools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// maxCredentialImportBatch 批量导入单请求 key 上限（设计场景「500 key 走号池」，
// 防一次粘贴上万条撑爆内存/SQL 循环）。
const maxCredentialImportBatch = 500

// 批量导入逐行状态/原因机器码（响应契约，前端映射展示文案；SKIPPED 行
// 用 Reason 区分 duplicate_in_batch / duplicate_in_pool / empty）。
const (
	importStatusImported         = "imported"
	importStatusSkipped          = "skipped"
	importStatusFailed           = "failed"
	importReasonDuplicateInBatch = "duplicate_in_batch"
	importReasonDuplicateInPool  = "duplicate_in_pool"
	importReasonEmpty            = "empty"
	importReasonEncryptFailed    = "encrypt_failed"
	importReasonDBFailed         = "db_failed"
)

// credentialCipher 批量导入对加密能力的依赖面：Hash（去重）+ Encrypt（落库）。
// 定义为接口：真实 Cipher 的 Encrypt 无业务可控失败点，测试注入失败实现
// 才能覆盖 failed 行类（行级失败回显的契约）。
type credentialCipher interface {
	Hash(plain string) string
	Encrypt(plain string) (string, error)
}

// BatchImportCredentials 批量粘贴导入凭据：去重（批内首现 + 池内现有）+ 加密落库 +
// 失败行回显。行级事务语义：单行加密/落库异常收集为 failed 行，不阻断其余行落库。
// POST /api/pools/:id/credentials/batch/import
// 请求体 {"keys": ["sk-1", ...]}（前端按行拆分后传数组，服务端不做文本解析）；
// 单请求上限 maxCredentialImportBatch 条。
func BatchImportCredentials(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	var req BatchImportCredentialsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.Keys) == 0 {
		httpresp.BadRequest(c, "No keys provided")
		return
	}
	if len(req.Keys) > maxCredentialImportBatch {
		httpresp.BadRequest(c, fmt.Sprintf("Batch exceeds limit of %d keys", maxCredentialImportBatch))
		return
	}
	ctx := c.Request.Context()
	if !requirePool(c, ctx, poolID) {
		return
	}
	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalServerError(c, "encryption not configured")
		return
	}
	data, err := importCredentialRows(ctx, repos().Credential, poolID, req.Keys, cipher)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to import credentials: "+err.Error())
		return
	}
	httpresp.Success(c, data)
}

// importCredentialRows 批量导入行处理（纯逻辑，与 HTTP 解耦便于测试）：
// 逐行 TrimSpace（顺带消化 CRLF 粘贴的 \r）→ 空行 skipped(empty) → 批内首现去重 →
// 池内 ExistingHashes 一次查重（防 N+1）→ 加密/落库，逐行收集 imported/skipped/failed。
// 查重查询失败属于基础设施异常，整体返回 error（handler 500），不做逐行兜底。
func importCredentialRows(ctx context.Context, repo repository.CredentialRepo, poolID uint, keys []string, cipher credentialCipher) (BatchImportData, error) {
	data := BatchImportData{
		Total: len(keys),
		Rows:  make([]BatchImportRow, 0, len(keys)),
	}
	if len(keys) == 0 {
		return data, nil
	}

	// 第一遍：逐行 trim + 批内首现去重，收集唯一哈希（一次批查池内，防 N+1）
	rowHashes := make([]string, len(keys)) // 每行对应哈希；空行/批内重复位留空串
	firstIndex := map[string]int{}         // hash → 批内首现行下标
	for i, raw := range keys {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		h := cipher.Hash(trimmed)
		rowHashes[i] = h
		if _, ok := firstIndex[h]; !ok {
			firstIndex[h] = i
		}
	}
	unique := make([]string, 0, len(firstIndex))
	for h := range firstIndex {
		unique = append(unique, h)
	}
	existing, err := repo.ExistingHashes(ctx, poolID, unique)
	if err != nil {
		return data, fmt.Errorf("check existing hashes: %w", err)
	}

	// 第二遍：按输入顺序逐行回显
	for i, raw := range keys {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			data.Skipped++
			data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: "", Status: importStatusSkipped, Reason: importReasonEmpty})
			continue
		}
		h := rowHashes[i]
		if firstIndex[h] != i {
			data.Skipped++
			data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: maskKey(trimmed), Status: importStatusSkipped, Reason: importReasonDuplicateInBatch})
			continue
		}
		if existing[h] {
			data.Skipped++
			data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: maskKey(trimmed), Status: importStatusSkipped, Reason: importReasonDuplicateInPool})
			continue
		}
		enc, err := cipher.Encrypt(trimmed)
		if err != nil {
			data.Failed++
			data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: maskKey(trimmed), Status: importStatusFailed, Reason: importReasonEncryptFailed})
			continue
		}
		if err := repo.Create(ctx, &models.Credential{Key: enc, KeyHash: h, PoolID: &poolID, Status: models.CredentialStatusActive}); err != nil {
			data.Failed++
			data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: maskKey(trimmed), Status: importStatusFailed, Reason: importReasonDBFailed})
			continue
		}
		data.Imported++
		data.Rows = append(data.Rows, BatchImportRow{Index: i, Key: maskKey(trimmed), Status: importStatusImported})
	}
	return data, nil
}

// BatchUpdateCredentialStatus 批量启停同一号池下的凭据（池内限界，不越池）。
// PATCH /api/pools/:id/credentials/batch/status
func BatchUpdateCredentialStatus(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	var req BatchCredentialStatusRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}
	if !models.IsValidCredentialStatus(req.Status) {
		httpresp.BadRequest(c, "Invalid status")
		return
	}
	ctx := c.Request.Context()
	if !requirePool(c, ctx, poolID) {
		return
	}
	updated, err := repos().Credential.UpdateStatusByIDs(ctx, poolID, req.IDs, req.Status)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}
	httpresp.Success(c, map[string]any{"updated": updated})
}

// BatchDeleteCredentials 批量删除同一号池下的凭据（池内限界，不越池）。
// DELETE /api/pools/:id/credentials/batch
func BatchDeleteCredentials(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	var req BatchCredentialDeleteRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}
	ctx := c.Request.Context()
	if !requirePool(c, ctx, poolID) {
		return
	}
	deleted, err := repos().Credential.DeleteByIDs(ctx, poolID, req.IDs)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete credentials: "+err.Error())
		return
	}
	httpresp.Success(c, map[string]any{"deleted": deleted})
}
