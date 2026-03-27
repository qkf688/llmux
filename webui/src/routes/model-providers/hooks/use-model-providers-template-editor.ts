import { useEffect, useState } from "react";
import { toast } from "sonner";
import {
  addModelTemplateItem,
  deleteModelTemplateItem,
  getModelTemplate,
  type ModelTemplate,
} from "@/lib/api";

type UseModelProvidersTemplateEditorInput = {
  templateEditorOpen: boolean;
  setTemplateEditorOpen: (open: boolean) => void;
  setTemplateLoading: (loading: boolean) => void;

  selectedModelId: number | null;

  templateNewItem: string;
  setTemplateNewItem: (value: string) => void;
};

export function useModelProvidersTemplateEditor({
  templateEditorOpen,
  setTemplateEditorOpen,
  setTemplateLoading,
  selectedModelId,
  templateNewItem,
  setTemplateNewItem,
}: UseModelProvidersTemplateEditorInput) {
  const [templateData, setTemplateData] = useState<ModelTemplate | null>(null);

  useEffect(() => {
    if (!templateEditorOpen || !selectedModelId) return;
    setTemplateLoading(true);
    getModelTemplate(selectedModelId)
      .then((data) => setTemplateData(data))
      .catch((err) => {
        const message = err instanceof Error ? err.message : String(err);
        toast.error(`加载模板失败: ${message}`);
      })
      .finally(() => setTemplateLoading(false));
  }, [selectedModelId, setTemplateLoading, templateEditorOpen]);

  const handleToggleTemplateEditor = () => {
    setTemplateEditorOpen(!templateEditorOpen);
  };

  const handleAddTemplateItem = async () => {
    if (!selectedModelId) return;
    const name = templateNewItem.trim();
    if (!name) return;
    setTemplateLoading(true);
    try {
      const data = await addModelTemplateItem(selectedModelId, name);
      setTemplateData(data);
      setTemplateNewItem("");
      toast.success("已添加模板项");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`添加模板项失败: ${message}`);
    } finally {
      setTemplateLoading(false);
    }
  };

  const handleDeleteTemplateItem = async (name: string) => {
    if (!selectedModelId) return;
    setTemplateLoading(true);
    try {
      const data = await deleteModelTemplateItem(selectedModelId, name);
      setTemplateData(data);
      toast.success("已删除手动模板项");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除模板项失败: ${message}`);
    } finally {
      setTemplateLoading(false);
    }
  };

  return {
    templateData,
    handleToggleTemplateEditor,
    handleAddTemplateItem,
    handleDeleteTemplateItem,
  };
}

