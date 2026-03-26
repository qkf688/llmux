import { createStore } from "zustand/vanilla";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

export type SettingsEditorState<TSettings extends object> = {
  saving: boolean;
  localSettings: TSettings | null;
  hasChanges: boolean;

  setSaving: (saving: boolean) => void;
  setHasChanges: (hasChanges: boolean) => void;
  setLocalSettings: (next: Updater<TSettings | null>) => void;

  syncFromServerSettings: (settings: TSettings | null) => void;
  updateLocalSettings: (updates: Partial<TSettings>) => void;
  markSaved: (settings: TSettings) => void;

  resetTransient: () => void;
};

export function createSettingsEditorStore<TSettings extends object>() {
  return createStore<SettingsEditorState<TSettings>>()((set) => ({
    saving: false,
    localSettings: null,
    hasChanges: false,

    setSaving: (saving: boolean) => set({ saving }),
    setHasChanges: (hasChanges: boolean) => set({ hasChanges }),
    setLocalSettings: (next: Updater<TSettings | null>) =>
      set((state) => ({ localSettings: resolveUpdater(next, state.localSettings) })),

    syncFromServerSettings: (settings: TSettings | null) => set({ localSettings: settings, hasChanges: false }),
    updateLocalSettings: (updates: Partial<TSettings>) =>
      set((state) => {
        if (!state.localSettings) {
          return state;
        }
        return { localSettings: { ...state.localSettings, ...updates }, hasChanges: true };
      }),
    markSaved: (settings: TSettings) => set({ localSettings: settings, hasChanges: false }),

    resetTransient: () => set({ saving: false, localSettings: null, hasChanges: false }),
  }));
}

