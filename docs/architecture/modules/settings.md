# settings 模块

> 本文件描述 Schema 驱动的系统设置域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 运行时配置 KV 与元数据驱动读写 |
| 目录位置 | `handler/settings/`；`service/settings/`；`models/setting_schema.go`、`setting_reader.go`、`setting_convert.go`；`repository/setting.go`；实体 `Setting` |
| 主要职责 | 声明 setting schema；统一默认值；管理端 Get/Update；通用 getter |

## 2. 职责与边界

- **负责什么**：设置项类型/默认/范围/枚举；读写 API；系统配置与健康检查相关设置入口；反射/schema 驱动更新校验
- **不负责什么**：各业务对设置变更的副作用本体（衰减在 chat、探测在 healthcheck、同步在 modelsync）
- **对外暴露**：`SettingSchemas`/`SettingSchemaForKey`/`ApplySettingDefaults`；`service/settings.Store`；`SettingRepo`；`handler/settings` REST
- **依赖谁**：`models`、`repository`、`httpresp`

## 3. 内部结构

```
handler/settings/          # get/update、system、health 设置
service/settings/          # Store / converter / reader
models/setting_schema.go   # schema 单一来源
models/setting_reader.go
models/setting_convert.go
repository/setting.go
models.Setting
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `SettingSchema` / `SettingSchemas` | 设置元数据 | `models/setting_schema.go` | 声明式列表 |
| `settings.Store` | 类型安全读写 | `service/settings/` | 同包 |
| `SettingRepo` | KV 持久化 | `repository/setting.go` | GORM |
| `GetSettingBool/Int/String` | 兼容通用读取 | `models/setting_reader.go` | 同文件 |
| `GetSettingBoolE/IntE`（Stage 33） | 「读失败可辨」读取：查询失败/存值非法 → err，key 不存在 → default（未设置是有效状态） | `models/setting_reader.go` | 同文件 |
| `settings.Reader.BoolE/IntE` + `settings.LogReadSkip`（Stage 33；Stage 36 限频） | Reader 的 E 变体 + 「读失败 → 跳过依赖该配置的写动作」留痕单源。留痕**按设置项每 60 秒最多一条**并带出窗口内被压掉的条数（`suppressed`）——一次请求会走过 5~8 个此类步骤，逐步留痕会让日志量 ≈ 请求量 × 步数。已告知口径：故障持续期总次数可由 `suppressed` 累加还原，但窗口内没有下一条时（故障恰在该窗口结束 / 进程重启）这段计数无下文，要精确总数须另设计数通道 | `service/settings/reader.go` | `modelsReader`；调用方 `service/adjustment`、`service/chat` 凭据结算、`service/healthcheck` |

## 5. 特殊约定

- **扩展点分布（现状）**：`models/setting_schema.go` + `handler/settings` DTO（如 `Settings`）+ 前端 `lib/api/.../settings.ts` 与表单 UI；Get/Update **分发**由 schema 驱动，但 **契约字段仍手同步三端**
- 交叉字段归一化（如 `normalizeUpdateSettingsRequest`）可能不在 schema 内，新增约束时需排查
- **热读未完全单路径**：并存 `models.GetSetting*`、`service/settings.Reader`、部分业务包自写 getter（默认值/min 语义可能不一致）
- 键名使用 `SettingKey*` 常量，禁止魔法字符串散落
- **通用设置响应组装按「目标结构体有没有这个位置」分流（Stage 36，2026-09-30）**：`buildSettingsResponse` 遍历 schema 时先过 `models.SettingFieldPresent`，为假则该键**静默跳过**；判据是「通用响应 DTO 里有没有对应字段」，**不是**按设置域粗筛——健康检测系设置在自己的独立域（`handler/settings/health.go`），在通用页没有位置是已知归属边界，此前却以「failed to set setting field」的形态每键报一条（一次 `GET /settings` 固定刷 7 条 WARN）。静默跳过的风险由**字段契约门**兜住：`TestGenericSettingsResponseFieldContract` 要求每个非独立域的设置项都在响应里有位置，并反向锁定 health 域中与通用页共享的那两个键（`count_health_check_as_success` / `count_health_check_as_failure`，它们**必须**继续出现在通用设置页）——新增设置项时该门会替人挡住漏字段
- **思考档位相关设置（Stage B 扩档）**：
  - `SettingKeyReasoningEffortDefaultValue`：Type=string，Default=`low`，Enum=`[minimal, low, medium, high, xhigh, max]`（6 档，不含 none/auto）。用于 `auto` 不支持且白名单空时的兜底 + `unknownStrategy=clamp_to_default` 时的回退值。
  - `SettingKeyReasoningEffortUnknownStrategy`：Type=string，Default=`clamp_to_default`，Enum=`[clamp_to_default, passthrough]`。用于请求中的 reasoning_effort 不在模型白名单时的处理策略。
  - 消费方：`service/chat` 的 `buildThinkingClampConfig`（`chat_settings.go`）经 `settingsReader` 读取这两个设置 + `ThinkingLevelsResolved` 白名单，构建 `transform.ThinkingClampConfig` 传入 chat 主路径钳制。
- **请求参数相关设置（全局超时/重试，2026-08 替代原 per-model time_out/max_retry）**：
  - `SettingKeyRequestHeaderTimeout`：int，Default=`30`，Min=1。**流式**请求单次尝试等上游响应头的窗口（`providers` 的 `ResponseHeaderTimeout`）。
  - `SettingKeyNonStreamResponseTimeout`：int，Default=`90`，Min=1。**非流式**请求单次尝试等上游返回完整回答的窗口（Stage 38，2026-10-03）：非流式上游要生成完整回答才发响应头，故该窗口语义上等价「整段生成预算」——沿用等头那个 30s，会让平均耗时接近 30s 的慢供应商长期撞线（实测 2.8% 稳定失败率）。**口径边界**：窗口由 transport 的 `ResponseHeaderTimeout` 施加，只覆盖到「收到响应头」为止；上游先发头再慢写正文的形态不受它约束（只受总超时约束）。
  - `SettingKeyRequestTotalTimeout`：int，Default=`600`（Stage 33 起，原 90），Min=1。**整个请求的硬时长上限**（Stage 33）：`handler/v1/chat.go` 入口经 `service.WithRequestTotalTimeout` 挂进请求 ctx 的 deadline，到点含流式输出一刀切、按超时归因结算；旧「重试循环 Deadline 计时器 / 虚拟路径 globalTimer」机制已移除，总上限单源在请求 ctx。
  - `SettingKeyStreamFirstByteTimeout`：int，Default=`15`，Min=1。流式响应头后首字节等待窗口，驱动 `service/chat` 提交门的首字节看门狗。
  - `SettingKeyRequestMaxRetry`：int，Default=`3`，Min=1。单候选池尝试次数。
  - 消费方：`service/chat` 的 `upstreamAttemptWindow`（按入站 `stream` 在 `getRequestHeaderTimeout` / `getNonStreamResponseTimeout` 间二选一，单源在 `chat_settings.go`；真实与虚拟两条路径共用）与 `getRequestTotalTimeout / getStreamFirstByteTimeout / getRequestMaxRetry`，均经 `settingsReader` 读取；总超时经 `chat.WithRequestTotalTimeout`（`service/chat_facade.go` 转发）在 `handler/v1/chat.go` 入口挂进请求 ctx；首字节等待的消费方是本包提交门（`chat_commit_gate.go`，2026-09-22 起看门狗迁入 `service/chat`，`handler/v1` 不再读取）。单次尝试窗口与总超时**解耦**：窗口到点不再拖垮后续候选窗口（`chat_balance.go` / `chat_balance_virtual.go`）；该窗口值同时随尝试传给失败文案（`chat_attempt.go` 的窗口超时归因，见 chat-gateway 模块）。
- **资源防御相关设置（Stage 2，2026-09-17）**：
  - `SettingKeyUpstreamMaxResponseMiB`：int，Default=`32`，Min=`1`。上游单条**非流式**响应体上限（MiB，不可关闭）；消费方 `service/chat` 的 `getUpstreamMaxResponseBytes` → `limitUpstreamResponseBody`（`upstream_body_limit.go`）在 attempt 内整读加限，超限判供应商异常→组织级转移（不冷却凭据）。入站侧不走配置：硬上限 `consts.MaxInboundBodyBytes`（20 MiB）+ 错误体 1 MiB 诊断单源（`common.ReadCappedBody`），防线刻意不可被运营调小。
  - `SettingKeyRawLogFieldMaxKiB`：int，Default=`256`，Min=`16`。原始采集落库文本的单字段上限（KiB），一个设置项盖 `RequestBody`/`RawRequestBody`/`RawResponseBody`/`Error` 四字段；消费方 `getRawLogFieldMaxBytes`（`chat_settings.go`）→ 采集点统一 `common.TruncateUTF8`（rune 边界+标记）。
  - `SettingKeyResponseLogMaxKiB`（Stage 23，2026-09-25）：int，Default=`2048`，Min=`16`，Max=`16384`。回复类日志的单条上限（KiB，「看全文」口径），盖非流式 `ResponseBody` 与流式 `ChatIO` 累积；消费方 `getResponseLogMaxBytes`（`chat_settings.go`）→ `truncateResponseBody` / `output_budget_accumulator` / `capOutputUnion`（`chat_record_persist.go`、`output_budget.go`），超限统一头尾保留 + `common.TruncateHeadTail` 标记。与上一项分工：四字段采集（采样口径）vs 回复整条（全文口径）。
- **缓存亲和相关设置（2026-09-23）**：
  - `SettingKeyCacheAffinityEnabled`：bool，Default=`true`。缓存亲和（粘性选路）总开关：同一对话钉住同一真实模型×关联×分组×凭据，提高供应商提示词缓存命中；关闭 = 选路行为与现状（加权随机+轮询）完全一致。消费方 `service/chat` 的 `getCacheAffinityEnabled`（`chat_settings.go`）→ 编排单源 `chat_affinity.go`（钉定读取与成功改钉的唯一入口）。
- **凭据健康相关设置（S4 #6-1/#6-2/#6-3：可配冷却窗口 + 按失败类型分窗 + 鉴权失败判停阈值 + 惰性探活间隔）**：
  - `SettingKeyCredHealthCooldown429Sec`：int，Default=`60`，Min=1。凭据 429 限流冷却窗口（秒）。
  - `SettingKeyCredHealthCooldownServerSec`：int，Default=`60`，Min=1。凭据服务端错误冷却窗口（秒，5xx/超时/网络）。
  - `SettingKeyCredHealthAuthFailThreshold`：int，Default=`3`，Min=1。凭据连续鉴权失败（401/403）判停阈值：达 N 后 Status=`temp_unsched`（选路剔除，探活恢复为 #6-3），成功请求重置计数。
  - `SettingKeyCredHealthProbeIntervalSec`：int，Default=`60`，Min=1。凭据惰性探活最小间隔（秒）：`temp_unsched` 距 `LastProbeAt` 不足该间隔则跳过探活。
  - 消费方：`service/chat` 的 `getCredHealthCooldown429Sec / getCredHealthCooldownServerSec / getCredHealthAuthFailThreshold / getCredHealthProbeIntervalSec`（`chat_settings.go`）经 `settingsReader` 读取 → `cooldownWindowForReason`（`chat_credential_cooldown.go`）按 reason 分窗写 `CooldownUntil`；判停阈值由凭据结算写桥 `persistCredentialAuthFailure`（`chat_credential_write_bridge.go`，credwrite worker 腿）读取后写 `temp_unsched`（401/403 不走冷却）；探活间隔由 chat 读出后注入 `credprobe.Request.Interval`（#6-3，credprobe 本身不读 settings）；选路侧 `channel` 的 `credentialUsable` 判定冷却中与非 active 状态剔除（`channel-routing.md`）。**Stage 33 显式降级**：前三个读取（读失败会引发写动作）走 E 变体——窗口读失败跳过冷却写、判停阈值读失败只计数不判停（`IncrementFailCountNoStop`），均留痕（`settings.LogReadSkip`）。

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
