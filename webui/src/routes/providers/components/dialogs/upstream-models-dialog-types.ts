import type { GroupRef, ProviderModel } from "@/lib/api";
import type { Setter } from "@/stores/core/updater";
import type { BatchTestProgress, ModelTestResult } from "../../types";

export interface UpstreamModelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providerName: string | undefined;

  modelsOpenId: number | null;
  modelsLoading: boolean;
  addingModels: boolean;

  providerModels: ProviderModel[];
  filteredProviderModels: ProviderModel[];
  availableGroups: GroupRef[];
  groupFilter: string;
  onGroupFilterChange: (value: string) => void;

  cachedModelsCount: number;
  savedModelSet: Set<string>;

  selectedUpstreamModels: string[];
  setSelectedUpstreamModels: Setter<string[]>;

  upstreamTestResults: Record<string, ModelTestResult>;
  upstreamBatchTesting: boolean;
  upstreamBatchTestProgress: BatchTestProgress;

  selectableModelIds: string[];
  isAllSelectableChecked: boolean;
  toggleSelectAll: () => void;

  handleBatchTestUpstreamAll: () => void | Promise<void>;
  handleBatchTestUpstreamSelected: () => void | Promise<void>;
  handleCancelUpstreamBatchTest: () => void;
  selectUpstreamSuccessful: () => void;
  selectUpstreamFailed: () => void;

  refreshUpstreamModels: () => void | Promise<void>;
  handleUpstreamSearchChange: (value: string) => void;
  handleAddUpstreamToAll: () => void | Promise<void>;

  handleTestUpstreamModel: (modelId: string, groupId?: number) => void | Promise<unknown>;
  copyModelName: (modelId: string) => void;
  clearUpstreamTestResults: () => void;
}
