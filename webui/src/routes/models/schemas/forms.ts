import { z } from "zod";

export const modelFormSchema = z.object({
  name: z.string().min(1, { message: "模型名称不能为空" }),
  remark: z.string(),
  io_log: z.boolean(),
  auto_associate: z.boolean(),
  supports_thinking: z.boolean(),
  thinking_levels: z.array(z.string()).optional(),
  // 模型固有能力真值（工具调用 / 结构化输出 / 视觉）：
  // 关联侧默认继承这三项，仅个别链路弱于模型时才在关联上 override。
  tool_call: z.boolean(),
  structured_output: z.boolean(),
  image: z.boolean(),
});

export type ModelFormValues = z.infer<typeof modelFormSchema>;
