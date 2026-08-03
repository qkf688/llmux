import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { CheckCircle, ChevronDown, RotateCcw } from "lucide-react";

type OperationScopeToolbarProps = {
  operationScope: "current" | "all";
  onOperationScopeChange: (value: "current" | "all") => void;
  selectedModelName: string;
  selectedModelId: number | null;
  resettingWeights: boolean;
  resettingPriorities: boolean;
  enablingAssociations: boolean;
  onResetWeights: () => void;
  onResetPriorities: () => void;
  onEnableAssociations: () => void;
};

export function OperationScopeToolbar({
  operationScope,
  onOperationScopeChange,
  selectedModelName,
  selectedModelId,
  resettingWeights,
  resettingPriorities,
  enablingAssociations,
  onResetWeights,
  onResetPriorities,
  onEnableAssociations,
}: OperationScopeToolbarProps) {
  const isGlobalScope = operationScope === "all";
  const disableGlobalActions = !selectedModelId && !isGlobalScope;

  return (
    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
      <span className="flex items-center gap-2">
        <span>范围：</span>
        <Select value={operationScope} onValueChange={(value) => onOperationScopeChange(value as "current" | "all")}>
          <SelectTrigger className="h-6 w-[96px] text-xs px-2">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="current">当前模型</SelectItem>
            <SelectItem value="all">全部模型</SelectItem>
          </SelectContent>
        </Select>
      </span>
      <span>
        模型：<span className="text-foreground">{selectedModelName}</span>
      </span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className="h-6 px-2 text-xs"
            disabled={disableGlobalActions}
          >
            启用与重置
            <ChevronDown className="ml-1.5 h-3.5 w-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem
            disabled={disableGlobalActions || enablingAssociations}
            onClick={onEnableAssociations}
            className="cursor-pointer"
          >
            {enablingAssociations ? (
              <Spinner className="mr-2 h-4 w-4" />
            ) : (
              <CheckCircle className="mr-2 h-4 w-4 text-success" />
            )}
            启用所有关联
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem
            disabled={disableGlobalActions || resettingWeights}
            onClick={onResetWeights}
            className="cursor-pointer"
          >
            {resettingWeights ? <Spinner className="mr-2 h-4 w-4" /> : <RotateCcw className="mr-2 h-4 w-4" />}
            重置权重
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={disableGlobalActions || resettingPriorities}
            onClick={onResetPriorities}
            className="cursor-pointer"
          >
            {resettingPriorities ? <Spinner className="mr-2 h-4 w-4" /> : <RotateCcw className="mr-2 h-4 w-4" />}
            重置优先级
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
