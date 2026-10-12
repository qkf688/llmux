import { z } from "zod";

// 头行只校验到形状层（与 api-keys 弹窗同口径，4a）：空名行由折叠转换跳过、
// 不打断保存；语法/撞名判定权威在后端 common/outboundheaders.go，拒收时 400 文案回显。
export const headerPairSchema = z.object({
  key: z.string(),
  value: z.string(),
});

export const formSchema = z.object({
  model_id: z.number().positive({ message: "模型ID必须大于0" }),
  // 上游模型名（provider_model）不在此表中：它只来自「模型列表」弹窗的选择，由 Create 的
  // overrides 传入。原先的「手动输入模型名称」输入框已删（影子路——它能产出的「目录里没有
  // 的名字」正规入口已覆盖，且会被 autoassoc 的清理判为无效），编辑态也不再改该字段
  // （后端 Update 缺省=不改，见 associations.md 的 provider_model 写入闸）。
  provider_id: z.number().min(0, { message: "提供商ID必须大于等于0" }),
  // 能力总控：inherit=三项全部跟随模型真值（不提交，后端写 NULL）；
  // custom=三项按下面的勾选显式指定。
  // 单独设这个字段而非让三项各自带三态，是因为「继承」与「显式等于模型真值」
  // 在 COALESCE 解析后等价，UI 上区分它俩只是给人添麻烦（见 associations.md）。
  // 取值域与 thinking_levels_mode 一致：同弹窗内两块枚举统一，用户不必学两套。
  capabilities_mode: z.enum(["inherit", "custom"]),
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

