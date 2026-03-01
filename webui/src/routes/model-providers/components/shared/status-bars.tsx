import { Spinner } from "@/components/ui/spinner";

type StatusBarsProps = {
  bars: boolean[] | undefined;
  successClassName: string;
  failClassName: string;
  barClassName?: string;
  emptyTextClassName?: string;
  showTitle?: boolean;
  successTitle?: string;
  failTitle?: string;
};

export function StatusBars({
  bars,
  successClassName,
  failClassName,
  barClassName = "w-1 h-6",
  emptyTextClassName = "text-xs text-gray-400",
  showTitle = false,
  successTitle = "成功",
  failTitle = "失败",
}: StatusBarsProps) {
  if (!bars) {
    return <Spinner />;
  }

  if (bars.length === 0) {
    return <div className={emptyTextClassName}>无数据</div>;
  }

  return (
    <div className="flex space-x-1 items-end h-6">
      {bars.map((isSuccess, index) => (
        <div
          key={index}
          className={`${barClassName} ${isSuccess ? successClassName : failClassName}`}
          title={showTitle ? (isSuccess ? successTitle : failTitle) : undefined}
        />
      ))}
    </div>
  );
}

