export function formatResponseTime(milliseconds: number): string {
  if (milliseconds < 1) {
    return `${(milliseconds * 1000).toFixed(2)} μs`;
  }

  if (milliseconds < 1000) {
    return `${milliseconds.toFixed(2)} ms`;
  }

  return `${(milliseconds / 1000).toFixed(2)} s`;
}

export function formatDateTime(value: string): string {
  return new Date(value).toLocaleString();
}

export function isHealthCheckSuccess(status: string): boolean {
  return status === "success";
}
