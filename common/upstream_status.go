package common

import "net/http"

// IsPassingStatus 判定上游响应状态码是否算「这次请求成功」——全项目唯一口径。
//
// 为什么是 [200,300) 而不是严格等于 200：OpenAI 兼容网关中有相当一部分照抄 OpenAI
// 某些接口的习惯回 201，它们工作正常、内容有效。旧的严格口径把这类供应商判为失败，
// 代价不止诊断报错——健康检查的失败累计会自动停用模型-供应商关联，线上白白少一条
// 可用线路，且停用原因不入库，事后只能从错误文本反推（Stage 5，2026-09-17 共识：
// 口径统一到宽松）。
//
// 为什么落在 common 而不是由 service/credprobe 导出：本判定被 7 处共用，其中两处在
// providers 层（拉模型列表），按依赖方向 providers 禁止 import service 子包；跨 2 个
// 以上域的纯逻辑必须下沉 common（AGENTS §4.4）。
//
// 边界：本函数**只管状态码**，不判响应内容。上游回 2xx 但内容并非一份合法回复
// （被重定向到登录页、回一页 HTML 错误页那类）不在此拦截——各调用点对内容有效性的
// 要求本就不同（聊天要能解析成回复、模型列表要能解析成清单、流式测试要看到真在吐字），
// 捆进同一个判定会让改聊天校验连带牵动模型列表（Stage 5 共识第 4 条，已记为残留待办）。
//
// 配套执法：非测试代码禁止再对上游状态码做裸比较判通过/失败，见 gate_upstream_status_test.go。
func IsPassingStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
