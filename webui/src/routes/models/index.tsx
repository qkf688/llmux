import { BatchSettingsDialog } from "./components/dialogs/batch-settings-dialog";
import { ModelDeleteDialog } from "./components/dialogs/model-delete-dialog";
import { ModelFormDialog } from "./components/dialogs/model-form-dialog";
import { ModelPickerDialog } from "./components/dialogs/model-picker-dialog";
import { ModelsListSection } from "./components/sections/models-list-section";
import { ModelsHeader } from "./components/sections/models-header";
import { ModelsToolbar } from "./components/sections/models-toolbar";
import { useModelsPage } from "./hooks/use-models-page";

/** 页面容器：只做组合，编排在 useModelsPage */
export default function ModelsPage() {
  const {
    headerProps,
    toolbarProps,
    listSectionProps,
    modelFormDialogProps,
    modelPickerDialogProps,
    batchSettingsDialogProps,
    modelDeleteDialogProps,
  } = useModelsPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4">
      <ModelsHeader {...headerProps} />
      <ModelsToolbar {...toolbarProps} />
      <ModelsListSection {...listSectionProps} />
      <ModelFormDialog {...modelFormDialogProps} />
      <ModelPickerDialog {...modelPickerDialogProps} />
      <BatchSettingsDialog {...batchSettingsDialogProps} />
      <ModelDeleteDialog {...modelDeleteDialogProps} />
    </div>
  );
}