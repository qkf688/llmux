import { z } from "zod";

export const modelFormSchema = z.object({
  name: z.string().min(1, { message: "模型名称不能为空" }),
  remark: z.string(),
  io_log: z.boolean(),
  auto_associate: z.boolean().optional(),
  supports_thinking: z.boolean(),
  thinking_levels: z.array(z.string()).optional(),
});

export type ModelFormValues = z.infer<typeof modelFormSchema>;
