import { createStore } from "zustand/vanilla";
import { queryClient } from "@/lib/query-client";
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
    // 会话失效即清 react-query 缓存（Stage 9 / find-issues 2026-09-16）：api-keys 全量
    // 明文等机密域数据驻留缓存时，gcTime 5min 内同机 DevTools 可查读，登出 ≠ 数据出内存。
    // 清空收口在本动作而非各调用点：登出与 401 失效都经此处，会话出口单一 owner；
    // 401 路径另有 window.location 硬重载兜底（内存整体销毁），此处清空对它无副作用。
    queryClient.clear();
    set({ token: null, user: null });
  },
}));

export function getAuthToken(): string | null {
  return authStore.getState().token ?? readAuthToken();
}
