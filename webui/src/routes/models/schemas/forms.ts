import { z } from "zod";

export const modelFormSchema = z.object({
  name: z.string().min(1, { message: "模型名称不能为空" }),
  remark: z.string(),
  max_retry: z.number().min(0, { message: "重试次数限制不能为负数" }),
  time_out: z.number().min(0, { message: "超时时间不能为负数" }),
  io_log: z.boolean(),
  auto_associate: z.boolean().optional(),
  supports_thinking: z.boolean(),
});

export type ModelFormValues = z.infer<typeof modelFormSchema>;

export const batchUpdateSchema = z
  .object({
    enableMaxRetry: z.boolean(),
    enableTimeOut: z.boolean(),
    max_retry: z.number().min(0, { message: "重试次数不能为负数" }),
    time_out: z.number().min(0, { message: "超时时间不能为负数" }),
  })
  .refine((data) => data.enableMaxRetry || data.enableTimeOut, {
    message: "至少选择一个字段进行更新",
  });

export type BatchUpdateValues = z.infer<typeof batchUpdateSchema>;
