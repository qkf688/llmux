export type StorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export function getBrowserLocalStorage(): StorageLike | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/** 会话级存储（跨整页刷新存活、关标签页即清）：一次性提示等「本轮会话有效」的状态用它。 */
export function getBrowserSessionStorage(): StorageLike | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function readStorageString(key: string, storage: StorageLike | null = getBrowserLocalStorage()): string | null {
  try {
    return storage?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

export function readStorageBoolean(
  key: string,
  storage: StorageLike | null = getBrowserLocalStorage(),
): boolean | null {
  const value = readStorageString(key, storage);
  if (value === null) return null;
  if (value === "true") return true;
  if (value === "false") return false;
  return null;
}

export function readStorageJSON<T>(
  key: string,
  storage: StorageLike | null = getBrowserLocalStorage(),
): T | null {
  const value = readStorageString(key, storage);
  if (value === null) return null;
  try {
    return JSON.parse(value) as T;
  } catch {
    return null;
  }
}

export function writeStorageString(
  key: string,
  value: string,
  storage: StorageLike | null = getBrowserLocalStorage(),
): void {
  try {
    storage?.setItem(key, value);
  } catch {
    // ignore storage failures (private mode, quota, etc.)
  }
}

export function writeStorageBoolean(
  key: string,
  value: boolean,
  storage: StorageLike | null = getBrowserLocalStorage(),
): void {
  writeStorageString(key, String(value), storage);
}

export function writeStorageJSON<T>(
  key: string,
  value: T,
  storage: StorageLike | null = getBrowserLocalStorage(),
): void {
  try {
    writeStorageString(key, JSON.stringify(value), storage);
  } catch {
    // ignore serialization failures
  }
}

export function removeStorageItem(key: string, storage: StorageLike | null = getBrowserLocalStorage()): void {
  try {
    storage?.removeItem(key);
  } catch {
    // ignore
  }
}
