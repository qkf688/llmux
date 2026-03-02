import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { DatabaseStats } from "@/lib/api";
import { Table as TableIcon } from "lucide-react";

type TableStatsCardProps = {
  tableStats: DatabaseStats["table_stats"];
};

export function TableStatsCard({ tableStats }: TableStatsCardProps) {
  return (
    <Card className="py-3 gap-3 sm:py-6 sm:gap-6">
      <CardHeader className="px-3 sm:px-6">
        <CardTitle className="flex items-center gap-2 text-sm sm:text-base">
          <TableIcon className="h-4 w-4 sm:h-5 sm:w-5" />
          表统计
        </CardTitle>
        <CardDescription className="hidden sm:block">数据库中各表的记录数量统计</CardDescription>
      </CardHeader>
      <CardContent className="px-3 sm:px-6">
        <div className="max-h-72 sm:max-h-none overflow-auto">
          <Table className="text-xs sm:text-sm">
          <TableHeader>
            <TableRow>
              <TableHead className="h-8">表名</TableHead>
              <TableHead className="h-8 text-right">记录数</TableHead>
              <TableHead className="hidden sm:table-cell h-8 text-right">占用空间</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tableStats.map((table) => (
              <TableRow key={table.name}>
                <TableCell className="py-1.5 px-2 font-medium max-w-[10rem] truncate sm:max-w-none">
                  {table.name}
                </TableCell>
                <TableCell className="py-1.5 px-2 text-right">{table.count.toLocaleString()}</TableCell>
                <TableCell className="hidden sm:table-cell py-1.5 px-2 text-right">
                  {table.estimated_size_human}
                </TableCell>
              </TableRow>
            ))}
            {tableStats.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
      </CardContent>
    </Card>
  );
}
