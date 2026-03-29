import { toast } from "sonner";
import { updateProvider, type VirtualModel } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import type { VirtualModelsPageState } from "@/stores/virtual-models";
import type { Provider } from "@/lib/api";

type UseVirtualModelsBlacklistInput = {
  providers: Provider[];
  selectedProviderIds: VirtualModelsPageState["selectedProviderIds"];
  refreshProvidersState: () => Promise<void>;
  setCurrentVirtualModel: VirtualModelsPageState["setCurrentVirtualModel"];
  setBlacklistedProviders: VirtualModelsPageState["setBlacklistedProviders"];
  setBlacklistDialogOpen: VirtualModelsPageState["setBlacklistDialogOpen"];
  setProviderSelectorDialogOpen: VirtualModelsPageState["setProviderSelectorDialogOpen"];
  setSelectedProviderIds: VirtualModelsPageState["setSelectedProviderIds"];
  setProviderSearchQuery: VirtualModelsPageState["setProviderSearchQuery"];
};

export function useVirtualModelsBlacklist({
  providers,
  selectedProviderIds,
  refreshProvidersState,
  setCurrentVirtualModel,
  setBlacklistedProviders,
  setBlacklistDialogOpen,
  setProviderSelectorDialogOpen,
  setSelectedProviderIds,
  setProviderSearchQuery,
}: UseVirtualModelsBlacklistInput) {
  const openBlacklistDialog = async (model: VirtualModel) => {
    setCurrentVirtualModel(model);
    try {
      await refreshProvidersState();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`获取提供商数据失败: ${message}`);
      setBlacklistedProviders(providers.filter((provider) => provider.blacklisted));
    }
    setBlacklistDialogOpen(true);
  };

  const handleBlacklistDialogOpenChange = (open: boolean) => {
    setBlacklistDialogOpen(open);
    if (!open) {
      setProviderSelectorDialogOpen(false);
    }
  };

  const openProviderSelectorDialog = () => {
    setSelectedProviderIds([]);
    setProviderSearchQuery("");
    setProviderSelectorDialogOpen(true);
  };

  const toggleProviderSelection = (providerId: number) => {
    setSelectedProviderIds((previous) =>
      previous.includes(providerId) ? previous.filter((id) => id !== providerId) : [...previous, providerId]
    );
  };

  const confirmAddBlacklistedProviders = async () => {
    if (selectedProviderIds.length === 0) {
      toast.error("请至少选择一个提供商");
      return;
    }

    try {
      for (const providerId of selectedProviderIds) {
        await updateProvider(providerId, { blacklisted: true });
      }
      toast.success(`成功拉黑 ${selectedProviderIds.length} 个提供商`);
      setProviderSelectorDialogOpen(false);
      await refreshProvidersState();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

  const removeBlacklistedProvider = async (providerId: number) => {
    try {
      await updateProvider(providerId, { blacklisted: false });
      toast.success("提供商已解除拉黑");
      await refreshProvidersState();
    } catch (error) {
      const message = toErrorMessage(error);
      toast.error(`操作失败: ${message}`);
    }
  };

  return {
    openBlacklistDialog,
    handleBlacklistDialogOpenChange,
    openProviderSelectorDialog,
    toggleProviderSelection,
    confirmAddBlacklistedProviders,
    removeBlacklistedProvider,
  };
}
