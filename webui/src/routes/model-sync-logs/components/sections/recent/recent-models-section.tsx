import Loading from "@/components/loading";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { AddedModel } from "@/lib/api";
import { formatSyncDate } from "../../../utils/formatters";

type RecentModelsSectionProps = {
  loading: boolean;
  syncTime: string;
  models: AddedModel[];
};

export function RecentModelsSection({ loading, syncTime, models }: RecentModelsSectionProps) {
  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
      {loading ? (
        <div className="flex h-full items-center justify-center">
          <Loading message="加载最近新增模型" />
        </div>
      ) : models.length === 0 ? (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
          暂无最近新增模型
        </div>
      ) : (
        <>
          <div className="p-3 border-b bg-muted/30">
            <p className="text-sm text-muted-foreground">
              最近同步时间: {formatSyncDate(syncTime)} · 共新增 {models.length} 个模型
            </p>
          </div>
          <div className="h-full overflow-auto">
            <div className="hidden sm:block w-full">
              <Table>
                <TableHeader className="sticky top-0 bg-secondary/80">
                  <TableRow>
                    <TableHead>模型名称</TableHead>
                    <TableHead>提供商</TableHead>
                    <TableHead>新增时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {models.map((model, index) => (
                    <TableRow key={`${model.provider_name}-${model.model_name}-${index}`}>
                      <TableCell className="font-mono text-sm">{model.model_name}</TableCell>
                      <TableCell>{model.provider_name}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {formatSyncDate(model.added_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <div className="sm:hidden px-2 py-3 divide-y divide-border">
              {models.map((model, index) => (
                <div key={`${model.provider_name}-${model.model_name}-${index}`} className="py-3 space-y-1">
                  <div className="font-mono text-sm">{model.model_name}</div>
                  <div className="text-xs text-muted-foreground">
                    {model.provider_name} · {formatSyncDate(model.added_at)}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
