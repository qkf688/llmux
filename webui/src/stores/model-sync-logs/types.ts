export type ModelSyncTab = "logs" | "recent" | "errors";

export type ModelSyncLogsPagePreferences = {
  activeTab: ModelSyncTab;
  showUnchanged: boolean;
};

