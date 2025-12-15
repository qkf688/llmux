import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { updateHealthCheckSettings, runHealthCheckAll } from "@/lib/api";
import type { HealthCheckSettings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

interface HealthCheckSettingsProps {
  healthCheckSettings: HealthCheckSettings | null;
  onHealthCheckSettingsChange: (settings: HealthCheckSettings) => void;
}

export function HealthCheckSettingsTab({ healthCheckSettings, onHealthCheckSettingsChange }: HealthCheckSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(healthCheckSettings);
  const [hasChanges, setHasChanges] = useState(false);
  const [runningHealthCheck, setRunningHealthCheck] = useState(false);

  const updateLocalSettings = (updates: Partial<HealthCheckSettings>) => {
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
      const updated = await updateHealthCheckSettings(localSettings);
      setLocalSettings(updated);
      onHealthCheckSettingsChange(updated);
      setHasChanges(false);
      toast.success("健康检测设置保存成功");
    } catch (error) {
      toast.error("保存健康检测设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setLocalSettings(healthCheckSettings);
    setHasChanges(false);
  };

  const handleRunHealthCheckAll = async () => {
    try {
      setRunningHealthCheck(true);
      await runHealthCheckAll();
      toast.success("已启动所有模型提供商的健康检测");
    } catch (error) {
      toast.error("启动健康检测失败: " + (error as Error).message);
    } finally {
      setRunningHealthCheck(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">健康检测</h2>
          <p className="text-sm text-muted-foreground">配置模型提供商的定时健康检测功能</p>
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
          <CardTitle>基本设置</CardTitle>
          <CardDescription>
            配置健康检测的基本参数
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="health-check-enabled" className="text-base font-medium">
                启用健康检测
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，系统会定时检测所有模型提供商的可用性。
              </p>
            </div>
            <Switch
              id="health-check-enabled"
              checked={localSettings?.enabled ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ enabled: checked })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="health-check-interval" className="text-base font-medium">
              检测间隔（分钟）
            </Label>
            <p className="text-sm text-muted-foreground">
              每隔多少分钟执行一次健康检测。
            </p>
            <Input
              id="health-check-interval"
              type="number"
              min={1}
              max={1440}
              value={localSettings?.interval ?? 60}
              onChange={(e) => updateLocalSettings({ interval: parseInt(e.target.value) || 60 })}
              className="w-32"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="health-check-log-retention-count" className="text-base font-medium">
              健康检测日志保留条数
            </Label>
            <p className="text-sm text-muted-foreground">
              系统自动保留的最新健康检测日志条数，设置为 0 表示不限制。
            </p>
            <Input
              id="health-check-log-retention-count"
              type="number"
              min={0}
              max={100000}
              value={localSettings?.log_retention_count ?? 0}
              onChange={(e) => updateLocalSettings({ log_retention_count: parseInt(e.target.value) || 0 })}
              className="w-32"
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>失败处理</CardTitle>
          <CardDescription>
            配置健康检测失败时的处理策略
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="health-check-failure-disable-enabled" className="text-base font-medium">
                启用失败自动禁用
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，当连续失败次数达到阈值时，自动禁用该供应商关联。
              </p>
            </div>
            <Switch
              id="health-check-failure-disable-enabled"
              checked={localSettings?.failure_disable_enabled ?? true}
              onCheckedChange={(checked) => updateLocalSettings({ failure_disable_enabled: checked })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="health-check-failure-threshold" className="text-base font-medium">
              失败次数阈值
            </Label>
            <p className="text-sm text-muted-foreground">
              连续检测失败次数的阈值，达到此值后的处理策略由上方开关控制。
            </p>
            <Input
              id="health-check-failure-threshold"
              type="number"
              min={1}
              max={10}
              value={localSettings?.failure_threshold ?? 3}
              onChange={(e) => updateLocalSettings({ failure_threshold: parseInt(e.target.value) || 3 })}
              className="w-32"
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="health-check-auto-enable" className="text-base font-medium">
                检测成功自动启用
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，当已禁用的模型提供商检测成功时，会自动重新启用。
              </p>
            </div>
            <Switch
              id="health-check-auto-enable"
              checked={localSettings?.auto_enable ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ auto_enable: checked })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>计入调用策略</CardTitle>
          <CardDescription>
            控制健康检测结果是否参与成功自增或失败衰减
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label className="text-base font-medium" htmlFor="count-health-check-as-success">
                健康检测计入成功调用
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，模型自动健康检测的成功结果也会触发权重/优先级自增。
              </p>
            </div>
            <Switch
              id="count-health-check-as-success"
              checked={localSettings?.count_health_check_as_success ?? true}
              onCheckedChange={(checked) => updateLocalSettings({ count_health_check_as_success: checked })}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label className="text-base font-medium" htmlFor="count-health-check-as-failure">
                健康检测计入失败调用衰减
              </Label>
              <p className="text-sm text-muted-foreground">
                开启后，健康检测失败会视作一次调用失败，触发权重/优先级衰减。
              </p>
            </div>
            <Switch
              id="count-health-check-as-failure"
              checked={localSettings?.count_health_check_as_failure ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ count_health_check_as_failure: checked })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>手动操作</CardTitle>
          <CardDescription>
            立即执行健康检测
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label className="text-base font-medium">手动执行检测</Label>
              <p className="text-sm text-muted-foreground">
                立即对所有模型提供商执行一次健康检测。
              </p>
            </div>
            <Button
              variant="outline"
              onClick={handleRunHealthCheckAll}
              disabled={runningHealthCheck}
            >
              {runningHealthCheck ? <Spinner className="w-4 h-4 mr-2" /> : null}
              执行检测
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
