import type { AuthState } from "@/stores/auth/auth-store";

export const selectAuthToken = (state: AuthState) => state.token;
export const selectAuthUser = (state: AuthState) => state.user;
export const selectIsAuthenticated = (state: AuthState) => Boolean(state.token);
export const selectSetSession = (state: AuthState) => state.setSession;
export const selectClearSession = (state: AuthState) => state.clearSession;
