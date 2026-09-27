# channel-routing 模块（供应商内部三层选路）

> 供应商号池体系的上层消费方：把编号池（pools 模块）的端点/分组/凭据数据选成一次可发起的上游请求。权威功能设计见 `.local/discussions/provider-channels/04-设计定案.md` 第 4/6 节（实施顺序见其中第 10 节 stage 表）。全局架构见 [README.md](../README.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 供应商内部三层选路：协议端点匹配（透传/转换）→ 分组白名单+价格权重 → 凭据轮询+冷却剔除，输出动态 config |
| 目录位置 | `service/channel/`；数据来源 `repository/`（endpoint.go / key_group.go / credential.go）；映射常量 `consts/protocol.go`；解密 `common/credentialcrypto` |
| 主要职责 | 把「请求模型 + 入站协议」选成一次具体上游调用所需的（端点、分组、凭据、动态 config）命中组合；轮询指针进程内内存态 |

## 2. 职责与边界

- **对外暴露**：`Selector.Select(snapshot, clientWire, upstreamModel, now, prefer) → SelectionResult`（S3-2 chat 链路接入点；`upstreamModel` = 上游模型名，chat 链路传关联的 `ProviderModel`，见 §5；`prefer` 为缓存亲和钉定偏好，零值 = 不钉、行为与历史一致）；`Selector.RetryCredential(snapshot, groupID, endpoint, now) → SelectionResult`（S3-3 组内故障转移：锁组/端点重选凭据）；`Assembler.Assemble(ctx, provider) → Snapshot`（装配输入）；**探活/诊断口径**（#20 起为同步、管理端浏览、credprobe、健康检测（#21）、testapi 连通测试（#19）共用）：`SelectPrimaryEndpoint` + `GroupCandidates` / `GroupCandidatesForModel` + `NewClientForGroup` / `ProbeGroup`（内含 `FirstUsableCredential` + `NewClientFromSelection`，→ 可直接打上游的客户端：`Models()` 或 `BuildReq` + `Do`）。**#24 按凭据测试是个例外**：只借 `SelectPrimaryEndpoint` 定位端点，凭据装配直取 `NewClientFromSelection`（喂指定那一条 key），**不经** `ProbeGroup`/`FirstUsableCredential`——后二者的「组内首个可用」语义会把坏 key 误判成同组另一把好 key 的健康，逐 key 探测必须绕开它们
- **依赖谁**：`repository.*`（构造注入 `*repository.Repositories`，不取包级默认）；`models` 四表实体；`consts.Protocol*`/`WireFormatOfProtocol`/`TypeOfProtocol`；`common/credentialcrypto.Default()`（main 装配注入）；`providers`（仅 `NewClientFromSelection` 用 `providers.New` 造客户端）
- **不负责什么**：发上游请求与协议字段转换（调用方各自决定打什么端点、超时多少）；透传判定与转换链路编排（chat 链路消费 SelectionResult 后自判）；凭据状态机写入（CooldownUntil 只读不写，S4 接管）；请求日志落库（#13 消费命中信息）
- **依赖方向**：`service/channel` → `providers` 是合法下沉方向（`service` → `providers`），且 channel 只**装配**客户端（选路产物 → `providers.New`），不发起调用——调用与超时留在 credprobe/modelsync/handler 侧

## 3. 内部结构

```
service/channel/
  types.go                # Snapshot（装配产出）/ SelectionResult（命中+动态 config）/ Prefer（缓存亲和钉定偏好）/ 三层 sentinel errors
  assemble.go             # Assembler: repository → Snapshot（凭据经 ListByGroups 按分组/号池收敛，三路数据 ID ASC 稳定）
  selector.go             # Select / RetryCredential 编排门面（三层 + 解密 + config 尾部复用）+ DefaultSelector 单例
                          # selectCredentialWithPrefer：钉定凭据须在可调度候选内、命中不推进轮询；失效回退 SelectCredential
  endpoint_select.go      # SelectEndpoint：入站 wire ∈ enabled 端点协议 → 透传路径；否则主协议端点（转换路径）
                          # SelectPrimaryEndpoint：诊断口径——主协议启用端点取 ID 最小，不碰轮询状态
  group_select.go         # SelectGroup：白名单过滤（空=不限）→ weight 聚合档 → 档间加权随机 → 档内轮询
                          # selectGroupWithPrefer：钉定组须过同套桶过滤 + 至少一条可调度凭据、命中不推进轮询；失效回退 SelectGroup
                          # GroupCandidates：诊断遍历候选（全部组、ID ASC，不过滤白名单/weight）
                          # GroupCandidatesForModel：连通测试候选（只按白名单过滤、ID ASC，不看 weight、不推进轮询）
  credential_select.go    # SelectCredential：active + 非冷却（CooldownUntil 读判定）→ 组内轮询（选择即推进）
                          # FirstUsableCredential：诊断口径——组内首个可用，确定性且不推进轮询指针
  config_build.go         # BuildConfig：Provider.Config JSON round-trip + base_url 继承链 + api_key 明文；DecryptCredentialKey 解密（供选路与 credprobe 共用）+ 空明文守卫（Stage 27，ErrCredentialNotConfigured）
  probe.go                # NewClientFromSelection：解密 → BuildConfig → DropCustomModels → TypeOfProtocol → providers.New（非 chat 流量唯一的客户端装配点）
                          # NewClientForGroup：诊断口径最小单元 = FirstUsableCredential + NewClientFromSelection（同步 / 浏览 / 健康检测共用）
                          # ProbeGroup：同上但回传链路标识（组 / 凭据 ID / 掩码 key / 实际出站 URL），掩码由包内同一份明文算出（testapi #19 与 #24 用）
                          # DropCustomModels：剥 custom_models，使 Models() 真打上游
  rrstate.go              # Selector: 轮询指针容器（sync.Mutex + map，分组档/凭据组两维）
                          # 测试：endpoint_group_select_test.go / credential_select_test.go / select_prefer_test.go /
                          #       config_build_test.go / assemble_test.go / rrstate_test.go / probe_test.go
consts/protocol.go        # Protocol* 常量 + ProtocolOfType（type→protocol）+ TypeOfProtocol / WireFormatOfProtocol（反向映射）
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Assembler.Assemble(ctx, provider) → *Snapshot` | 一次拉取端点/分组/凭据归并成快照；端点/分组/凭据统一按 ID ASC 排序（轮询取模基线）；凭据按本供应商分组与号池的 ID 集合收敛（`CredentialRepo.ListByGroups`），内联与号池两侧归入同一分组 | `service/channel/assemble.go` | `repository.Endpoint/KeyGroup/Credential` |
| `Selector.Select(snapshot, clientWire, upstreamModel, now, prefer) → SelectionResult` | 三层编排：端点 → 分组 → 凭据 → 解密 → config；每层错误携带 sentinel。`prefer`（缓存亲和，2026-09-23）是**偏好覆盖**：分组/凭据两层命中可用即直选且不推进轮询指针，失效静默逐层回退常规选择，不改成败判定；端点层无偏好（对同一 wire 恒确定，见 §5）；`upstreamModel` 是**上游模型名**（= `ModelWithProvider.ProviderModel`，白名单命名空间，见 §5） | `service/channel/selector.go` | 同包三层函数 |
| `Selector.RetryCredential(snapshot, groupID, endpoint, now) → SelectionResult` | 组内重选凭据（#13 故障转移）：锁**原分组/原端点**重选下一条可用凭据（冷却中的被剔除、轮询指针推进）；返回新的 `SelectionResult`（含按新凭据重建的 `Config`/`UpstreamURL`）；组内无可用 → `ErrNoCredentialAvailable` 上抛，由 chat 链路做组织级淘汰（设计定案「分组失败 → 供应商整体失败」，本层不换组） | `service/channel/selector.go` | 复用 `SelectCredential` + `DecryptCredentialKey` + `BuildConfig`（与 `Select` 同 tail） |
| `SelectEndpoint` | 同协议 wire 匹配选端点（透传）；无匹配回落主协议端点（`ProtocolOfType(Provider.Type)`，转换路径）；disabled 与未知协议端点不参与；无可用 → `ErrEndpointUnavailable` | `service/channel/endpoint_select.go` | 同文件 |
| `SelectGroup` | 白名单过滤（空=不限；白名单 = **上游模型名**，见 §5）→ 按 weight 聚合档 → `balancer.WeightedRandom` 档间随机 → 档内轮询；weight<=0 不参与；无命中 → `ErrNoGroupMatches` | `service/channel/group_select.go` | `balancer.WeightedRandom`（复用） |
| `SelectCredential` | active 且非冷却（`CooldownUntil > now` 剔除）→ 组内轮询选择即推进；空/全不可用 → `ErrNoCredentialAvailable` | `service/channel/credential_select.go` | `Selector.nextCredentialInRR` |
| `BuildConfig` | Provider.Config round-trip 保留未知字段；base_url 继承链（端点 URL 非空覆盖，否则原 base_url）；api_key 恒为选中凭据明文；两者皆空报错；返回 (config, upstreamURL) | `service/channel/config_build.go` | 同文件 |
| `DecryptCredentialKey` | 解密凭据明文；`credentialcrypto.Default()==nil` 明确报错（无降级）；解密成功但明文为空（含全空白）→ 包装 `ErrCredentialNotConfigured` 报错（Stage 27 空凭据守卫：空明文 = 配置缺失，放行即带空凭据头打上游挨 401，把配置问题伪装成上游鉴权失败；判据在解密后单点，四条装配路径自动同享）；选路与 `service/credprobe` 共用 | `service/channel/config_build.go` | `credentialcrypto.Default()` |
| `SelectPrimaryEndpoint(providerType, endpoints) → (Endpoint, bool)` | **诊断口径端点**：启用 + 协议 = `ProtocolOfType(providerType)` + ID 最小（入参 ID ASC 由 Assembler 保证）。不做入站 wire 匹配、不推进轮询状态；返回 bool 让调用方各自措辞（同步 `SkipReason` / 浏览报错 / 健康检测与连通测试带供应商名） | `service/channel/endpoint_select.go` | modelsync（同步 + 浏览）、`service/healthcheck.doCheck`、`handler/testapi`（#19）；#24 按凭据测试 |
| `GroupCandidates(snapshot) → []KeyGroup` | **诊断遍历分组候选**：全部组、ID ASC 副本。刻意不过滤白名单（白名单正是靠这次同步才填上，过滤＝先有鸡还是先有蛋）也不看 weight（`weight<=0` 只表示不参与生产选路，其凭据仍是真 key）；返回副本防调用方污染快照 | `service/channel/group_select.go` | 同上 |
| `GroupCandidatesForModel(snapshot, upstreamModel) → []KeyGroup` | **连通测试的分组候选**：只按白名单过滤（空=不限；与生产选路同一**上游模型名**命名空间，调用方传 `ProviderModel`）+ 限本供应商 + ID ASC；不排除 `weight<=0`（权重是流量分摊不是可用性），也不推进轮询。与 `SelectGroup`（生产会不会走）/`GroupCandidates`（哪些组值得逐个拉目录）三者语义各不同，合并任一条都会让另一条失真 | `service/channel/group_select.go` | `handler/testapi.ResolveAssociationLink`（关联级三端点） |
| `FirstUsableCredential(snapshot, groupID, now) → Credential` | **诊断口径凭据**：组内首个可用（跳过 disabled/error/temp_unsched/冷却），连续调用结果不漂移，且**不推进** `Selector` 轮询指针（管理员一次点击不得挪动生产 key 分配）。代价已知并接受：只验第一把，逐 key 健康是 credprobe / #24 的职责。无可用 → `ErrNoCredentialAvailable`（与 `SelectCredential` 同一 sentinel 与文案） | `service/channel/credential_select.go` | 同上 |
| `NewClientFromSelection(provider, endpoint, cred) → providers.Provider` | 非 chat 流量**唯一**客户端装配点：解密 → `BuildConfig` → `DropCustomModels` → `TypeOfProtocol` → `providers.New`。绕过它直用 `Provider.Config` 即不带 Authorization 打上游 → 恒 401（S6 起 config 不含 api_key）。未知端点协议显式失败；只装配不发请求（超时/调用由 caller 决定） | `service/channel/probe.go` | modelsync（经 `NewClientForGroup`）、`service/credprobe.probeModels`、`service/healthcheck.doCheck`、`handler/testapi`（经 `ProbeGroup`） |
| `NewClientForGroup(snapshot, group, endpoint, now) → providers.Provider` | **逐组诊断的最小单元**：`FirstUsableCredential` → `NewClientFromSelection` 这两步在任何调用方写出来都一样，收成一份以免漂移（副本会导致有的路径推进轮询、有的不推进）。组内无可用凭据 → 透传 `ErrNoCredentialAvailable`，调用方按「跳过该组、继续其余组」处理 | `service/channel/probe.go` | modelsync（`syncKeyGroup` + `ListUpstreamModels`）、`service/healthcheck.doCheck` |
| `ProbeGroup(snapshot, group, endpoint, now) → GroupProbe` | 同 `NewClientForGroup`，额外回传**实际用到的链路标识**（`Group` / `CredentialID` / `KeyMasked` / `UpstreamURL` / `Protocol`）：管理端诊断的失败响应只显示到分组一层（C2 定案），「用的哪把 key」只剩日志这一条线索。明文 key 收在函数内、只对外暴露掩码（与喂进 config 的是同一份解密结果，不出现第二条解密路径）。解密命中 `ErrCredentialNotConfigured`（空明文，Stage 27）→ 转 `ErrNoCredentialAvailable` 系可跳组失败（文案「未配置 API Key」）；其余解密失败保持全局性失败直接上抛 | `service/channel/probe.go` | `handler/testapi.buildLink`（#19）。（#24 按凭据测试**不经**此函数——它要测指定 key 而非组内首个可用，直接走 `NewClientFromSelection`） |
| `DropCustomModels(config) → string` | 剥 `custom_models` 使 `Models()` 真打上游（不剥则 openai/anthropic 本地短路返回自定义列表 → 管理端浏览拿不到上游授权集、探活把失效 key 误判成功并误恢复 active）。空串/无该键原样返回（不重跑 marshal 改键序） | `service/channel/probe.go` | 同文件 |
| `consts.TypeOfProtocol` / `WireFormatOfProtocol` | 端点协议 → provider 注册类型 / wire format（responses ↔ openai-res；未知协议不兜底）；与 `ProtocolOfType` 反向 | `consts/protocol.go` | 同文件 |
| 轮询指针语义 | 进程内内存态 + `sync.Mutex`；选择即推进单轨（无成功后回调——凭据层失败转移本就跳选）；多实例部署不同步可接受 | `service/channel/rrstate.go` | 同文件 |

## 5. 特殊约定

- **错误分层语义**：三层 sentinel（`ErrEndpointUnavailable` / `ErrNoGroupMatches` / `ErrNoCredentialAvailable`）供 chat 链路按层处置——凭据层失败由 #13 组内故障转移消化（`RetryCredential` 换 key，冷却写库在 chat 侧），`ErrNoCredentialAvailable` 上抛即组耗尽 → 组织级淘汰；分组/端点层耗尽同样判供应商整体失败；`ConsecutiveFailures` 组织级语义与凭据级失败不重叠（#13 起层内耗尽才累计）。`ErrCredentialNotConfigured`（Stage 27 空明文守卫）包装在 `ErrNoCredentialAvailable` 系语义下：调用方 `errors.Is` 判「无可用凭据」时同时命中，处置完全一致（本地失败、零上游请求）
- **无 DB/无时钟依赖**：选择函数输入为内存快照，冷却判定时钟由 `Select` 的 `now` 参数注入；IO 收敛在 `assemble.go`（读库）与 `DecryptCredentialKey`（解密单例）；`RetryCredential` 不写冷却——CooldownUntil 与凭据状态的写库是 chat 链路职责，且经 `service/credwrite` 队列异步落库（冷却 `chat_credential_cooldown.go`；鉴权判停 `chat_credential_auth_fail.go`，#6-2 起连败达阈值写 `temp_unsched`；探活恢复 `service/credprobe`，#6-3），本模块只读判定（`credentialUsable` 按 Status≠active 与冷却剔除，新增状态枚举无需改本模块）；`probe.go` 是本包唯一 import `providers` 的文件，且只**装配**客户端（`providers.New`），打上游与超时策略留在调用方（credprobe/modelsync/handler）
- **两套口径不可混用（#20 立）**：**生产选路**＝`Select` / `RetryCredential`（入站 wire 匹配 + 白名单 + 加权随机 + 轮询推进，必须复用 `DefaultSelector()` 才谈得上压力分摊）；**探活/诊断**＝`SelectPrimaryEndpoint` + `GroupCandidates`（同步/浏览/健康检测）或 `GroupCandidatesForModel`（连通测试）+ `FirstUsableCredential` + `NewClientFromSelection`（确定性、无状态、不碰 `DefaultSelector()`）。把诊断流量接进生产口径的代价已被论证：管理端批量测试（前端并发 3）会把生产轮询指针按点击次数推进，一次点击静默重排真实流量的 key 分配；且上游目录里的模型名通常不在分组白名单，走 `SelectGroup` 会直接 `ErrNoGroupMatches`，把完全可用的 key 测成失败。诊断口径内部分组候选再分两档，判据是「白名单此刻是不是可信输入」：拉目录/健康检测不能被尚未同步的白名单挡在门外，而连通测试要给的是「生产走得到吗」这个结论
- **白名单命名空间 = 上游模型名（2026-09-26 收口）**：`KeyGroup.Models` 装的是**供应商侧的 model 标识**（`ModelWithProvider.ProviderModel`），四个消费/生产点同一套名字——同步写侧（modelsync 把 `/models` 返回的上游 ID 原样写入）、组织级目录（autoassoc 要求关联的上游名 ∈ 白名单并集 ∪ `custom_models`）、生产选路（chat 链路传 `modelWithProvider.ProviderModel` 给 `Select` 的 `upstreamModel`）、连通测试（`GroupCandidatesForModel`）。传网关模型名（`Model.Name` / 请求里的 `model`）会让重命名关联（`ProviderModel ≠ Model.Name`，该字段的本职用途）在名单非空后恒 `ErrNoGroupMatches` → 候选淘汰 → `/v1` 回 500，而管理端连通测试报「通过」——两套名字必须只留一套。`.local/discussions/provider-channels/04-设计定案.md` §4「请求模型过白名单过滤（空=不限）」的「请求模型」按上游模型名理解（§8「白名单 = 该分组 key 的实际授权模型集」是同义），分歧已消
- **档间加权 = 单组权重而非档总权重**：同权重组由轮询打散为等概率，档权重只表达「某权重档的选中概率」，防止多组低权把单个高权组挤出随机池
- **组内候选顺序稳定性**：装配按凭据 ID ASC 排序，repository List 不保证顺序；轮询取模的正确性依赖该不变式
- **缓存亲和偏好（`Prefer`，2026-09-23）**：`Prefer{GroupID, CredentialID}` 是 chat 层传入的钉定偏好（来源 `balancer` 粘性表，编排单源 `service/chat/chat_affinity.go`）——命中可用即直选且**不推进轮询指针**（钉定流量不得挪动常规流量的分摊位置），失效静默回退该层常规选择。刻意**不含 EndpointID**：端点按「入站 wire 命中或主协议」确定性选择（设计定案不做同协议多 URL），同一对话 wire 不变即天然稳定。钉定组还须**至少一条可调度凭据**（死组不钉，防请求锁死在失败组上）；`RetryCredential` 不接 prefer（钉定凭据刚失败/冷却，锁组重选语义不变）
- **快照语义**：单供应商数据；`CredentialsByGroup` 组键恒存在（含空切片），选择路径统一收口 `ErrNoCredentialAvailable`
- 契约测试分布在同目录 `*_test.go`：白名单/加权分布/轮询互斥并发（`rrstate_test.go` 锁指针精确语义）/冷却到期恢复/config 被 `providers.New` 反序列化/诊断口径确定性（`FirstUsableCredential` 不推进轮询指针、`NewClientFromSelection` 的凭据注入与 `custom_models` 剥离、`GroupCandidatesForModel` 只按白名单过滤且限本供应商，见 `probe_test.go` 与 `endpoint_group_select_test.go`）

---

*本文档随 provider-channels S3-1 落地（2026-08-31）首次建立；S3-2 接入 chat 链路后更新「对外暴露」与 interactions.md；#20（2026-09-09）抽出探活/诊断共享装配后补「两套口径」约定与 `probe.go` 契约；2026-09-23 缓存亲和 `Prefer` 偏好落地（分组/凭据两层，不钉端点）；2026-09-26 白名单命名空间收口为上游模型名（重命名关联此前在生产恒 `no group matches`）。*