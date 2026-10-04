import type { UseFormReturn } from "react-hook-form";
import type { ModelWithProvider, Settings } from "@/lib/api";
import { headersToRows } from "@/lib/header-rows";
import { toTriState } from "../utils/tri-state";
import type { Updater } from "@/stores/core/updater";
import type { FormValues } from "../form-schema";
import type { ProviderModelSelection } from "../types";

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersAssociationDialogInput = {
  form: UseFormReturn<FormValues>;
  settings: Settings | null;
  selectedModelId: number | null;

  setOpen: (open: boolean) => void;
  setEditingAssociation: (association: ModelWithProvider | null) => void;
  setSelectedProviderModels: Setter<ProviderModelSelection[]>;
};

export function useModelProvidersAssociationDialog({
  form,
  settings,
  selectedModelId,
  setOpen,
  setEditingAssociation,
  setSelectedProviderModels,
}: UseModelProvidersAssociationDialogInput) {
  const openEditDialog = (association: ModelWithProvider) => {
    setEditingAssociation(association);
    setSelectedProviderModels([]);
    const headerPairs = headersToRows(association.CustomerHeaders);
    form.reset({
      model_id: association.ModelID,
      provider_model: association.ProviderModel,
      provider_id: association.ProviderID,
      // 能力三态：null=继承 model 真值，true/false=本链路 override。
      // 用 == null 而非 === null：键名一旦漂移（如后端误加 omitempty）读到的会是
      // undefined，严格判等会恒判 "true" 并在保存时把继承静默改写成 override。
      tool_call: toTriState(association.ToolCall),
      structured_output: toTriState(association.StructuredOutput),
      image: toTriState(association.Image),
      with_header: association.WithHeader,
      weight: association.Weight,
      priority: association.Priority ?? 100,
      max_tokens: association.MaxTokens ?? 0,
      customer_headers: headerPairs.length ? headerPairs : [],
      supports_thinking: toTriState(association.SupportsThinking),
      // thinking_levels 三态：null=继承，非 null=自定义（空切片=不约束）。
      // 用 == null 而非 === null：后端契约保证该键始终存在且继承态为 null，
      // 但键名一旦漂移（如误加 omitempty 或改 tag）读到的会是 undefined，
      // 那时严格判等会让此处恒判 custom，并在保存时把"继承"静默改写成"不约束"。
      // 宽松判等把这类契约故障降级为"显示继承"而非写坏数据。
      thinking_levels_mode: association.ThinkingLevels == null ? "inherit" : "custom",
      thinking_levels_custom: association.ThinkingLevels ?? [],
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingAssociation(null);
    setSelectedProviderModels([]);
    // 默认值取自 setting schema（与 models/setting_schema.go 同步），
    // fallback 与 schema 默认一致：100 / 100。settings 加载完成后即被真实值覆盖。
    const defaultWeight = settings?.auto_weight_decay_default || 100;
    const defaultPriority = settings?.auto_priority_decay_default || 100;
    form.reset({
      model_id: selectedModelId || 0,
      provider_model: "",
      provider_id: 0,
      tool_call: "inherit", // 默认继承 model 的能力真值
      structured_output: "inherit",
      image: "inherit",
      with_header: false,
      weight: defaultWeight,
      priority: defaultPriority,
      max_tokens: 0,
      customer_headers: [],
      supports_thinking: "inherit", // 默认继承 model
      thinking_levels_mode: "inherit", // 默认继承 model
      thinking_levels_custom: [],
    });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}
