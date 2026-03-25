import { useStore } from "zustand";
import { authStore, type AuthState } from "@/stores/auth/auth-store";

export function useAuthStore<T>(selector: (state: AuthState) => T): T {
  return useStore(authStore, selector);
}

