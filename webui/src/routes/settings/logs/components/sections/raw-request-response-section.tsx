import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { LogsSettingsSectionProps } from "../../types";

function buildRawRequestResponseUpdates(
  localSettings: LogsSettingsSectionProps["localSettings"],
  updates: Partial<NonNullable<NonNullable<LogsSettingsSectionProps["localSettings"]>["log_raw_request_response"]>>
) {
  return {
    log_raw_request_response: {
      ...localSettings?.log_raw_request_response,
      request_headers: localSettings?.log_raw_request_response?.request_headers ?? false,
      request_body: localSettings?.log_raw_request_response?.request_body ?? false,
      raw_request_body: localSettings?.log_raw_request_response?.raw_request_body ?? false,
      response_headers: localSettings?.log_raw_request_response?.response_headers ?? false,
      response_body: localSettings?.log_raw_request_response?.response_body ?? false,
      raw_response_body: localSettings?.log_raw_request_response?.raw_response_body ?? false,
      ...updates,
    },
  };
}

export function RawRequestResponseSection({ localSettings, updateLocalSettings }: LogsSettingsSectionProps) {
  return (
    <div className="space-y-3 md:space-y-4">
      <div className="space-y-0.5">
        <Label className="text-sm md:text-base font-medium">记录原始请求响应</Label>
        <p className="text-xs md:text-sm text-muted-foreground">
          选择需要记录的内容。您可以根据需要选择性记录以优化存储空间。
          <br />
          <span className="text-amber-600 dark:text-amber-500">注意：记录内容越多，日志存储空间占用越大。</span>
        </p>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div className="space-y-0.5">
          <Label htmlFor="log-raw-errors-only" className="text-xs md:text-sm font-medium">
            仅保留错误日志
          </Label>
          <p className="text-xs text-muted-foreground">
            开启后，仅在请求失败时保留下方选择的原始请求/响应内容；成功请求会自动清空这些字段。
          </p>
        </div>
        <Switch
          id="log-raw-errors-only"
          checked={localSettings?.log_raw_request_response_errors_only ?? false}
          onCheckedChange={(checked) => {
            updateLocalSettings({ log_raw_request_response_errors_only: checked });
          }}
        />
      </div>

      <div className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-request-headers" className="text-xs md:text-sm font-medium">
              请求头
            </Label>
            <p className="text-xs text-muted-foreground">记录客户端发送的 HTTP 请求头信息</p>
          </div>
          <Switch
            id="log-request-headers"
            checked={localSettings?.log_raw_request_response?.request_headers ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { request_headers: checked }));
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-request-body" className="text-xs md:text-sm font-medium">
              请求体
            </Label>
            <p className="text-xs text-muted-foreground">记录发给上游的请求体（协议转换后的）</p>
          </div>
          <Switch
            id="log-request-body"
            checked={localSettings?.log_raw_request_response?.request_body ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { request_body: checked }));
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-raw-request-body" className="text-xs md:text-sm font-medium">
              原始请求体
            </Label>
            <p className="text-xs text-muted-foreground">记录客户端发来的原始请求体（协议转换前的，用于调试格式转换问题）</p>
          </div>
          <Switch
            id="log-raw-request-body"
            checked={localSettings?.log_raw_request_response?.raw_request_body ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { raw_request_body: checked }));
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-response-headers" className="text-xs md:text-sm font-medium">
              响应头
            </Label>
            <p className="text-xs text-muted-foreground">记录服务端返回的 HTTP 响应头信息</p>
          </div>
          <Switch
            id="log-response-headers"
            checked={localSettings?.log_raw_request_response?.response_headers ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { response_headers: checked }));
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-response-body" className="text-xs md:text-sm font-medium">
              响应体
            </Label>
            <p className="text-xs text-muted-foreground">记录转换后的响应体内容（实际返回给客户端的内容）</p>
          </div>
          <Switch
            id="log-response-body"
            checked={localSettings?.log_raw_request_response?.response_body ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { response_body: checked }));
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="log-raw-response-body" className="text-xs md:text-sm font-medium">
              原始响应体
            </Label>
            <p className="text-xs text-muted-foreground">记录格式转换前的原始响应体（用于调试格式转换问题）</p>
          </div>
          <Switch
            id="log-raw-response-body"
            checked={localSettings?.log_raw_request_response?.raw_response_body ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings(buildRawRequestResponseUpdates(localSettings, { raw_response_body: checked }));
            }}
          />
        </div>
      </div>
    </div>
  );
}

