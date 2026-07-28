"use client"

import type { ComponentType, ReactNode } from "react";
import { useState, useEffect, Suspense, lazy } from "react";
import { motion } from "motion/react";
import { Activity, BarChart3, Bot, CalendarDays, Database, HardDrive, MessageSquare } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import Loading from "@/components/loading";
import { useHomeMetrics } from "@/hooks/api/use-home";
import { formatCompactCount } from "@/lib/formatters";

// 懒加载图表组件
const ModelRankingList = lazy(() => import("@/components/charts/model-ranking").then(module => ({ default: module.ModelRankingList })));
const ProviderRankCard = lazy(() => import("@/components/charts/provider-ranking").then(module => ({ default: module.ProviderRankCard })));
const ActivityHeatmapCard = lazy(() => import("@/components/charts/activity-heatmap").then(module => ({ default: module.ActivityHeatmapCard })));
const TrendCard = lazy(() => import("@/components/charts/trend-card").then(module => ({ default: module.TrendCard })));

type AnimatedMetricValueProps = {
  value: number;
  duration?: number;
  formatter?: (value: number) => { value: string; unit?: string };
};

const AnimatedMetricValue = ({ value, duration = 900, formatter }: AnimatedMetricValueProps) => {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let startTime: number | null = null;
    const animateCount = (timestamp: number) => {
      if (!startTime) startTime = timestamp;
      const progress = timestamp - startTime;
      const progressRatio = Math.min(progress / duration, 1);
      const currentValue = progressRatio * value;
      
      setCount(currentValue);
      
      if (progress < duration) {
        requestAnimationFrame(animateCount);
      }
    };
    
    requestAnimationFrame(animateCount);
  }, [value, duration]);

  const formatted = formatter ? formatter(count) : { value: Math.round(count).toLocaleString(), unit: "" };

  return (
    <div className="flex items-baseline gap-1 tabular-nums">
      <div className="text-xl sm:text-2xl font-semibold leading-none">{formatted.value}</div>
      {formatted.unit ? <div className="text-xs text-muted-foreground leading-none">{formatted.unit}</div> : null}
    </div>
  );
};

type IconComponent = ComponentType<{ className?: string }>;

function TopMetricCard({
  index,
  title,
  headerIcon: HeaderIcon,
  children,
}: {
  index: number;
  title: string;
  headerIcon: IconComponent;
  children: ReactNode;
}) {
  return (
    <motion.section
      className="rounded-3xl bg-card border p-5 text-card-foreground flex flex-row items-center gap-4 custom-shadow"
      initial={{ opacity: 0, y: 20, filter: "blur(8px)" }}
      animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
      transition={{
        duration: 0.5,
        ease: [0.2, 0, 0, 1] as const,
        delay: index * 0.08,
      }}
    >
      <div className="flex flex-col items-center justify-center gap-3 border-r border-border/50 pr-4 py-1 self-stretch">
        <HeaderIcon className="w-4 h-4" />
        <h3 className="font-medium text-sm [writing-mode:vertical-lr]">{title}</h3>
      </div>
      <div className="flex flex-col gap-4 flex-1 min-w-0">{children}</div>
    </motion.section>
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

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <TopMetricCard index={0} title="今日" headerIcon={Activity}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-primary/10 text-primary">
            <AnimatedMetricValue value={todayMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-1)]/10 text-[color:var(--chart-1)]">
            <AnimatedMetricValue value={todayMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard index={1} title="本月" headerIcon={CalendarDays}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-[color:var(--chart-6)]/10 text-[color:var(--chart-6)]">
            <AnimatedMetricValue value={totalMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-2)]/10 text-[color:var(--chart-2)]">
            <AnimatedMetricValue value={totalMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard index={2} title="全部" headerIcon={BarChart3}>
          <MetricItem icon={MessageSquare} label="请求" toneClassName="bg-[color:var(--chart-9)]/10 text-[color:var(--chart-9)]">
            <AnimatedMetricValue value={allMetrics.reqs} formatter={compactCountFormatter} />
          </MetricItem>
          <MetricItem icon={Bot} label="Tokens" toneClassName="bg-[color:var(--chart-10)]/10 text-[color:var(--chart-10)]">
            <AnimatedMetricValue value={allMetrics.tokens} formatter={compactCountFormatter} />
          </MetricItem>
        </TopMetricCard>

        <TopMetricCard index={3} title="数据库" headerIcon={Database}>
          <MetricItem icon={HardDrive} label="大小" toneClassName="bg-[color:var(--chart-3)]/10 text-[color:var(--chart-3)]">
            <div className="text-xl sm:text-2xl font-semibold leading-none tabular-nums">
              {dbStats?.file_size_human ?? "-"}
            </div>
          </MetricItem>
          <MetricItem icon={HardDrive} label="使用率" toneClassName="bg-[color:var(--chart-4)]/10 text-[color:var(--chart-4)]">
            <AnimatedMetricValue
              value={dbUsagePercent}
              formatter={(v) => ({ value: v.toFixed(1), unit: "%" })}
            />
          </MetricItem>
        </TopMetricCard>
      </div>

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
 
