import { useState } from "react";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface ProvidersToolbarProps {
  syncingAll: boolean;
  onSyncAllProviders: () => void;

  nameFilter: string;
  setNameFilter: (value: string) => void;

  typeFilter: string;
  setTypeFilter: (value: string) => void;
  availableTypes: string[];

  flushNameFilter: () => void;
  onCreateProvider: () => void;
}

export function ProvidersToolbar({
  syncingAll,
  onSyncAllProviders,
  nameFilter,
  setNameFilter,
  typeFilter,
  setTypeFilter,
  availableTypes,
  flushNameFilter,
  onCreateProvider,
}: ProvidersToolbarProps) {
  const [syncConfirmOpen, setSyncConfirmOpen] = useState(false);

  const handleConfirmSync = () => {
    setSyncConfirmOpen(false);
    onSyncAllProviders();
  };

  return (
    <>
      {/* 页面头：标题 + 右侧动作 */}
      <div className="flex flex-wrap items-center gap-2 flex-shrink-0">
        <h2 className="text-2xl font-bold tracking-tight whitespace-nowrap shrink-0">提供商管理</h2>

        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setSyncConfirmOpen(true)}
            disabled={syncingAll}
          >
            <RefreshCw className={syncingAll ? "animate-spin" : ""} />
            {syncingAll ? "同步中..." : "同步上游"}
          </Button>
          <Button size="sm" onClick={onCreateProvider}>
            添加提供商
          </Button>
        </div>
      </div>

      {/* 搜索 + 筛选 独立卡片：与 HTML 基准 .card.toolbar 对齐 */}
      <div className="flex flex-wrap items-center gap-2 rounded-xl border bg-card px-2.5 py-2 shadow-sm flex-shrink-0">
        <div className="relative flex-1 min-w-[180px]">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
          <Input
            aria-label="搜索提供商名称"
            placeholder="搜索名称"
            value={nameFilter}
            onChange={(e) => setNameFilter(e.target.value)}
            className="h-8 pl-7 text-xs"
          />
        </div>

        <Select
          value={typeFilter}
          onValueChange={(value) => {
            setTypeFilter(value);
            flushNameFilter();
          }}
        >
          <SelectTrigger aria-label="按类型筛选" className="h-8 w-[140px] text-xs">
            <SelectValue placeholder="类型" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部</SelectItem>
            {availableTypes.map((type) => (
              <SelectItem key={type} value={type}>
                {type}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <AlertDialog open={syncConfirmOpen} onOpenChange={setSyncConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认一键同步</AlertDialogTitle>
            <AlertDialogDescription>
              此操作将向所有已配置的提供商发起模型列表同步，可能涉及新增或删除模型记录。确认继续？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmSync}>确认同步</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

