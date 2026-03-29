export function toErrorMessage(error: unknown, fallback = "未知错误"): string {
  if (error instanceof Error) {
    const message = error.message?.trim();
    return message ? message : fallback;
  }

  if (typeof error === "string") {
    const message = error.trim();
    return message ? message : fallback;
  }

  if (typeof error === "number" || typeof error === "boolean" || typeof error === "bigint") {
    return String(error);
  }

  if (error && typeof error === "object") {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string") {
      const trimmed = message.trim();
      if (trimmed) {
        return trimmed;
      }
    }
  }

  return fallback;
}

