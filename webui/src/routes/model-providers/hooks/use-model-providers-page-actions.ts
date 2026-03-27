import type { ModelWithProvider } from "@/lib/api";

type UseModelProvidersPageActionsInput = {
  selectedModelId: number | null;
  modelProviders: ModelWithProvider[];
  setDeleteId: (id: number | null) => void;
  setCollapsedProviders: (next: (previous: Record<number, boolean>) => Record<number, boolean>) => void;
  loadProviderStatus: (providers: ModelWithProvider[], modelId: number) => Promise<void>;
  executePreviewAction: () => Promise<void>;
  handleAddTemplateItem: () => Promise<void>;
  handleDeleteTemplateItem: (name: string) => Promise<void>;
};

export function useModelProvidersPageActions({
  selectedModelId,
  modelProviders,
  setDeleteId,
  setCollapsedProviders,
  loadProviderStatus,
  executePreviewAction,
  handleAddTemplateItem,
  handleDeleteTemplateItem,
}: UseModelProvidersPageActionsInput) {
  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const toggleProviderCollapse = (providerId: number) => {
    setCollapsedProviders((prev) => ({
      ...prev,
      [providerId]: !prev[providerId],
    }));
  };

  const refreshStatus = () => {
    if (selectedModelId) {
      void loadProviderStatus(modelProviders, selectedModelId);
    }
  };

  const handleDeleteDialogChange = (openValue: boolean) => {
    if (!openValue) {
      setDeleteId(null);
    }
  };

  const confirmPreviewAction = () => {
    void executePreviewAction();
  };

  const addTemplateItem = () => {
    void handleAddTemplateItem();
  };

  const deleteTemplateItem = (name: string) => {
    void handleDeleteTemplateItem(name);
  };

  return {
    openDeleteDialog,
    toggleProviderCollapse,
    refreshStatus,
    handleDeleteDialogChange,
    confirmPreviewAction,
    addTemplateItem,
    deleteTemplateItem,
  };
}

