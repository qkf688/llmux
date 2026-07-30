// 纯类型聚合文件：用 ReturnType 引用各领域 hook 产出，组装上下文对象类型。
// 装配器只读上下文，不再接收 80+ 扁平字段（学 models 的 ReturnType 模式）。
import type { Model } from "@/lib/api";
import type { useModelProvidersPageLocalState } from "./use-model-providers-page-local-state";
import type { useModelProvidersPageStoreState } from "./use-model-providers-page-store";
import type { useModelProvidersAssociationStatus } from "./use-model-providers-association-status";
import type { useModelProvidersAssociationFilters } from "./use-model-providers-association-filters";
import type { useModelProvidersModelListVisibility } from "./use-model-providers-model-list-visibility";
import type { useModelProvidersModelListSelection } from "./use-model-providers-model-list-selection";
import type { useModelProvidersAssociationForm } from "./use-model-providers-association-form";
import type { useModelProvidersBlacklist } from "./use-model-providers-blacklist";
import type { useModelProvidersTemplateEditor } from "./use-model-providers-template-editor";
import type { useModelProvidersAssociationMutations } from "./use-model-providers-association-mutations";
import type { useModelProvidersAssociationStatusToggle } from "./use-model-providers-association-status-toggle";
import type { useModelProvidersOperationScope } from "./use-model-providers-operation-scope";
import type { useModelProvidersPreview } from "./use-model-providers-preview";
import type { useModelProvidersTesting } from "./use-model-providers-testing";
import type { useModelProvidersBatch } from "./use-model-providers-batch";
import type { useModelProvidersAssociationDialog } from "./use-model-providers-association-dialog";

/** 内联 modelChange（原 useModelProvidersModelChange）的产出类型。 */
type ModelChangeCtx = { handleModelChange: (modelId: string) => void };
/** 内联 pageActions（原 useModelProvidersPageActions）的产出类型。 */
type PageActionsCtx = {
  openDeleteDialog: (id: number) => void;
  toggleProviderCollapse: (providerId: number) => void;
  refreshStatus: () => void;
  handleDeleteDialogChange: (openValue: boolean) => void;
  confirmPreviewAction: () => void;
  addTemplateItem: () => void;
  deleteTemplateItem: (name: string) => void;
};

/**
 * Composition root 组装的上下文对象（学 models 的 ReturnType 模式）。
 * 装配器只读这些对象，不再接收 80+ 扁平字段。
 */
export interface ModelProvidersPageContext {
  // 派生数据
  localState: ReturnType<typeof useModelProvidersPageLocalState>;
  store: ReturnType<typeof useModelProvidersPageStoreState>;
  associationStatus: ReturnType<typeof useModelProvidersAssociationStatus>;
  filters: ReturnType<typeof useModelProvidersAssociationFilters>;
  modelListVisibility: ReturnType<typeof useModelProvidersModelListVisibility>;

  // 对话框与表单
  form: ReturnType<typeof useModelProvidersAssociationForm>;
  blacklist: ReturnType<typeof useModelProvidersBlacklist>;
  templateEditor: ReturnType<typeof useModelProvidersTemplateEditor>;

  // 写操作
  mutations: ReturnType<typeof useModelProvidersAssociationMutations>;
  statusToggle: ReturnType<typeof useModelProvidersAssociationStatusToggle>;
  operationScope: ReturnType<typeof useModelProvidersOperationScope>;
  preview: ReturnType<typeof useModelProvidersPreview>;
  modelChange: ModelChangeCtx;

  // 测试与批量
  testing: ReturnType<typeof useModelProvidersTesting>;
  batch: ReturnType<typeof useModelProvidersBatch>;

  // 页面级薄包装
  pageActions: PageActionsCtx;
  associationDialog: ReturnType<typeof useModelProvidersAssociationDialog>;
  modelListSelection: ReturnType<typeof useModelProvidersModelListSelection>;

  // 页面级派生
  selectedModel: Model | null;
  isGlobalScope: boolean;
  shouldShowInitialLoading: boolean;
}

/** Section 装配器只读上下文的子集。 */
export type SectionPropsContext = Pick<
  ModelProvidersPageContext,
  | "store"
  | "filters"
  | "localState"
  | "associationStatus"
  | "batch"
  | "operationScope"
  | "preview"
  | "modelChange"
  | "pageActions"
  | "mutations"
  | "statusToggle"
  | "testing"
  | "associationDialog"
  | "templateEditor"
  | "blacklist"
  | "selectedModel"
  | "isGlobalScope"
>;

/** Dialog 装配器只读上下文的子集。 */
export type DialogPropsContext = Pick<
  ModelProvidersPageContext,
  | "store"
  | "localState"
  | "form"
  | "blacklist"
  | "templateEditor"
  | "mutations"
  | "testing"
  | "modelListVisibility"
  | "modelListSelection"
  | "preview"
  | "pageActions"
>;
