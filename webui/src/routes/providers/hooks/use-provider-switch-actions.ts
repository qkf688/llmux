import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { updateProvider, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";

export function useProviderSwitchActions() {
  const queryClient = useQueryClient();
  const [updatingFilter, setUpdatingFilter] = useState<Record<number, boolean>>({});
  const [updatingAssociationTrigger, setUpdatingAssociationTrigger] = useState<Record<number, boolean>>({});

  const patchProviderInCache = (providerId: number, patch: Partial<Provider>) => {
    queryClient.setQueriesData<Provider[]>(
      { queryKey: providerKeys.lists() },
      (old) => old?.map((item) => (item.ID === providerId ? { ...item, ...patch } : item)),
    );
  };

  const handleToggleModelEndpoint = async (provider: Provider) => {
    const newValue = !(provider.ModelEndpoint ?? true);
    try {
      await updateProvider(provider.ID, {
        name: provider.Name,
        type: provider.Type,
        config: provider.Config,
        console: provider.Console || "",
        proxy: provider.Proxy || "",
        model_endpoint: newValue,
      });
      patchProviderInCache(provider.ID, { ModelEndpoint: newValue });
      toast.success(`已${newValue ? "启用" : "禁用"}模型端点`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新模型端点失败: ${message}`);
      console.error(err);
    }
  };

  const handleToggleModelFilter = async (provider: Provider, enabled: boolean) => {
    setUpdatingFilter((prev) => ({ ...prev, [provider.ID]: true }));
    try {
      await updateProvider(provider.ID, { model_filter_enabled: enabled });
      patchProviderInCache(provider.ID, { ModelFilterEnabled: enabled });
      toast.success("已更新模型过滤设置");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新失败: ${message}`);
      console.error(err);
    } finally {
      setUpdatingFilter((prev) => ({ ...prev, [provider.ID]: false }));
    }
  };

  const handleToggleAssociationTrigger = async (provider: Provider, enabled: boolean) => {
    setUpdatingAssociationTrigger((prev) => ({ ...prev, [provider.ID]: true }));
    try {
      await updateProvider(provider.ID, { blacklisted: !enabled });
      patchProviderInCache(provider.ID, { blacklisted: !enabled });
      toast.success(`已${enabled ? "开启" : "关闭"}该提供商的自动关联/一键关联触发`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新关联触发设置失败: ${message}`);
      console.error(err);
    } finally {
      setUpdatingAssociationTrigger((prev) => ({ ...prev, [provider.ID]: false }));
    }
  };

  return {
    updatingFilter,
    updatingAssociationTrigger,
    handleToggleModelEndpoint,
    handleToggleModelFilter,
    handleToggleAssociationTrigger,
  };
}
