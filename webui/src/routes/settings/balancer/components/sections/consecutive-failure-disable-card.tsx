import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { BalancerSettingsSectionProps } from "../../types";

export function ConsecutiveFailureDisableCard({
  localSettings,
  updateLocalSettings,
}: BalancerSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>连续失败禁用</CardTitle>
        <CardDescription>配置连续调用失败后的自动禁用策略</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="consecutive-failure-disable-enabled" className="text-sm md:text-base font-medium">
              启用连续失败禁用
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，模型关联在连续调用失败达到阈值时会自动禁用。
            </p>
          </div>
          <Switch
            id="consecutive-failure-disable-enabled"
            checked={localSettings?.consecutive_failure_disable_enabled ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ consecutive_failure_disable_enabled: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="consecutive-failure-threshold" className="text-sm md:text-base font-medium">
            连续失败次数
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">连续调用失败达到此值时自动禁用该模型关联。</p>
          <Input
            id="consecutive-failure-threshold"
            type="number"
            min={1}
            max={100}
            value={localSettings?.consecutive_failure_threshold ?? 3}
            onChange={(e) => {
              updateLocalSettings({ consecutive_failure_threshold: parseInt(e.target.value) || 3 });
            }}
            className="w-32"
            disabled={!localSettings?.consecutive_failure_disable_enabled}
          />
        </div>
      </CardContent>
    </Card>
  );
}

