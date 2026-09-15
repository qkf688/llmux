import Loading from "@/components/loading";
import type { ApiKeyItem } from "@/lib/api";
import { ApiKeysCardList } from "./api-keys-card-list";

type ApiKeysListSectionProps = {
  apiKeys: ApiKeyItem[];
  isLoading?: boolean;
  isError?: boolean;
  onToggle: (apiKey: ApiKeyItem) => void;
  onEdit: (apiKey: ApiKeyItem) => void;
  /** 请求删除（弹确认框），不直接删除 */
  onRequestDelete: (apiKey: ApiKeyItem) => void;
  busy?: boolean;
};

/**
 * 列表主体：加载/失败/空态 + octopus 式卡片列表。
 * 不再包 TableCard——卡片行本身即独立卡，外面再套表格壳会出现卡中卡；
 * 桌面/移动也不再分两视图（卡片布局天然响应式，双视图曾要求 reveal 态
 * 提升到 hook 共享，明文下掉后该复杂度一并消失）。
 */
export function ApiKeysListSection({
  apiKeys,
  isLoading = false,
  isError = false,
  onToggle,
  onEdit,
  onRequestDelete,
  busy = false,
}: ApiKeysListSectionProps) {
  if (isLoading) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center">
        <Loading message="加载 API Keys" />
      </div>
    );
  }
  if (isError) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center text-sm text-destructive">
        加载 API Keys 失败，请刷新重试
      </div>
    );
  }
  if (apiKeys.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">
        暂无 API Key，点右上「新建 Key」创建后，即可用于调用 /v1 接口
      </div>
    );
  }
  return (
    <ApiKeysCardList
      apiKeys={apiKeys}
      onToggle={onToggle}
      onEdit={onEdit}
      onRequestDelete={onRequestDelete}
      busy={busy}
    />
  );
}
