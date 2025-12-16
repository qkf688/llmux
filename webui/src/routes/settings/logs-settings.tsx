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
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">日志管理</h2>
          <p className="text-sm text-muted-foreground">配置日志保留策略和管理日志数据</p>
        </div>
        <div className="flex gap-2">
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
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="log-retention-count" className="text-base font-medium">
              日志保留条数
            </Label>
            <p className="text-sm text-muted-foreground">
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

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="log-raw-request-response" className="text-base font-medium">
                记录原始请求响应
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，系统会在日志中记录完整的原始请求和响应内容。
                <br />
                <span className="text-amber-600 dark:text-amber-500">注意：这会显著增加日志存储空间占用。</span>
              </p>
            </div>
            <Switch
              id="log-raw-request-response"
              checked={localSettings?.log_raw_request_response ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ log_raw_request_response: checked })}
            />
          </div>

          <div className="flex items-center justify-between space-x-4">
            <div className="space-y-0.5">
              <Label className="text-base font-medium" htmlFor="disable-all-logs">
                完全关闭日志记录
              </Label>
              <p className="text-sm text-muted-foreground">
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
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="disable-performance-tracking" className="text-base font-medium">
                关闭性能追踪
              </Label>
              <p className="text-sm text-muted-foreground">
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

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="disable-token-counting" className="text-base font-medium">
                关闭 Token 统计
              </Label>
              <p className="text-sm text-muted-foreground">
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

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="enable-request-trace" className="text-base font-medium">
                启用请求追踪
              </Label>
              <p className="text-sm text-muted-foreground">
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
        </CardContent>
      </Card>
    </div>
  );
}
