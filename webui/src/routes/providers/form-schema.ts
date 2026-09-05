import { z } from "zod";

/** 支持类型 = provider type 单选（主类型）+ 出站协议多选；type 与 protocol 一一对应。
 * schema 的 superRefine 与 SupportTypesField 共用本表，避免两处定义漂移。 */
export const SUPPORTED_TYPE_OPTIONS = [
  { type: "openai", protocol: "openai", label: "OpenAI（chat/completions）" },
  { type: "openai-res", protocol: "responses", label: "OpenAI Responses" },
  { type: "anthropic", protocol: "anthropic", label: "Anthropic" },
] as const;

/** 协议 → 展示名（端点行 / 主类型展示） */
export const PROTOCOL_LABEL: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  responses: "OpenAI Responses",
};

/** 协议端点：protocol × url × enabled（同协议多 URL = 多条记录，启停由用户配置决定） */
export const providerFormEndpointSchema = z.object({
  protocol: z.string().min(1),
  /** 留空 = 继承 Provider 默认 Base URL；填 = 覆盖（如 anthropic → …/claude/v1） */
  url: z.string(),
  enabled: z.boolean(),
});

export type ProviderFormEndpoint = z.infer<typeof providerFormEndpointSchema>;

/** 凭据分组：名称 × 价格权重 × 模型白名单 × 凭据来源（内联 Key / 关联号池二选一） */
export const providerFormGroupSchema = z.object({
  name: z.string().min(1, { message: "分组名称不能为空" }),
  /** 价格导向权重：便宜的分组权重高 → 加权随机命中概率大 */
  weight: z.number().int().min(0).max(100),
  /** 模型白名单，逗号分隔；空 = 不限 */
  models: z.string(),
  source: z.enum(["inline", "pool"]),
  inlineKeys: z.string(),
  /** 回填时该组有凭据但全部无法解密显示的条数（后端详情逐条回空串），仅 detailToFormValues 填充。
   * 必须显式传递：[""].join("\n") 退化为空串，与「全新空组」无法从 value 区分（review W1'） */
  inlineKeysFailedCount: z.number().optional(),
  poolId: z.string(),
});

export type ProviderFormGroup = z.infer<typeof providerFormGroupSchema>;

export const providerFormSchema = z.object({
  name: z.string().min(1, { message: "提供商名称不能为空" }),
  type: z.string().min(1, { message: "提供商类型不能为空" }),
  base_url: z.string().min(1, { message: "Base URL 不能为空" }),
  // 顶层 api_key 已废除（S6）：凭据只存在于分组内联 keys / 关联号池，config 不再承载明文 key
  beta: z.string().optional(),
  version: z.string().optional(),
  auth_type: z.string().optional(),
  console: z.string().optional(),
  custom_models: z.string().optional(),
  proxy: z.string().optional(),
  model_endpoint: z.boolean().optional(),
  model_filter_enabled: z.boolean().optional(),
  /** 出站协议勾选（openai / anthropic / responses），至少一个 */
  protocols: z.array(z.string()).min(1, { message: "至少勾选一个出站协议" }).optional(),
  /** 端点/分组数组 min(1)：空数组会在提交时被拦（禁删光交互由 UI disabled 承担，此处是兜底）。
   * 保留 `.optional()`：仅 undefined 放行、空数组必报，历史遗留无子表的供应商仍能打开补齐后保存。 */
  endpoints: z.array(providerFormEndpointSchema).min(1, { message: "至少保留一个协议端点" }).optional(),
  groups: z.array(providerFormGroupSchema).min(1, { message: "至少保留一个凭据分组" }).optional(),
}).superRefine((val, ctx) => {
  // 显式主类型的不变量：type 必须 ∈ 已勾选协议（后端按 type 分发 providers.New，type 指向不存在
  // 的协议会导致「勾了协议却按别的类型实例化」）。type 空 = 新建尚未选择主类型，放行由 UI 首勾选写入。
  if (val.type && val.protocols && val.protocols.length > 0) {
    const option = SUPPORTED_TYPE_OPTIONS.find((o) => o.type === val.type);
    if (!option || !val.protocols.includes(option.protocol)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["type"],
        message: "主类型必须是已勾选的支持类型之一",
      });
    }
  }
});

export type ProviderFormValues = z.infer<typeof providerFormSchema>;

export const defaultProviderFormValues: ProviderFormValues = {
  name: "",
  type: "",
  base_url: "",
  beta: "",
  version: "",
  auth_type: "x-api-key",
  console: "",
  custom_models: "",
  proxy: "",
  model_endpoint: true,
  model_filter_enabled: false,
  protocols: ["openai"],
  endpoints: [{ protocol: "openai", url: "", enabled: true }],
  groups: [{ name: "默认组", weight: 1, models: "", source: "inline", inlineKeys: "", poolId: "" }],
};
