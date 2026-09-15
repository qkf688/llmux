import Loading from "@/components/loading";
import { TableCard } from "@/components/table-card";
import type { ApiKeyItem } from "@/lib/api";
import { ApiKeysTableDesktop } from "./api-keys-desktop-table";
import { ApiKeysMobileList } from "./api-keys-mobile-list";

type ApiKeysListSectionProps = {
  apiKeys: ApiKeyItem[];
  isLoading?: boolean;
  isError?: boolean;
  revealedId: number | null;
  onToggleReveal: (id: number) => void;
  onToggle: (apiKey: ApiKeyItem) => void;
  onEdit: (apiKey: ApiKeyItem) => void;
  onRequestDelete: (apiKey: ApiKeyItem) => void;
  busy?: boolean;
};

export function ApiKeysListSection({
  apiKeys,
  isLoading = false,
  isError = false,
  revealedId,
  onToggleReveal,
  onToggle,
  onEdit,
  onRequestDelete,
  busy = false,
}: ApiKeysListSectionProps) {
  return (
    <TableCard>
      {isLoading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载 API Keys" />
        </div>
      ) : isError ? (
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-destructive">
          加载 API Keys 失败，请刷新重试
        </div>
      ) : apiKeys.length === 0 ? (
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
          暂无 API Key，创建后即可用于调用 /v1 接口
        </div>
      ) : (
        <div className="flex h-full flex-col">
          <ApiKeysTableDesktop
            apiKeys={apiKeys}
            revealedId={revealedId}
            onToggleReveal={onToggleReveal}
            onToggle={onToggle}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
            busy={busy}
          />
          <ApiKeysMobileList
            apiKeys={apiKeys}
            revealedId={revealedId}
            onToggleReveal={onToggleReveal}
            onToggle={onToggle}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
            busy={busy}
          />
        </div>
      )}
    </TableCard>
  );
}
