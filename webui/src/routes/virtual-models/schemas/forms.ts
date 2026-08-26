import { z } from "zod";

export const VIRTUAL_MODEL_STRATEGY_VALUES = ["priority", "round_robin", "random"] as const;

export const virtualModelFormSchema = z.object({
  name: z.string().min(1, { message: "虚拟模型名称不能为空" }),
  description: z.string(),
  strategy: z.enum(VIRTUAL_MODEL_STRATEGY_VALUES),
  io_log: z.boolean(),
  enabled: z.boolean(),
});

export type VirtualModelFormValues = z.infer<typeof virtualModelFormSchema>;

export const defaultVirtualModelFormValues: VirtualModelFormValues = {
  name: "",
  description: "",
  strategy: "priority",
  io_log: false,
  enabled: true,
};

export const mappingFormSchema = z.object({
  real_model_id: z.number().min(1, { message: "请选择真实模型" }),
  priority: z.number().min(0, { message: "优先级不能为负数" }),
  weight: z.number().min(1, { message: "权重必须大于0" }),
  enabled: z.boolean(),
});

export type MappingFormValues = z.infer<typeof mappingFormSchema>;

export const defaultMappingFormValues: MappingFormValues = {
  real_model_id: 0,
  priority: 10,
  weight: 5,
  enabled: true,
};
