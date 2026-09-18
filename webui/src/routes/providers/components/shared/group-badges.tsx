import { Spinner } from "@/components/ui/spinner";
import type { GroupRef } from "@/lib/api";
import type { ModelTestResult } from "../../types";

interface GroupBadgesProps {
  groups: GroupRef[] | undefined;
  /** 按分组 ID 索引的测试状态（由父层从结果表算好传入，徽标组件不感知结果键格式） */
  groupStatuses?: Record<number, ModelTestResult>;
  /** 点击徽标＝测该「模型 × 分组」；缺省时徽标不可点 */
  onGroupClick?: (group: GroupRef) => void;
  disabled?: boolean;
}

/**
 * 分组徽标平铺（C2）：每组一个、max-width truncate、无组时不渲染。
 * 有 onGroupClick 时可点＝按分组测试；结果两态（绿✓/红✕），失败不显示错误详情。
 * 刻意不设原生 title 作为说明文案（语义由 aria-label 承载）。
 */
export function GroupBadges({ groups, groupStatuses, onGroupClick, disabled }: GroupBadgesProps) {
  if (!groups || groups.length === 0) return null;

  return (
    <div className="flex items-center gap-1 flex-wrap min-w-0">
      {groups.map((g) => {
        const status = groupStatuses?.[g.ID];
        const isLoading = status?.loading === true;
        const isCancelled = status?.cancelled === true;
        const isSuccess = status?.success === true;
        const isFail = status?.success === false;
        const clickable = !!onGroupClick && !disabled;

        const cls = isLoading
          ? "bg-muted text-muted-foreground opacity-70"
          : isCancelled
            ? "bg-muted text-muted-foreground"
            : isSuccess
              ? "bg-success-tint text-success"
              : isFail
                ? "bg-destructive-tint text-destructive"
                : "bg-muted text-muted-foreground";

        const content = (
          <>
            {isLoading ? (
              <Spinner className="h-2.5 w-2.5 mr-0.5" />
            ) : isCancelled ? (
              <span className="mr-0.5" aria-hidden>—</span>
            ) : isSuccess ? (
              <span className="mr-0.5" aria-hidden>✓</span>
            ) : isFail ? (
              <span className="mr-0.5" aria-hidden>✕</span>
            ) : null}
            <span className="truncate">{g.Name}</span>
          </>
        );

        if (!clickable) {
          return (
            <span
              key={g.ID}
              className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium max-w-[120px] ${cls}`}
            >
              {content}
            </span>
          );
        }

        return (
          <button
            key={g.ID}
            type="button"
            disabled={disabled || isLoading}
            onClick={() => onGroupClick(g)}
            aria-label={`测试分组 ${g.Name}`}
            className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium max-w-[120px] cursor-pointer hover:opacity-80 disabled:cursor-not-allowed ${cls}`}
          >
            {content}
          </button>
        );
      })}
    </div>
  );
}
