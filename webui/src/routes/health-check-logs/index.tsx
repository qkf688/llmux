import { HealthCheckResultDialog } from "@/components/health-check-result-dialog";
import { ClearHealthCheckLogsDialog } from "./components/dialogs/clear-health-check-logs-dialog";
import { HealthCheckLogDetailDialog } from "./components/dialogs/health-check-log-detail-dialog";
import { HealthCheckBanner } from "./components/sections/health-check-banner";
import { HealthCheckFilters } from "./components/sections/health-check-filters";
import { HealthCheckHeader } from "./components/sections/health-check-header";
import { HealthCheckPagination } from "./components/sections/health-check-pagination";
import { HealthCheckLogsListSection } from "./components/sections/list/health-check-logs-list-section";
import { useHealthCheckLogsPage } from "./hooks/use-health-check-logs-page";

export default function HealthCheckLogsPage() {
  const page = useHealthCheckLogsPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4">
      <HealthCheckBanner
        batchId={page.backgroundBatchId}
        resultDialogOpen={page.resultDialogOpen}
        completed={page.backgroundCheckComplete}
        onShowProgress={page.showProgressDialog}
        onDismiss={page.handleDismissBanner}
      />

      <HealthCheckHeader
        runningBackgroundCheck={page.runningBackgroundCheck}
        clearingLogs={page.clearingLogs}
        onRunHealthCheck={() => {
          void page.handleRunHealthCheck();
        }}
        onOpenClearDialog={() => page.setClearDialogOpen(true)}
        onRefresh={page.refreshLogs}
      />

      <HealthCheckFilters
        filters={page.filters}
        models={page.models}
        providers={page.providers}
        onFilterChange={page.handleFilterChange}
      />

      <HealthCheckLogsListSection
        loading={page.loading}
        hasLogs={page.hasLogs}
        logs={page.logs}
        onOpenDetail={page.openDetailDialog}
        footer={
          <HealthCheckPagination
            total={page.total}
            page={page.page}
            pages={page.pages}
            pageSize={page.pageSize}
            onPageChange={page.handlePageChange}
            onPageSizeChange={page.handlePageSizeChange}
          />
        }
      />

      <HealthCheckLogDetailDialog
        open={page.detailDialogOpen}
        log={page.detailLog}
        onOpenChange={page.handleDetailDialogOpenChange}
      />

      <ClearHealthCheckLogsDialog
        open={page.clearDialogOpen}
        clearing={page.clearingLogs}
        onOpenChange={page.setClearDialogOpen}
        onConfirm={() => {
          void page.confirmClearLogs();
        }}
      />

      <HealthCheckResultDialog
        open={page.resultDialogOpen}
        onOpenChange={page.handleResultDialogOpenChange}
        batchId={page.currentBatchId}
      />
    </div>
  );
}
