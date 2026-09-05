import { useNumberPoolsPage } from "./hooks/use-number-pools-page";
import { PoolsHeader } from "./components/sections/pools-header";
import { PoolsListSection } from "./components/sections/pools-list-section";
import { PoolFormDialog } from "./components/dialogs/pool-form-dialog";
import { PoolDetailDialog } from "@/components/number-pools/pool-detail-dialog";

export default function NumberPoolsPage() {
  const page = useNumberPoolsPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4">
      <PoolsHeader poolCount={page.pools.length} onCreate={page.openCreateForm} />
      <PoolsListSection
        pools={page.pools}
        isLoading={page.isLoading}
        isError={page.isError}
        onOpenDetail={page.openDetail}
        onEdit={page.openEditForm}
        onDelete={(id) => void page.handlePoolDelete(id)}
      />
      <PoolFormDialog
        open={page.formOpen}
        onOpenChange={page.setFormOpen}
        pool={page.formPool}
        onSaved={page.handlePoolSaved}
        isSaving={page.isSaving}
      />
      <PoolDetailDialog
        open={page.detailOpen}
        onOpenChange={page.setDetailOpen}
        pool={page.detailPool}
      />
    </div>
  );
}
