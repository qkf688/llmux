import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";

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
      <Button
        variant="outline"
        size="sm"
        className="h-6 px-2 text-xs"
        onClick={onResetWeights}
        disabled={disableGlobalActions || resettingWeights}
      >
        {resettingWeights ? <Spinner className="w-3 h-3 mr-1.5" /> : null}
        重置权重
      </Button>
      <Button
        variant="outline"
        size="sm"
        className="h-6 px-2 text-xs"
        onClick={onResetPriorities}
        disabled={disableGlobalActions || resettingPriorities}
      >
        {resettingPriorities ? <Spinner className="w-3 h-3 mr-1.5" /> : null}
        重置优先级
      </Button>
      <Button
        variant="outline"
        size="sm"
        className="h-6 px-2 text-xs"
        onClick={onEnableAssociations}
        disabled={disableGlobalActions || enablingAssociations}
      >
        {enablingAssociations ? <Spinner className="w-3 h-3 mr-1.5" /> : null}
        启用所有关联
      </Button>
    </div>
  );
}

