import { apiRequest } from "../../core/client";

export interface MetricsData {
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

export async function getModelCounts(): Promise<ModelCount[]> {
  return apiRequest<ModelCount[]>("/metrics/counts");
}
