package credentials

import "github.com/qkf688/llmux/service/credview"

// CredentialListData 凭据列表分页信封。items 用 credview.View——与供应商分组详情里的
// 内联凭据项同一形状（凭据视图单源），前端两处共用同一套字段与状态展示。
type CredentialListData struct {
	Items    []credview.View `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// CreateCredentialRequest 凭据创建请求体（每条 = 一个明文 key）。
// PoolID 必填：分组内联凭据的唯一写入口是供应商表单同步（providerapi.syncGroups），
// 这里只服务「往号池里加一条 key」——两条路径都能写 group_id 会让内联凭据的归属与
// 去重语义漂移（表单按 hash 复用行、池导入按池内 hash 去重，语义不同源）。
type CreateCredentialRequest struct {
	PoolID uint   `json:"pool_id"`
	Key    string `json:"key"`
	Note   string `json:"note"`
}

// UpdateCredentialRequest 凭据单条更新请求体（note 可清空，status 可切）。
// 均用指针区分「字段缺席」与「显式置空/改值」。
type UpdateCredentialRequest struct {
	Note   *string `json:"note"`
	Status *string `json:"status"`
}

// BatchCredentialStatusRequest 批量启停请求体。
type BatchCredentialStatusRequest struct {
	IDs    []uint `json:"ids"`
	Status string `json:"status"`
}

// BatchCredentialDeleteRequest 批量删除请求体。
type BatchCredentialDeleteRequest struct {
	IDs []uint `json:"ids"`
}

// CredentialRawData 明文查看响应（受控端点 GET /credentials/:id/raw）。
type CredentialRawData struct {
	Key string `json:"key"`
}
