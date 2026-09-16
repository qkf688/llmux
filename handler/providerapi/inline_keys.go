package providerapi

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"gorm.io/gorm"
)

// GetGroupInlineKeys 内联组明文查看（受控端点）：解密返回该组全部凭据明文，每项带
// 凭据 id 供前端按 id 归位（不依赖数组顺序），供编辑弹窗「显示」动作替换掩码行。
// 明文永不出常规详情响应——与号池 GetCredentialRaw 同一受控语义（JWT 管理域信任级 +
// 显式用户动作才拉取）。响应带 `Cache-Control: no-store`：明文落进浏览器磁盘缓存或
// 中间代理就绕过了前端的生命周期守卫。
// GET /api/providers/:id/groups/:groupId/inline-keys/raw
func GetGroupInlineKeys(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	groupID, ok := httpx.ParseUintParamAllowZero(c, "groupId")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().Provider.Get(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Provider not found")
			return
		}
		httpresp.InternalServerError(c, err)
		return
	}

	group, err := repos().KeyGroup.Get(ctx, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Group not found")
			return
		}
		httpresp.InternalServerError(c, err)
		return
	}
	// 越组访问按 404 处理：不暴露分组存在性（对齐号池 getCredentialInPool 语义）
	if group.ProviderID != id {
		httpresp.NotFound(c, "Group not found")
		return
	}
	if group.PoolID != nil {
		httpresp.BadRequest(c, "Pool group has no inline keys")
		return
	}

	creds, err := repos().Credential.ListByGroups(ctx, []uint{group.ID}, nil)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalErrorMessage(c, "encryption not configured", nil)
		return
	}

	keys := make([]InlineKeyRawItem, 0, len(creds))
	for _, cred := range creds {
		plain, decErr := cipher.Decrypt(cred.Key)
		if decErr != nil {
			// 任一条失败整组拒绝：明文替换是覆盖动作，部分成功会静默丢无法显示的条目。
			// （与号池 raw 单条 500 一致的失败语义，扩展为整组。）
			slog.Warn("inline key decrypt failed for raw view", "credential_id", cred.ID, "group_id", group.ID, "error", decErr)
			httpresp.InternalErrorMessage(c, "Failed to decrypt keys", decErr)
			return
		}
		keys = append(keys, InlineKeyRawItem{ID: cred.ID, Key: plain})
	}

	httpresp.SuccessNoStore(c, InlineKeysRawData{Keys: keys})
}
