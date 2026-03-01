export const API_BASE = "/api";

type ApiResponseEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

function getAuthToken(): string | null {
  return localStorage.getItem("authToken");
}

function buildAuthHeaders(headers: HeadersInit = {}): HeadersInit {
  const token = getAuthToken();
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...headers,
  };
}

function handleUnauthorized(): never {
  window.location.href = "/login";
  throw new Error("Unauthorized");
}

export async function fetchWithAuth(endpoint: string, options: RequestInit = {}): Promise<Response> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: buildAuthHeaders(options.headers),
  });

  if (response.status === 401) {
    handleUnauthorized();
  }

  return response;
}

export async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await fetchWithAuth(endpoint, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...buildAuthHeaders(options.headers),
    },
  });

  if (!response.ok) {
    throw new Error(`API request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as ApiResponseEnvelope<T>;
  if (payload.code !== 200) {
    throw new Error(`${payload.message}`);
  }

  return payload.data as T;
}
