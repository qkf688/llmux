"use client";

export type CompactCount = {
  raw: number;
  value: string;
  unit: string;
};

export function formatCompactCount(input?: number): CompactCount {
  const raw = typeof input === "number" ? input : 0;
  const abs = Math.abs(raw);

  if (abs >= 1_000_000_000) {
    return { raw, value: (raw / 1_000_000_000).toFixed(2), unit: "B" };
  }
  if (abs >= 1_000_000) {
    return { raw, value: (raw / 1_000_000).toFixed(2), unit: "M" };
  }
  if (abs >= 1_000) {
    return { raw, value: (raw / 1_000).toFixed(2), unit: "K" };
  }

  return { raw, value: Math.round(raw).toLocaleString(), unit: "" };
}

export function formatPercent(value?: number, digits = 1): string {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "-";
  }
  return `${(value * 100).toFixed(digits)}%`;
}

export function formatTimeNs(nanoseconds?: number): string {
  if (typeof nanoseconds !== "number" || Number.isNaN(nanoseconds)) {
    return "-";
  }

  if (nanoseconds < 1000) {
    return `${nanoseconds.toFixed(2)} ns`;
  }
  if (nanoseconds < 1_000_000) {
    return `${(nanoseconds / 1000).toFixed(2)} μs`;
  }
  if (nanoseconds < 1_000_000_000) {
    return `${(nanoseconds / 1_000_000).toFixed(2)} ms`;
  }
  return `${(nanoseconds / 1_000_000_000).toFixed(2)} s`;
}

/** ISO/RFC3339 时间串 → 本地可读格式（新页面统一用本函数；存量 routes/*
 *  各自的本地 formatDateTime 副本沉并另行登记，locale 不一致属存量债） */
export function formatDateTime(value?: string): string {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN");
}

