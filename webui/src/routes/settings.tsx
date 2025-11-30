import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { getSettings, updateSettings, resetModelWeights } from "@/lib/api";
import type { Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [originalSettings, setOriginalSettings] = useState<Settings | null>(null);

  useEffect(() => {
    loadSettings();
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

  const checkHasChanges = (newSettings: Settings) => {
    if (!originalSettings) return;
    const changed =
      originalSettings.strict_capability_match !== newSettings.strict_capability_match ||
      originalSettings.auto_weight_decay !== newSettings.auto_weight_decay ||
      originalSettings.auto_weight_decay_default !== newSettings.auto_weight_decay_default ||
      originalSettings.auto_weight_decay_step !== newSettings.auto_weight_decay_step;
    setHasChanges(changed);
  };

  const handleResetAllWeights = async () => {
    try {
      setResetting(true);
      const result = await resetModelWeights();
      toast.success(`已重置 ${result.updated} 个模型关联的权重到 ${result.default_weight}`);
    } catch (error) {
      toast.error("重置权重失败: " + (error as Error).message);
    } finally {
      setResetting(false);
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

            {settings?.auto_weight_decay && (
              <>
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
                    value={settings?.auto_weight_decay_default ?? 100}
                    onChange={(e) => handleAutoWeightDecayDefaultChange(parseInt(e.target.value) || 100)}
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
                        将所有模型关联的权重重置为默认值 ({settings?.auto_weight_decay_default ?? 100})。
                      </p>
                    </div>
                    <Button
                      variant="outline"
                      onClick={handleResetAllWeights}
                      disabled={resetting}
                    >
                      {resetting ? <Spinner className="w-4 h-4 mr-2" /> : null}
                      重置权重
                    </Button>
                  </div>
                </div>
              </>
            )}
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
