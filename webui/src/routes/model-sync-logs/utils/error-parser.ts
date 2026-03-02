import type { ParsedModelSyncError } from "../types";

export function parseModelSyncError(errorMsg: string): ParsedModelSyncError {
  const statusCodeMatch = errorMsg.match(/status code:\s*(\d+)/);
  const responseMatch = errorMsg.match(/response:\s*(.+)$/);

  let statusCode: string | null = null;
  let responseBody: string | null = null;

  if (statusCodeMatch) {
    statusCode = statusCodeMatch[1];
  }

  if (responseMatch) {
    const rawResponseBody = responseMatch[1];
    try {
      const parsed = JSON.parse(rawResponseBody);
      responseBody = JSON.stringify(parsed, null, 2);
    } catch {
      responseBody = rawResponseBody;
    }
  }

  return {
    statusCode,
    responseBody,
    originalError: errorMsg,
  };
}
