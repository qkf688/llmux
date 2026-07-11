import { ProvidersListSection } from "./components/sections/providers-list-section";
import { ProvidersToolbar } from "./components/sections/providers-toolbar";
import { ProviderFormDialog } from "./components/dialogs/provider-form-dialog";
import { AllModelsDialog } from "./components/dialogs/all-models-dialog";
import { UpstreamModelsDialog } from "./components/dialogs/upstream-models-dialog";
import { useProvidersPage } from "./hooks/use-providers-page";

/** 页面容器：只做组合，编排在 useProvidersPage */
export default function ProvidersPage() {
  const {
    toolbarProps,
    listSectionProps,
    providerFormDialogProps,
    allModelsDialogProps,
    upstreamModelsDialogProps,
  } = useProvidersPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <ProvidersToolbar {...toolbarProps} />
      <ProvidersListSection {...listSectionProps} />
      <ProviderFormDialog {...providerFormDialogProps} />
      <AllModelsDialog {...allModelsDialogProps} />
      <UpstreamModelsDialog {...upstreamModelsDialogProps} />
    </div>
  );
}