import type { AssociationBatchTestResult } from "../../types";
import type { Model, Provider } from "@/lib/api";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import {
  CheckCircle,
  ChevronDown,
  Filter,
  Plus,
  Settings2,
  SlidersHorizontal,
  TestTube,
  TestTubes,
  Trash2,
  X,
  XCircle,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ActionMenuDialog, type ActionMenuProps } from "../dialogs/action-menu-dialog";
import { BatchActionSheet } from "../dialogs/batch-action-sheet";
import { BatchCapabilitiesDialog } from "../dialogs/batch-capabilities-dialog";
import { BatchDeleteDialog } from "../dialogs/batch-delete-dialog";
import { FilterSheet } from "../dialogs/filter-sheet";

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
  batchActionSheetOpen: boolean;
  onBatchActionSheetOpenChange: (open: boolean) => void;
  batchCapabilitiesDialogOpen: boolean;
  onBatchCapabilitiesDialogOpenChange: (open: boolean) => void;
  batchUpdatingCapabilities: boolean;
  onBatchUpdateCapabilities: (capabilities: { tool_call?: boolean; structured_output?: boolean; image?: boolean }) => Promise<void>;
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
  onOpenCreateDialog: () => void;
  // ActionMenuDialog（操作菜单）
  actionMenuProps: ActionMenuProps;
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
  batchActionSheetOpen,
  onBatchActionSheetOpenChange,
  batchCapabilitiesDialogOpen,
  onBatchCapabilitiesDialogOpenChange,
  batchUpdatingCapabilities,
  onBatchUpdateCapabilities,
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
  onOpenCreateDialog,
  actionMenuProps,
}: AssociationFilterPanelProps) {
  const successfulCount = Object.values(associationTestResults).filter((result) => result.success === true).length;
  const failedCount = Object.values(associationTestResults).filter((result) => result.success === false).length;
  const hasResults = Object.keys(associationTestResults).length > 0;

  const [actionMenuDialogOpen, setActionMenuDialogOpen] = useState(false);

  // 模型选择器（移动端/桌面端共用，避免重复）
  const modelSelect = (
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
  );

  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      {/* 移动端：模型选择器 + 筛选按钮合并一行 */}
      <div className="flex gap-2 sm:hidden">
        <div className="flex-1 min-w-0">{modelSelect}</div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onFilterPanelOpenChange(true)}
          className="flex-shrink-0 h-8 text-xs"
        >
          <span className="flex items-center gap-1.5">
            <Filter className="h-4 w-4" />
            <span>筛选</span>
            {activeFilterCount > 0 && (
              <span className="bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded-full">
                {activeFilterCount}
              </span>
            )}
          </span>
        </Button>
      </div>

      {/* 移动端：筛选激活条件芯片行 */}
      {activeFilterCount > 0 && (
        <div className="flex flex-wrap gap-1.5 sm:hidden">
          {selectedProviderType !== "all" && (
            <FilterChip label="类型" value={selectedProviderType} onRemove={() => onSelectedProviderTypeChange("all")} />
          )}
          {selectedProviderFilter !== "all" && (
            <FilterChip
              label="提供商"
              value={providers.find((p) => p.ID.toString() === selectedProviderFilter)?.Name ?? selectedProviderFilter}
              onRemove={() => onSelectedProviderFilterChange("all")}
            />
          )}
          {selectedStatusFilter !== "all" && (
            <FilterChip
              label="状态"
              value={selectedStatusFilter === "enabled" ? "已启用" : "未启用"}
              onRemove={() => onSelectedStatusFilterChange("all")}
            />
          )}
        </div>
      )}

      {/* 桌面端：4列 grid（模型 + 3个筛选器） */}
      <div className="hidden sm:grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div className="flex flex-col gap-1 text-xs">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">关联模型</Label>
          {modelSelect}
        </div>

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

      {/* 移动端：搜索框 + 图标按钮组（单行） */}
      <div className="flex flex-col gap-2 sm:hidden">
        <Input
          placeholder="搜索提供商、模型名称或ID..."
          value={searchKeyword}
          onChange={(event) => onSearchKeywordChange(event.target.value)}
          className="h-8 text-xs"
        />
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8 flex-shrink-0"
            onClick={() => setActionMenuDialogOpen(true)}
            aria-label="操作菜单"
          >
            <Settings2 className="h-4 w-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8 relative flex-shrink-0"
            onClick={() => onBatchActionSheetOpenChange(true)}
          >
            <SlidersHorizontal className="h-4 w-4" />
            {selectedAssociationCount > 0 && (
              <span className="absolute -top-1 -right-1 bg-primary text-primary-foreground text-[10px] min-w-[16px] h-[16px] flex items-center justify-center rounded-full px-1 leading-none">
                {selectedAssociationCount}
              </span>
            )}
          </Button>
          <Button onClick={onOpenCreateDialog} disabled={!selectedModelId} className="h-8 text-xs flex-1">
            <Plus className="h-4 w-4" />
            添加关联
          </Button>
        </div>
      </div>

      {/* 桌面端：搜索框 + 按钮组 */}
      <div className="hidden sm:flex flex-row gap-2">
        <div className="flex-1">
          <Input
            placeholder="搜索提供商、模型名称或ID..."
            value={searchKeyword}
            onChange={(event) => onSearchKeywordChange(event.target.value)}
            className="h-8 text-xs"
          />
        </div>
        <div className="flex gap-2 flex-shrink-0">
          <Button
            variant="outline"
            className="h-8 text-xs"
            onClick={() => setActionMenuDialogOpen(true)}
          >
            <Settings2 className="mr-1.5 h-4 w-4" />
            操作
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-8 text-xs">
                批量操作
                {selectedAssociationCount > 0 && <span className="ml-1">({selectedAssociationCount})</span>}
                <ChevronDown className="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>

            <DropdownMenuContent align="start" className="w-56">
              <DropdownMenuItem
                disabled={selectedAssociationCount === 0}
                onSelect={() => {
                  window.setTimeout(() => onBatchCapabilitiesDialogOpenChange(true), 0);
                }}
                className="cursor-pointer"
              >
                <SlidersHorizontal className="mr-2 h-4 w-4" />
                <span>批量设置能力</span>
                {selectedAssociationCount > 0 && (
                  <span className="ml-auto text-xs text-muted-foreground">{selectedAssociationCount}</span>
                )}
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                disabled={selectedAssociationCount === 0}
                onSelect={() => {
                  window.setTimeout(() => onBatchDeleteDialogOpenChange(true), 0);
                }}
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
                  <CheckCircle className="mr-2 h-4 w-4 text-success" />
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
                  <XCircle className="mr-2 h-4 w-4 text-warning" />
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
                <CheckCircle className="mr-2 h-4 w-4 text-success" />
                <span>选择成功项</span>
                {successfulCount > 0 && <span className="ml-auto text-xs text-muted-foreground">{successfulCount}</span>}
              </DropdownMenuItem>

              <DropdownMenuItem disabled={!hasResults || failedCount === 0} onClick={onSelectAllFailed} className="cursor-pointer">
                <XCircle className="mr-2 h-4 w-4 text-destructive" />
                <span>选择失败项</span>
                {failedCount > 0 && <span className="ml-auto text-xs text-muted-foreground">{failedCount}</span>}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button onClick={onOpenCreateDialog} disabled={!selectedModelId} className="h-8 text-xs">
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

      <BatchCapabilitiesDialog
        open={batchCapabilitiesDialogOpen}
        onOpenChange={onBatchCapabilitiesDialogOpenChange}
        selectedCount={selectedAssociationCount}
        updating={batchUpdatingCapabilities}
        onConfirm={onBatchUpdateCapabilities}
      />

      <FilterSheet
        open={filterPanelOpen}
        onOpenChange={onFilterPanelOpenChange}
        selectedProviderType={selectedProviderType}
        onSelectedProviderTypeChange={onSelectedProviderTypeChange}
        selectedProviderFilter={selectedProviderFilter}
        onSelectedProviderFilterChange={onSelectedProviderFilterChange}
        selectedStatusFilter={selectedStatusFilter}
        onSelectedStatusFilterChange={onSelectedStatusFilterChange}
        providers={providers}
        providerTypes={providerTypes}
        onReset={() => {
          onSelectedProviderTypeChange("all");
          onSelectedProviderFilterChange("all");
          onSelectedStatusFilterChange("all");
        }}
      />

      <ActionMenuDialog
        open={actionMenuDialogOpen}
        onOpenChange={setActionMenuDialogOpen}
        {...actionMenuProps}
      />

      <BatchActionSheet
        open={batchActionSheetOpen}
        onOpenChange={onBatchActionSheetOpenChange}
        selectedAssociationCount={selectedAssociationCount}
        filteredAssociationCount={filteredAssociationCount}
        batchUpdatingStatus={batchUpdatingStatus}
        batchTesting={batchTesting}
        associationTestResults={associationTestResults}
        onBatchUpdateStatus={onBatchUpdateStatus}
        onBatchTestSelected={onBatchTestSelected}
        onBatchTestAll={onBatchTestAll}
        onSelectAllSuccessful={onSelectAllSuccessful}
        onSelectAllFailed={onSelectAllFailed}
        onOpenBatchCapabilitiesDialog={() => {
          window.setTimeout(() => onBatchCapabilitiesDialogOpenChange(true), 0);
        }}
        onOpenBatchDeleteDialog={() => {
          window.setTimeout(() => onBatchDeleteDialogOpenChange(true), 0);
        }}
      />
    </div>
  );
}

// 筛选条件芯片：显示当前激活的筛选条件，点 × 清除单个筛选
type FilterChipProps = {
  label: string;
  value: string;
  onRemove: () => void;
};

function FilterChip({ label, value, onRemove }: FilterChipProps) {
  return (
    <span className="inline-flex items-center gap-1 bg-secondary text-secondary-foreground border border-border rounded-full py-0.5 pl-2.5 pr-1 text-xs">
      {label}: {value}
      <button
        onClick={onRemove}
        aria-label={`清除${label}筛选`}
        className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-muted hover:bg-destructive transition-colors"
      >
        <X className="h-3 w-3" />
      </button>
    </span>
  );
}
