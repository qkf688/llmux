import { apiRequest } from "../../core/client";
import type { AuthUser } from "@/stores/auth";

export interface LoginRequest {
  password: string;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export interface RotateAPIKeyResponse {
  api_key: string;
}

export async function login(password: string): Promise<LoginResponse> {
  return apiRequest<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ password } satisfies LoginRequest),
  });
}

export async function getCurrentUser(): Promise<AuthUser> {
  return apiRequest<AuthUser>("/auth/me");
}

export async function rotateAPIKey(): Promise<RotateAPIKeyResponse> {
  return apiRequest<RotateAPIKeyResponse>("/auth/api-key/rotate", {
    method: "POST",
  });
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await apiRequest<null>("/auth/password", {
    method: "POST",
    body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
  });
}
