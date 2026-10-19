import type { UseFormReturn } from "react-hook-form";
import { toast } from "sonner";
import {
  createModelProvider,
  deleteModelProvider,
  updateModelProvider,
  updateModelProviderProtocols,
  type Settings,
  type ModelWithProvider,
} from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import { protocolLabel } from "@/lib/protocols";
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
        protocols_mode: "inherit",
        protocols: [],
        primary_protocol: "",
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
        protocols_mode: "inherit",
        protocols: [],
        primary_protocol: "",
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

  // 协议检测的一键落地：把推荐**整项覆盖**该关联的协议设置（允许集合 + 主格式）。
  // 覆盖而非合并：检测是按这个模型实测的结论，留着「检测说不通」的协议等于让请求继续
  // 打到坏路上；要保留别的协议，用户可以在关联弹窗里手动勾。
  // 集合里勾多条是为了让客户端用哪种协议进来就走哪条，省掉没必要的格式转换；主格式兜住
  // 「客户端发来集合外的协议」时该转成哪种格式。两者必须同时写，否则集合外协议没有转换目标。
  // 走窄端点（只写协议两列）：全量 PUT 会把能力/thinking/请求头透传一起清掉。
  const applyProtocolRecommendation = async (id: number, protocols: string[], primary: string) => {
    try {
      await updateModelProviderProtocols(id, { protocols, primary_protocol: primary });
      toast.success(
        `已勾选 ${protocols.length} 条协议、主格式 ${protocolLabel(primary)}（替换了原有协议设置）`,
      );
      if (selectedModelId) {
        void fetchModelProviders(selectedModelId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`写入协议设置失败: ${message}`);
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

  return { handleCreate, handleUpdate, handleDelete, applyProtocolRecommendation };
}
