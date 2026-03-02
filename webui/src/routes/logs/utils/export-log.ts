import type { ChatLog } from "@/lib/api";

type ExportPayload = {
  log_id: number;
  created_at: string;
  model_name: string;
  provider_name: string;
  provider_model: string;
  status: string;
  request: {
    headers: string | null;
    body: string | null;
  };
  response: {
    headers: string | null;
    body: string | null;
    raw_body: string | null;
  };
};

function buildExportPayload(log: ChatLog): ExportPayload {
  return {
    log_id: log.ID,
    created_at: log.CreatedAt,
    model_name: log.Name,
    provider_name: log.ProviderName,
    provider_model: log.ProviderModel,
    status: log.Status,
    request: {
      headers: log.RequestHeaders ?? null,
      body: log.RequestBody ?? null,
    },
    response: {
      headers: log.ResponseHeaders ?? null,
      body: log.ResponseBody ?? null,
      raw_body: log.RawResponseBody ?? null,
    },
  };
}

export function exportRequestResponse(log: ChatLog) {
  const exportData = buildExportPayload(log);
  const jsonString = JSON.stringify(exportData, null, 2);
  const blob = new Blob([jsonString], { type: "application/json" });
  const url = URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.href = url;
  link.download = `log-${log.ID}-request-response-${Date.now()}.json`;

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
