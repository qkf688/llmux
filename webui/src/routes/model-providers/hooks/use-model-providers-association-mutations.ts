import type { UseFormReturn } from "react-hook-form";
import { toast } from "sonner";
import {
  createModelProvider,
  deleteModelProvider,
  updateModelProvider,
  type Settings,
  type ModelWithProvider,
} from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import type { FormValues } from "../form-schema";
import type { ProviderModelSelection } from "../types";

type Setter<T> = (value: Updater<T>) => void;
type CreatePayload = Parameters<typeof createModelProvider>[0];

type UseModelProvidersAssociationMutationsInput = {
  form: UseFormReturn<FormValues>;
  buildPayload: (values: FormValues, overrides?: { providerId?: number; providerModel?: string }) => CreatePayload;

  selectedModelId: number | null;
  settings: Settings | null;
  fetchModelProviders: (modelId: number) => Promise<void>;

  isSubmitting: boolean;
  setIsSubmitting: (loading: boolean) => void;

  selectedProviderModels: ProviderModelSelection[];
  setSelectedProviderModels: Setter<ProviderModelSelection[]>;

  setOpen: (open: boolean) => void;

  editingAssociation: ModelWithProvider | null;
  setEditingAssociation: (association: ModelWithProvider | null) => void;

  deleteId: number | null;
  setDeleteId: (id: number | null) => void;
};

export function useModelProvidersAssociationMutations({
  form,
  buildPayload,
  selectedModelId,
  settings,
  fetchModelProviders,
  isSubmitting,
  setIsSubmitting,
  selectedProviderModels,
  setSelectedProviderModels,
  setOpen,
  editingAssociation,
  setEditingAssociation,
  deleteId,
  setDeleteId,
}: UseModelProvidersAssociationMutationsInput) {
  const handleCreate = async (values: FormValues) => {
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      if (selectedProviderModels.length > 0) {
        const promises = selectedProviderModels.map(({ providerId, modelId }) =>
          createModelProvider(
            buildPayload(values, {
              providerId,
              providerModel: modelId,
            })
          )
        );
        await Promise.all(promises);
        toast.success(`成功创建 ${selectedProviderModels.length} 个模型提供商关联`);
      } else {
        const modelName = values.provider_name?.trim();
        if (!modelName) {
          toast.error("请选择模型或手动输入模型名称");
          return;
        }
        if (!values.provider_id || values.provider_id <= 0) {
          toast.error("请选择具体的提供商");
          return;
        }
        await createModelProvider(
          buildPayload(values, {
            providerId: values.provider_id,
            providerModel: modelName,
          })
        );
        toast.success("模型提供商关联创建成功");
      }

      setOpen(false);
      form.reset({
        model_id: selectedModelId || 0,
        provider_name: "",
        provider_id: 0,
        tool_call: true,
        structured_output: false,
        image: false,
        with_header: false,
        // 默认值取自 setting schema（与 models/setting_schema.go 同步），fallback = 100。
        weight: settings?.auto_weight_decay_default || 100,
        priority: settings?.auto_priority_decay_default || 100,
        max_tokens: 0,
        customer_headers: [],
      });
      setSelectedProviderModels([]);
      if (selectedModelId) {
        void fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`创建模型提供商关联失败: ${message}`);
      console.error(err);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleUpdate = async (values: FormValues) => {
    if (!editingAssociation) return;

    try {
      await updateModelProvider(editingAssociation.ID, buildPayload(values));
      setOpen(false);
      toast.success("模型提供商关联更新成功");
      setEditingAssociation(null);
      form.reset({
        model_id: 0,
        provider_name: "",
        provider_id: 0,
        tool_call: false,
        structured_output: false,
        image: false,
        with_header: false,
        weight: 1,
        priority: 100,
        max_tokens: 0,
        customer_headers: [],
      });
      if (selectedModelId) {
        void fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`更新模型提供商关联失败: ${message}`);
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteModelProvider(deleteId);
      setDeleteId(null);
      if (selectedModelId) {
        void fetchModelProviders(selectedModelId);
      }
      toast.success("模型提供商关联删除成功");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`删除模型提供商关联失败: ${message}`);
      console.error(err);
    }
  };

  return { handleCreate, handleUpdate, handleDelete };
}
