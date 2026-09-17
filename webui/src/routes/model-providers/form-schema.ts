import { z } from "zod";

// 头行只校验到形状层（与 api-keys 弹窗同口径，4a）：空名行由折叠转换跳过、
// 不打断保存；语法/撞名判定权威在后端 common/outboundheaders.go，拒收时 400 文案回显。
export const headerPairSchema = z.object({
  key: z.string(),
  value: z.string(),
});

export const formSchema = z.object({
  model_id: z.number().positive({ message: "模型ID必须大于0" }),
  // 上游模型名（供应商侧的 model 标识），非供应商名称——与后端 provider_model 键同名
  provider_model: z.string().default(""),
  provider_id: z.number().min(0, { message: "提供商ID必须大于等于0" }),
  tool_call: z.boolean(),
  structured_output: z.boolean(),
  image: z.boolean(),
  with_header: z.boolean(),
  weight: z.number().positive({ message: "权重必须大于0" }),
  priority: z.number().min(0, { message: "优先级必须大于等于0" }),
  max_tokens: z.number().int().min(0, { message: "max_tokens 上限必须大于等于0" }).optional(),
  customer_headers: z.array(headerPairSchema).default([]),
  // 三态："inherit"=继承 model，"true"/"false"=override
  supports_thinking: z.enum(["inherit", "true", "false"]),
  // thinking_levels 三态："inherit"=继承 model，"custom"=自定义白名单
  // custom 模式下 thinking_levels_custom 数组为白名单内容
  thinking_levels_mode: z.enum(["inherit", "custom"]),
  thinking_levels_custom: z.array(z.string()).default([]),
});

export type FormValues = z.input<typeof formSchema>;

