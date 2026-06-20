import type { UseFormReturn } from "react-hook-form";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { createProvider, updateProvider, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { buildConfigFromForm } from "../utils/config";

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
        const config = buildConfigFromForm(values);
        await updateProvider(editingProvider.ID, {
          name: values.name,
          type: values.type,
          config: config,
          console: values.console || "",
          proxy: values.proxy || "",
          model_endpoint: values.model_endpoint,
          model_filter_enabled: values.model_filter_enabled,
          auth_type: values.type === "anthropic" ? (values.auth_type || "x-api-key") : undefined,
        });
        setOpen(false);
        toast.success(`提供商 ${values.name} 更新成功`);
        setEditingProvider(null);
        form.reset({ ...defaultProviderFormValues });
        void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`更新提供商失败: ${message}`);
        console.error(err);
      }
      return;
    }

    try {
      const config = buildConfigFromForm(values);
      await createProvider({
        name: values.name,
        type: values.type,
        config: config,
        console: values.console || "",
        proxy: values.proxy || "",
        model_endpoint: values.model_endpoint ?? true,
        model_filter_enabled: values.model_filter_enabled ?? false,
        auth_type: values.type === "anthropic" ? (values.auth_type || "x-api-key") : undefined,
      });
      setOpen(false);
      toast.success(`提供商 ${values.name} 创建成功`);
      form.reset({ ...defaultProviderFormValues });
      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建提供商失败: ${message}`);
      console.error(err);
    }
  };

  return { handleSubmitProvider };
}
