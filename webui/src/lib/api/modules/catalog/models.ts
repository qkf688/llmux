import { apiRequest } from "../../core/client";

export interface Model {
  ID: number;
  Name: string;
  Remark: string;
  IOLog: boolean;
  // 二态开关：后端是普通 bool，「未设置」不是可表达状态（曾声明的三态已收回）。
  auto_associate: boolean;
  supports_thinking: boolean;
  thinking_levels?: string[];
  // 模型固有能力真值（关联侧默认继承这三项）：
  // tool_call 工具调用 / structured_output 结构化输出 / image 视觉。
  // 键名为 snake_case（后端显式 json tag），与 ModelWithProvider 上的 PascalCase 相反。
  tool_call: boolean;
  structured_output: boolean;
  image: boolean;
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
  tool_call?: boolean;
  structured_output?: boolean;
  image?: boolean;
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
    // 能力真值：不发字段 = 保留存量（后端条件写），
    // 避免不带该字段的保存把已开启的能力静默重置为 false。
    tool_call?: boolean;
    structured_output?: boolean;
    image?: boolean;
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
