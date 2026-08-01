import { apiRequest } from "../../core/client";

export interface ModelWithProvider {
  ID: number;
  ModelID: number;
  ProviderModel: string;
  ProviderID: number;
  ToolCall: boolean;
  StructuredOutput: boolean;
  Image: boolean;
  WithHeader: boolean;
  CustomerHeaders: Record<string, string> | null;
  Status: boolean | null;
  Weight: number;
  Priority: number;
  MaxTokens: number | null;
}

export interface AssociationPreview {
  model_id: number;
  model_name: string;
  provider_id: number;
  provider_name: string;
  provider_model: string;
}

export interface ModelProviderTestResult {
  error?: string;
  message?: string;
  [key: string]: unknown;
}

export async function getModelProviders(modelId: number): Promise<ModelWithProvider[]> {
  return apiRequest<ModelWithProvider[]>(`/model-providers?model_id=${modelId}`);
}

export async function getModelProviderHealthStatus(
  modelProviderId: number,
  limit = 10
): Promise<boolean[]> {
  const params = new URLSearchParams({
    model_provider_id: modelProviderId.toString(),
    limit: limit.toString(),
  });
  return apiRequest<boolean[]>(`/model-providers/health-status?${params.toString()}`);
}

export async function getModelProviderStatus(
  providerId: number,
  modelName: string,
  providerModel: string
): Promise<boolean[]> {
  const params = new URLSearchParams({
    provider_id: providerId.toString(),
    model_name: modelName,
    provider_model: providerModel,
  });
  return apiRequest<boolean[]>(`/model-providers/status?${params.toString()}`);
}

export async function createModelProvider(association: {
  model_id: number;
  provider_name: string;
  provider_id: number;
  tool_call: boolean;
  structured_output: boolean;
  image: boolean;
  with_header: boolean;
  customer_headers: Record<string, string>;
  weight: number;
  priority?: number;
  max_tokens?: number;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>("/model-providers", {
    method: "POST",
    body: JSON.stringify(association),
  });
}

export async function updateModelProvider(
  id: number,
  association: {
    model_id?: number;
    provider_name?: string;
    provider_id?: number;
    tool_call?: boolean;
    structured_output?: boolean;
    image?: boolean;
    with_header?: boolean;
    customer_headers?: Record<string, string>;
    weight?: number;
    priority?: number;
    max_tokens?: number;
  }
): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}`, {
    method: "PUT",
    body: JSON.stringify(association),
  });
}

export async function updateModelProviderStatus(id: number, status: boolean): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export async function deleteModelProvider(id: number): Promise<void> {
  await apiRequest<void>(`/model-providers/${id}`, {
    method: "DELETE",
  });
}

export async function batchDeleteModelProviders(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/model-providers/batch", {
    method: "DELETE",
    body: JSON.stringify({ ids }),
  });
}

export async function batchUpdateModelProvidersStatus(
  ids: number[],
  status: boolean
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/model-providers/batch/status", {
    method: "PATCH",
    body: JSON.stringify({ ids, status }),
  });
}

export async function batchUpdateModelProvidersCapabilities(
  ids: number[],
  capabilities: {
    tool_call?: boolean;
    structured_output?: boolean;
    image?: boolean;
  }
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/model-providers/batch/capabilities", {
    method: "PATCH",
    body: JSON.stringify({ ids, ...capabilities }),
  });
}

export async function testModelProvider(id: number): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/${id}`);
}

export async function testModelProviderStructuredOutput(id: number): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/structured-output/${id}`);
}

export async function previewAutoAssociate(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>("/model-providers/auto-associate/preview");
}

export async function autoAssociateModels(): Promise<{ added: number; failed: number }> {
  return apiRequest<{ added: number; failed: number }>("/model-providers/auto-associate", {
    method: "POST",
  });
}

export async function getProviderBlacklist(): Promise<{ blacklisted_ids: number[] }> {
  return apiRequest<{ blacklisted_ids: number[] }>("/providers/blacklist");
}

export async function updateProviderBlacklist(providerIds: number[]): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/providers/blacklist", {
    method: "PUT",
    body: JSON.stringify({ provider_ids: providerIds }),
  });
}

export async function previewCleanInvalid(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>("/model-providers/clean-invalid/preview");
}

export async function cleanInvalidAssociations(): Promise<{ removed: number; failed: number }> {
  return apiRequest<{ removed: number; failed: number }>("/model-providers/clean-invalid", {
    method: "POST",
  });
}
