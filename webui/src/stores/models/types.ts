import type { Model } from "@/lib/api";

export type ModelsPagePreferences = {
  searchQuery: string;
  selectedProviderId: string;
  modelSearchQuery: string;
};

export const DEFAULT_MODELS_PAGE_PREFERENCES: ModelsPagePreferences = {
  searchQuery: "",
  selectedProviderId: "all",
  modelSearchQuery: "",
};

export type ModelsDialogsState = {
  formDialogOpen: boolean;
  editingModel: Model | null;
  deletingModel: Model | null;
  batchDeleteDialogOpen: boolean;
  modelPickerOpen: boolean;
};

