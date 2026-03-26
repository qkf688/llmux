import { createStore } from "zustand/vanilla";
import type { HealthCheckSettings, Settings } from "@/lib/api";

export type SettingsPageState = {
  settings: Settings | null;
  healthCheckSettings: HealthCheckSettings | null;
  loading: boolean;

  setSettings: (settings: Settings | null) => void;
  setHealthCheckSettings: (settings: HealthCheckSettings | null) => void;
  setLoading: (loading: boolean) => void;

  resetTransient: () => void;
};

export const settingsPageStore = createStore<SettingsPageState>()((set) => ({
  settings: null,
  healthCheckSettings: null,
  loading: true,

  setSettings: (settings: Settings | null) => set({ settings }),
  setHealthCheckSettings: (settings: HealthCheckSettings | null) => set({ healthCheckSettings: settings }),
  setLoading: (loading: boolean) => set({ loading }),

  resetTransient: () => set({ settings: null, healthCheckSettings: null, loading: true }),
}));

