import Loading from "@/components/loading";
import { BlacklistManagementDialog } from "./components/dialogs/blacklist/blacklist-management-dialog";
import { ProviderSelectorDialog } from "./components/dialogs/blacklist/provider-selector-dialog";
import { MappingBatchDialog } from "./components/dialogs/mappings/mapping-batch-dialog";
import { BatchDeleteMappingsDialog } from "./components/dialogs/mappings/batch-delete-mappings-dialog";
import { MappingFormDialog } from "./components/dialogs/mappings/mapping-form-dialog";
import { MappingManagementDialog } from "./components/dialogs/mappings/mapping-management-dialog";
import { VirtualModelDeleteDialog } from "./components/dialogs/virtual-model-delete-dialog";
import { VirtualModelFormDialog } from "./components/dialogs/virtual-model-form-dialog";
import { VirtualModelsHeader } from "./components/sections/virtual-models-header";
import { VirtualModelsTable } from "./components/sections/virtual-models-table";
import { useVirtualModelsPage } from "./hooks/use-virtual-models-page";
import { getStrategyLabel } from "./utils/strategy";

export default function VirtualModelsPage() {
  const page = useVirtualModelsPage();

  if (page.loading) {
    return <Loading />;
  }

  return (
    <div className="space-y-4">
      <VirtualModelsHeader onCreate={page.openCreateVirtualModel} />

      <VirtualModelsTable
        virtualModels={page.virtualModels}
        onManageMappings={(model) => {
          void page.openMappingsDialog(model);
        }}
        onManageBlacklist={(model) => {
          void page.openBlacklistDialog(model);
        }}
        onEdit={page.openEditVirtualModel}
        onDelete={(model) => page.requestDeleteVirtualModel(model.ID)}
        getStrategyLabel={getStrategyLabel}
      />

      <VirtualModelFormDialog
        open={page.modelDialogOpen}
        onOpenChange={page.setModelDialogOpen}
        editingModel={page.editingModel}
        form={page.virtualModelForm}
        onSubmit={(values) => {
          void page.submitVirtualModel(values);
        }}
      />

      <VirtualModelDeleteDialog
        open={page.modelToDeleteId !== null}
        onOpenChange={(open) => {
          if (!open) {
            page.closeDeleteDialog();
          }
        }}
        onConfirm={() => {
          void page.confirmDeleteVirtualModel();
        }}
      />

      <MappingManagementDialog
        open={page.mappingsDialogOpen}
        onOpenChange={page.handleMappingsDialogOpenChange}
        virtualModelName={page.currentVirtualModel?.Name}
        totalMappingsCount={page.mappings.length}
        mappings={page.filteredMappings}
        getRealModelName={page.getRealModelName}
        searchQuery={page.mappingSearchQuery}
        onSearchQueryChange={page.setMappingSearchQuery}
        selectedMappingIds={page.selectedMappingIds}
        isAllFilteredSelected={page.isAllFilteredSelected}
        isSomeFilteredSelected={page.isSomeFilteredSelected}
        onSelectAllFiltered={page.selectAllFilteredMappings}
        onToggleMappingSelection={page.toggleMappingSelection}
        onOpenBatchDialog={page.openBatchMappingDialog}
        onOpenBatchDeleteDialog={page.openBatchDeleteDialog}
        onEditMapping={page.openEditMappingDialog}
        onDeleteMapping={(mappingId) => {
          void page.deleteMapping(mappingId);
        }}
      />

      <BatchDeleteMappingsDialog
        open={page.mappingBatchDeleteDialogOpen}
        onOpenChange={page.setMappingBatchDeleteDialogOpen}
        selectedCount={page.selectedMappingIds.size}
        deleting={page.batchDeleting}
        onConfirm={() => {
          void page.confirmBatchDelete();
        }}
      />

      <MappingFormDialog
        open={page.mappingFormDialogOpen}
        onOpenChange={page.setMappingFormDialogOpen}
        form={page.mappingForm}
        editingMapping={page.editingMapping}
        realModels={page.realModels}
        onSubmit={(values) => {
          void page.submitMapping(values);
        }}
      />

      <MappingBatchDialog
        open={page.mappingBatchDialogOpen}
        onOpenChange={page.setMappingBatchDialogOpen}
        models={page.filteredModels}
        mappedModelIds={page.mappedModelIds}
        selectedModelIds={page.selectedModelIds}
        searchQuery={page.modelSearchQuery}
        onSearchQueryChange={page.setModelSearchQuery}
        onSelectAll={page.selectAllBatchModels}
        onInvertSelection={page.invertBatchModelSelection}
        onClearSelection={page.clearBatchModelSelection}
        onToggleModelSelection={page.toggleBatchModelSelection}
        batchPriority={page.batchPriority}
        onBatchPriorityChange={page.setBatchPriority}
        batchWeight={page.batchWeight}
        onBatchWeightChange={page.setBatchWeight}
        batchEnabled={page.batchEnabled}
        onBatchEnabledChange={page.setBatchEnabled}
        onSubmit={() => {
          void page.submitBatchMapping();
        }}
      />

      <BlacklistManagementDialog
        open={page.blacklistDialogOpen}
        onOpenChange={page.handleBlacklistDialogOpenChange}
        blacklistedProviders={page.blacklistedProviders}
        onOpenProviderSelector={page.openProviderSelectorDialog}
        onRemoveProvider={(providerId) => {
          void page.removeBlacklistedProvider(providerId);
        }}
      />

      <ProviderSelectorDialog
        open={page.providerSelectorDialogOpen}
        onOpenChange={page.setProviderSelectorDialogOpen}
        providers={page.filteredProviders}
        selectedProviderIds={page.selectedProviderIds}
        searchQuery={page.providerSearchQuery}
        onSearchQueryChange={page.setProviderSearchQuery}
        onToggleSelection={page.toggleProviderSelection}
        onConfirm={() => {
          void page.confirmAddBlacklistedProviders();
        }}
      />
    </div>
  );
}
