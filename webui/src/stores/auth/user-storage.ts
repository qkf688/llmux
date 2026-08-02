import { readStorageJSON, removeStorageItem, writeStorageJSON } from "@/stores/core/storage";
import type { AuthUser } from "@/stores/auth/auth-store";

export const AUTH_USER_STORAGE_KEY = "authUser";

export function readAuthUser(): AuthUser | null {
  return readStorageJSON<AuthUser>(AUTH_USER_STORAGE_KEY);
}

export function writeAuthUser(user: AuthUser): void {
  writeStorageJSON(AUTH_USER_STORAGE_KEY, user);
}

export function clearAuthUser(): void {
  removeStorageItem(AUTH_USER_STORAGE_KEY);
}
