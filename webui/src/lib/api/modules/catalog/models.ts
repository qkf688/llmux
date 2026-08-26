import { apiRequest } from "../../core/client";

export interface Model {
  ID: number;
  Name: string;
  Remark: string;
  IOLog: boolean;
  auto_associate?: boolean;
  supports_thinking: boolean;
  thinking_levels?: string[];
}

export interface ModelTemplateItem {
  name: string;
  sources: string[];
}

export interface ModelTemplate {
  model_id: number;
  model_name: string;
  items: ModelTemplateItem[];
}

export async function getModels(): Promise<Model[]> {
  return apiRequest<Model[]>("/models");
}

export async function createModel(model: {
  name: string;
  remark: string;
  io_log: boolean;
  supports_thinking?: boolean;
  thinking_levels?: string[];
}): Promise<Model> {
  return apiRequest<Model>("/models", {
    method: "POST",
    body: JSON.stringify(model),
  });
}

export async function updateModel(
  id: number,
  model: {
    name?: string;
    remark?: string;
    io_log?: boolean;
    auto_associate?: boolean;
    supports_thinking?: boolean;
    thinking_levels?: string[];
  }
): Promise<Model> {
  return apiRequest<Model>(`/models/${id}`, {
    method: "PUT",
    body: JSON.stringify(model),
  });
}

export async function deleteModel(id: number): Promise<void> {
  await apiRequest<void>(`/models/${id}`, {
    method: "DELETE",
  });
}

export async function batchDeleteModels(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/models/batch", {
    method: "DELETE",
    body: JSON.stringify({ ids }),
  });
}

export async function batchUpdateModels(params: {
  ids: number[];
  auto_associate?: boolean;
}): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/models/batch", {
    method: "PUT",
    body: JSON.stringify(params),
  });
}

export async function getModelTemplate(modelId: number): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template`);
}

export async function addModelTemplateItem(modelId: number, name: string): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template/items`, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export async function deleteModelTemplateItem(modelId: number, name: string): Promise<ModelTemplate> {
  return apiRequest<ModelTemplate>(`/models/${modelId}/template/items`, {
    method: "DELETE",
    body: JSON.stringify({ name }),
  });
}
