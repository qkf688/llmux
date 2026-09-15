import { useApiKeysPage } from "./hooks/use-api-keys-page";
import { ApiKeysHeader } from "./components/sections/api-keys-header";
import { ApiKeysListSection } from "./components/sections/api-keys-list-section";
import { ApiKeyFormDialog } from "./components/dialogs/api-key-form-dialog";
import { ApiKeyDeleteDialog } from "./components/dialogs/api-key-delete-dialog";

export default function ApiKeysPage() {
  const page = useApiKeysPage();

  return (
    <div className="h-full min-h-0 flex flex-col gap-4">
      <ApiKeysHeader keyCount={page.apiKeys.length} onCreate={page.openCreateForm} />
      <ApiKeysListSection
        apiKeys={page.apiKeys}
        isLoading={page.isLoading}
        isError={page.isError}
        revealedId={page.revealedId}
        onToggleReveal={page.toggleReveal}
        onToggle={page.toggleApiKey}
        onEdit={page.openEditForm}
        onRequestDelete={page.requestDelete}
        busy={page.isSaving}
      />
      <ApiKeyFormDialog
        open={page.formOpen}
        onOpenChange={page.setFormOpen}
        apiKey={page.editingKey}
        onSaved={page.handleApiKeySaved}
        isSaving={page.isSaving}
      />
      <ApiKeyDeleteDialog
        open={page.deleteOpen}
        onOpenChange={page.setDeleteOpen}
        keyName={page.deleteKeyName}
        onConfirm={() => void page.handleDeleteConfirm()}
      />
    </div>
  );
}
