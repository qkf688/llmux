import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { DatabaseStats } from "@/lib/api";
import { Table as TableIcon } from "lucide-react";

type TableStatsCardProps = {
  tableStats: DatabaseStats["table_stats"];
};

export function TableStatsCard({ tableStats }: TableStatsCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TableIcon className="h-5 w-5" />
          表统计
        </CardTitle>
        <CardDescription>数据库中各表的记录数量统计</CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>表名</TableHead>
              <TableHead className="text-right">记录数</TableHead>
              <TableHead className="text-right">占用空间</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tableStats.map((table) => (
              <TableRow key={table.name}>
                <TableCell className="font-medium">{table.name}</TableCell>
                <TableCell className="text-right">{table.count.toLocaleString()}</TableCell>
                <TableCell className="text-right">{table.estimated_size_human}</TableCell>
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
      </CardContent>
    </Card>
  );
}
