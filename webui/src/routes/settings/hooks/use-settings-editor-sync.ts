import { useEffect } from "react";

type SyncMode = "always" | "when_clean";

type BaseParams<TSettings extends object> = {
  serverSettings: TSettings | null;
  syncFromServerSettings: (settings: TSettings | null) => void;
  resetTransient: () => void;
};

type AlwaysParams<TSettings extends object> = BaseParams<TSettings> & {
  syncMode: "always";
};

type WhenCleanParams<TSettings extends object> = BaseParams<TSettings> & {
  syncMode: "when_clean";
  hasChanges: boolean;
};

export function useSettingsEditorSync<TSettings extends object>(params: AlwaysParams<TSettings> | WhenCleanParams<TSettings>) {
  const { resetTransient, serverSettings, syncFromServerSettings, syncMode } = params;
  const hasChanges = syncMode === "when_clean" ? params.hasChanges : undefined;

  useEffect(() => {
    if (syncMode !== "always") {
      return;
    }

    syncFromServerSettings(serverSettings);
    return () => {
      resetTransient();
    };
  }, [resetTransient, serverSettings, syncFromServerSettings, syncMode]);

  useEffect(() => {
    if (syncMode !== "when_clean") {
      return;
    }

    if (!hasChanges) {
      syncFromServerSettings(serverSettings);
    }
  }, [
    hasChanges,
    serverSettings,
    syncFromServerSettings,
    syncMode,
  ]);

  useEffect(() => {
    if (syncMode !== "when_clean") {
      return;
    }

    return () => {
      resetTransient();
    };
  }, [resetTransient, syncMode]);
}

export type { SyncMode as SettingsEditorSyncMode };
