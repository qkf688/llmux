import { BatchDeleteLogsDialog } from "./components/dialogs/confirmations/batch-delete-logs-dialog";
import { ClearAllLogsDialog } from "./components/dialogs/confirmations/clear-all-logs-dialog";
import { ClearFilteredLogsDialog } from "./components/dialogs/confirmations/clear-filtered-logs-dialog";
import { DeleteLogDialog } from "./components/dialogs/confirmations/delete-log-dialog";
import { LogDetailDialog } from "./components/dialogs/detail/log-detail-dialog";
import { LogsFiltersSection } from "./components/sections/logs-filters";
import { LogsHeader } from "./components/sections/logs-header";
import { LogsListSection } from "./components/sections/list/logs-list-section";
import { LogsPagination } from "./components/sections/logs-pagination";
import { useLogsPage } from "./hooks/use-logs-page";

export default function LogsPage() {
  const page = useLogsPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <LogsHeader
        selectedCount={page.selectedCount}
        deleting={page.isDeleting}
        clearingAll={page.isClearingAll}
        clearingFiltered={page.isClearingFiltered}
        canClearFiltered={page.canClearFiltered}
        onRefresh={page.refreshLogs}
        onOpenBatchDelete={page.openBatchDeleteDialog}
        onOpenClearAll={() => page.setClearAllDialogOpen(true)}
        onOpenClearFiltered={() => page.setClearFilteredDialogOpen(true)}
      />

      <LogsFiltersSection
        filters={page.filters}
        models={page.models}
        providers={page.providers}
        userAgents={page.userAgents}
        availableStyles={page.availableStyles}
        onFilterChange={page.handleFilterChange}
      />

      <LogsListSection
        loading={page.loading}
        hasLogs={page.hasLogs}
        logs={page.logs}
        page={page.page}
        selectedIds={page.selectedIds}
        isAllSelected={page.isAllSelected}
        isSomeSelected={page.isSomeSelected}
        deleting={page.isDeleting}
        canViewChatIO={page.canViewChatIO}
        onSelectAll={page.handleSelectAll}
        onSelectOne={page.handleSelectOne}
        onOpenDetail={page.openDetailDialog}
        onViewChatIO={page.handleViewChatIO}
        onOpenDelete={page.openDeleteDialog}
      />

      <LogsPagination
        total={page.total}
        page={page.page}
        pages={page.pages}
        pageSize={page.pageSize}
        onPageChange={page.handlePageChange}
        onPageSizeChange={page.handlePageSizeChange}
      />

      <LogDetailDialog
        open={page.detailDialogOpen}
        log={page.selectedLog}
        onOpenChange={page.handleDetailDialogOpenChange}
        onExportLog={page.handleExportLog}
      />

      <DeleteLogDialog
        open={page.deleteDialogOpen}
        deleting={page.isDeleting}
        onOpenChange={page.handleDeleteDialogOpenChange}
        onConfirm={() => {
          void page.confirmDeleteLog();
        }}
      />

      <BatchDeleteLogsDialog
        open={page.batchDeleteDialogOpen}
        selectedCount={page.selectedCount}
        deleting={page.isDeleting}
        onOpenChange={page.setBatchDeleteDialogOpen}
        onConfirm={() => {
          void page.confirmBatchDelete();
        }}
      />

      <ClearAllLogsDialog
        open={page.clearAllDialogOpen}
        clearing={page.isClearingAll}
        onOpenChange={page.setClearAllDialogOpen}
        onConfirm={() => {
          void page.confirmClearAllLogs();
        }}
      />

      <ClearFilteredLogsDialog
        open={page.clearFilteredDialogOpen}
        clearing={page.isClearingFiltered}
        filtersSummary={page.filtersSummary}
        onOpenChange={page.setClearFilteredDialogOpen}
        onConfirm={() => {
          void page.confirmClearFilteredLogs();
        }}
      />
    </div>
  );
}
