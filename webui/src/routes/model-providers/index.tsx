import Loading from "@/components/loading";
import { AssociationFormDialog } from "./components/dialogs/association-form-dialog";
import { BlacklistDialog } from "./components/dialogs/blacklist-dialog";
import { ModelListDialog } from "./components/dialogs/model-list-dialog";
import { PreviewDialog } from "./components/dialogs/preview-dialog";
import { TemplateEditorDialog } from "./components/dialogs/template-editor-dialog";
import { TestDialog } from "./components/dialogs/test-dialog";
import { AssociationFilterPanel } from "./components/sections/association-filter-panel";
import { BatchTestProgressCard } from "./components/sections/batch-test-progress-card";
import { AssociationListSection } from "./components/sections/associations/association-list-section";
import { useModelProvidersPage } from "./hooks/use-model-providers-page";

export default function ModelProvidersPage() {
  const {
    shouldShowInitialLoading,
    statusError,
    associationFilterPanelProps,
    batchTestProgressCardProps,
    associationListSectionProps,
    blacklistDialogProps,
    templateEditorDialogProps,
    associationFormDialogProps,
    testDialogProps,
    modelListDialogProps,
    previewDialogProps,
  } = useModelProvidersPage();

  if (shouldShowInitialLoading) {
    return <Loading message="加载模型和提供商" />;
  }

  return (
    <div className="h-full min-h-0 flex flex-col gap-3 p-1">
      <div className="flex flex-col gap-2 flex-shrink-0">
        <h2 className="text-base font-semibold tracking-tight sm:text-xl">模型提供商关联</h2>
      </div>

      <AssociationFilterPanel {...associationFilterPanelProps} />

      <BatchTestProgressCard {...batchTestProgressCardProps} />

      {statusError && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {statusError}
        </div>
      )}

      <AssociationListSection {...associationListSectionProps} />

      <BlacklistDialog {...blacklistDialogProps} />

      <TemplateEditorDialog {...templateEditorDialogProps} />

      <AssociationFormDialog {...associationFormDialogProps} />

      <TestDialog {...testDialogProps} />

      <ModelListDialog {...modelListDialogProps} />

      <PreviewDialog {...previewDialogProps} />
    </div>
  );
}
