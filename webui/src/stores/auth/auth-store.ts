import { createStore } from "zustand/vanilla";
import { clearAuthToken, readAuthToken, writeAuthToken } from "@/stores/auth/token-storage";

export type AuthState = {
  token: string | null;
  setToken: (token: string) => void;
  clearToken: () => void;
};

const initialToken = readAuthToken();

export const authStore = createStore<AuthState>()((set) => ({
  token: initialToken,
  setToken: (token: string) => {
    const normalized = token.trim();
    if (!normalized) return;
    writeAuthToken(normalized);
    set({ token: normalized });
  },
  clearToken: () => {
    clearAuthToken();
    set({ token: null });
  },
}));

export function getAuthToken(): string | null {
  return authStore.getState().token ?? readAuthToken();
}

