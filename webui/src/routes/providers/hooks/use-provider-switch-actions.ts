import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { updateProvider, type Provider } from "@/lib/api";
import { providerKeys, invalidateAfterProviderWrite } from "@/hooks/api/use-providers";

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

  // 三个开关都打 PUT /providers/:id，而后端对每条该请求都无条件
  // bgtask.Go(TriggerAutoAssociate) + bgtask.Go(TriggerAutoClean)（见
  // handler/providerapi/crud.go UpdateProvider）——是否真的增删关联由全局开关决定，
  // 前端按字段无法判定，故三个开关一律失效关联页缓存。
  // 其中「关联触发」（blacklisted）是唯一有确定因果的一条：associate 跳过黑名单供应商，
  // 取消拉黑会在自动关联开启时新建关联行；其余两个（model_endpoint /
  // model_filter_enabled）不参与 GetProviderModels，实际影响仅来自上述无条件触发。
  //
  // lists: false —— 三个开关都已用 patchProviderInCache 就地打好补丁，整表重取是多余请求。
  const invalidateAssociations = () => {
    invalidateAfterProviderWrite(queryClient, { lists: false });
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
      invalidateAssociations();
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
      invalidateAssociations();
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
      invalidateAssociations();
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
