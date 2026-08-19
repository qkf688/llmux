package responses

import (
	upstream "github.com/qkf688/llmux/service/responses"
	"github.com/qkf688/llmux/service/transform/shared"
)

// ClaimedRequestKeys 返回本协议入站解析实际认领的顶层键集合。
//
// 注意 upstream.ResponsesRequest 同一个 struct 双向复用（入站 Unmarshal + 出站
// Marshal），所以它的 tag 集合天然既是入站认领键也是出站 emit 键，不存在两侧
// 漂移的可能。
func ClaimedRequestKeys() map[string]struct{} {
	return shared.ClaimedJSONKeys(upstream.ResponsesRequest{})
}
