package credentials

import "github.com/qkf688/llmux/service/credview"

// CredentialListData 凭据列表分页信封。items 用 credview.View（凭据视图单源，
// 与号池详情共用同一套字段与状态展示）。
type CredentialListData struct {
	Items    []credview.View `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// CreateCredentialRequest 凭据创建请求体（每条 = 一个明文 key）。
// PoolID 必填：凭据一律归属号池，供应商分组经 KeyGroup.PoolID 引用号池取凭据，
// 供应商表单自身不持有凭据。
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
