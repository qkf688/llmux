import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { updateSettings, resetModelWeights, resetModelPriorities, enableAllAssociations } from "@/lib/api";
import type { Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

interface BalancerSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export function BalancerSettings({ settings, onSettingsChange }: BalancerSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(settings);
  const [hasChanges, setHasChanges] = useState(false);
  const [resettingWeights, setResettingWeights] = useState(false);
  const [resettingPriorities, setResettingPriorities] = useState(false);
  const [enablingAssociations, setEnablingAssociations] = useState(false);

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
      toast.success("负载均衡设置保存成功");
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">负载均衡</h2>
          <p className="text-sm text-muted-foreground">配置权重、优先级和自动调整策略</p>
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

      {/* 权重衰减 */}
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
              </p>
            </div>
            <Switch
              id="auto-weight-decay"
              checked={localSettings?.auto_weight_decay ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ auto_weight_decay: checked })}
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
              value={localSettings?.auto_weight_decay_default ?? 5}
              onChange={(e) => updateLocalSettings({ auto_weight_decay_default: parseInt(e.target.value) || 5 })}
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
              value={localSettings?.auto_weight_decay_step ?? 1}
              onChange={(e) => updateLocalSettings({ auto_weight_decay_step: parseInt(e.target.value) || 1 })}
              className="w-32"
            />
          </div>

          <div className="pt-4 border-t">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label className="text-base font-medium">重置所有权重</Label>
                <p className="text-sm text-muted-foreground">
                  将所有模型关联的权重重置为默认值 ({localSettings?.auto_weight_decay_default ?? 5})。
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

      {/* 优先级衰减 */}
      <Card>
        <CardHeader>
          <CardTitle>自动优先级衰减</CardTitle>
          <CardDescription>
            配置调用失败时自动降低供应商优先级的行为
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
              </p>
            </div>
            <Switch
              id="auto-priority-decay"
              checked={localSettings?.auto_priority_decay ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ auto_priority_decay: checked })}
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
              value={localSettings?.auto_priority_decay_default ?? 10}
              onChange={(e) => updateLocalSettings({ auto_priority_decay_default: parseInt(e.target.value) || 10 })}
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
              value={localSettings?.auto_priority_decay_step ?? 1}
              onChange={(e) => updateLocalSettings({ auto_priority_decay_step: parseInt(e.target.value) || 1 })}
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
                  开启后，当优先级降到禁用阈值时，自动禁用该供应商关联。
                </p>
              </div>
              <Switch
                id="auto-priority-decay-disable-enabled"
                checked={localSettings?.auto_priority_decay_disable_enabled ?? true}
                onCheckedChange={(checked) => updateLocalSettings({ auto_priority_decay_disable_enabled: checked })}
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
                value={localSettings?.auto_priority_decay_threshold ?? 90}
                onChange={(e) => updateLocalSettings({ auto_priority_decay_threshold: parseInt(e.target.value) || 90 })}
                className="w-32"
              />
            </div>
          </div>

          <div className="pt-4 border-t space-y-4">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label className="text-base font-medium">重置所有优先级</Label>
                <p className="text-sm text-muted-foreground">
                  将所有模型关联的优先级重置为默认值 ({localSettings?.auto_priority_decay_default ?? 10})。
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

      {/* 成功自增 */}
      <Card>
        <CardHeader>
          <CardTitle>成功调用自增</CardTitle>
          <CardDescription>
            配置成功调用后自动提升权重与优先级的策略
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
              checked={localSettings?.auto_success_increase ?? true}
              onCheckedChange={(checked) => updateLocalSettings({ auto_success_increase: checked })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="auto-weight-increase-step" className="text-base font-medium">
              权重增加步长
            </Label>
            <p className="text-sm text-muted-foreground">
              每次调用成功后增加的权重值。
            </p>
            <Input
              id="auto-weight-increase-step"
              type="number"
              min={1}
              max={100}
              value={localSettings?.auto_weight_increase_step ?? 1}
              onChange={(e) => updateLocalSettings({ auto_weight_increase_step: parseInt(e.target.value) || 1 })}
              className="w-32"
              disabled={!localSettings?.auto_success_increase}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="auto-weight-increase-max" className="text-base font-medium">
              权重增加上限
            </Label>
            <p className="text-sm text-muted-foreground">
              成功自增后的最大权重值，防止无限增长。
            </p>
            <Input
              id="auto-weight-increase-max"
              type="number"
              min={1}
              max={10000}
              value={localSettings?.auto_weight_increase_max ?? 5}
              onChange={(e) => updateLocalSettings({ auto_weight_increase_max: parseInt(e.target.value) || 5 })}
              className="w-32"
              disabled={!localSettings?.auto_success_increase}
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
              value={localSettings?.auto_priority_increase_step ?? 1}
              onChange={(e) => updateLocalSettings({ auto_priority_increase_step: parseInt(e.target.value) || 1 })}
              className="w-32"
              disabled={!localSettings?.auto_success_increase}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="auto-priority-increase-max" className="text-base font-medium">
              优先级增加上限
            </Label>
            <p className="text-sm text-muted-foreground">
              成功自增后的最大优先级值。
            </p>
            <Input
              id="auto-priority-increase-max"
              type="number"
              min={0}
              max={10000}
              value={localSettings?.auto_priority_increase_max ?? 10}
              onChange={(e) => updateLocalSettings({ auto_priority_increase_max: parseInt(e.target.value) || 10 })}
              className="w-32"
              disabled={!localSettings?.auto_success_increase}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>模型关联自动化</CardTitle>
          <CardDescription>
            配置模型关联的自动添加和清理功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="auto-associate-on-add" className="text-base font-medium">
                添加时自动关联
              </Label>
              <p className="text-sm text-muted-foreground">
                新增提供商添加"全部模型"时，或提供商增加模型时，自动关联到模板匹配的模型（包含 Model.Name、既有关联 ProviderModel 与手动模板项）
              </p>
            </div>
            <Switch
              id="auto-associate-on-add"
              checked={localSettings?.auto_associate_on_add ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ auto_associate_on_add: checked })}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="auto-clean-on-delete" className="text-base font-medium">
                删除时自动清理
              </Label>
              <p className="text-sm text-muted-foreground">
                提供商被删除或模型减少时，自动清除无效的模型关联
              </p>
            </div>
            <Switch
              id="auto-clean-on-delete"
              checked={localSettings?.auto_clean_on_delete ?? false}
              onCheckedChange={(checked) => updateLocalSettings({ auto_clean_on_delete: checked })}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
