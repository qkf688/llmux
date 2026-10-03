import { apiRequest } from "../../core/client";

export interface MetricsData {
  reqs: number;
  tokens: number;
}

export interface DailyMetricsData {
  date: string; // YYYY-MM-DD
  reqs: number;
  tokens: number;
}

export interface HourlyMetricsData {
  hour: number; // 0-23
  reqs: number;
  tokens: number;
}

export interface ModelCount {
  model: string;
  calls: number;
  /** 三列恒为非负整数。0 既可能是「没消耗」也可能是「早于 token 统计起点、
   * 从未统计过」（2026-09-09 之前的调用，其日志已被保留策略清空），
   * 两种情况在库里无法区分——展示层须一律按「未知」呈现，不得显示成 0。 */
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export async function getMetrics(days: number): Promise<MetricsData> {
  return apiRequest<MetricsData>(`/metrics/use/${days}`);
}

export async function getDailyMetrics(days: number): Promise<DailyMetricsData[]> {
  return apiRequest<DailyMetricsData[]>(`/metrics/dailies/${days}`);
}

export async function getHourlyMetricsToday(): Promise<HourlyMetricsData[]> {
  return apiRequest<HourlyMetricsData[]>("/metrics/hourlies/today");
}

export async function getTotalMetrics(): Promise<MetricsData> {
  return apiRequest<MetricsData>("/metrics/total");
}

export async function getRealModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>("/metrics/real-model-counts");
}
