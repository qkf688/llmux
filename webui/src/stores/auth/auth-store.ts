import { createStore } from "zustand/vanilla";
import { clearAuthToken, readAuthToken, writeAuthToken } from "@/stores/auth/token-storage";
import { clearAuthUser, readAuthUser, writeAuthUser } from "@/stores/auth/user-storage";

export type AuthUser = {
  id: number;
  username: string;
  role: string;
};

export type AuthState = {
  token: string | null;
  user: AuthUser | null;
  setSession: (token: string, user: AuthUser) => void;
  clearSession: () => void;
};

const initialToken = readAuthToken();
const initialUser = readAuthUser();

export const authStore = createStore<AuthState>()((set) => ({
  token: initialToken,
  user: initialUser,
  setSession: (token: string, user: AuthUser) => {
    const normalized = token.trim();
    if (!normalized) return;
    writeAuthToken(normalized);
    writeAuthUser(user);
    set({ token: normalized, user });
  },
  clearSession: () => {
    clearAuthToken();
    clearAuthUser();
    set({ token: null, user: null });
  },
}));

export function getAuthToken(): string | null {
  return authStore.getState().token ?? readAuthToken();
}
