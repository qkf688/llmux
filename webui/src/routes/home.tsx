"use client"

import type { ComponentType, ReactNode } from "react";
import { Suspense, lazy } from "react";
import { Activity, BarChart3, Bot, CalendarDays, Database, HardDrive, MessageSquare } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import Loading from "@/components/loading";
import { useHomeMetrics } from "@/hooks/api/use-home";
import { formatCompactCount } from "@/lib/formatters";

// 懒加载图表组件
const ModelRankingList = lazy(() => import("@/components/charts/model-ranking").then(module => ({ default: module.ModelRankingList })));
const ProviderRankCard = lazy(() => import("@/components/charts/provider-ranking").then(module => ({ default: module.ProviderRankCard })));
const ActivityHeatmapCard = lazy(() => import("@/components/charts/activity-heatmap").then(module => ({ default: module.ActivityHeatmapCard })));
const TrendCard = lazy(() => import("@/components/charts/trend-card").then(module => ({ default: module.TrendCard })));

// home 数字展示样式约定：大数字 + 单位（复用共享数字滚动组件）
const AnimatedMetric = ({
  value,
  formatter,
}: {
  value: number;
  formatter?: (value: number) => { value: string; unit?: string };
}) => (
  <AnimatedNumber
    value={value}
    formatter={formatter}
    className="text-xl sm:text-2xl font-semibold leading-none"
  />
);

type IconComponent = ComponentType<{ className?: string }>;

function TopMetricCard({
  title,
  headerIcon: HeaderIcon,
  children,
}: {
  title: string;
  headerIcon: IconComponent;
  children: ReactNode;
}) {
  return (
    // StaggerItem 是入场动画载体，motion 会在其上残留内联 transform，覆盖 hover-lift 的 CSS hover 变换，
    // 因此卡片样式与 hover 微交互放在内层非 motion 的 div 上。错峰节奏由外层 StaggerList 编排。
    <StaggerItem>
      <div className="rounded-3xl bg-card border p-5 text-card-foreground flex flex-row items-center gap-4 shadow-3xl hover-lift hover-border">
        <div className="flex flex-col items-center justify-center gap-3 border-r border-border/50 pr-4 py-1 self-stretch">
          <HeaderIcon className="w-4 h-4" />
          <h3 className="font-medium text-sm [writing-mode:vertical-lr]">{title}</h3>
        </div>
        <div className="flex flex-col gap-4 flex-1 min-w-0">{children}</div>
      </div>
    </StaggerItem>
  );
}

