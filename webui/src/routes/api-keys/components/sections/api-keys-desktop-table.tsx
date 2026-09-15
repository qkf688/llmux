import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ApiKeyItem } from "@/lib/api";
import { formatDateTime } from "@/lib/formatters";
import { ApiKeysKeyCell } from "./api-keys-key-cell";

type ApiKeysDesktopTableProps = {
  apiKeys: ApiKeyItem[];
  revealedId: number | null;
  onToggleReveal: (id: number) => void;
  onToggle: (apiKey: ApiKeyItem) => void;
  onEdit: (apiKey: ApiKeyItem) => void;
  /** 请求删除（弹确认框），不直接删除 */
  onRequestDelete: (apiKey: ApiKeyItem) => void;
  /** 任一 mutation 在途时禁用启停开关，防 PATCH 视觉未动期间的连点 */
  busy?: boolean;
};

export function ApiKeysTableDesktop({
  apiKeys,
  revealedId,
  onToggleReveal,
  onToggle,
  onEdit,
  onRequestDelete,
  busy = false,
}: ApiKeysDesktopTableProps) {
  return (
    <div className="hidden sm:block w-full overflow-x-auto">
      <Table className="min-w-[820px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
          <TableRow className="hover:bg-secondary/90">
            <TableHead>ID</TableHead>
            <TableHead>名称</TableHead>
            <TableHead>API Key</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>创建时间</TableHead>
            <TableHead className="w-[100px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {apiKeys.map((apiKey) => (
            <TableRow key={apiKey.id}>
              <TableCell className="font-mono text-xs text-muted-foreground">{apiKey.id}</TableCell>
              <TableCell className="font-medium">{apiKey.name}</TableCell>
              <TableCell>
                <ApiKeysKeyCell
                  apiKey={apiKey}
                  revealed={revealedId === apiKey.id}
                  onToggleReveal={onToggleReveal}
                />
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Switch
                    checked={apiKey.enabled}
                    disabled={busy}
                    onCheckedChange={() => onToggle(apiKey)}
                    aria-label={`${apiKey.enabled ? "停用" : "启用"} ${apiKey.name}`}
                  />
                  <span className="text-xs text-muted-foreground">{apiKey.enabled ? "启用" : "停用"}</span>
                </div>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">{formatDateTime(apiKey.created_at)}</TableCell>
              <TableCell>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2"
                    onClick={() => onEdit(apiKey)}
                    aria-label={`重命名 ${apiKey.name}`}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 px-2 text-destructive hover:text-destructive"
                    onClick={() => onRequestDelete(apiKey)}
                    aria-label={`删除 ${apiKey.name}`}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
