import { zodResolver } from "@hookform/resolvers/zod";
import { useFieldArray, useForm } from "react-hook-form";
import { formSchema, type FormValues } from "../form-schema";

export function useModelProvidersAssociationForm() {
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      model_id: 0,
      provider_name: "",
      provider_id: 0,
      tool_call: true,
      structured_output: true,
      image: true,
      with_header: false,
      weight: 5,
      priority: 10,
      max_tokens: 0,
      customer_headers: [],
      supports_thinking: "inherit", // 默认继承 model
    },
  });

  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: "customer_headers",
  });

  return { form, headerFields, appendHeader, removeHeader };
}
