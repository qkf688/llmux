import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm } from "react-hook-form";
import { formSchema, type FormValues } from "../form-schema";

export function useModelProvidersAssociationForm() {
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      model_id: 0,
      provider_id: 0,
      // 能力默认全继承；勾选初值 false 只是占位——openEdit/openCreate 会用
      // 模型真值重填，避免用户切到「自定义」时看到与生效能力不符的空勾选。
      capabilities_mode: "inherit",
      tool_call: false,
      structured_output: false,
      image: false,
      with_header: false,
      weight: 5,
      priority: 10,
      max_tokens: 0,
      customer_headers: [],
      supports_thinking: "inherit", // 默认继承 model
      thinking_levels_mode: "inherit", // 默认继承 model
      thinking_levels_custom: [],
      protocols_mode: "inherit", // 默认继承供应商那层（协议集合 + 主格式）
      protocols: [],
      primary_protocol: "",
    },
  });

  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: "customer_headers",
  });

  return { form, headerFields, appendHeader, removeHeader };
}
