import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import type { ApiKeyItem } from "@/lib/api";
import { formatDateTime } from "@/lib/formatters";
import { ApiKeysKeyCell } from "./api-keys-key-cell";

type ApiKeysMobileListProps = {
  apiKeys: ApiKeyItem[];
  revealedId: number | null;
  onToggleReveal: (id: number) => void;
  onToggle: (apiKey: ApiKeyItem) => void;
  onEdit: (apiKey: ApiKeyItem) => void;
  onRequestDelete: (apiKey: ApiKeyItem) => void;
  busy?: boolean;
};

export function ApiKeysMobileList({
  apiKeys,
  revealedId,
  onToggleReveal,
  onToggle,
  onEdit,
  onRequestDelete,
  busy = false,
}: ApiKeysMobileListProps) {
  return (
    <StaggerList className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border">
      {apiKeys.map((apiKey) => (
        <StaggerItem key={apiKey.id} className="py-2 space-y-2 my-0.5 px-1">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <h3 className="font-semibold text-[13px] leading-snug break-words">{apiKey.name}</h3>
              <p className="text-[11px] text-muted-foreground">{formatDateTime(apiKey.created_at)}</p>
            </div>
            {/* 状态文案交给 Switch 的 aria-label + 动态小字，避免与开关文案冲突 */}
            <div className="flex shrink-0 items-center gap-2">
              <span className="text-[11px] text-muted-foreground">{apiKey.enabled ? "启用" : "停用"}</span>
              <Switch
                checked={apiKey.enabled}
                disabled={busy}
                onCheckedChange={() => onToggle(apiKey)}
                aria-label={`${apiKey.enabled ? "停用" : "启用"} ${apiKey.name}`}
              />
            </div>
          </div>
          <ApiKeysKeyCell
            apiKey={apiKey}
            revealed={revealedId === apiKey.id}
            onToggleReveal={onToggleReveal}
            size="xs"
          />
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => onEdit(apiKey)}>
              重命名
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-destructive hover:text-destructive"
              onClick={() => onRequestDelete(apiKey)}
            >
              删除
            </Button>
          </div>
        </StaggerItem>
      ))}
    </StaggerList>
  );
}
