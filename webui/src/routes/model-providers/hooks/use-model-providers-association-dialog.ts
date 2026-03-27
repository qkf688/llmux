import type { UseFormReturn } from "react-hook-form";
import type { ModelWithProvider, Settings } from "@/lib/api";
import type { FormValues } from "../form-schema";
import type { ProviderModelSelection } from "../types";

type Updater<T> = T | ((previous: T) => T);
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
    const headerPairs = Object.entries(association.CustomerHeaders || {}).map(([key, value]) => ({
      key,
      value,
    }));
    form.reset({
      model_id: association.ModelID,
      provider_name: association.ProviderModel,
      provider_id: association.ProviderID,
      tool_call: association.ToolCall,
      structured_output: association.StructuredOutput,
      image: association.Image,
      with_header: association.WithHeader,
      weight: association.Weight,
      priority: association.Priority ?? 100,
      customer_headers: headerPairs.length ? headerPairs : [],
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingAssociation(null);
    setSelectedProviderModels([]);
    const defaultWeight = settings?.auto_weight_decay_default || 5;
    const defaultPriority = settings?.auto_priority_decay_default || 10;
    form.reset({
      model_id: selectedModelId || 0,
      provider_name: "",
      provider_id: 0,
      tool_call: true,
      structured_output: false,
      image: false,
      with_header: false,
      weight: defaultWeight,
      priority: defaultPriority,
      customer_headers: [],
    });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}

