import { apiRequest } from "../../core/client";

export interface VirtualModel {
  ID: number;
  Name: string;
  Description: string;
  Strategy: string;
  MaxRetry: number;
  TimeOut: number;
  IOLog: boolean;
  Enabled: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface VirtualModelMapping {
  ID: number;
  VirtualModelID: number;
  RealModelID: number;
  Priority: number;
  Weight: number;
  Enabled: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface VirtualModelStats {
  virtual_model_id: number;
  virtual_model_name: string;
  strategy: string;
  total_mappings: number;
  enabled_mappings: number;
  disabled_mappings: number;
}

export interface BatchCreateResult {
  success_count: number;
  failed_count: number;
  success_items: VirtualModelMapping[];
  failed_items: BatchFailedItem[];
}

export interface BatchFailedItem {
  real_model_id: number;
  reason: string;
}

export async function getVirtualModels(): Promise<VirtualModel[]> {
  return apiRequest<VirtualModel[]>("/virtual-models");
}

export async function createVirtualModel(virtualModel: {
  name: string;
  description: string;
  strategy: string;
  max_retry: number;
  time_out: number;
  io_log: boolean;
  enabled: boolean;
}): Promise<VirtualModel> {
  return apiRequest<VirtualModel>("/virtual-models", {
    method: "POST",
    body: JSON.stringify(virtualModel),
  });
}

export async function updateVirtualModel(
  id: number,
  virtualModel: {
    name: string;
    description: string;
    strategy: string;
    max_retry: number;
    time_out: number;
    io_log: boolean;
    enabled: boolean;
  }
): Promise<VirtualModel> {
  return apiRequest<VirtualModel>(`/virtual-models/${id}`, {
    method: "PUT",
    body: JSON.stringify(virtualModel),
  });
}

export async function deleteVirtualModel(id: number): Promise<void> {
  return apiRequest<void>(`/virtual-models/${id}`, {
    method: "DELETE",
  });
}

export async function getVirtualModelMappings(id: number): Promise<VirtualModelMapping[]> {
  return apiRequest<VirtualModelMapping[]>(`/virtual-models/${id}/mappings`);
}

export async function createVirtualModelMapping(
  id: number,
  mapping: {
    real_model_id: number;
    priority: number;
    weight: number;
    enabled: boolean;
  }
): Promise<VirtualModelMapping> {
  return apiRequest<VirtualModelMapping>(`/virtual-models/${id}/mappings`, {
    method: "POST",
    body: JSON.stringify(mapping),
  });
}

export async function updateVirtualModelMapping(
  id: number,
  mappingId: number,
  mapping: {
    real_model_id: number;
    priority: number;
    weight: number;
    enabled: boolean;
  }
): Promise<VirtualModelMapping> {
  return apiRequest<VirtualModelMapping>(`/virtual-models/${id}/mappings/${mappingId}`, {
    method: "PUT",
    body: JSON.stringify(mapping),
  });
}

export async function deleteVirtualModelMapping(id: number, mappingId: number): Promise<void> {
  return apiRequest<void>(`/virtual-models/${id}/mappings/${mappingId}`, {
    method: "DELETE",
  });
}

export async function getVirtualModelStats(id: number): Promise<VirtualModelStats> {
  return apiRequest<VirtualModelStats>(`/virtual-models/${id}/stats`);
}

export async function batchCreateVirtualModelMapping(
  id: number,
  mappings: Array<{
    real_model_id: number;
    priority: number;
    weight: number;
    enabled: boolean;
  }>
): Promise<BatchCreateResult> {
  return apiRequest<BatchCreateResult>(`/virtual-models/${id}/mappings/batch`, {
    method: "POST",
    body: JSON.stringify({ mappings }),
  });
}
