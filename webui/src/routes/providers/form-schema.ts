import { z } from "zod";

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
  poolId: z.string(),
});

export type ProviderFormGroup = z.infer<typeof providerFormGroupSchema>;

export const providerFormSchema = z.object({
  name: z.string().min(1, { message: "提供商名称不能为空" }),
  type: z.string().min(1, { message: "提供商类型不能为空" }),
  base_url: z.string().min(1, { message: "Base URL 不能为空" }),
  api_key: z.string().min(1, { message: "API Key 不能为空" }),
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
  endpoints: z.array(providerFormEndpointSchema).optional(),
  groups: z.array(providerFormGroupSchema).optional(),
});

export type ProviderFormValues = z.infer<typeof providerFormSchema>;

export const defaultProviderFormValues: ProviderFormValues = {
  name: "",
  type: "",
  base_url: "",
  api_key: "",
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
