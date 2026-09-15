package apikeys

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// createApiKeyRequest：value 可省略（省略 = 系统生成随机合规 key，裁决见
// .plan/plan.md Rulings）；name 仅展示用、允许重名（方案 §5）；headers 可选
// （key 级自定义出站头，#6 方案 §2 切片 B——管理员显式通道，不设黑名单）。
type createApiKeyRequest struct {
	Name    string            `json:"name"`
	Value   string            `json:"value"`
	Headers map[string]string `json:"headers"`
}

// updateApiKeyRequest：三态指针——nil = 不改该字段。不提供 value：key 值
// 创建后不可改（轮换 = 新建 + 停用/删除，方案 §2 定案）。headers 同为三态指针：
// nil（JSON 缺该键）= 不改；指向 map（含空 map {}）= 整体覆盖，空 map 即清空
// （AGENTS §5 三态禁 omitempty——「不改」与「清空」两语义必须可区分）。
type updateApiKeyRequest struct {
	Name    *string            `json:"name"`
	Enabled *bool              `json:"enabled"`
	Headers *map[string]string `json:"headers"`
}

// apiKeyResponse 管理端形状。api_key = 解密明文（复制用，故响应恒 no-store）；
// 解密失败退化为 ****（与 credentialcrypto.MaskStored 同一降级口径：掩密文
// 形似真实值毫无意义，绝不能输出）。headers 键恒在无 omitempty：null=未配置，
// 缺键会让前端无法区分「未配置」与「旧后端」（AGENTS §5 三态纪律）。
type apiKeyResponse struct {
	ID        uint              `json:"id"`
	Name      string            `json:"name"`
	APIKey    string            `json:"api_key"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	Headers   map[string]string `json:"headers"`
}

func toApiKeyResponse(key *models.ApiKey, cipher *credentialcrypto.Cipher) apiKeyResponse {
	plain, err := cipher.Decrypt(key.Key)
	if err != nil {
		slog.Warn("decrypt api key for echo failed, masking", "api_key_id", key.ID, "error", err)
		plain = maskedApiKey
	}
	return apiKeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		APIKey:    plain,
		Enabled:   key.Enabled,
		CreatedAt: key.CreatedAt,
		Headers:   key.Headers,
	}
}

// maskedApiKey 解密失败降级值，与 credentialcrypto.MaskStored 失败分支同口径。
const maskedApiKey = "****"

// requireCipher 取包级加密设施；未装配时已写回 500 并返回 false。
// 各 handler 必须在**任何写库动作之前**调用：缺密钥时更新已生效却回 500 会让
// 客户端无法判断 PATCH 落没落。
func requireCipher(c *gin.Context) (*credentialcrypto.Cipher, bool) {
	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalServerError(c, "credential encryption key not configured")
		return nil, false
	}
	return cipher, true
}

// CreateApiKey 规范化/生成明文 → Encrypt+Hash 落库。查重不预查：撞 hash 唯一
// 索引由仓储翻译为 ErrApiKeyExists → HTTP 409（方案 §5 定案）。
func CreateApiKey(c *gin.Context) {
	var req createApiKeyRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpresp.BadRequest(c, "Key name is required")
		return
	}
	user := middleware.CurrentUser(c)
	if user == nil {
		httpresp.Unauthorized(c, "not authenticated")
		return
	}
	cipher, ok := requireCipher(c)
	if !ok {
		return
	}

	plain := strings.TrimSpace(req.Value)
	if plain == "" {
		generated, err := common.GenerateCallerKeyValue()
		if err != nil {
			httpresp.InternalServerError(c, "failed to generate api key")
			return
		}
		plain = generated
	} else {
		normalized, valid := common.NormalizeCallerKeyValue(plain)
		if !valid {
			httpresp.BadRequest(c, "api key value invalid: total length must be at least 16 characters after sk-llmux- prefix is added")
			return
		}
		plain = normalized
	}

	encrypted, err := cipher.Encrypt(plain)
	if err != nil {
		httpresp.InternalServerError(c, "failed to encrypt api key")
		return
	}
	cleanHeaders, err := sanitizeOutboundHeaders(req.Headers)
	if err != nil {
		httpresp.BadRequest(c, "invalid headers: "+err.Error())
		return
	}
	row := &models.ApiKey{
		OwnerID: user.ID,
		Name:    name,
		Key:     encrypted,
		KeyHash: cipher.Hash(plain),
		// Enabled 无 gorm default tag，建库语义「新 key 即可用」在此显式赋值
		Enabled: true,
		// Create 走 struct 路径，serializer:json 自动编码（PATCH map 路径才需手动
		// marshal——两路径不对称由契约测试钉死）；headers 已过写入闸规范化
		Headers: cleanHeaders,
	}
	if err := repos().ApiKey.Create(c.Request.Context(), row); err != nil {
		if errors.Is(err, repository.ErrApiKeyExists) {
			httpresp.Conflict(c, "api key value already exists")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}
	httpresp.SuccessNoStore(c, toApiKeyResponse(row, cipher))
}

// ListApiKeys 返回全部 key（含 disabled）。明文回显端点：no-store 与 Success
// 绑定在 SuccessNoStore 内，不可能只做一半。
func ListApiKeys(c *gin.Context) {
	cipher, ok := requireCipher(c)
	if !ok {
		return
	}
	keys, err := repos().ApiKey.List(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	items := make([]apiKeyResponse, 0, len(keys))
	for i := range keys {
		items = append(items, toApiKeyResponse(&keys[i], cipher))
	}
	httpresp.SuccessNoStore(c, items)
}

// UpdateApiKey 重命名/启停（部分更新）。回显含明文，仍走 no-store。
func UpdateApiKey(c *gin.Context) {
	id, ok := httpx.ParseUintParam(c, "id")
	if !ok {
		return
	}
	// 先取 cipher 再写库：缺密钥不能让更新「已生效但回 500」的半知状态发生
	cipher, ok := requireCipher(c)
	if !ok {
		return
	}
	var req updateApiKeyRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	fields := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			httpresp.BadRequest(c, "Key name cannot be empty")
			return
		}
		fields["name"] = name
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.Headers != nil {
		// 与 Create 同一写入闸（语法关 + 规范化 + 重名拒斥），两入口不留裸通道
		cleanHeaders, err := sanitizeOutboundHeaders(*req.Headers)
		if err != nil {
			httpresp.BadRequest(c, "invalid headers: "+err.Error())
			return
		}
		// UpdateFields 走 map[string]any，GORM serializer:json 对 map-Updates 不生效
		// （已知坑）——序列化收在 handler（DTO 翻译职责），repository 不感知 JSON。
		// 空 map 编码为 "{}" 即「清空」，与 nil（键缺席=不改）语义正交。
		// map[string]string 恒可编码，err 分支纯防御（真触发说明 runtime 故障，非用户输入）。
		encoded, err := json.Marshal(cleanHeaders)
		if err != nil {
			slog.Error("marshal api key headers failed (unreachable for map[string]string)", "api_key_id", id, "error", err)
			httpresp.InternalServerError(c, "failed to encode headers")
			return
		}
		fields["headers"] = string(encoded)
	}
	if len(fields) == 0 {
		httpresp.BadRequest(c, "No fields to update")
		return
	}

	rows, err := repos().ApiKey.UpdateFields(c.Request.Context(), id, fields)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	if rows == 0 {
		httpresp.NotFound(c, "api key not found")
		return
	}
	updated, err := repos().ApiKey.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// update 与回读之间被并发删除：语义是「没了」，不是服务器故障
			httpresp.NotFound(c, "api key not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}
	httpresp.SuccessNoStore(c, toApiKeyResponse(updated, cipher))
}

// DeleteApiKey 硬删（仓储层 Unscoped，防软删残留挡死同值重建）。在途请求不受
// 影响：/v1 鉴权只在入口执行一次（方案 §2 同款语义）。
func DeleteApiKey(c *gin.Context) {
	id, ok := httpx.ParseUintParam(c, "id")
	if !ok {
		return
	}
	rows, err := repos().ApiKey.Delete(c.Request.Context(), id)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	if rows == 0 {
		httpresp.NotFound(c, "api key not found")
		return
	}
	httpresp.Success(c, gin.H{"deleted": rows})
}
