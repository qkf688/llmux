import { ExportConfigDialog } from "./components/dialogs/export-config-dialog";
import { ExportDatabaseDialog } from "./components/dialogs/export-database-dialog";
import { ImportConfigDialog } from "./components/dialogs/import-config-dialog";
import { VacuumDialog } from "./components/dialogs/vacuum-dialog";
import { DatabaseErrorState } from "./components/sections/database-error-state";
import { DatabaseHeader } from "./components/sections/database-header";
import { DatabaseInfoCard } from "./components/sections/database-info-card";
import { DatabaseLoadingState } from "./components/sections/database-loading-state";
import { DatabaseStatsCards } from "./components/sections/database-stats-cards";
import { TableStatsCard } from "./components/sections/table-stats-card";
import { useDatabasePage } from "./hooks/use-database-page";

export default function DatabasePage() {
  const page = useDatabasePage();

  return (
    <div className="space-y-4 sm:space-y-6">
      <DatabaseHeader
        loading={page.loading}
        vacuuming={page.vacuuming}
        exporting={page.exporting}
        exportingDatabase={page.exportingDatabase}
        importing={page.importing}
        onBack={page.goBackHome}
        onRefresh={page.refreshStats}
        onOpenExportConfig={() => page.setExportConfigDialogOpen(true)}
        onOpenExportDatabase={() => page.setExportDatabaseDialogOpen(true)}
        onOpenImport={() => page.handleImportDialogOpenChange(true)}
        onOpenVacuum={() => page.setVacuumDialogOpen(true)}
      />

      {page.loading ? (
        <DatabaseLoadingState />
      ) : page.stats ? (
        <>
          <DatabaseStatsCards stats={page.stats} usageRate={page.usageRate} />
          <TableStatsCard tableStats={page.stats.table_stats} />
          <DatabaseInfoCard stats={page.stats} />
        </>
      ) : (
        <DatabaseErrorState />
      )}

      <VacuumDialog
        open={page.vacuumDialogOpen}
        vacuuming={page.vacuuming}
        onOpenChange={page.setVacuumDialogOpen}
        onConfirm={() => {
          void page.handleVacuum();
        }}
      />

      <ExportDatabaseDialog
        open={page.exportDatabaseDialogOpen}
        exporting={page.exportingDatabase}
        onOpenChange={page.setExportDatabaseDialogOpen}
        onConfirm={() => {
          void page.handleExportDatabase();
        }}
      />

      <ExportConfigDialog
        open={page.exportConfigDialogOpen}
        exporting={page.exporting}
        exportTypes={page.exportTypes}
        onOpenChange={page.setExportConfigDialogOpen}
        onToggleType={page.toggleExportType}
        onConfirm={() => {
          void page.handleExportConfig();
        }}
      />

      <ImportConfigDialog
        open={page.importDialogOpen}
        importing={page.importing}
        mode={page.importMode}
        selectedFile={page.selectedFile}
        importTypes={page.importTypes}
        previewLoading={page.previewLoading}
        previewError={page.previewError}
        previewData={page.previewData}
        inputKey={page.importFileInputKey}
        onOpenChange={page.handleImportDialogOpenChange}
        onModeChange={page.setImportMode}
        onToggleType={page.toggleImportType}
        onFileChange={(file) => {
          void page.handleImportFileChange(file);
        }}
        onConfirm={() => {
          void page.handleImportConfig();
        }}
      />
    </div>
  );
}
