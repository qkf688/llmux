import { apiRequest, apiRequestAnonymous } from "../../core/client";
import type { AuthUser } from "@/stores/auth";

export interface LoginRequest {
  password: string;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export async function login(password: string): Promise<LoginResponse> {
  // 匿名请求：登录失败回 401，但语义是「密码不对」而非「登录过期」，
  // 走 apiRequest 会被当成过期踢回登录页并刷掉刚写的错误提示。
  return apiRequestAnonymous<LoginResponse>("/auth/login", {
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
