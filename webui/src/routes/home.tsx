"use client"

import { useState, useEffect, Suspense, lazy } from "react";
import { useNavigate } from "react-router-dom";
import { Database } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Loading from "@/components/loading";
import {
  getMetrics,
  getModelCounts,
  getDatabaseStats
} from "@/lib/api";
import type { MetricsData, ModelCount, DatabaseStats } from "@/lib/api";
import { toast } from "sonner";

// 懒加载图表组件
const ChartPieDonutText = lazy(() => import("@/components/charts/pie-chart").then(module => ({ default: module.ChartPieDonutText })));
const ModelRankingChart = lazy(() => import("@/components/charts/bar-chart").then(module => ({ default: module.ModelRankingChart })));

// Animated counter component
const AnimatedCounter = ({ value, duration = 1000 }: { value: number; duration?: number }) => {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let startTime: number | null = null;
    const animateCount = (timestamp: number) => {
      if (!startTime) startTime = timestamp;
      const progress = timestamp - startTime;
      const progressRatio = Math.min(progress / duration, 1);
      const currentValue = Math.floor(progressRatio * value);
      
      setCount(currentValue);
      
      if (progress < duration) {
        requestAnimationFrame(animateCount);
      }
    };
    
    requestAnimationFrame(animateCount);
  }, [value, duration]);

  return <div className="text-3xl font-bold">{count.toLocaleString()}</div>;
};

export default function Home() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [activeChart, setActiveChart] = useState<"distribution" | "ranking">("distribution");
  
  // Real data from APIs
  const [todayMetrics, setTodayMetrics] = useState<MetricsData>({ reqs: 0, tokens: 0 });
  const [totalMetrics, setTotalMetrics] = useState<MetricsData>({ reqs: 0, tokens: 0 });
  const [modelCounts, setModelCounts] = useState<ModelCount[]>([]);
  const [dbStats, setDbStats] = useState<DatabaseStats | null>(null);

  useEffect(() => {
    Promise.all([fetchTodayMetrics(), fetchTotalMetrics(), fetchModelCounts(), fetchDatabaseStats()]);
  }, []);
  
  const fetchTodayMetrics = async () => {
    try {
      const data = await getMetrics(0);
      setTodayMetrics(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取今日指标失败: ${message}`);
      console.error(err);
    }
  };
  
  const fetchTotalMetrics = async () => {
    try {
      const data = await getMetrics(30); // Get last 30 days for "total" metrics
      setTotalMetrics(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取总计指标失败: ${message}`);
      console.error(err);
    }
  };
  
  const fetchModelCounts = async () => {
    try {
      const data = await getModelCounts();
      setModelCounts(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取模型调用统计失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchDatabaseStats = async () => {
    try {
      const data = await getDatabaseStats();
      setDbStats(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(`获取数据库统计失败: ${message}`);
    }
  };

  if (loading) return <Loading message="加载系统概览" />;

  return (
    <div className="space-y-6 overflow-auto h-full">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>今日请求</CardTitle>
            <CardDescription>今日处理的请求总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={todayMetrics.reqs} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>今日Tokens</CardTitle>
            <CardDescription>今日处理的Tokens总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={todayMetrics.tokens} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>本月请求</CardTitle>
            <CardDescription>最近30天处理的请求总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={totalMetrics.reqs} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>本月Tokens</CardTitle>
            <CardDescription>最近30天处理的Tokens总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={totalMetrics.tokens} />
          </CardContent>
        </Card>
       </div>

      {/* 数据库概览卡片 */}
      {dbStats && (
        <Card
          className="cursor-pointer hover:bg-accent/50 transition-colors"
          onClick={() => navigate("/database")}
        >
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <div>
              <CardTitle className="text-base">数据库状态</CardTitle>
              <CardDescription>点击查看详情</CardDescription>
            </div>
            <Database className="h-5 w-5 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <div className="text-2xl font-bold">{dbStats.file_size_human}</div>
                <p className="text-xs text-muted-foreground">文件大小</p>
              </div>
              <div>
                <div className="text-2xl font-bold">
                  {dbStats.page_count > 0
                    ? ((dbStats.page_count - dbStats.free_pages) / dbStats.page_count * 100).toFixed(1)
                    : "0"}%
                </div>
                <p className="text-xs text-muted-foreground">使用率</p>
              </div>
              <div>
                <div className="text-2xl font-bold">
                  {dbStats.table_stats.reduce((sum, t) => sum + t.count, 0).toLocaleString()}
                </div>
                <p className="text-xs text-muted-foreground">总记录数</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
       
      <Card>
        <CardHeader>
          <CardTitle>模型数据分析</CardTitle>
          <CardDescription>模型调用统计分析</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2 mb-4">
            <Button 
              variant={activeChart === "distribution" ? "default" : "outline"} 
              onClick={() => setActiveChart("distribution")}
            >
              调用次数分布
            </Button>
            <Button 
              variant={activeChart === "ranking" ? "default" : "outline"} 
              onClick={() => setActiveChart("ranking")}
            >
              调用次数排行
            </Button>
          </div>
          <div className="mt-4">
            <Suspense fallback={<div className="h-64 flex items-center justify-center">
              <Loading message="加载图表..." />
            </div>}>
              {activeChart === "distribution" ? <ChartPieDonutText data={modelCounts} /> : <ModelRankingChart data={modelCounts} />}
            </Suspense>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
 
