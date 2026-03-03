import { ClearLogsDialog } from "./components/dialogs/clear-logs-dialog";
import { ModelSyncDetailDialog } from "./components/dialogs/model-sync-detail-dialog";
import { LogsActionsBar } from "./components/sections/logs/logs-actions-bar";
import { LogsListSection } from "./components/sections/logs/logs-list-section";
import { LogsPagination } from "./components/sections/logs/logs-pagination";
import { ModelSyncHeader } from "./components/sections/model-sync-header";
import { ModelSyncStatsStrip } from "./components/sections/model-sync-stats-strip";
import { ModelSyncTabs } from "./components/sections/model-sync-tabs";
import { RecentErrorsSection } from "./components/sections/recent/recent-errors-section";
import { RecentModelsSection } from "./components/sections/recent/recent-models-section";
import { useModelSyncLogsPage } from "./hooks/use-model-sync-logs-page";

export default function ModelSyncLogsPage() {
  const page = useModelSyncLogsPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <ModelSyncHeader
        syncing={page.syncing}
        onSyncNow={() => {
          void page.handleSyncNow();
        }}
      />

      <ModelSyncStatsStrip stats={page.stats} loading={page.statsLoading} />

      <ModelSyncTabs activeTab={page.activeTab} onChange={page.switchToTab} />

      {page.activeTab === "logs" ? (
        <>
          <LogsActionsBar
            logsCount={page.logs.length}
            selectedCount={page.selectedCount}
            allSelected={page.allSelected}
            showUnchanged={page.showUnchanged}
            onToggleSelectAll={page.handleToggleSelectAll}
            onDeleteSelected={() => {
              void page.handleDeleteSelected();
            }}
            onOpenClearDialog={() => page.setClearDialogOpen(true)}
            onShowUnchangedChange={page.handleShowUnchangedChange}
          />

          <LogsListSection
            loading={page.loading}
            logs={page.logs}
            allSelected={page.allSelected}
            isLogSelected={page.isLogSelected}
            onToggleSelectAll={page.handleToggleSelectAll}
            onToggleSelectLog={page.handleToggleSelectLog}
            onOpenDetail={page.openDetailLog}
          />

          <LogsPagination
            page={page.page}
            totalPages={page.totalPages}
            paginationText={page.paginationText}
            onPageChange={page.handlePageChange}
          />
        </>
      ) : page.activeTab === "errors" ? (
        <RecentErrorsSection
          loading={page.errorsLoading}
          logs={page.recentErrors}
          providersById={page.providersById}
          togglingProviderIds={page.togglingProviderIds}
          selectedCount={page.selectedErrorProvidersCount}
          allSelected={page.allErrorProvidersSelected}
          clearing={page.clearingErrors}
          onToggleModelEndpoint={page.handleToggleProviderModelEndpoint}
          onToggleSelectAll={page.handleToggleSelectAllErrorProviders}
          isProviderSelected={page.isErrorProviderSelected}
          onToggleSelectProvider={page.handleToggleSelectErrorProvider}
          onClearSelected={() => {
            void page.handleClearSelectedErrors();
          }}
          onClearAll={() => {
            void page.handleClearAllErrors();
          }}
          onOpenDetail={page.openDetailLog}
        />
      ) : (
        <RecentModelsSection loading={page.recentLoading} syncTime={page.syncTime} models={page.recentModels} />
      )}

      <ModelSyncDetailDialog log={page.detailLog} onClose={page.closeDetailLog} />

      <ClearLogsDialog
        open={page.clearDialogOpen}
        onOpenChange={page.setClearDialogOpen}
        onConfirm={() => {
          void page.handleClearAll();
        }}
      />
    </div>
  );
}
