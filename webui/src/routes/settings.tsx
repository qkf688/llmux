import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { getSettings, updateSettings, resetModelWeights, resetModelPriorities, enableAllAssociations, getHealthCheckSettings, updateHealthCheckSettings, runHealthCheckAll } from "@/lib/api";
import type { Settings, HealthCheckSettings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [healthCheckSettings, setHealthCheckSettings] = useState<HealthCheckSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savingHealthCheck, setSavingHealthCheck] = useState(false);
  const [resettingWeights, setResettingWeights] = useState(false);
  const [resettingPriorities, setResettingPriorities] = useState(false);
  const [enablingAssociations, setEnablingAssociations] = useState(false);
  const [runningHealthCheck, setRunningHealthCheck] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [hasHealthCheckChanges, setHasHealthCheckChanges] = useState(false);
  const [originalSettings, setOriginalSettings] = useState<Settings | null>(null);
  const [originalHealthCheckSettings, setOriginalHealthCheckSettings] = useState<HealthCheckSettings | null>(null);

  useEffect(() => {
    loadSettings();
    loadHealthCheckSettings();
  }, []);

  const loadSettings = async () => {
    try {
      setLoading(true);
      const data = await getSettings();
      setSettings(data);
      setOriginalSettings(data);
      setHasChanges(false);
    } catch (error) {
      toast.error("加载设置失败: " + (error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const loadHealthCheckSettings = async () => {
    try {
      const data = await getHealthCheckSettings();
      setHealthCheckSettings(data);
      setOriginalHealthCheckSettings(data);
      syncHealthCheckCountFlags(data.count_health_check_as_success, data.count_health_check_as_failure);
      setHasHealthCheckChanges(false);
    } catch (error) {
      toast.error("加载健康检测设置失败: " + (error as Error).message);
    }
  };

  const handleSave = async () => {
    if (!settings) return;

    try {
      setSaving(true);
      const updated = await updateSettings(settings);
      setSettings(updated);
      setOriginalSettings(updated);
      setHasChanges(false);
      toast.success("设置保存成功");
    } catch (error) {
      toast.error("保存设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    if (originalSettings) {
      setSettings(originalSettings);
      setHasChanges(false);
    }
  };

  const handleStrictCapabilityMatchChange = (checked: boolean) => {
    if (settings) {
      const newSettings = { ...settings, strict_capability_match: checked };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoWeightDecayChange = (checked: boolean) => {
    if (settings) {
      const newSettings = { ...settings, auto_weight_decay: checked };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoWeightDecayDefaultChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_weight_decay_default: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoWeightDecayStepChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_weight_decay_step: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoSuccessIncreaseChange = (checked: boolean) => {
    if (settings) {
      const newSettings = { ...settings, auto_success_increase: checked };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoWeightIncreaseStepChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_weight_increase_step: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoWeightIncreaseMaxChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_weight_increase_max: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityDecayChange = (checked: boolean) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_decay: checked };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityDecayDefaultChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_decay_default: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityDecayStepChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_decay_step: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityDecayThresholdChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_decay_threshold: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityDecayDisableEnabledChange = (checked: boolean) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_decay_disable_enabled: checked };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityIncreaseStepChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_increase_step: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleAutoPriorityIncreaseMaxChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, auto_priority_increase_max: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const handleLogRetentionCountChange = (value: number) => {
    if (settings) {
      const newSettings = { ...settings, log_retention_count: value };
      setSettings(newSettings);
      checkHasChanges(newSettings);
    }
  };

  const syncHealthCheckCountFlags = (success: boolean, failure: boolean) => {
    if (settings) {
      const merged = {
        ...settings,
        count_health_check_as_success: success,
        count_health_check_as_failure: failure,
      };
      setSettings(merged);
      checkHasChanges(merged);
    }
  };

  const checkHasChanges = (newSettings: Settings, baseline: Settings | null = originalSettings) => {
    if (!baseline) return;
    const changed =
      baseline.strict_capability_match !== newSettings.strict_capability_match ||
      baseline.auto_weight_decay !== newSettings.auto_weight_decay ||
      baseline.auto_weight_decay_default !== newSettings.auto_weight_decay_default ||
      baseline.auto_weight_decay_step !== newSettings.auto_weight_decay_step ||
      baseline.auto_success_increase !== newSettings.auto_success_increase ||
      baseline.auto_weight_increase_step !== newSettings.auto_weight_increase_step ||
      baseline.auto_weight_increase_max !== newSettings.auto_weight_increase_max ||
      baseline.auto_priority_decay !== newSettings.auto_priority_decay ||
      baseline.auto_priority_decay_default !== newSettings.auto_priority_decay_default ||
      baseline.auto_priority_decay_step !== newSettings.auto_priority_decay_step ||
      baseline.auto_priority_decay_threshold !== newSettings.auto_priority_decay_threshold ||
      baseline.auto_priority_decay_disable_enabled !== newSettings.auto_priority_decay_disable_enabled ||
      baseline.auto_priority_increase_step !== newSettings.auto_priority_increase_step ||
      baseline.auto_priority_increase_max !== newSettings.auto_priority_increase_max ||
      baseline.log_retention_count !== newSettings.log_retention_count ||
      baseline.count_health_check_as_success !== newSettings.count_health_check_as_success ||
      baseline.count_health_check_as_failure !== newSettings.count_health_check_as_failure;
    setHasChanges(changed);
  };

  const handleResetAllWeights = async () => {
    try {
      setResettingWeights(true);
      const result = await resetModelWeights();
      toast.success(`已重置 ${result.updated} 个模型关联的权重到 ${result.default_weight}`);
    } catch (error) {
      toast.error("重置权重失败: " + (error as Error).message);
    } finally {
      setResettingWeights(false);
    }
  };

  const handleResetAllPriorities = async () => {
    try {
      setResettingPriorities(true);
      const result = await resetModelPriorities();
      toast.success(`已重置 ${result.updated} 个模型关联的优先级到 ${result.default_priority}`);
    } catch (error) {
      toast.error("重置优先级失败: " + (error as Error).message);
    } finally {
      setResettingPriorities(false);
    }
  };

  const handleEnableAllAssociations = async () => {
    try {
      setEnablingAssociations(true);
      const result = await enableAllAssociations();
      toast.success(`已启用 ${result.updated} 个模型关联`);
    } catch (error) {
      toast.error("启用关联失败: " + (error as Error).message);
    } finally {
      setEnablingAssociations(false);
    }
  };

  // Health Check Settings handlers
  const handleHealthCheckEnabledChange = (checked: boolean) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, enabled: checked };
      setHealthCheckSettings(newSettings);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckIntervalChange = (value: number) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, interval: value };
      setHealthCheckSettings(newSettings);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckFailureThresholdChange = (value: number) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, failure_threshold: value };
      setHealthCheckSettings(newSettings);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckAutoEnableChange = (checked: boolean) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, auto_enable: checked };
      setHealthCheckSettings(newSettings);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckLogRetentionCountChange = (value: number) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, log_retention_count: value };
      setHealthCheckSettings(newSettings);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckCountAsSuccessChange = (checked: boolean) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, count_health_check_as_success: checked };
      setHealthCheckSettings(newSettings);
      syncHealthCheckCountFlags(newSettings.count_health_check_as_success, newSettings.count_health_check_as_failure);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const handleHealthCheckCountAsFailureChange = (checked: boolean) => {
    if (healthCheckSettings) {
      const newSettings = { ...healthCheckSettings, count_health_check_as_failure: checked };
      setHealthCheckSettings(newSettings);
      syncHealthCheckCountFlags(newSettings.count_health_check_as_success, newSettings.count_health_check_as_failure);
      checkHealthCheckHasChanges(newSettings);
    }
  };

  const checkHealthCheckHasChanges = (newSettings: HealthCheckSettings) => {
    if (!originalHealthCheckSettings) return;
    const changed =
      originalHealthCheckSettings.enabled !== newSettings.enabled ||
      originalHealthCheckSettings.interval !== newSettings.interval ||
      originalHealthCheckSettings.failure_threshold !== newSettings.failure_threshold ||
      originalHealthCheckSettings.auto_enable !== newSettings.auto_enable ||
      originalHealthCheckSettings.log_retention_count !== newSettings.log_retention_count ||
      originalHealthCheckSettings.count_health_check_as_success !== newSettings.count_health_check_as_success ||
      originalHealthCheckSettings.count_health_check_as_failure !== newSettings.count_health_check_as_failure;
    setHasHealthCheckChanges(changed);
  };

  const handleSaveHealthCheckSettings = async () => {
    if (!healthCheckSettings) return;

    try {
      setSavingHealthCheck(true);
      const updated = await updateHealthCheckSettings(healthCheckSettings);
      setHealthCheckSettings(updated);
      setOriginalHealthCheckSettings(updated);
      if (settings) {
        const mergedSettings = {
          ...settings,
          count_health_check_as_success: updated.count_health_check_as_success,
          count_health_check_as_failure: updated.count_health_check_as_failure,
        };
        const updatedBaseline = originalSettings
          ? {
              ...originalSettings,
              count_health_check_as_success: updated.count_health_check_as_success,
              count_health_check_as_failure: updated.count_health_check_as_failure,
            }
          : null;
        setOriginalSettings(updatedBaseline);
        setSettings(mergedSettings);
        checkHasChanges(mergedSettings, updatedBaseline);
      }
      setHasHealthCheckChanges(false);
      toast.success("健康检测设置保存成功");
    } catch (error) {
      toast.error("保存健康检测设置失败: " + (error as Error).message);
    } finally {
      setSavingHealthCheck(false);
    }
  };

  const handleResetHealthCheckSettings = () => {
    if (originalHealthCheckSettings) {
      setHealthCheckSettings(originalHealthCheckSettings);
      setHasHealthCheckChanges(false);
    }
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

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="container mx-auto py-6 space-y-6 max-w-4xl">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">系统设置</h1>
            <p className="text-muted-foreground">管理系统全局配置</p>
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
              保存设置
            </Button>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>路由设置</CardTitle>
            <CardDescription>
              配置请求路由和负载均衡相关选项
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="strict-capability-match" className="text-base font-medium">
                  严格能力匹配
                </Label>
                <p className="text-sm text-muted-foreground">
                  开启后，系统会根据请求的能力需求（工具调用、结构化输出、图片处理）筛选供应商。
                  <br />
                  关闭后，系统将忽略能力匹配条件，允许请求发送到任何启用的供应商。
                </p>
              </div>
              <Switch
                id="strict-capability-match"
                checked={settings?.strict_capability_match ?? true}
                onCheckedChange={handleStrictCapabilityMatchChange}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>自动权重衰减</CardTitle>
            <CardDescription>
              配置调用失败时自动降低供应商权重的行为
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="auto-weight-decay" className="text-base font-medium">
                  启用自动权重衰减
                </Label>
                <p className="text-sm text-muted-foreground">
                  开启后，每次调用失败时，系统会自动减少对应供应商关联的权重。
                  <br />
                  权重越低，该供应商被选中的概率越小。
                </p>
              </div>
              <Switch
                id="auto-weight-decay"
                checked={settings?.auto_weight_decay ?? false}
                onCheckedChange={handleAutoWeightDecayChange}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-weight-decay-default" className="text-base font-medium">
                默认权重值
              </Label>
              <p className="text-sm text-muted-foreground">
                重置权重时使用的默认值，也是新创建关联的推荐权重值。
              </p>
              <Input
                id="auto-weight-decay-default"
                type="number"
                min={1}
                max={1000}
                value={settings?.auto_weight_decay_default ?? 5}
                onChange={(e) => handleAutoWeightDecayDefaultChange(parseInt(e.target.value) || 5)}
                className="w-32"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-weight-decay-step" className="text-base font-medium">
                衰减步长
              </Label>
              <p className="text-sm text-muted-foreground">
                每次调用失败时减少的权重值。
              </p>
              <Input
                id="auto-weight-decay-step"
                type="number"
                min={1}
                max={100}
                value={settings?.auto_weight_decay_step ?? 1}
                onChange={(e) => handleAutoWeightDecayStepChange(parseInt(e.target.value) || 1)}
                className="w-32"
              />
            </div>

            <div className="pt-4 border-t">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base font-medium">重置所有权重</Label>
                  <p className="text-sm text-muted-foreground">
                    将所有模型关联的权重重置为默认值 ({settings?.auto_weight_decay_default ?? 5})。
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={handleResetAllWeights}
                  disabled={resettingWeights}
                >
                  {resettingWeights ? <Spinner className="w-4 h-4 mr-2" /> : null}
                  重置权重
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>自动优先级衰减</CardTitle>
            <CardDescription>
              配置调用失败时自动降低供应商优先级的行为。优先级决定选择顺序，优先级高的供应商优先被选择；优先级相同时按权重随机选择。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="auto-priority-decay" className="text-base font-medium">
                  启用自动优先级衰减
                </Label>
                <p className="text-sm text-muted-foreground">
                  开启后，每次调用失败时，系统会自动减少对应供应商关联的优先级。
                  <br />
                  当优先级降到阈值时，该供应商关联将被自动禁用。
                </p>
              </div>
              <Switch
                id="auto-priority-decay"
                checked={settings?.auto_priority_decay ?? false}
                onCheckedChange={handleAutoPriorityDecayChange}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-priority-decay-default" className="text-base font-medium">
                默认优先级值
              </Label>
              <p className="text-sm text-muted-foreground">
                新创建关联的默认优先级值，也是重置优先级时使用的值。
              </p>
              <Input
                id="auto-priority-decay-default"
                type="number"
                min={1}
                max={1000}
                value={settings?.auto_priority_decay_default ?? 100}
                onChange={(e) => handleAutoPriorityDecayDefaultChange(parseInt(e.target.value) || 100)}
                className="w-32"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-priority-decay-step" className="text-base font-medium">
                衰减步长
              </Label>
              <p className="text-sm text-muted-foreground">
                每次调用失败时减少的优先级值。
              </p>
              <Input
                id="auto-priority-decay-step"
                type="number"
                min={1}
                max={100}
                value={settings?.auto_priority_decay_step ?? 1}
                onChange={(e) => handleAutoPriorityDecayStepChange(parseInt(e.target.value) || 1)}
                className="w-32"
              />
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="auto-priority-decay-disable-enabled" className="text-base font-medium">
                    启用自动禁用
                  </Label>
                  <p className="text-sm text-muted-foreground">
                    开启后，当优先级降到禁用阈值时，自动禁用该供应商关联。关闭后仅衰减优先级，不会自动禁用。
                  </p>
                </div>
                <Switch
                  id="auto-priority-decay-disable-enabled"
                  checked={settings?.auto_priority_decay_disable_enabled ?? true}
                  onCheckedChange={handleAutoPriorityDecayDisableEnabledChange}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="auto-priority-decay-threshold" className="text-base font-medium">
                  禁用阈值
                </Label>
                <p className="text-sm text-muted-foreground">
                  当优先级降到此值或以下时的处理策略，由上方开关控制是否自动禁用。
                </p>
                <Input
                  id="auto-priority-decay-threshold"
                  type="number"
                  min={0}
                  max={100}
                  value={settings?.auto_priority_decay_threshold ?? 90}
                  onChange={(e) => handleAutoPriorityDecayThresholdChange(parseInt(e.target.value) || 90)}
                  className="w-32"
                />
              </div>
            </div>

            <div className="pt-4 border-t space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base font-medium">重置所有优先级</Label>
                  <p className="text-sm text-muted-foreground">
                    将所有模型关联的优先级重置为默认值 ({settings?.auto_priority_decay_default ?? 100})。
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={handleResetAllPriorities}
                  disabled={resettingPriorities}
                >
                  {resettingPriorities ? <Spinner className="w-4 h-4 mr-2" /> : null}
                  重置优先级
                </Button>
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base font-medium">启用所有关联</Label>
                  <p className="text-sm text-muted-foreground">
                    重新启用所有已禁用的模型关联。
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={handleEnableAllAssociations}
                  disabled={enablingAssociations}
                >
                  {enablingAssociations ? <Spinner className="w-4 h-4 mr-2" /> : null}
                  启用所有关联
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>成功调用自增</CardTitle>
            <CardDescription>
              配置成功调用后自动提升权重与优先级的策略，便于恢复被衰减的供应商表现。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="auto-success-increase" className="text-base font-medium">
                  启用成功自增
                </Label>
                <p className="text-sm text-muted-foreground">
                  关闭后，成功调用不会自动提升权重或优先级。
                </p>
              </div>
              <Switch
                id="auto-success-increase"
                checked={settings?.auto_success_increase ?? true}
                onCheckedChange={handleAutoSuccessIncreaseChange}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-weight-increase-step" className="text-base font-medium">
                权重增加步长
              </Label>
              <p className="text-sm text-muted-foreground">
                每次调用成功后增加的权重值，上限受下方限制影响。
              </p>
              <Input
                id="auto-weight-increase-step"
                type="number"
                min={1}
                max={100}
                value={settings?.auto_weight_increase_step ?? 1}
                onChange={(e) => handleAutoWeightIncreaseStepChange(parseInt(e.target.value) || 1)}
                className="w-32"
                disabled={!settings?.auto_success_increase}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-weight-increase-max" className="text-base font-medium">
                权重增加上限
              </Label>
              <p className="text-sm text-muted-foreground">
                成功自增后的最大权重值，防止无限增长。默认 100。
              </p>
              <Input
                id="auto-weight-increase-max"
                type="number"
                min={1}
                max={10000}
                value={settings?.auto_weight_increase_max ?? 100}
                onChange={(e) => handleAutoWeightIncreaseMaxChange(parseInt(e.target.value) || 100)}
                className="w-32"
                disabled={!settings?.auto_success_increase}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-priority-increase-step" className="text-base font-medium">
                优先级增加步长
              </Label>
              <p className="text-sm text-muted-foreground">
                每次调用成功后增加的优先级值。
              </p>
              <Input
                id="auto-priority-increase-step"
                type="number"
                min={1}
                max={100}
                value={settings?.auto_priority_increase_step ?? 1}
                onChange={(e) => handleAutoPriorityIncreaseStepChange(parseInt(e.target.value) || 1)}
                className="w-32"
                disabled={!settings?.auto_success_increase}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="auto-priority-increase-max" className="text-base font-medium">
                优先级增加上限
              </Label>
              <p className="text-sm text-muted-foreground">
                成功自增后的最大优先级值。避免超过预期范围，默认 100。
              </p>
              <Input
                id="auto-priority-increase-max"
                type="number"
                min={0}
                max={10000}
                value={settings?.auto_priority_increase_max ?? 100}
                onChange={(e) => handleAutoPriorityIncreaseMaxChange(parseInt(e.target.value) || 100)}
                className="w-32"
                disabled={!settings?.auto_success_increase}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>日志管理</CardTitle>
            <CardDescription>
              配置日志保留策略和管理日志数据
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
                value={settings?.log_retention_count ?? 100}
                onChange={(e) => handleLogRetentionCountChange(parseInt(e.target.value) || 0)}
                className="w-32"
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>健康检测</CardTitle>
            <CardDescription>
              配置模型提供商的定时健康检测功能
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-4 rounded-lg border p-4">
              <div className="space-y-1">
                <Label className="text-base font-medium">健康检测计入调用策略</Label>
                <p className="text-sm text-muted-foreground">
                  控制健康检测结果是否参与成功自增或失败衰减，避免与调用统计混淆。
                </p>
              </div>
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
                  checked={healthCheckSettings?.count_health_check_as_success ?? true}
                  onCheckedChange={handleHealthCheckCountAsSuccessChange}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base font-medium" htmlFor="count-health-check-as-failure">
                    健康检测计入失败调用衰减
                  </Label>
                  <p className="text-sm text-muted-foreground">
                    开启后，健康检测失败会视作一次调用失败，触发权重/优先级衰减，帮助自动下调异常供应商。
                  </p>
                </div>
                <Switch
                  id="count-health-check-as-failure"
                  checked={healthCheckSettings?.count_health_check_as_failure ?? false}
                  onCheckedChange={handleHealthCheckCountAsFailureChange}
                />
              </div>
            </div>

            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="health-check-enabled" className="text-base font-medium">
                  启用健康检测
                </Label>
                <p className="text-sm text-muted-foreground">
                  开启后，系统会定时检测所有模型提供商的可用性。
                  <br />
                  检测失败超过阈值后，会自动禁用该模型提供商。
                </p>
              </div>
              <Switch
                id="health-check-enabled"
                checked={healthCheckSettings?.enabled ?? false}
                onCheckedChange={handleHealthCheckEnabledChange}
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
                value={healthCheckSettings?.interval ?? 60}
                onChange={(e) => handleHealthCheckIntervalChange(parseInt(e.target.value) || 60)}
                className="w-32"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="health-check-failure-threshold" className="text-base font-medium">
                失败次数阈值
              </Label>
              <p className="text-sm text-muted-foreground">
                连续检测失败达到此次数后，自动禁用该模型提供商。
              </p>
              <Input
                id="health-check-failure-threshold"
                type="number"
                min={1}
                max={10}
                value={healthCheckSettings?.failure_threshold ?? 3}
                onChange={(e) => handleHealthCheckFailureThresholdChange(parseInt(e.target.value) || 3)}
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
                value={healthCheckSettings?.log_retention_count ?? 0}
                onChange={(e) => handleHealthCheckLogRetentionCountChange(parseInt(e.target.value) || 0)}
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
                checked={healthCheckSettings?.auto_enable ?? false}
                onCheckedChange={handleHealthCheckAutoEnableChange}
              />
            </div>

            <div className="pt-4 border-t flex gap-2 justify-end">
              <Button
                variant="outline"
                onClick={handleResetHealthCheckSettings}
                disabled={!hasHealthCheckChanges || savingHealthCheck}
              >
                重置
              </Button>
              <Button
                onClick={handleSaveHealthCheckSettings}
                disabled={!hasHealthCheckChanges || savingHealthCheck}
              >
                {savingHealthCheck ? <Spinner className="w-4 h-4 mr-2" /> : null}
                保存健康检测设置
              </Button>
            </div>

            <div className="pt-4 border-t space-y-4">
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
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>关于</CardTitle>
            <CardDescription>
              系统信息
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2 text-sm text-muted-foreground">
              <p><strong>LLMUX</strong> - LLM 代理网关</p>
              <p>支持多供应商负载均衡、请求路由和监控</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
