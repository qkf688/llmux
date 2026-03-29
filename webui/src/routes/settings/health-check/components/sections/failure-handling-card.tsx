import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { HealthCheckSettingsSectionProps } from "../../types";

export function FailureHandlingCard({ localSettings, updateLocalSettings }: HealthCheckSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>失败处理</CardTitle>
        <CardDescription>配置健康检测失败时的处理策略</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="health-check-failure-disable-enabled" className="text-sm md:text-base font-medium">
              启用失败自动禁用
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，当连续失败次数达到阈值时，自动禁用该供应商关联。
            </p>
          </div>
          <Switch
            id="health-check-failure-disable-enabled"
            checked={localSettings?.failure_disable_enabled ?? true}
            onCheckedChange={(checked) => updateLocalSettings({ failure_disable_enabled: checked })}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="health-check-failure-threshold" className="text-sm md:text-base font-medium">
            失败次数阈值
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">
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

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="health-check-auto-enable" className="text-sm md:text-base font-medium">
              检测成功自动启用
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
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
  );
}

