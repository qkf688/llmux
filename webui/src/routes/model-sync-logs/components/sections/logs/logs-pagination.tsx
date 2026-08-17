import { Button } from "@/components/ui/button";

type LogsPaginationProps = {
  page: number;
  totalPages: number;
  paginationText: string;
  onPageChange: (page: number) => void;
};

export function LogsPagination({ page, totalPages, paginationText, onPageChange }: LogsPaginationProps) {
  return (
    <div className="flex items-center justify-center gap-2 flex-shrink-0">
      <Button
        variant="outline"
        size="sm"
        className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
        onClick={() => onPageChange(page - 1)}
        disabled={page === 1}
      >
        上一页
      </Button>
      <span className="sm:text-sm text-xs">{paginationText}</span>
      <Button
        variant="outline"
        size="sm"
        className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
        onClick={() => onPageChange(page + 1)}
        disabled={page === totalPages}
      >
        下一页
      </Button>
    </div>
  );
}
