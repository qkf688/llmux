import type { UseFormReturn } from "react-hook-form";
import type { Provider } from "@/lib/api";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { parseConfigToForm } from "../utils/config";
import { protocolOfType } from "../utils/schedule";

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
    const schedule = configFields.schedule;
    // 无 _schedule 时按原 type 映射协议，并派生对应默认端点（不能写死 openai：否则老 anthropic/openai-res 供应商端点区被过滤成空白）
    const fallbackProtocols = schedule?.protocols?.length ? schedule.protocols : [protocolOfType(provider.Type)];
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
      // S0 原型：调度配置回填（老 Provider 无 _schedule 时用默认形态：1 协议 + 1 端点 + 默认分组）
      protocols: fallbackProtocols,
      // 同协议多条端点（旧版残留数据）去重，每协议只保留第一条
      endpoints: schedule?.endpoints?.length
        ? schedule.endpoints.filter((e, idx, arr) => arr.findIndex((x) => x.protocol === e.protocol) === idx)
        : fallbackProtocols.map((p) => ({ protocol: p, url: "", enabled: true })),
      groups: schedule?.groups?.length ? schedule.groups : defaultProviderFormValues.groups,
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    setShowApiKey(false);
    // 默认预勾选 OpenAI（defaultProviderFormValues.protocols），type 必须同步，否则 zod 校验静默拦截且错误无处渲染
    form.reset({ ...defaultProviderFormValues, type: "openai" });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}

