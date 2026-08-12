import { Boxes, Hash } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  STICKY_ACTIONS_CELL_CLS,
  STICKY_ACTIONS_HEAD_CLS,
  STICKY_HEADER_CLS,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import { extractAllModels } from "../../utils/config";
import { ProviderRowActions } from "./provider-row-actions";

interface ProvidersDesktopTableProps {
  providers: Provider[];
  updatingFilter: Record<number, boolean>;
  updatingAssociationTrigger: Record<number, boolean>;

  onOpenAllModelsDialog: (provider: Provider) => void | Promise<void>;
  onToggleModelEndpoint: (provider: Provider) => void | Promise<void>;
  onToggleAssociationTrigger: (
    provider: Provider,
    checked: boolean,
  ) => void | Promise<void>;
  onToggleModelFilter: (
    provider: Provider,
    checked: boolean,
  ) => void | Promise<void>;
  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onHandleClearAssociations: (providerId: number) => void | Promise<void>;
  onHandleDelete: (providerId: number) => void | Promise<void>;
}

export function ProvidersDesktopTable({
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  onOpenAllModelsDialog,
  onToggleModelEndpoint,
  onToggleAssociationTrigger,
  onToggleModelFilter,
  onEditProvider,
  onOpenModelsDialog,
  onHandleClearAssociations,
  onHandleDelete,
}: ProvidersDesktopTableProps) {
  return (
    <div className="hidden sm:block w-full overflow-x-auto">
      <Table className="min-w-[860px]">
        <TableHeader className={`z-10 ${STICKY_HEADER_CLS}`}>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>名称</TableHead>
            <TableHead>类型</TableHead>
            <TableHead>全部模型</TableHead>
            <TableHead>模型端点</TableHead>
            <TableHead>关联触发</TableHead>
            <TableHead>模型过滤</TableHead>
            {/* 操作列钉右：横向滚动时始终可见，与关联表格保持一致 */}
            <TableHead className={`w-[92px] ${STICKY_ACTIONS_HEAD_CLS}`}>
              操作
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {providers.map((provider) => {
            const allModels = extractAllModels(provider.Config);
            return (
              <TableRow key={provider.ID} className="group">
                <TableCell className="font-mono text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1">
                    <Hash className="size-3 opacity-60" />
                    {provider.ID}
                  </span>
                </TableCell>
                <TableCell className="font-medium">{provider.Name}</TableCell>
                <TableCell>
                  {/* pill 标签：与 HTML 基准 .tag 对齐 */}
                  <span
                    className={
                      "inline-flex items-center rounded-full border bg-card " +
                      "px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground"
                    }
                  >
                    {provider.Type}
                  </span>
                </TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => onOpenAllModelsDialog(provider)}
                    className="gap-1.5"
                  >
                    <Boxes className="size-4 opacity-70" />
                    {allModels.length}
                  </Button>
                </TableCell>
                <TableCell>
                  <Switch
                    checked={provider.ModelEndpoint ?? true}
                    onCheckedChange={() => onToggleModelEndpoint(provider)}
                  />
                </TableCell>
                <TableCell>
                  <Switch
                    checked={!(provider.blacklisted ?? false)}
                    onCheckedChange={(checked) =>
                      onToggleAssociationTrigger(provider, checked)
                    }
                    disabled={updatingAssociationTrigger[provider.ID]}
                  />
                </TableCell>
                <TableCell>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className="flex items-center">
                        <Switch
                          checked={provider.ModelFilterEnabled ?? false}
                          onCheckedChange={(checked) =>
                            onToggleModelFilter(provider, checked)
                          }
                          disabled={updatingFilter[provider.ID]}
                        />
                      </div>
                    </TooltipTrigger>
                    <TooltipContent>启用模型过滤</TooltipContent>
                  </Tooltip>
                </TableCell>
                <TableCell className={STICKY_ACTIONS_CELL_CLS}>
                  <ProviderRowActions
                    provider={provider}
                    onEditProvider={onEditProvider}
                    onOpenModelsDialog={onOpenModelsDialog}
                    onHandleClearAssociations={onHandleClearAssociations}
                    onHandleDelete={onHandleDelete}
                  />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
