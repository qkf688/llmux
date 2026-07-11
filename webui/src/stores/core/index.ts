export type { StorageLike } from "./storage";
export {
  getBrowserLocalStorage,
  readStorageBoolean,
  readStorageJSON,
  readStorageString,
  removeStorageItem,
  writeStorageBoolean,
  writeStorageJSON,
  writeStorageString,
} from "./storage";

export type { Updater } from "./updater";
export { resolveUpdater } from "./updater";