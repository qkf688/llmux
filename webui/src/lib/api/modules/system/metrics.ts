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

export async function getModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>("/metrics/counts");
}

export async function getRealModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>("/metrics/real-model-counts");
}
