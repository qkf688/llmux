import { readStorageJSON, writeStorageJSON } from "@/stores/core/storage";
import type { ModelSyncLogsPagePreferences } from "@/stores/model-sync-logs/types";

const MODEL_SYNC_LOGS_PREFERENCES_KEY = "modelSyncLogs.pagePreferences";

export function readModelSyncLogsPagePreferences(): ModelSyncLogsPagePreferences {
  const stored = readStorageJSON<Partial<ModelSyncLogsPagePreferences>>(MODEL_SYNC_LOGS_PREFERENCES_KEY);

  const activeTab = stored?.activeTab;
  const showUnchanged = stored?.showUnchanged;

  return {
    activeTab: activeTab === "logs" || activeTab === "recent" || activeTab === "errors" ? activeTab : "logs",
    showUnchanged: typeof showUnchanged === "boolean" ? showUnchanged : false,
  };
}

export function writeModelSyncLogsPagePreferences(preferences: ModelSyncLogsPagePreferences): void {
  writeStorageJSON(MODEL_SYNC_LOGS_PREFERENCES_KEY, preferences);
}

