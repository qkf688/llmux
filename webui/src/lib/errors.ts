/** 测试/错误类型值域，镜像后端 common.ErrorTypeFromStatus 与 common 的 ErrorType* 常量
 * （诊断测试 error_type 的单一来源，testapi 与 #24-B 按凭据测试共用） */
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

/** 请求取消（`fetch` 在中止信号触发时抛出的 `AbortError`）与真实失败必须分开：
 *  中止的测试不是「这个供应商/模型不行」的证据，混进失败会污染结论与计数。
 *  判据取 `name`，兼容 `DOMException` 与测试里手造的带 name 的 Error。 */
export function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && (error as { name?: unknown }).name === "AbortError"
  );
}

