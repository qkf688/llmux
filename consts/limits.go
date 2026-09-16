package consts

// MaxInboundBodyBytes 入站请求体硬上限（20 MiB，Stage 2 资源防御，2026-09-17 共识定值）。
// 合法聊天请求体通常在几百 KB 内，最大常见撑大场景是 base64 图片塞进消息
// （10MB 图编码后约 13MB），20 MiB 覆盖单图大图请求、只挡病态灌包（与 OpenAI
// 自身 ~25MB 请求限同量级）。`/v1/*` 由调用方 API key 鉴权、攻击者可反复触发，
// 超限直接拒绝（413：主流 SDK 不会自动重试 413，不放大灌包）。
// 管理端导入接口复用同一常量（配置导入 payload 远小于此值，只挡异常）。
const MaxInboundBodyBytes = 20 << 20
