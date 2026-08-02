"use client";

import { useId, useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { addDays, formatDateYYYYMMDD, startOfDay } from "@/lib/date";
import { formatCompactCount } from "@/lib/formatters";
import type { DailyMetricsData, HourlyMetricsData } from "@/lib/api";

type ChartMetricType = "count" | "tokens";
type ChartPeriod = "today" | "7" | "30" | "365";

function formatMMDD(dateStr: string): string {
  const parts = dateStr.split("-");
  if (parts.length !== 3) return dateStr;
  return `${parts[1]}/${parts[2]}`;
}

function periodLabel(period: ChartPeriod): string {
  if (period === "today") return "今日";
  return `${period}天`;
}

function nextPeriod(period: ChartPeriod): ChartPeriod {
  const periods: ChartPeriod[] = ["today", "7", "30", "365"];
  const currentIndex = periods.indexOf(period);
  const nextIndex = (currentIndex + 1) % periods.length;
  return periods[nextIndex];
}

export function TrendCard({
  dailyData,
  hourlyData,
}: {
  dailyData: DailyMetricsData[];
  hourlyData: HourlyMetricsData[];
}) {
  const [metricType, setMetricType] = useState<ChartMetricType>("count");
  const [period, setPeriod] = useState<ChartPeriod>("today");
  const gradientId = useId().replace(/:/g, "");

  const chartConfig = useMemo(() => {
    const color = metricType === "count" ? "var(--chart-2)" : "var(--chart-3)";
    return {
      metric: { label: metricType === "count" ? "次数" : "Tokens", color },
    };
  }, [metricType]);

  const dailyMap = useMemo(() => {
    const map = new Map<string, DailyMetricsData>();
    for (const item of dailyData ?? []) {
      map.set(item.date, item);
    }
    return map;
  }, [dailyData]);

  const filledDaily = useMemo(() => {
    if (period === "today") return [];
    const days = Number(period);
    const today = startOfDay(new Date());
    const start = addDays(today, -(days - 1));
    const result: DailyMetricsData[] = [];
    for (let i = 0; i < days; i++) {
      const dateStr = formatDateYYYYMMDD(addDays(start, i));
      const stat = dailyMap.get(dateStr);
      result.push({
        date: dateStr,
        reqs: stat?.reqs ?? 0,
        tokens: stat?.tokens ?? 0,
      });
    }
    return result;
  }, [dailyMap, period]);

  const chartData = useMemo(() => {
    if (period === "today") {
      return (hourlyData ?? []).map((stat) => ({
        label: `${String(stat.hour).padStart(2, "0")}:00`,
        metric: metricType === "count" ? stat.reqs : stat.tokens,
      }));
    }

    return filledDaily.map((stat) => ({
      label: formatMMDD(stat.date),
      metric: metricType === "count" ? stat.reqs : stat.tokens,
    }));
  }, [filledDaily, hourlyData, metricType, period]);

  const totals = useMemo(() => {
    if (period === "today") {
      const reqs = (hourlyData ?? []).reduce((acc, item) => acc + (item.reqs ?? 0), 0);
      const tokens = (hourlyData ?? []).reduce((acc, item) => acc + (item.tokens ?? 0), 0);
      return { reqs, tokens };
    }
    const reqs = filledDaily.reduce((acc, item) => acc + (item.reqs ?? 0), 0);
    const tokens = filledDaily.reduce((acc, item) => acc + (item.tokens ?? 0), 0);
    return { reqs, tokens };
  }, [filledDaily, hourlyData, period]);

  const handlePeriodClick = () => {
    setPeriod((prev) => nextPeriod(prev));
  };

  const tickFormatter = (value: number) => {
    const formatted = formatCompactCount(value);
    return `${formatted.value}${formatted.unit}`;
  };

  return (
    <div className="rounded-3xl bg-card border pt-4 pb-0 text-card-foreground shadow-3xl hover-border">
      <div className="px-4 pb-2 space-y-2">
        <div className="flex justify-between items-center">
          <h3 className="font-semibold text-base">趋势</h3>
          <Tabs value={metricType} onValueChange={(value) => setMetricType(value as ChartMetricType)}>
            <TabsList className="h-9">
              <TabsTrigger value="count" className="text-xs sm:text-sm">
                次数
              </TabsTrigger>
              <TabsTrigger value="tokens" className="text-xs sm:text-sm">
                Tokens
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>

        <div className="flex justify-between items-start">
          <div className="flex gap-2 text-sm">
            <div>
              <div className="text-xs text-muted-foreground">总请求</div>
              <div className="text-xl font-semibold">
                <AnimatedNumber value={totals.reqs} />
              </div>
            </div>
            <div className="w-px bg-border self-stretch"></div>
            <div>
              <div className="text-xs text-muted-foreground">总Tokens</div>
              <div className="text-xl font-semibold">
                <AnimatedNumber value={totals.tokens} />
              </div>
            </div>
          </div>

          <div
            className="flex gap-2 text-sm cursor-pointer hover:opacity-80 transition-opacity select-none"
            onClick={handlePeriodClick}
          >
            <div>
              <div className="text-xs text-muted-foreground">时间段</div>
              <div className="text-base font-semibold">{periodLabel(period)}</div>
            </div>
          </div>
        </div>
      </div>

      <ChartContainer config={chartConfig} className="h-40 w-full aspect-auto">
        <AreaChart accessibilityLayer data={chartData}>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--color-metric)" stopOpacity={1.0} />
              <stop offset="95%" stopColor="var(--color-metric)" stopOpacity={0.1} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="label" tickLine={false} axisLine={false} minTickGap={16} />
          <YAxis tickLine={false} axisLine={false} tickFormatter={tickFormatter} />
          <ChartTooltip cursor={false} content={<ChartTooltipContent indicator="line" />} />
          <Area type="monotone" dataKey="metric" stroke="var(--color-metric)" fill={`url(#${gradientId})`} />
        </AreaChart>
      </ChartContainer>
    </div>
  );
}
