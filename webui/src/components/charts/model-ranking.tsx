"use client";

import { useMemo } from "react";
import { TrendingUp } from "lucide-react";
import type { ModelCount } from "@/lib/api";
import { formatCompactCount } from "@/lib/formatters";

function rankToneClass(rank: number) {
  if (rank === 1) return "bg-[color:var(--chart-7)]/15 text-[color:var(--chart-7)]";
  if (rank === 2) return "bg-[color:var(--chart-6)]/15 text-[color:var(--chart-6)]";
  if (rank === 3) return "bg-[color:var(--chart-8)]/15 text-[color:var(--chart-8)]";
  return "bg-muted/60 text-muted-foreground";
}

export function ModelRankingList({
  data,
  topN = 10,
}: {
  data: ModelCount[];
  topN?: number;
}) {
  const ranked = useMemo(() => {
    const rows = [...(data ?? [])];
    rows.sort((a, b) => (b.calls ?? 0) - (a.calls ?? 0));
    return rows.slice(0, topN);
  }, [data, topN]);

  const totalCalls = useMemo(() => {
    return ranked.reduce((acc, item) => acc + (item.calls ?? 0), 0);
  }, [ranked]);

  if (ranked.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <TrendingUp className="w-12 h-12 mb-3 opacity-30" />
        <p className="text-sm">暂无数据</p>
      </div>
    );
  }

  return (
    <div className="space-y-3 max-h-[300px] overflow-y-auto">
      {ranked.map((item, index) => {
        const rank = index + 1;
        const calls = formatCompactCount(item.calls);
        const share = totalCalls > 0 ? ((item.calls ?? 0) / totalCalls) * 100 : 0;

        return (
          <div
            key={item.model}
            className="flex items-center gap-3 p-3 rounded-2xl hover:bg-accent/5 transition-colors"
          >
            <div
              className={`w-8 h-8 rounded-lg flex items-center justify-center font-bold text-sm shrink-0 ${rankToneClass(rank)}`}
            >
              {rank}
            </div>

            <div className="flex-1 min-w-0">
              <p className="font-medium text-sm truncate" title={item.model}>
                {item.model}
              </p>
              <div className="flex items-center gap-2 text-xs text-muted-foreground mt-1">
                <span>占比:</span>
                <span className="tabular-nums">{share.toFixed(1)}%</span>
              </div>
            </div>

            <div className="text-right shrink-0 font-semibold text-base tabular-nums">
              {calls.value}
              <span className="text-xs text-muted-foreground">{calls.unit}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

