import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { updateSettings } from "@/lib/api";
import type { Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

interface LogsSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export function LogsSettings({ settings, onSettingsChange }: LogsSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(settings);
  const [hasChanges, setHasChanges] = useState(false);

  // 同步父组件的 settings 变化到 localSettings
  useEffect(() => {
    setLocalSettings(settings);
    setHasChanges(false);
  }, [settings]);

  const updateLocalSettings = (updates: Partial<Settings>) => {
    if (localSettings) {
      const newSettings = { ...localSettings, ...updates };
      setLocalSettings(newSettings);
      setHasChanges(true);
    }
  };

  const handleSave = async () => {
    if (!localSettings) return;

    try {
      setSaving(true);
      const updated = await updateSettings(localSettings);
      setLocalSettings(updated);
      onSettingsChange(updated);
      setHasChanges(false);
      toast.success("日志设置保存成功");
    } catch (error) {
      toast.error("保存设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setLocalSettings(settings);
    setHasChanges(false);
  };

  return (
    <div className="space-y-4 md:space-y-6">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h2 className="text-lg md:text-xl font-semibold">日志管理</h2>
          <p className="text-xs md:text-sm text-muted-foreground">配置日志保留策略和管理日志数据</p>
        </div>
        <div className="flex gap-1.5 md:gap-2">
          <Button
            variant="outline"
            onClick={handleReset}
            disabled={!hasChanges || saving}
          >
            重置
          </Button>
          <Button
            onClick={handleSave}
            disabled={!hasChanges || saving}
          >
            {saving ? <Spinner className="w-4 h-4 mr-2" /> : null}
            保存
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>日志配置</CardTitle>
          <CardDescription>
            管理系统日志的保留和记录策略
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="space-y-1.5">
            <Label htmlFor="log-retention-count" className="text-sm md:text-base font-medium">
              日志保留条数
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              系统自动保留的最新日志条数。设置为 0 表示不限制。
              <br />
              修改此设置后，超出保留条数的旧日志将被自动删除。
            </p>
            <Input
              id="log-retention-count"
              type="number"
              min={0}
              max={100000}
              value={localSettings?.log_retention_count ?? 100}
              onChange={(e) => updateLocalSettings({ log_retention_count: parseInt(e.target.value) || 0 })}
              className="w-32"
            />
          </div>

          <div className="space-y-3 md:space-y-4">
            <div className="space-y-0.5">
              <Label className="text-sm md:text-base font-medium">
                记录原始请求响应
              </Label>
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
                onCheckedChange={(checked) => updateLocalSettings({ log_raw_request_response_errors_only: checked })}
              />
            </div>

            <div className="space-y-2.5 md:space-y-3 pl-3 md:pl-4 border-l-2 border-muted">
              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <Label htmlFor="log-request-headers" className="text-xs md:text-sm font-medium">
                    请求头
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    记录客户端发送的完整 HTTP 请求头信息
                  </p>
                </div>
                <Switch
                  id="log-request-headers"
                  checked={localSettings?.log_raw_request_response?.request_headers ?? false}
                  onCheckedChange={(checked) => updateLocalSettings({
                    log_raw_request_response: {
                      ...localSettings?.log_raw_request_response,
                      request_headers: checked,
                      request_body: localSettings?.log_raw_request_response?.request_body ?? false,
                      response_headers: localSettings?.log_raw_request_response?.response_headers ?? false,
                      response_body: localSettings?.log_raw_request_response?.response_body ?? false,
                      raw_response_body: localSettings?.log_raw_request_response?.raw_response_body ?? false,
                    }
                  })}
                />
              </div>

              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <Label htmlFor="log-request-body" className="text-xs md:text-sm font-medium">
                    请求体
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    记录完整的请求体内容（包含提示词等）
                  </p>
                </div>
                <Switch
                  id="log-request-body"
                  checked={localSettings?.log_raw_request_response?.request_body ?? false}
                  onCheckedChange={(checked) => updateLocalSettings({
                    log_raw_request_response: {
                      ...localSettings?.log_raw_request_response,
                      request_headers: localSettings?.log_raw_request_response?.request_headers ?? false,
                      request_body: checked,
                      response_headers: localSettings?.log_raw_request_response?.response_headers ?? false,
                      response_body: localSettings?.log_raw_request_response?.response_body ?? false,
                      raw_response_body: localSettings?.log_raw_request_response?.raw_response_body ?? false,
                    }
                  })}
                />
              </div>

              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <Label htmlFor="log-response-headers" className="text-xs md:text-sm font-medium">
                    响应头
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    记录服务端返回的 HTTP 响应头信息
                  </p>
                </div>
                <Switch
                  id="log-response-headers"
                  checked={localSettings?.log_raw_request_response?.response_headers ?? false}
                  onCheckedChange={(checked) => updateLocalSettings({
                    log_raw_request_response: {
                      ...localSettings?.log_raw_request_response,
                      request_headers: localSettings?.log_raw_request_response?.request_headers ?? false,
                      request_body: localSettings?.log_raw_request_response?.request_body ?? false,
                      response_headers: checked,
                      response_body: localSettings?.log_raw_request_response?.response_body ?? false,
                      raw_response_body: localSettings?.log_raw_request_response?.raw_response_body ?? false,
                    }
                  })}
                />
              </div>

              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <Label htmlFor="log-response-body" className="text-xs md:text-sm font-medium">
                    响应体
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    记录转换后的响应体内容（实际返回给客户端的内容）
                  </p>
                </div>
                <Switch
                  id="log-response-body"
                  checked={localSettings?.log_raw_request_response?.response_body ?? false}
                  onCheckedChange={(checked) => updateLocalSettings({
                    log_raw_request_response: {
                      ...localSettings?.log_raw_request_response,
                      request_headers: localSettings?.log_raw_request_response?.request_headers ?? false,
                      request_body: localSettings?.log_raw_request_response?.request_body ?? false,
                      response_headers: localSettings?.log_raw_request_response?.response_headers ?? false,
                      response_body: checked,
                      raw_response_body: localSettings?.log_raw_request_response?.raw_response_body ?? false,
                    }
                  })}
                />
              </div>

              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <Label htmlFor="log-raw-response-body" className="text-xs md:text-sm font-medium">
                    原始响应体
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    记录格式转换前的原始响应体（用于调试格式转换问题）
                  </p>
                </div>
                <Switch
                  id="log-raw-response-body"
                  checked={localSettings?.log_raw_request_response?.raw_response_body ?? false}
                  onCheckedChange={(checked) => updateLocalSettings({
                    log_raw_request_response: {
                      ...localSettings?.log_raw_request_response,
                      request_headers: localSettings?.log_raw_request_response?.request_headers ?? false,
                      request_body: localSettings?.log_raw_request_response?.request_body ?? false,
                      response_headers: localSettings?.log_raw_request_response?.response_headers ?? false,
                      response_body: localSettings?.log_raw_request_response?.response_body ?? false,
                      raw_response_body: checked,
                    }
                  })}
                />
              </div>
            </div>
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label className="text-sm md:text-base font-medium" htmlFor="disable-all-logs">
                完全关闭日志记录
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统将不记录任何请求日志，可大幅提升性能（提升100-200%）。
                <br />
                建议仅在极致性能要求下使用，关闭后无法在界面查看请求历史。
              </p>
            </div>
            <Switch
              id="disable-all-logs"
              checked={localSettings?.disable_all_logs ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ disable_all_logs: checked })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>性能优化</CardTitle>
          <CardDescription>
            通过关闭部分功能来提升系统性能和降低资源消耗
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="disable-performance-tracking" className="text-sm md:text-base font-medium">
                关闭性能追踪
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                关闭后，系统将不再记录首包时间和 TPS（每秒 token 数）等性能指标。
                <br />
                可减少时间计算和统计开销，适度提升性能。
              </p>
            </div>
            <Switch
              id="disable-performance-tracking"
              checked={localSettings?.disable_performance_tracking ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ disable_performance_tracking: checked })}
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="disable-token-counting" className="text-sm md:text-base font-medium">
                关闭 Token 统计
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                关闭后，系统将不再统计和记录 token 使用量（输入/输出 token 数）。
                <br />
                可减少 JSON 解析和字段提取开销，适度提升性能。
              </p>
            </div>
            <Switch
              id="disable-token-counting"
              checked={localSettings?.disable_token_counting ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ disable_token_counting: checked })}
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="enable-request-trace" className="text-sm md:text-base font-medium">
                启用请求追踪
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统将使用 HTTP 追踪来监控网络请求的详细信息（如首字节时间）。
                <br />
                关闭可减少少量追踪开销，但会影响调试能力。建议保持开启。
              </p>
            </div>
            <Switch
              id="enable-request-trace"
              checked={localSettings?.enable_request_trace ?? true}
              onCheckedChange={(checked) => updateLocalSettings({ enable_request_trace: checked })}
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="strip-response-headers" className="text-sm md:text-base font-medium">
                移除不必要的响应头
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统将只保留核心响应头（Content-Type、X-Request-Id 等），移除其他响应头。
                <br />
                可减少网络传输数据量，适度提升性能。
              </p>
            </div>
            <Switch
              id="strip-response-headers"
              checked={localSettings?.strip_response_headers ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ strip_response_headers: checked })}
            />
          </div>

        </CardContent>
      </Card>
    </div>
  );
}
