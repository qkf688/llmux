import { useState } from "react";
import { toast } from "sonner";
import {
  autoAssociateModels,
  cleanInvalidAssociations,
  previewAutoAssociate,
  previewCleanInvalid,
  type AssociationPreview,
} from "@/lib/api";

type PreviewType = "associate" | "clean";

type UseModelProvidersPreviewInput = {
  selectedModelId: number | null;
  previewType: PreviewType;
  setPreviewType: (value: PreviewType) => void;
  setPreviewDialogOpen: (open: boolean) => void;
  setExecuting: (executing: boolean) => void;
  fetchModelProviders: (modelId: number) => Promise<void>;
};

export function useModelProvidersPreview({
  selectedModelId,
  previewType,
  setPreviewType,
  setPreviewDialogOpen,
  setExecuting,
  fetchModelProviders,
}: UseModelProvidersPreviewInput) {
  const [previewData, setPreviewData] = useState<AssociationPreview[]>([]);

  const handleAutoAssociate = async () => {
    try {
      setPreviewType("associate");
      const data = await previewAutoAssociate();
      setPreviewData(data);
      setPreviewDialogOpen(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取预览失败: ${message}`);
    }
  };

  const handleCleanInvalid = async () => {
    try {
      setPreviewType("clean");
      const data = await previewCleanInvalid();
      setPreviewData(data);
      setPreviewDialogOpen(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取预览失败: ${message}`);
    }
  };

  const executePreviewAction = async () => {
    try {
      setExecuting(true);
      if (previewType === "associate") {
        const result = await autoAssociateModels();
        if (result.failed > 0) {
          toast.warning(`成功添加 ${result.added} 个关联，${result.failed} 个失败`);
        } else {
          toast.success(`成功添加 ${result.added} 个关联`);
        }
      } else {
        const result = await cleanInvalidAssociations();
        if (result.failed > 0) {
          toast.warning(`成功清除 ${result.removed} 个无效关联，${result.failed} 个失败`);
        } else {
          toast.success(`成功清除 ${result.removed} 个无效关联`);
        }
      }
      setPreviewDialogOpen(false);
      if (selectedModelId) {
        await fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`操作失败: ${message}`);
    } finally {
      setExecuting(false);
    }
  };

  return { previewData, handleAutoAssociate, handleCleanInvalid, executePreviewAction };
}
