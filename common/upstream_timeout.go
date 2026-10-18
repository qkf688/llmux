package common

import "time"

// 诊断类上游请求的时间预算单源（Stage 19）。
//
// 为什么集中：健康检查与两处 Models() 曾各写一份 30*time.Second——今天恰好一致，明天任何一处
// 单独改值就会出现「同一条链路探针说超时、健康检查说成功」这类互相矛盾的结论。诊断族
// （service/healthcheck / service/credprobe / providers / handler/testapi / handler/pools）
// 的时间字面量被机械门 gate_diag_timeout_test.go 禁写（含 const/var 声明行）：新增诊断时间
// 预算一律在此登记「值 + 为什么是这个值」，调用点只引名字，漂移只剩「刻意改常量」这一处。
// 例外（非诊断超时语义，如 Transport 调优、单位换算）走行级标记 //gate:diag-timeout-allow <理由>。
//
// 生产聊天超时不在此列：它来自设置项 SettingKeyRequestHeaderTimeout（流式等头，默认 30s）
// 与 SettingKeyNonStreamResponseTimeout（非流式整段预算，默认 90s，Stage 38 起按入站
// stream 分流），两者语义独立，不走这一族。
const (
	// DiagnosticResponseHeaderTimeout 诊断类请求的 transport（响应头）超时：healthcheck 与
	// 两处 Models() 拉取共用的默认。client 整体超时恒 0（providers/cache.go），这个值管
	// 「多久收到上游第一个响应头」。
	DiagnosticResponseHeaderTimeout = 30 * time.Second

	// DiagnosticModelsProbeTimeout Models 轻探（credprobe.probeModels）的整体超时兜底：
	// 探活只求「key 活着」，刻意短于聊天预算。
	DiagnosticModelsProbeTimeout = 5 * time.Second

	// DiagnosticCompletionTimeout completion / 流式探针的整体超时兜底（Timeout 未显式传时）。
	// 调用方应按业务显式传值（见 DiagnosticCredentialProbeTimeout / DiagnosticConnectivityTestTimeout）。
	DiagnosticCompletionTimeout = 30 * time.Second

	// DiagnosticTransportBuffer transport（响应头）超时相对 ctx deadline 的余量：刻意让
	// ctx 先于 transport 超时，错误归类（ErrorTypeTimeout）才稳定落在 ctx 侧。
	DiagnosticTransportBuffer = 1 * time.Second

	// DiagnosticCredentialProbeTimeout pools「按凭据测试」的整体超时：批量点测要快速返回，
	// 刻意短于连通测试预算。
	DiagnosticCredentialProbeTimeout = 15 * time.Second

	// DiagnosticConnectivityTestTimeout testapi 连通测试（completion 探针）的整体超时。
	DiagnosticConnectivityTestTimeout = 60 * time.Second

	// DiagnosticReactHeaderTimeout testapi ReAct 流式长任务的 transport（响应头）超时：
	// 刻意大于诊断默认（30s），兼容慢启动的流式上游。
	DiagnosticReactHeaderTimeout = 60 * time.Second

	// DiagnosticProtocolProbeTimeout testapi 协议支持检测的单条探针超时。一次检测要并发
	// 打 3 条协议 × 3 类探针（非流式 / 流式 / 无效模型指纹），沿用连通测试的 60s 会让
	// 串行段最坏拖到数分钟；每条只求「这条路通不通」而非生成质量，故刻意压到 30s。
	// 超时只判该协议本次不可用，不改任何配置、无副作用。
	DiagnosticProtocolProbeTimeout = 30 * time.Second
)
