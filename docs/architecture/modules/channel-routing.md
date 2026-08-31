# channel-routing 模块（供应商内部三层选路）

> 供应商号池体系的上层消费方：把编号池（pools 模块）的端点/分组/凭据数据选成一次可发起的上游请求。权威功能设计见 `.local/discussions/provider-channels/04-设计定案.md` 第 4/6 节（实施顺序见其中第 10 节 stage 表）。全局架构见 [README.md](../README.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 供应商内部三层选路：协议端点匹配（透传/转换）→ 分组白名单+价格权重 → 凭据轮询+冷却剔除，输出动态 config |
| 目录位置 | `service/channel/`；数据来源 `repository/`（endpoint.go / key_group.go / credential.go）；映射常量 `consts/protocol.go`；解密 `common/credentialcrypto` |
| 主要职责 | 把「请求模型 + 入站协议」选成一次具体上游调用所需的（端点、分组、凭据、动态 config）命中组合；轮询指针进程内内存态 |

## 2. 职责与边界

- **对外暴露**：`Selector.Select(snapshot, clientWire, modelName, now) → SelectionResult`（S3-2 chat 链路接入点）；`Selector.RetryCredential(snapshot, groupID, endpoint, now) → SelectionResult`（S3-3 组内故障转移：锁组/端点重选凭据）；`Assembler.Assemble(ctx, provider) → Snapshot`（装配输入）
- **依赖谁**：`repository.*`（构造注入 `*repository.Repositories`，不取包级默认）；`models` 四表实体；`consts.Protocol*`/`WireFormatOfProtocol`；`common/credentialcrypto.Default()`（main 装配注入）
- **不负责什么**：上游请求构造与协议适配（providers）；透传判定与转换链路编排（chat 链路消费 SelectionResult 后自判）；凭据状态机写入（CooldownUntil 只读不写，S4 接管）；请求日志落库（#13 消费命中信息）
- **依赖方向**：`service/channel` 不 import `providers`——config 组装是 JSON 字符串操作，产物可被 `providers.New` 反序列化即可

## 3. 内部结构

```
service/channel/
  types.go                # Snapshot（装配产出）/ SelectionResult（命中+动态 config）/ 三层 sentinel errors
  assemble.go             # Assembler: repository → Snapshot（凭据经 ListByGroups 按分组/号池收敛，三路数据 ID ASC 稳定）
  endpoint_select.go      # SelectEndpoint：入站 wire ∈ enabled 端点协议 → 透传路径；否则主协议端点（转换路径）
  group_select.go         # SelectGroup：白名单过滤（空=不限）→ weight 聚合档 → 档间加权随机 → 档内轮询
  credential_select.go    # SelectCredential：active + 非冷却（CooldownUntil 读判定）→ 组内轮询（选择即推进）
  config_build.go         # BuildConfig：Provider.Config JSON round-trip + base_url 继承链 + api_key 明文；plainCredentialKey 解密
  rrstate.go              # Selector: 轮询指针容器（sync.Mutex + map，分组档/凭据组两维）+ Select 编排门面
                          # 测试：endpoint_group_select_test.go / credential_select_test.go /
                          #       config_build_test.go / assemble_test.go / rrstate_test.go
consts/protocol.go        # Protocol* 常量 + ProtocolOfType（type→protocol）+ TypeOfProtocol / WireFormatOfProtocol（反向映射）
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Assembler.Assemble(ctx, provider) → *Snapshot` | 一次拉取端点/分组/凭据归并成快照；端点/分组/凭据统一按 ID ASC 排序（轮询取模基线）；凭据按本供应商分组与号池的 ID 集合收敛（`CredentialRepo.ListByGroups`），内联与号池两侧归入同一分组 | `service/channel/assemble.go` | `repository.Endpoint/KeyGroup/Credential` |
| `Selector.Select(snapshot, clientWire, modelName, now) → SelectionResult` | 三层编排：端点 → 分组 → 凭据 → 解密 → config；每层错误携带 sentinel | `service/channel/selector.go` | 同包三层函数 |
| `Selector.RetryCredential(snapshot, groupID, endpoint, now) → SelectionResult` | 组内重选凭据（#13 故障转移）：锁**原分组/原端点**重选下一条可用凭据（冷却中的被剔除、轮询指针推进）；返回新的 `SelectionResult`（含按新凭据重建的 `Config`/`UpstreamURL`）；组内无可用 → `ErrNoCredentialAvailable` 上抛，由 chat 链路做组织级淘汰（设计定案「分组失败 → 供应商整体失败」，本层不换组） | `service/channel/selector.go` | 复用 `SelectCredential` + `plainCredentialKey` + `BuildConfig`（与 `Select` 同 tail） |
| `SelectEndpoint` | 同协议 wire 匹配选端点（透传）；无匹配回落主协议端点（`ProtocolOfType(Provider.Type)`，转换路径）；disabled 与未知协议端点不参与；无可用 → `ErrEndpointUnavailable` | `service/channel/endpoint_select.go` | 同文件 |
| `SelectGroup` | 白名单过滤（空=不限）→ 按 weight 聚合档 → `balancer.WeightedRandom` 档间随机 → 档内轮询；weight<=0 不参与；无命中 → `ErrNoGroupMatches` | `service/channel/group_select.go` | `balancer.WeightedRandom`（复用） |
| `SelectCredential` | active 且非冷却（`CooldownUntil > now` 剔除）→ 组内轮询选择即推进；空/全不可用 → `ErrNoCredentialAvailable` | `service/channel/credential_select.go` | `Selector.nextCredentialInRR` |
| `BuildConfig` | Provider.Config round-trip 保留未知字段；base_url 继承链（端点 URL 非空覆盖，否则原 base_url）；api_key 恒为选中凭据明文；两者皆空报错；返回 (config, upstreamURL) | `service/channel/config_build.go` | 同文件 |
| `plainCredentialKey` | 解密凭据明文；`credentialcrypto.Default()==nil` 明确报错（无降级） | `service/channel/config_build.go` | `credentialcrypto.Default()` |
| `consts.TypeOfProtocol` / `WireFormatOfProtocol` | 端点协议 → provider 注册类型 / wire format（responses ↔ openai-res；未知协议不兜底）；与 `ProtocolOfType` 反向 | `consts/protocol.go` | 同文件 |
| 轮询指针语义 | 进程内内存态 + `sync.Mutex`；选择即推进单轨（无成功后回调——凭据层失败转移本就跳选）；多实例部署不同步可接受 | `service/channel/rrstate.go` | 同文件 |

## 5. 特殊约定

- **错误分层语义**：三层 sentinel（`ErrEndpointUnavailable` / `ErrNoGroupMatches` / `ErrNoCredentialAvailable`）供 chat 链路按层处置——凭据层失败由 #13 组内故障转移消化（`RetryCredential` 换 key，冷却写库在 chat 侧），`ErrNoCredentialAvailable` 上抛即组耗尽 → 组织级淘汰；分组/端点层耗尽同样判供应商整体失败；`ConsecutiveFailures` 组织级语义与凭据级失败不重叠（#13 起层内耗尽才累计）
- **无 DB/无时钟依赖**：选择函数输入为内存快照，冷却判定时钟由 `Select` 的 `now` 参数注入；IO 收敛在 `assemble.go`（读库）与 `plainCredentialKey`（解密单例）；`RetryCredential` 不写冷却——CooldownUntil 与凭据状态的写库是 chat 链路职责（冷却 `chat_credential_cooldown.go`；鉴权判停 `chat_credential_auth_fail.go`，#6-2 起连败达阈值写 `temp_unsched`），本模块只读判定（`credentialUsable` 按 Status≠active 与冷却剔除，新增状态枚举无需改本模块）
- **档间加权 = 单组权重而非档总权重**：同权重组由轮询打散为等概率，档权重只表达「某权重档的选中概率」，防止多组低权把单个高权组挤出随机池
- **组内候选顺序稳定性**：装配按凭据 ID ASC 排序，repository List 不保证顺序；轮询取模的正确性依赖该不变式
- **快照语义**：单供应商数据；`CredentialsByGroup` 组键恒存在（含空切片），选择路径统一收口 `ErrNoCredentialAvailable`
- 契约测试分布在同目录 `*_test.go`：白名单/加权分布/轮询互斥并发（`rrstate_test.go` 锁指针精确语义）/冷却到期恢复/config 被 `providers.New` 反序列化

---

*本文档随 provider-channels S3-1 落地（2026-08-31）首次建立；S3-2 接入 chat 链路后更新「对外暴露」与 interactions.md。*