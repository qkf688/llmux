import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

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
  return (
    <>
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">提供商管理</h2>
          </div>
          <div className="flex w-full sm:w-auto items-center justify-end gap-2">
            <Button variant="secondary" size="sm" onClick={onSyncAllProviders} disabled={syncingAll} className="h-9">
              {syncingAll ? "同步中..." : "一键同步上游模型"}
            </Button>
          </div>
        </div>
      </div>

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

