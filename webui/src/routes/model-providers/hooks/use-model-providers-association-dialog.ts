import type { UseFormReturn } from "react-hook-form";
import type { Model, ModelWithProvider, Settings } from "@/lib/api";
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
  // 打开弹窗时用来解析「勾选初值」：关联侧为 nil（继承）时，勾选框必须显示
  // 模型真值，而不是 false——否则用户从「继承」切到「自定义」一保存就把能力全禁用了。
  models: Model[];

  setOpen: (open: boolean) => void;
  setEditingAssociation: (association: ModelWithProvider | null) => void;
  setSelectedProviderModels: Setter<ProviderModelSelection[]>;
};

/** 能力三项的键名映射：关联实体（PascalCase，无 tag）↔ 模型实体（snake_case tag）↔ 表单键。 */
const CAPABILITY_KEYS = [
  { assoc: "ToolCall", model: "tool_call", form: "tool_call" },
  { assoc: "StructuredOutput", model: "structured_output", form: "structured_output" },
  { assoc: "Image", model: "image", form: "image" },
] as const;

export function useModelProvidersAssociationDialog({
  form,
  settings,
  selectedModelId,
  models,
  setOpen,
  setEditingAssociation,
  setSelectedProviderModels,
}: UseModelProvidersAssociationDialogInput) {
  // 找当前选中模型的能力真值；未选中或未加载到时按 false 兜底。
  const findModelCaps = (modelId: number | null) =>
    CAPABILITY_KEYS.reduce<Record<string, boolean>>(
      (acc, { model, form: formKey }) => {
        const found = modelId === null ? undefined : models.find((m) => m.ID === modelId);
        acc[formKey] = found?.[model] ?? false;
        return acc;
      },
      {}
    );

  const openEditDialog = (association: ModelWithProvider) => {
    setEditingAssociation(association);
    setSelectedProviderModels([]);
    const headerPairs = headersToRows(association.CustomerHeaders);
    const modelCaps = findModelCaps(association.ModelID);

    // 总控初始态：三项全为 nil 才算「继承」；出现任何一个 override 就落在「自定义」，
    // 这样混合态与存量数据都能原样进 UI，不会被二值开关抹平。
    // 用 == null 而非 === null：键名一旦漂移（如后端误加 omitempty）读到的会是 undefined，
    // 严格判等会恒判 custom 并在保存时把继承静默改写成 override；宽松判等降级为「显示继承」。
    const allInherit =
      association.ToolCall == null && association.StructuredOutput == null && association.Image == null;

    // 勾选初值：override 态取其值；nil（继承）取模型真值——继承与「显式等于模型真值」
    // 在 COALESCE 解析后等价，故这样初始化保证「切到自定义再保存」不改变任何生效行为。
    const capValues = CAPABILITY_KEYS.reduce<Record<string, boolean>>((acc, { assoc, form: formKey }) => {
      const override = association[assoc];
      acc[formKey] = override == null ? modelCaps[formKey] : override;
      return acc;
    }, {});

    form.reset({
      model_id: association.ModelID,
      provider_model: association.ProviderModel,
      provider_id: association.ProviderID,
      capabilities_mode: allInherit ? "inherit" : "custom",
      tool_call: capValues.tool_call,
      structured_output: capValues.structured_output,
      image: capValues.image,
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
    // 勾选初值同样取模型真值：新建默认「继承」，用户切到「自定义」时看到的
    // 是当前实际生效的能力，不是一排空勾选框。
    const capValues = findModelCaps(selectedModelId);

    form.reset({
      model_id: selectedModelId || 0,
      provider_model: "",
      provider_id: 0,
      capabilities_mode: "inherit", // 默认三项全部继承模型真值
      tool_call: capValues.tool_call,
      structured_output: capValues.structured_output,
      image: capValues.image,
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
