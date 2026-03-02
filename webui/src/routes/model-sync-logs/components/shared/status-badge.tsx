import { AlertTriangle, CheckCircle, MinusCircle, XCircle } from "lucide-react";

type StatusBadgeProps = {
  status: string;
  textClassName?: string;
  iconClassName?: string;
};

export function StatusBadge({ status, textClassName = "text-sm", iconClassName = "h-4 w-4" }: StatusBadgeProps) {
  if (status === "success") {
    return (
      <div className="flex items-center gap-1 text-green-600">
        <CheckCircle className={iconClassName} />
        <span className={`font-medium ${textClassName}`}>成功</span>
      </div>
    );
  }

  if (status === "unchanged") {
    return (
      <div className="flex items-center gap-1 text-blue-600">
        <MinusCircle className={iconClassName} />
        <span className={`font-medium ${textClassName}`}>无变化</span>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className="flex items-center gap-1 text-red-600">
        <XCircle className={iconClassName} />
        <span className={`font-medium ${textClassName}`}>错误</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1 text-muted-foreground">
      <AlertTriangle className={iconClassName} />
      <span className={textClassName}>未知</span>
    </div>
  );
}
