import type { AuthState } from "@/stores/auth/auth-store";

export const selectAuthToken = (state: AuthState) => state.token;
export const selectIsAuthenticated = (state: AuthState) => Boolean(state.token);
export const selectSetToken = (state: AuthState) => state.setToken;
export const selectClearToken = (state: AuthState) => state.clearToken;

