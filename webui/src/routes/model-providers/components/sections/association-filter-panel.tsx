import type { AssociationBatchTestResult } from "../../types";
import type { Model, Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import { CheckCircle, ChevronDown, ChevronUp, Filter, MoreHorizontal, TestTube, TestTubes, Trash2, XCircle } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { BatchDeleteDialog } from "../dialogs/batch-delete-dialog";

type AssociationFilterPanelProps = {
  filterPanelOpen: boolean;
  onFilterPanelOpenChange: (open: boolean) => void;
  activeFilterCount: number;
  selectedModelId: number | null;
  models: Model[];
  onModelChange: (value: string) => void;
  selectedProviderType: string;
  onSelectedProviderTypeChange: (value: string) => void;
  selectedProviderFilter: string;
  onSelectedProviderFilterChange: (value: string) => void;
  selectedStatusFilter: string;
  onSelectedStatusFilterChange: (value: string) => void;
  searchKeyword: string;
  onSearchKeywordChange: (value: string) => void;
  providers: Provider[];
  providerTypes: string[];
  selectedAssociationCount: number;
  batchUpdatingStatus: boolean;
  batchTesting: boolean;
  filteredAssociationCount: number;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  onBatchDeleteDialogOpenChange: (open: boolean) => void;
  batchDeleteDialogOpen: boolean;
  batchDeleting: boolean;
  onBatchDeleteConfirm: () => void;
  onBatchUpdateStatus: (status: boolean) => void;
  onBatchTestSelected: () => void;
  onBatchTestAll: () => void;
  onSelectAllSuccessful: () => void;
  onSelectAllFailed: () => void;
  onToggleTemplateEditor: () => void;
  onOpenBlacklistDialog: () => void;
  onAutoAssociate: () => void;
  onCleanInvalid: () => void;
  onOpenCreateDialog: () => void;
};

export function AssociationFilterPanel({
  filterPanelOpen,
  onFilterPanelOpenChange,
  activeFilterCount,
  selectedModelId,
  models,
  onModelChange,
  selectedProviderType,
  onSelectedProviderTypeChange,
  selectedProviderFilter,
  onSelectedProviderFilterChange,
  selectedStatusFilter,
  onSelectedStatusFilterChange,
  searchKeyword,
  onSearchKeywordChange,
  providers,
  providerTypes,
  selectedAssociationCount,
  batchUpdatingStatus,
  batchTesting,
  filteredAssociationCount,
  associationTestResults,
  onBatchDeleteDialogOpenChange,
  batchDeleteDialogOpen,
  batchDeleting,
  onBatchDeleteConfirm,
  onBatchUpdateStatus,
  onBatchTestSelected,
  onBatchTestAll,
  onSelectAllSuccessful,
  onSelectAllFailed,
  onToggleTemplateEditor,
  onOpenBlacklistDialog,
  onAutoAssociate,
  onCleanInvalid,
  onOpenCreateDialog,
}: AssociationFilterPanelProps) {
  const successfulCount = Object.values(associationTestResults).filter((result) => result.success === true).length;
  const failedCount = Object.values(associationTestResults).filter((result) => result.success === false).length;
  const hasResults = Object.keys(associationTestResults).length > 0;

  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4 lg:gap-2">
        <div className="flex flex-col gap-1 text-xs">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">关联模型</Label>
          <Select value={selectedModelId?.toString() || ""} onValueChange={onModelChange}>
            <SelectTrigger className="h-8 w-full text-xs px-2">
              <SelectValue placeholder="选择模型" />
            </SelectTrigger>
            <SelectContent>
              {models.map((model) => (
                <SelectItem key={model.ID} value={model.ID.toString()}>
                  {model.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="sm:hidden">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onFilterPanelOpenChange(!filterPanelOpen)}
            className="w-full justify-between h-8 text-xs"
          >
            <span className="flex items-center gap-2">
              <Filter className="h-4 w-4" />
              <span>筛选与操作</span>
              {activeFilterCount > 0 && (
                <span className="bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded-full">
                  {activeFilterCount}
                </span>
              )}
            </span>
            {filterPanelOpen ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </Button>
        </div>

        <div className="hidden sm:flex flex-col gap-1 text-xs">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商类型</Label>
          <Select value={selectedProviderType} onValueChange={onSelectedProviderTypeChange}>
            <SelectTrigger className="h-8 w-full text-xs px-2">
              <SelectValue placeholder="按类型筛选" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部类型</SelectItem>
              {providerTypes.map((type) => (
                <SelectItem key={type} value={type}>
                  {type}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="hidden sm:flex flex-col gap-1 text-xs">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">具体提供商</Label>
          <Select value={selectedProviderFilter} onValueChange={onSelectedProviderFilterChange}>
            <SelectTrigger className="h-8 w-full text-xs px-2">
              <SelectValue placeholder="按提供商筛选" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部提供商</SelectItem>
              {providers.map((provider) => (
                <SelectItem key={provider.ID} value={provider.ID.toString()}>
                  {provider.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="hidden sm:flex flex-col gap-1 text-xs">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">启用状态</Label>
          <Select value={selectedStatusFilter} onValueChange={onSelectedStatusFilterChange}>
            <SelectTrigger className="h-8 w-full text-xs px-2">
              <SelectValue placeholder="按状态筛选" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="enabled">已启用</SelectItem>
              <SelectItem value="disabled">未启用</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div
        className={cn(
          "sm:hidden overflow-hidden transition-all duration-300 ease-in-out",
          filterPanelOpen ? "max-h-[220px] opacity-100" : "max-h-0 opacity-0"
        )}
      >
        <div className="grid grid-cols-1 gap-2 pt-1">
          <div className="flex flex-col gap-1 text-xs">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商类型</Label>
            <Select value={selectedProviderType} onValueChange={onSelectedProviderTypeChange}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="按类型筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部类型</SelectItem>
                {providerTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1 text-xs">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">具体提供商</Label>
            <Select value={selectedProviderFilter} onValueChange={onSelectedProviderFilterChange}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="按提供商筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部提供商</SelectItem>
                {providers.map((provider) => (
                  <SelectItem key={provider.ID} value={provider.ID.toString()}>
                    {provider.Name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1 text-xs">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">启用状态</Label>
            <Select value={selectedStatusFilter} onValueChange={onSelectedStatusFilterChange}>
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="按状态筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="enabled">已启用</SelectItem>
                <SelectItem value="disabled">未启用</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row gap-2">
        <div className="flex-1">
          <Input
            placeholder="搜索提供商、模型名称或ID..."
            value={searchKeyword}
            onChange={(event) => onSearchKeywordChange(event.target.value)}
            className="h-8 text-xs"
          />
        </div>
        <div className="flex gap-2 sm:flex-shrink-0 flex-wrap">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-8 text-xs flex-1 sm:flex-initial">
                批量操作
                {selectedAssociationCount > 0 && <span className="ml-1">({selectedAssociationCount})</span>}
                <ChevronDown className="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>

            <DropdownMenuContent align="start" className="w-56">
              <DropdownMenuItem
                disabled={selectedAssociationCount === 0}
                onClick={() => onBatchDeleteDialogOpenChange(true)}
                className="cursor-pointer"
              >
                <Trash2 className="mr-2 h-4 w-4 text-destructive" />
                <span>批量删除</span>
                {selectedAssociationCount > 0 && (
                  <span className="ml-auto text-xs text-muted-foreground">{selectedAssociationCount}</span>
                )}
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                disabled={selectedAssociationCount === 0 || batchUpdatingStatus}
                onClick={() => onBatchUpdateStatus(true)}
                className="cursor-pointer"
              >
                {batchUpdatingStatus ? (
                  <Spinner className="mr-2 h-4 w-4" />
                ) : (
                  <CheckCircle className="mr-2 h-4 w-4 text-green-600" />
                )}
                <span>批量启用</span>
              </DropdownMenuItem>

              <DropdownMenuItem
                disabled={selectedAssociationCount === 0 || batchUpdatingStatus}
                onClick={() => onBatchUpdateStatus(false)}
                className="cursor-pointer"
              >
                {batchUpdatingStatus ? (
                  <Spinner className="mr-2 h-4 w-4" />
                ) : (
                  <XCircle className="mr-2 h-4 w-4 text-orange-600" />
                )}
                <span>批量停用</span>
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                disabled={selectedAssociationCount === 0 || batchTesting}
                onClick={onBatchTestSelected}
                className="cursor-pointer"
              >
                <TestTube className="mr-2 h-4 w-4" />
                <span>批量测试选中</span>
              </DropdownMenuItem>

              <DropdownMenuItem
                disabled={filteredAssociationCount === 0 || batchTesting}
                onClick={onBatchTestAll}
                className="cursor-pointer"
              >
                {batchTesting ? <Spinner className="mr-2 h-4 w-4" /> : <TestTubes className="mr-2 h-4 w-4" />}
                <span>批量测试全部</span>
                <span className="ml-auto text-xs text-muted-foreground">{filteredAssociationCount}</span>
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                disabled={!hasResults || successfulCount === 0}
                onClick={onSelectAllSuccessful}
                className="cursor-pointer"
              >
                <CheckCircle className="mr-2 h-4 w-4 text-green-600" />
                <span>选择成功项</span>
                {successfulCount > 0 && <span className="ml-auto text-xs text-muted-foreground">{successfulCount}</span>}
              </DropdownMenuItem>

              <DropdownMenuItem disabled={!hasResults || failedCount === 0} onClick={onSelectAllFailed} className="cursor-pointer">
                <XCircle className="mr-2 h-4 w-4 text-red-600" />
                <span>选择失败项</span>
                {failedCount > 0 && <span className="ml-auto text-xs text-muted-foreground">{failedCount}</span>}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-8 text-xs flex-1 sm:flex-initial">
                更多
                <MoreHorizontal className="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              <DropdownMenuItem disabled={!selectedModelId} onClick={onToggleTemplateEditor} className="cursor-pointer">
                模板编辑
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onOpenBlacklistDialog} className="cursor-pointer">
                拉黑管理
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onAutoAssociate} className="cursor-pointer">
                一键关联
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onCleanInvalid} className="cursor-pointer">
                清除无效
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button onClick={onOpenCreateDialog} disabled={!selectedModelId} className="h-8 text-xs flex-1 sm:flex-initial">
            添加关联
          </Button>
        </div>
      </div>

      <BatchDeleteDialog
        open={batchDeleteDialogOpen}
        onOpenChange={onBatchDeleteDialogOpenChange}
        selectedCount={selectedAssociationCount}
        deleting={batchDeleting}
        onConfirm={onBatchDeleteConfirm}
      />
    </div>
  );
}
