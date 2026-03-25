import { readStorageString, removeStorageItem, writeStorageString } from "@/stores/core/storage";

export const AUTH_TOKEN_STORAGE_KEY = "authToken";

export function readAuthToken(): string | null {
  return readStorageString(AUTH_TOKEN_STORAGE_KEY);
}

export function writeAuthToken(token: string): void {
  writeStorageString(AUTH_TOKEN_STORAGE_KEY, token);
}

export function clearAuthToken(): void {
  removeStorageItem(AUTH_TOKEN_STORAGE_KEY);
}

