/** 测试/错误类型值域，镜像后端 handler/testapi 的 error_type（getErrorTypeFromStatus + 硬编码调用点产出） */
export const ERROR_TYPES = ["network", "auth", "provider", "timeout", "validation", "unknown"] as const;

export type ErrorType = (typeof ERROR_TYPES)[number];

/** 运行时校验字符串是否为合法错误类型 */
export function isErrorType(value: unknown): value is ErrorType {
  return typeof value === "string" && (ERROR_TYPES as readonly string[]).includes(value);
}

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

