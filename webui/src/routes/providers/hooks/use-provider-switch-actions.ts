import { useState } from "react";
import { toast } from "sonner";
import { updateProvider, type Provider } from "@/lib/api";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

type UseProviderSwitchActionsInput = {
  setProviders: Setter<Provider[]>;
};

export function useProviderSwitchActions({ setProviders }: UseProviderSwitchActionsInput) {
  const [updatingFilter, setUpdatingFilter] = useState<Record<number, boolean>>({});
  const [updatingAssociationTrigger, setUpdatingAssociationTrigger] = useState<Record<number, boolean>>({});

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
      setProviders((prev) => prev.map((item) => (item.ID === provider.ID ? { ...item, ModelEndpoint: newValue } : item)));
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
      setProviders((prev) => prev.map((item) => (item.ID === provider.ID ? { ...item, ModelFilterEnabled: enabled } : item)));
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
      setProviders((prev) => prev.map((item) => (item.ID === provider.ID ? { ...item, blacklisted: !enabled } : item)));
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
