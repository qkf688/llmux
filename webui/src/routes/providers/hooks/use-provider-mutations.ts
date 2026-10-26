import type { UseFormReturn } from "react-hook-form";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { createProvider, updateProvider, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";
import { invalidateModelProviders } from "@/hooks/api/use-model-providers";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { buildProviderPayload } from "../utils/config";

type UseProviderMutationsInput = {
  form: UseFormReturn<ProviderFormValues>;
  editingProvider: Provider | null;
  setEditingProvider: (provider: Provider | null) => void;
  setOpen: (open: boolean) => void;
};

export function useProviderMutations({
  form,
  editingProvider,
  setEditingProvider,
  setOpen,
}: UseProviderMutationsInput) {
  const queryClient = useQueryClient();

  const handleSubmitProvider = async (values: ProviderFormValues) => {
    if (editingProvider) {
      try {
        // 表单编辑：全量携带 protocols/endpoints/groups，走结构化四表同步
        await updateProvider(
          editingProvider.ID,
          buildProviderPayload(values, {
            name: values.name,
            type: values.type,
            console: values.console || "",
            proxy: values.proxy || "",
            model_endpoint: values.model_endpoint,
            model_filter_enabled: values.model_filter_enabled,
          })
        );
        setOpen(false);
        toast.success(`提供商 ${values.name} 更新成功`);
        setEditingProvider(null);
        form.reset({ ...defaultProviderFormValues });
        void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
        // 后端 PUT /providers/:id 之后会 TriggerAutoAssociate + TriggerAutoClean（按开关），
        // 实际改写 model_with_providers；模型关联页读另一套缓存族，不一起失效就会在
        // staleTime（60s）内继续显示陈旧关联行。
        invalidateModelProviders(queryClient);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`更新提供商失败: ${message}`);
        console.error(err);
      }
      return;
    }

    try {
      await createProvider(
        buildProviderPayload(values, {
          name: values.name,
          type: values.type,
          console: values.console || "",
          proxy: values.proxy || "",
          model_endpoint: values.model_endpoint ?? true,
          model_filter_enabled: values.model_filter_enabled ?? false,
        })
      );
      setOpen(false);
      toast.success(`提供商 ${values.name} 创建成功`);
      form.reset({ ...defaultProviderFormValues });
      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      // 同 Update：POST /providers 之后后端会 TriggerAutoAssociate，可能新增关联行
      invalidateModelProviders(queryClient);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建提供商失败: ${message}`);
      console.error(err);
    }
  };

  return { handleSubmitProvider };
}