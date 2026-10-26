import { TrendingUp } from "lucide-react";

/**
 * 排行类图表的空状态。
 *
 * 模型排行与供应商排行共用同一形态（图标 + 文案 + 居中留白）。两处各写一份时，
 * 改文案或调间距必然只落一处，于是两个排行页出现肉眼可见的差异——而这种差异
 * 没人会去比对，只能各看各的都觉得没问题。故抽成单源。
 */
export function RankEmptyState({ message = "暂无数据" }: { message?: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
      <TrendingUp className="w-12 h-12 mb-3 opacity-30" />
      <p className="text-sm">{message}</p>
      <p className="mt-1 text-xs text-muted-foreground/70">发起调用后这里会出现排行</p>
    </div>
  );
}
