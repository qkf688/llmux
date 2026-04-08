"use client";

import { useMemo, useState } from "react";
import { TrendingUp } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { ProviderMetric } from "@/lib/api";
import { formatCompactCount, formatPercent, formatTimeNs } from "@/lib/formatters";

type RankSortMode = "count" | "tokens";

function rankToneClass(rank: number) {
  if (rank === 1) return "bg-[color:var(--chart-7)]/15 text-[color:var(--chart-7)]";
  if (rank === 2) return "bg-[color:var(--chart-6)]/15 text-[color:var(--chart-6)]";
  if (rank === 3) return "bg-[color:var(--chart-8)]/15 text-[color:var(--chart-8)]";
  return "bg-muted/60 text-muted-foreground";
}

export function ProviderRankCard({
  data,
  defaultMode = "count",
  topN = 10,
}: {
  data: ProviderMetric[];
  defaultMode?: RankSortMode;
  topN?: number;
}) {
  const [mode, setMode] = useState<RankSortMode>(defaultMode);

  const ranked = useMemo(() => {
    const rows = [...(data ?? [])];
    if (mode === "tokens") {
      rows.sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0));
    } else {
      rows.sort((a, b) => (b.total_requests ?? 0) - (a.total_requests ?? 0));
    }
    return rows.slice(0, topN);
  }, [data, mode, topN]);

  const renderList = () => {
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
          const success = formatCompactCount(item.success_count);
          const failed = formatCompactCount(item.failure_count);
          const totalTokens = formatCompactCount(item.total_tokens);

          return (
            <div
              key={`${item.provider_id}-${item.provider_name}`}
              className="flex items-center gap-3 p-3 rounded-2xl hover:bg-accent/5 transition-colors"
            >
              <div
                className={`w-8 h-8 rounded-lg flex items-center justify-center font-bold text-sm shrink-0 ${rankToneClass(rank)}`}
              >
                {rank}
              </div>

              <div className="flex-1 min-w-0">
                <p className="font-medium text-sm truncate">{item.provider_name}</p>
                <div className="flex items-center gap-2 text-xs text-muted-foreground mt-1">
                  <span>成功率:</span>
                  <span>{formatPercent(item.success_rate, 1)}</span>
                  <span className="text-muted-foreground/40">·</span>
                  <span>首包:</span>
                  <span>{formatTimeNs(item.avg_response_time)}</span>
                </div>
              </div>

              <div className="flex items-center gap-1 text-right shrink-0">
                {mode === "count" ? (
                  <div className="flex items-center gap-1 text-sm font-medium tabular-nums">
                    <span className="text-[color:var(--chart-1)]">
                      {success.value}
                      <span className="text-xs text-muted-foreground">{success.unit}</span>
                    </span>
                    <span className="text-muted-foreground/40 font-light">/</span>
                    <span className="text-destructive">
                      {failed.value}
                      <span className="text-xs text-muted-foreground">{failed.unit}</span>
                    </span>
                  </div>
                ) : (
                  <span className="font-semibold text-base tabular-nums">
                    {totalTokens.value}
                    <span className="text-xs text-muted-foreground">{totalTokens.unit}</span>
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <div className="rounded-3xl bg-card text-card-foreground border p-4">
      <Tabs value={mode} onValueChange={(value) => setMode(value as RankSortMode)}>
        <div className="flex items-center justify-between gap-3">
          <h3 className="font-semibold text-base">供应商排行</h3>
          <TabsList className="h-9">
            <TabsTrigger value="count" className="text-xs sm:text-sm">
              次数
            </TabsTrigger>
            <TabsTrigger value="tokens" className="text-xs sm:text-sm">
              Tokens
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="count">{renderList()}</TabsContent>
        <TabsContent value="tokens">{renderList()}</TabsContent>
      </Tabs>
    </div>
  );
}

