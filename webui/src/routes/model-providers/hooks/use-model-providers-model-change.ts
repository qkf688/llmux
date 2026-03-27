import { useCallback } from "react";
import type { UseFormReturn } from "react-hook-form";
import type { FormValues } from "../form-schema";

type UseModelProvidersModelChangeInput = {
  searchParams: URLSearchParams;
  setSearchParams: (next: URLSearchParams) => void;
  setSelectedModelId: (value: number | null) => void;
  clearSelectedAssociationIds: () => void;
  clearSelectedProviderModels: () => void;
  closeTemplateEditor: () => void;
  resetAssociationTestResults: () => void;
  resetSelectedStatusFilter: () => void;
  form: UseFormReturn<FormValues>;
};

export function useModelProvidersModelChange({
  searchParams,
  setSearchParams,
  setSelectedModelId,
  clearSelectedAssociationIds,
  clearSelectedProviderModels,
  closeTemplateEditor,
  resetAssociationTestResults,
  resetSelectedStatusFilter,
  form,
}: UseModelProvidersModelChangeInput) {
  const handleModelChange = useCallback(
    (modelId: string) => {
      const id = parseInt(modelId);
      setSelectedModelId(id);
      clearSelectedAssociationIds(); // 切换模型时清空选择
      clearSelectedProviderModels();
      closeTemplateEditor();
      resetAssociationTestResults(); // 切换模型时清空测试结果
      resetSelectedStatusFilter(); // 切换模型时重置启用状态筛选器
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set("modelId", id.toString());
      setSearchParams(nextParams);
      form.setValue("model_id", id);
    },
    [
      form,
      searchParams,
      setSearchParams,
      setSelectedModelId,
      clearSelectedAssociationIds,
      clearSelectedProviderModels,
      closeTemplateEditor,
      resetAssociationTestResults,
      resetSelectedStatusFilter,
    ]
  );

  return { handleModelChange };
}
