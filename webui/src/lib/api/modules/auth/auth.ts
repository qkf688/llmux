import { apiRequest } from "../../core/client";
import type { AuthUser } from "@/stores/auth";

export interface LoginRequest {
  password: string;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export async function login(password: string): Promise<LoginResponse> {
  return apiRequest<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ password } satisfies LoginRequest),
  });
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await apiRequest<null>("/auth/password", {
    method: "POST",
    body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
  });
}
