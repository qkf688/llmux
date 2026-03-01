import { apiRequest } from "../../core/client";

export interface Provider {
  ID: number;
  Name: string;
  Type: string;
  Config: string;
  Console: string;
  Proxy: string;
  ModelEndpoint?: boolean;
  ModelFilterEnabled?: boolean;
  AuthType?: string;
  blacklisted?: boolean;
}

export interface ProviderTemplate {
  type: string;
  template: string;
}

export interface ProviderModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

export async function getProviders(filters: { name?: string; type?: string } = {}): Promise<Provider[]> {
  const params = new URLSearchParams();
  if (filters.name) {
    params.append("name", filters.name);
  }
  if (filters.type) {
    params.append("type", filters.type);
  }

  const queryString = params.toString();
  const endpoint = queryString ? `/providers?${queryString}` : "/providers";
  return apiRequest<Provider[]>(endpoint);
}

export async function createProvider(provider: {
  name: string;
  type: string;
  config: string;
  console: string;
  proxy: string;
  model_endpoint?: boolean;
  model_filter_enabled?: boolean;
  blacklisted?: boolean;
  auth_type?: string;
}): Promise<Provider> {
  return apiRequest<Provider>("/providers", {
    method: "POST",
    body: JSON.stringify(provider),
  });
}

export async function updateProvider(
  id: number,
  provider: {
    name?: string;
    type?: string;
    config?: string;
    console?: string;
    proxy?: string;
    model_endpoint?: boolean;
    model_filter_enabled?: boolean;
    blacklisted?: boolean;
    auth_type?: string;
  }
): Promise<Provider> {
  return apiRequest<Provider>(`/providers/${id}`, {
    method: "PUT",
    body: JSON.stringify(provider),
  });
}

export async function deleteProvider(id: number): Promise<void> {
  await apiRequest<void>(`/providers/${id}`, {
    method: "DELETE",
  });
}

export async function getProviderTemplates(): Promise<ProviderTemplate[]> {
  return apiRequest<ProviderTemplate[]>("/providers/template");
}

export async function getProviderModels(
  providerId: number,
  options: { source?: "upstream" | "all" } = {}
): Promise<ProviderModel[]> {
  const params = new URLSearchParams();
  if (options.source) {
    params.append("source", options.source);
  }

  const queryString = params.toString();
  const endpoint = queryString
    ? `/providers/models/${providerId}?${queryString}`
    : `/providers/models/${providerId}`;
  return apiRequest<ProviderModel[]>(endpoint);
}

export async function testProviderModel(providerId: number, model: string): Promise<any> {
  return apiRequest<any>(`/providers/${providerId}/test`, {
    method: "POST",
    body: JSON.stringify({ model }),
  });
}

export async function clearProviderAssociations(
  providerId: number
): Promise<{ provider_id: number; provider_name: string; deleted_count: number }> {
  return apiRequest<{ provider_id: number; provider_name: string; deleted_count: number }>(
    `/providers/${providerId}/associations`,
    {
      method: "DELETE",
    }
  );
}
