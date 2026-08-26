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

## 5. 特殊约定

- **扩展点分布（现状）**：`models/setting_schema.go` + `handler/settings` DTO（如 `Settings`）+ 前端 `lib/api/.../settings.ts` 与表单 UI；Get/Update **分发**由 schema 驱动，但 **契约字段仍手同步三端**
- 交叉字段归一化（如 `normalizeUpdateSettingsRequest`）可能不在 schema 内，新增约束时需排查
- **热读未完全单路径**：并存 `models.GetSetting*`、`service/settings.Reader`、部分业务包自写 getter（默认值/min 语义可能不一致）
- 键名使用 `SettingKey*` 常量，禁止魔法字符串散落
- **思考档位相关设置（Stage B 扩档）**：
  - `SettingKeyReasoningEffortDefaultValue`：Type=string，Default=`low`，Enum=`[minimal, low, medium, high, xhigh, max]`（6 档，不含 none/auto）。用于 `auto` 不支持且白名单空时的兜底 + `unknownStrategy=clamp_to_default` 时的回退值。
  - `SettingKeyReasoningEffortUnknownStrategy`：Type=string，Default=`clamp_to_default`，Enum=`[clamp_to_default, passthrough]`。用于请求中的 reasoning_effort 不在模型白名单时的处理策略。
  - 消费方：`service/chat` 的 `buildThinkingClampConfig`（`chat_settings.go`）经 `settingsReader` 读取这两个设置 + `ThinkingLevelsResolved` 白名单，构建 `transform.ThinkingClampConfig` 传入 chat 主路径钳制。
- **请求参数相关设置（全局超时/重试，2026-08 替代原 per-model time_out/max_retry）**：
  - `SettingKeyRequestHeaderTimeout`：int，Default=`30`，Min=1。单次尝试等上游响应头的窗口（`providers` 的 `ResponseHeaderTimeout`）。
  - `SettingKeyRequestTotalTimeout`：int，Default=`90`，Min=1。整个请求的预算上限（真实路径=重试循环 Deadline；虚拟路径=跨真实模型共享 globalTimer）。
  - `SettingKeyStreamFirstByteTimeout`：int，Default=`15`，Min=1。流式响应头后首字节等待窗口，驱动 `handler/v1` 的首字节看门狗。
  - `SettingKeyRequestMaxRetry`：int，Default=`3`，Min=1。单候选池尝试次数。
  - 消费方：`service/chat` 的 `getRequestHeaderTimeout / getRequestTotalTimeout / GetStreamFirstByteTimeout / getRequestMaxRetry`（`chat_settings.go`）经 `settingsReader` 读取；`handler/v1` 经 `service` facade 读首字节等待。响应头超时与总超时**解耦**：等头超时不再拖垮后续候选窗口（`chat_balance.go` / `chat_balance_virtual.go`）。

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
