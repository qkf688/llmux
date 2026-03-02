import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { HEALTH_CHECK_PAGE_SIZE_OPTIONS } from "../../types";

type HealthCheckPaginationProps = {
  total: number;
  page: number;
  pages: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
};

export function HealthCheckPagination({
  total,
  page,
  pages,
  pageSize,
  onPageChange,
  onPageSizeChange,
}: HealthCheckPaginationProps) {
  const hasPages = pages > 0;
  const displayPage = hasPages ? page : 0;

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 flex-shrink-0 border-t pt-2">
      <div className="text-sm text-muted-foreground whitespace-nowrap">
        共 {total} 条，第 {displayPage} / {pages} 页
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Select value={String(pageSize)} onValueChange={(value) => onPageSizeChange(Number(value))}>
            <SelectTrigger className="h-8 text-xs">
              <SelectValue placeholder="条数" />
            </SelectTrigger>
            <SelectContent>
              {HEALTH_CHECK_PAGE_SIZE_OPTIONS.map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="icon"
            onClick={() => onPageChange(page - 1)}
            disabled={!hasPages || page <= 1}
            aria-label="上一页"
          >
            <ChevronLeft className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            onClick={() => onPageChange(page + 1)}
            disabled={!hasPages || page >= pages}
            aria-label="下一页"
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
