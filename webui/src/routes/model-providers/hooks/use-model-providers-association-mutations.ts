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
// buildPayload 不再产出 provider_model（它只来自模型列表选择，由 handleCreate 在调用点
// 逐条注入），故其返回类型是 CreatePayload 去掉该键。
type AssociationPayload = Omit<CreatePayload, "provider_model">;

type UseModelProvidersAssociationMutationsInput = {
  form: UseFormReturn<FormValues>;
  buildPayload: (values: FormValues, overrides?: { providerId?: number }) => AssociationPayload;

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
      // 上游模型名一律来自「模型列表」弹窗的选择：手动输入框已删（影子路，它能产出的
      // 「目录里没有的名字」正规入口已覆盖且会被 autoassoc 清理判无效），故此处不再有
      // 「没选模型时退回手输名」这条分支——没选就是没得建。
      if (selectedProviderModels.length === 0) {
        toast.error("请先在模型列表中选择模型");
        return;
      }
      const promises = selectedProviderModels.map(({ providerId, modelId }) =>
        createModelProvider({
          ...buildPayload(values, { providerId }),
          // 每条选中模型的 provider_model 各自不同，故在调用点注入而非进 buildPayload。
          provider_model: modelId,
        })
      );
      await Promise.all(promises);
      toast.success(`成功创建 ${selectedProviderModels.length} 个模型提供商关联`);

      setOpen(false);
      form.reset({
        model_id: selectedModelId || 0,
        provider_id: 0,
        capabilities_mode: "inherit",
        tool_call: false,
        structured_output: false,
        image: false,
        with_header: false,
        // 默认值取自 setting schema（与 models/setting_schema.go 同步），fallback = 100。
        weight: settings?.auto_weight_decay_default || 100,
        priority: settings?.auto_priority_decay_default || 100,
        max_tokens: 0,
        customer_headers: [],
        supports_thinking: "inherit", // 默认继承 model
        thinking_levels_mode: "inherit", // 默认继承 model
        thinking_levels_custom: [],
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
        provider_id: 0,
        capabilities_mode: "inherit",
        tool_call: false,
        structured_output: false,
        image: false,
        with_header: false,
        weight: 1,
        priority: 100,
        max_tokens: 0,
        customer_headers: [],
        supports_thinking: "inherit", // 默认继承 model
        thinking_levels_mode: "inherit", // 默认继承 model
        thinking_levels_custom: [],
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
