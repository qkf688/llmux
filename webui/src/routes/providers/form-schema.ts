import { z } from "zod";

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
};
