import type { GroupRef, Provider } from "@/lib/api";
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
  modelGroupMap: Map<string, GroupRef[]>;

  allModelsList: string[];
  filteredAllModels: string[];

  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;

  allModelsTypeFilter: AllModelsTypeFilter;
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => void;

  availableGroups: GroupRef[];
  groupFilter: string;
  onGroupFilterChange: (value: string) => void;

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

  handleTestAllModel: (modelId: string, groupId?: number) => void | Promise<unknown>;
  copyModelName: (modelId: string) => void;
  handleRemoveModelFromAll: (modelId: string) => void | Promise<void>;
  clearAllTestResults: () => void;

  customModelInput: string;
  setCustomModelInput: (value: string) => void;
  /** 返回 true 表示确有新增并写入成功（成功路径会清空输入草稿） */
  handleAddCustomModels: () => Promise<boolean>;
}
