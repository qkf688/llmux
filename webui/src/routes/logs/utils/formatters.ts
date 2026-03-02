export const formatTime = (nanoseconds: number): string => {
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
};

export const formatDurationValue = (value?: number) =>
  typeof value === "number" ? formatTime(value) : "-";

export const formatTokenValue = (value?: number) =>
  typeof value === "number" ? value.toLocaleString() : "-";

export const formatTpsValue = (value?: number) =>
  typeof value === "number" ? value.toFixed(2) : "-";

export const formatDateTime = (value: string) => new Date(value).toLocaleString();

export const formatByteLength = (value?: string) =>
  value ? `${value.length.toLocaleString()} 字节` : "-";