function MetricItem({
  icon: ItemIcon,
  label,
  toneClassName,
  children,
}: {
  icon: IconComponent;
  label: string;
  toneClassName: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center gap-3">
      <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${toneClassName}`}>
        <ItemIcon className="w-5 h-5" />
      </div>
      <div className="flex flex-col min-w-0">
        <span className="text-xs text-muted-foreground">{label}</span>
        {children}
      </div>
    </div>
  );
}

export default function Home() {
  const results = useHomeMetrics();

  const todayMetrics = results[0].data ?? { reqs: 0, tokens: 0 };
  const totalMetrics = results[1].data ?? { reqs: 0, tokens: 0 };
  const allMetrics = results[2].data ?? { reqs: 0, tokens: 0 };
  const dailyMetrics = results[3].data ?? [];
  const hourlyMetrics = results[4].data ?? [];
  const realModelCounts = results[5].data ?? [];
  const requestedModelCounts = results[6].data ?? [];
  const dbStats = results[7].data ?? null;
  const providerMetrics = results[8].data ?? [];

  // KPI 查询（索引 0,1,2,7）loading 时才阻塞整页；图表查询（索引 3,4,5,6,8）渐进填充
  const kpiLoading = results[0].isPending || results[1].isPending || results[2].isPending || results[7].isPending;

  const dbUsagePercent = dbStats && dbStats.page_count > 0
    ? ((dbStats.page_count - dbStats.free_pages) / dbStats.page_count) * 100
    : 0;

  if (kpiLoading) return <Loading message="加载系统概览" />;

  const compactCountFormatter = (value: number) => {
    const formatted = formatCompactCount(value);
    return { value: formatted.value, unit: formatted.unit };
  };

  return (
    <div className="space-y-4 sm:space-y-6 overflow-auto h-full">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-lg sm:text-2xl font-bold">系统概览</h1>
          <p className="text-xs sm:text-sm text-muted-foreground">
            快速查看今日/本月调用、模型统计与数据库状态
          </p>
        </div>
      </div>

      <StaggerList className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <TopMetricCard title="今日" headerIcon={Activity}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-primary/10 text-primary">
            <AnimatedMetric value={todayMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-1)]/10 text-[color:var(--chart-1)]">
            <AnimatedMetric value={todayMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard title="本月" headerIcon={CalendarDays}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-[color:var(--chart-6)]/10 text-[color:var(--chart-6)]">
            <AnimatedMetric value={totalMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-2)]/10 text-[color:var(--chart-2)]">
            <AnimatedMetric value={totalMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard title="全部" headerIcon={BarChart3}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-[color:var(--chart-9)]/10 text-[color:var(--chart-9)]">
            <AnimatedMetric value={allMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-10)]/10 text-[color:var(--chart-10)]">
            <AnimatedMetric value={allMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard title="数据库" headerIcon={Database}>
          <MetricItem icon={HardDrive} label="大小" toneClassName="bg-[color:var(--chart-3)]/10 text-[color:var(--chart-3)]">
            <div className="text-xl sm:text-2xl font-semibold leading-none tabular-nums">
              {dbStats?.file_size_human ?? "-"}
            </div>
          </MetricItem>
          <MetricItem icon={HardDrive} label="使用率" toneClassName="bg-[color:var(--chart-4)]/10 text-[color:var(--chart-4)]">
            <AnimatedMetric
              value={dbUsagePercent}
              formatter={(v) => ({ value: v.toFixed(1), unit: "%" })}
            />
          </MetricItem>
        </TopMetricCard>
      </StaggerList>

      <Suspense fallback={<div className="py-10 flex items-center justify-center">
        <Loading message="加载一年活跃度..." />
      </div>}>
        <ActivityHeatmapCard data={dailyMetrics} />
      </Suspense>

      <Suspense fallback={<div className="py-10 flex items-center justify-center">
        <Loading message="加载趋势..." />
      </div>}>
        <TrendCard dailyData={dailyMetrics} hourlyData={hourlyMetrics} />
      </Suspense>

      <Card className="py-4 gap-3 sm:py-6 sm:gap-6">
        <CardHeader className="px-3 sm:px-6">
          <CardTitle className="text-sm font-medium sm:text-base">模型数据分析</CardTitle>
          <CardDescription className="hidden sm:block">真实命中模型与用户请求模型对比</CardDescription>
        </CardHeader>
        <CardContent className="px-3 sm:px-6">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
            <div className="rounded-2xl border bg-card/50 p-3 sm:p-4">
              <div className="text-xs text-muted-foreground mb-2">真实命中模型排行</div>
              <Suspense fallback={<Loading message="加载真实模型排行..." />}>
                <ModelRankingList data={realModelCounts} />
              </Suspense>
            </div>
            <div className="rounded-2xl border bg-card/50 p-3 sm:p-4">
              <div className="text-xs text-muted-foreground mb-2">用户请求模型排行</div>
              <Suspense fallback={<Loading message="加载请求模型排行..." />}>
                <ModelRankingList data={requestedModelCounts} />
              </Suspense>
            </div>
          </div>
        </CardContent>
      </Card>

      <Suspense fallback={<div className="py-10 flex items-center justify-center">
        <Loading message="加载供应商排行..." />
      </div>}>
        <ProviderRankCard data={providerMetrics} />
      </Suspense>
    </div>
  );
}
 
