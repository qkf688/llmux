import type { UseFormReturn } from "react-hook-form";
import type { Provider } from "@/lib/api";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { parseConfigToForm } from "../utils/config";

type UseProviderDialogInput = {
  form: UseFormReturn<ProviderFormValues>;
  setOpen: (open: boolean) => void;
  setEditingProvider: (provider: Provider | null) => void;
  setShowApiKey: (show: boolean) => void;
};

export function useProviderDialog({ form, setOpen, setEditingProvider, setShowApiKey }: UseProviderDialogInput) {
  const openEditDialog = (provider: Provider) => {
    setEditingProvider(provider);
    setShowApiKey(false);
    const configFields = parseConfigToForm(provider.Config);
    form.reset({
      name: provider.Name,
      type: provider.Type,
      base_url: configFields.base_url,
      api_key: configFields.api_key,
      beta: configFields.beta || "",
      version: configFields.version || "",
      auth_type: configFields.auth_type || "x-api-key",
      console: provider.Console || "",
      custom_models: configFields.custom_models.join("\n"),
      proxy: provider.Proxy || "",
      model_endpoint: provider.ModelEndpoint ?? true,
      model_filter_enabled: provider.ModelFilterEnabled ?? false,
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    setShowApiKey(false);
    form.reset({ ...defaultProviderFormValues });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}

