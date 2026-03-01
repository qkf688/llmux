import { apiRequest } from "../../core/client";

export interface ModelSyncLog {
  ID: number;
  ProviderID: number;
  ProviderName: string;
  Status: string;
  Error?: string;
  AddedCount: number;
  RemovedCount: number;
  AddedModels: string[];
  RemovedModels: string[];
  SyncedAt: string;
}

export interface ModelSyncLogsResponse {
  data: ModelSyncLog[];
  pagination: {
    page: number;
    page_size: number;
    total: number;
    total_pages: number;
  };
}

export interface ModelSyncStats {
  last_sync_at?: string;
  next_sync_at?: string;
  sync_enabled: boolean;
  sync_interval: number;
  total_providers: number;
  providers_with_updates: number;
  providers_unchanged: number;
  providers_with_errors: number;
  providers_never_synced: number;
}

export interface AddedModel {
  model_name: string;
  provider_name: string;
  added_at: string;
}

export interface RecentAddedModelsResponse {
  data: AddedModel[];
  sync_time?: string;
  total_count: number;
}

export async function syncProviderModels(providerId: number): Promise<ModelSyncLog | { message: string }> {
  return apiRequest<ModelSyncLog | { message: string }>(`/model-sync/${providerId}`, {
    method: "POST",
  });
}

export async function syncAllProviderModels(): Promise<{
  message?: string;
  logs?: ModelSyncLog[];
  synced_providers?: number;
  added_total?: number;
  removed_total?: number;
}> {
  return apiRequest("/model-sync/all", {
    method: "POST",
  });
}

export async function getModelSyncStats(): Promise<ModelSyncStats> {
  return apiRequest<ModelSyncStats>("/model-sync/stats");
}

export async function getModelSyncLogs(
  params: {
    page?: number;
    page_size?: number;
    provider_id?: number;
    show_unchanged?: boolean;
  } = {}
): Promise<ModelSyncLogsResponse> {
  const queryParams = new URLSearchParams();
  if (params.page) {
    queryParams.append("page", params.page.toString());
  }
  if (params.page_size) {
    queryParams.append("page_size", params.page_size.toString());
  }
  if (params.provider_id) {
    queryParams.append("provider_id", params.provider_id.toString());
  }
  if (params.show_unchanged) {
    queryParams.append("show_unchanged", "true");
  }

  const queryString = queryParams.toString();
  const endpoint = queryString ? `/model-sync/logs?${queryString}` : "/model-sync/logs";
  return apiRequest<ModelSyncLogsResponse>(endpoint);
}

export async function deleteModelSyncLogs(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/model-sync/logs", {
    method: "DELETE",
    body: JSON.stringify({ ids }),
  });
}

export async function clearModelSyncLogs(): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/model-sync/logs/clear", {
    method: "DELETE",
  });
}

export async function getRecentAddedModels(): Promise<RecentAddedModelsResponse> {
  return apiRequest<RecentAddedModelsResponse>("/model-sync/recent-added-models");
}
