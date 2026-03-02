import type { ModelSyncLog } from "@/lib/api";

export type ModelSyncTab = "logs" | "recent";

export interface ParsedModelSyncError {
  statusCode: string | null;
  responseBody: string | null;
  originalError: string;
}

export type StatusTone = "success" | "unchanged" | "error" | "unknown";

export interface StatusPresentation {
  tone: StatusTone;
  label: string;
}

export type SelectedLogState = Set<number>;

export function isLogSelected(selected: SelectedLogState, log: ModelSyncLog): boolean {
  return selected.has(log.ID);
}
