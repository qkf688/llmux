import { useState } from "react";
import { FaSync } from "react-icons/fa";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">提供商管理</h2>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSyncConfirmOpen(true)}
              disabled={syncingAll}
              className="h-9"
            >
              <FaSync className={syncingAll ? "animate-spin" : ""} />
              {syncingAll ? "同步中..." : "一键同步上游模型"}
            </Button>
          </div>
        </div>
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

      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:gap-4">
          <div className="flex flex-col gap-1 text-xs col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商名称</Label>
            <Input
              placeholder="输入名称"
              value={nameFilter}
              onChange={(e) => setNameFilter(e.target.value)}
              className="h-8 w-full text-xs px-2"
            />
          </div>

          <div className="flex flex-col gap-1 text-xs col-span-1">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">类型</Label>
            <Select
              value={typeFilter}
              onValueChange={(value) => {
                setTypeFilter(value);
                flushNameFilter();
              }}
            >
              <SelectTrigger className="h-8 w-full text-xs px-2">
                <SelectValue placeholder="选择类型" />
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

          <div className="flex items-end col-span-2 sm:col-span-1 sm:justify-end">
            <Button onClick={onCreateProvider} className="h-8 w-full text-xs sm:w-auto sm:ml-auto">
              添加提供商
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}

