export type Updater<T> = T | ((previous: T) => T);

export type Setter<T> = (value: Updater<T>) => void;

export function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}