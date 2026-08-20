# 模块间交互方式

> 本文件描述模块之间的交互方式。各模块详情见 `modules/{x}.md`；全局架构见 [README.md](README.md)。
> 每种交互方式**一句话说明 + 一个最小示例**，不展开多步流程。

## 1. 同步调用

上层通过模块暴露的 Service / Handler 门面调用下层。

```
handler/v1 → service.BalanceChat / Beforer / Processer
  → service/chat → (可选) virtualmodel.SelectRealModelsOrdered
  → transform.ProcessRequest → providers.BuildReq → 上游 HTTP
handler/providerapi → repository.ProviderRepo / models.DB
handler/healthcheck → service.GetHealthChecker()
handler/modelsync → service.NewModelSyncService
```

## 2. 异步 / 后台任务

无领域事件总线；跨请求的异步以进程内后台 goroutine 为主。

```
main → service.GetHealthChecker().Start(ctx)      # 定时健康检查
main → service.NewModelSyncService(...).StartAutoSync(ctx)  # 定时模型同步
```

## 3. Hook 回调（跨域解耦）

跨 service 包的副作用通过 Hook 接口注入，避免循环依赖。契约由**调用方包**定义，实现侧包级注入。

```
healthcheck → AdjustmentHooks → service/adjustment 调整权重/优先级
  （关联 Status / ConsecutiveFailures 仍由 healthcheck 直接写库；hooks 由 adjustment init 注入）
modelsync   → ActionHooks     → service/autoassoc.Associate / CleanInvalid
  （HTTP / provider CRUD 同走 service/autoassoc，见 associations 模块）
```

## 4. 注册表扩展（OCP）

在 `init()` 或显式 Register 中绑定实现，调用方只查表。**注册表不等于「只改一个文件」**——各扩展点实际涉及的位置见对应 `modules/{x}.md`；扩展操作步骤见 [../guides/extension.md](../guides/extension.md)。

```
providers.Register + RegisterMetadata          # 工厂层 OCP；外围或有 type 分支
chat.RegisterBeforer / RegisterProcesser       # 须配合 v1 路由与 transform
transform.RegisterAdapter (+ streaming.RegisterRealtimeRoute)
virtualmodel.RegisterSelector                  # 分发 OCP；算法可能仍在 Service
handler 子包 Register* → 修改 handler.RegisterAll 挂接
webui route-config.ts 追加路由项
```

## 5. 共享模型

跨模块共享的类型集中定义，避免重复与 N×M 爆炸。

```
models/unified/* + models/unified.go  # 请求侧多在 unified/；部分响应类型仍在 unified.go 门面
models.Setting + setting_schema.go    # 设置 KV + 元数据（DTO 与前端类型仍手同步）
models.Model / Provider / ModelWithProvider / VirtualModel* / ChatLog*  # 持久化实体
```

## 6. 配置驱动

运行时行为优先读设置表 / 环境变量，而非改代码。

```
环境变量 TOKEN / GIN_MODE / TZ
models.Setting（schema 默认值 + 管理端 /settings）
Provider.Config / Proxy / Blacklisted
VirtualModel.Strategy、ModelWithProvider.Weight/Priority/Status
```

## 7. 门面与默认注入

入口绑定默认实现，handler 依赖根包门面而非深层实现路径。

```
main → models.Init + repository.SetDefault
handler → service 门面（chat_facade / transform_facade / virtual_model / healthcheck / model_sync）
  # 门面 = 兼容 re-export / 装配，不是第二套业务逻辑
handler → repository.Default()（repos.go）  # service 侧 chat/chatstats/healthcheck/adjustment 亦同；存量 models.DB 直连并存
```

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-08-21*
