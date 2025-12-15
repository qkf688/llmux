import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { updateSettings } from "@/lib/api";
import type { Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

interface RoutingSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export function RoutingSettings({ settings, onSettingsChange }: RoutingSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(settings);
  const [hasChanges, setHasChanges] = useState(false);

  const handleStrictCapabilityMatchChange = (checked: boolean) => {
    if (localSettings) {
      const newSettings = { ...localSettings, strict_capability_match: checked };
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
      toast.success("路由设置保存成功");
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
          <h2 className="text-xl font-semibold">路由设置</h2>
          <p className="text-sm text-muted-foreground">配置请求路由和负载均衡相关选项</p>
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
          <CardTitle>能力匹配</CardTitle>
          <CardDescription>
            配置请求路由时的能力匹配策略
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
              checked={localSettings?.strict_capability_match ?? true}
              onCheckedChange={handleStrictCapabilityMatchChange}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
