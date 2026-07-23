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

- **扩展最小改动集（现状）**：`models/setting_schema.go` + `handler/settings` DTO（如 `Settings`）+ 前端 `lib/api/.../settings.ts` 与表单 UI；Get/Update **分发**由 schema 驱动，但 **契约字段仍手同步三端**
- 交叉字段归一化（如 `normalizeUpdateSettingsRequest`）可能不在 schema 内，新增约束时需排查
- **热读未完全单路径**：并存 `models.GetSetting*`、`service/settings.Reader`、部分业务包自写 getter（默认值/min 语义可能不一致）
- 键名使用 `SettingKey*` 常量，禁止魔法字符串散落

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
