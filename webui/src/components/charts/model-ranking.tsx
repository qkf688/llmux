"use client";

import { useMemo, useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RankEmptyState } from "@/components/charts/rank-empty-state";
import { rankToneClass } from "@/components/charts/rank-tone";
import type { ModelCount } from "@/lib/api";
import { formatCompactCount } from "@/lib/formatters";

type RankSortMode = "count" | "tokens";

/** 一次排行的主指标取值：次数模式下取 calls，token 模式下取 total_tokens。 */
function metricValue(item: ModelCount, mode: RankSortMode) {
  return mode === "tokens" ? item.total_tokens ?? 0 : item.calls ?? 0;
}

/** 一格数字的显示形态。known=false 表示「没有数据」而非「数值为 0」——
 *  两种情况在库里无法区分（见 tokenDisplay），所以把状态显式带出来给渲染层判，
 *  别让调用方靠比较 value 是不是破折号来猜。 */
type MetricCell = { value: string; unit: string; known: boolean };

function numberCell(value: number): MetricCell {
  return { ...formatCompactCount(value), known: true };
}

/** token 的显示形态。0 不显示成数字：库里分不清「没消耗 token」与「早于 token
 *  统计起点、那批调用的日志已被保留策略清空」——后者是实打实发生过上百上千次
 *  调用的模型，显示成 0 会让人以为它白跑了那么多。 */
function tokenDisplay(totalTokens: number): MetricCell {
  if (totalTokens <= 0) return { value: "—", unit: "", known: false };
  return numberCell(totalTokens);
}

/** 行底量级条：宽度 = 本行 / 当前模式榜首。排行榜的量级常差一个数量级
 *  （1.6B 对 3.5K），光看数字要在心里做除法；条形让差距直接落在视网膜上。
 * 末位太细时给一个可见下限，否则 0.1% 的行与 0 行长得一模一样。 */
const MIN_BAR_PERCENT = 1.5;

export function ModelRankingList({
  data,
}: {
  data: ModelCount[];
}) {
  const [mode, setMode] = useState<RankSortMode>("count");

  const ranked = useMemo(() => {
    const rows = [...(data ?? [])];
    rows.sort((a, b) => metricValue(b, mode) - metricValue(a, mode));
    return rows;
  }, [data, mode]);

  const maxMetric = useMemo(
    () => ranked.reduce((max, item) => Math.max(max, metricValue(item, mode)), 0),
    [ranked, mode],
  );

  const renderList = () => {
    if (ranked.length === 0) {
      return <RankEmptyState />;
    }

    return (
      <div className="space-y-2 max-h-[300px] overflow-y-auto">
        {ranked.map((item, index) => {
          const rank = index + 1;
          const value = metricValue(item, mode);
          const main = mode === "tokens" ? tokenDisplay(item.total_tokens) : numberCell(item.calls);
          const secondary = mode === "tokens" ? numberCell(item.calls) : tokenDisplay(item.total_tokens);
          const barPercent =
            maxMetric > 0 && value > 0
              ? Math.max((value / maxMetric) * 100, MIN_BAR_PERCENT)
              : 0;

          return (
            <div
              key={item.model}
              className="group relative rounded-2xl transition-colors hover:bg-accent/5"
            >
              <div className="flex items-center gap-3 p-3">
                <div
                  className={`w-8 h-8 rounded-lg flex items-center justify-center font-bold text-sm shrink-0 ${rankToneClass(rank)}`}
                >
                  {rank}
                </div>

                <div className="flex-1 min-w-0">
                  <p className="font-medium text-sm truncate" title={item.model}>
                    {item.model}
                  </p>
                  {/* 副指标分三档：标签最弱、数值次强、当前模式的主指标最强。
                      两个模式的副指标用同一套排版，切 Tab 时视觉重量不跳变。 */}
                  <p
                    className="text-xs text-muted-foreground mt-0.5 truncate"
                    title={mode === "tokens" ? undefined : tokenTitle(item)}
                  >
                    {mode === "tokens" ? "调用" : "token"}
                    <span
                      className={`ml-1 tabular-nums ${
                        secondary.known ? "text-sm font-medium text-foreground" : "text-muted-foreground"
                      }`}
                    >
                      {secondary.value}
                    </span>
                    {secondary.unit && <span className="text-muted-foreground">{secondary.unit}</span>}
                  </p>
                </div>

                <div
                  className="text-right shrink-0 font-semibold text-base tabular-nums text-[color:var(--chart-1-strong)]"
                  title={mode === "tokens" ? tokenTitle(item) : undefined}
                >
                  {main.value}
                  <span className="text-xs text-muted-foreground">{main.unit}</span>
                </div>
              </div>

              <div
                className="h-0.5 rounded-full bg-[color:var(--chart-1)]/70 mx-3 mb-2 transition-[width] duration-300"
                style={{ width: `calc(${barPercent}% - 1.5rem)` }}
              />
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <Tabs value={mode} onValueChange={(value) => setMode(value as RankSortMode)}>
      <div className="flex items-center justify-between gap-3 mb-3">
        <span className="text-xs text-muted-foreground">真实命中模型排行</span>
        <TabsList className="h-8">
          <TabsTrigger value="count" className="text-xs sm:text-sm">
            次数
          </TabsTrigger>
          <TabsTrigger value="tokens" className="text-xs sm:text-sm">
            Token
          </TabsTrigger>
        </TabsList>
      </div>
      <TabsContent value="count">{renderList()}</TabsContent>
      <TabsContent value="tokens">{renderList()}</TabsContent>
    </Tabs>
  );
}

/** token 的悬停说明：有值时给输入/输出拆分，无值时说清是「未统计」而不是「零消耗」。 */
function tokenTitle(item: ModelCount) {
  if (item.total_tokens <= 0) {
    return `${item.calls} 次调用发生于 token 统计启用之前，日志已随保留策略清理，token 无法回溯`;
  }
  const prompt = formatCompactCount(item.prompt_tokens);
  const completion = formatCompactCount(item.completion_tokens);
  return `输入 ${prompt.value}${prompt.unit} · 输出 ${completion.value}${completion.unit}`;
}
