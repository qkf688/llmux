import type { Provider } from "@/lib/api";
import type { Setter } from "@/stores/core/updater";
import type {
  AllModelsTypeFilter,
  BatchTestProgress,
  ModelTestResult,
  UpstreamStatus,
} from "../../types";

export const FILTER_OPTIONS: {
  readonly key: AllModelsTypeFilter;
  readonly label: string;
}[] = [
  { key: "all", label: "全部" },
  { key: "upstream", label: "上游" },
  { key: "custom", label: "自定义" },
];

export interface AllModelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  allModelsProvider: Provider | null;

  upstreamStatus: UpstreamStatus;
  upstreamModelsList: string[];
  upstreamSet: Set<string>;

  allModelsList: string[];
  filteredAllModels: string[];

  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;

  allModelsTypeFilter: AllModelsTypeFilter;
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => void;

  allModelsTestResults: Record<string, ModelTestResult>;
  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;
  syncingModels: boolean;
  addingModels: boolean;

  selectedAllModels: string[];
  setSelectedAllModels: Setter<string[]>;
  isAllFilteredSelected: boolean;
  toggleSelectAllModels: () => void;

  handleSyncUpstreamModels: () => void | Promise<void>;
  handleBatchTestAll: () => void | Promise<void>;
  handleBatchTestSelected: () => void | Promise<void>;
  handleCancelBatchTest: () => void;
  selectAllSuccessful: () => void;
  selectAllFailed: () => void;
  handleRemoveSelectedModels: () => void | Promise<void>;

  handleTestAllModel: (modelId: string) => void | Promise<unknown>;
  copyModelName: (modelId: string) => void;
  handleRemoveModelFromAll: (modelId: string) => void | Promise<void>;

  customModelInput: string;
  setCustomModelInput: (value: string) => void;
  handleAddCustomModels: () => void | Promise<void>;
}
