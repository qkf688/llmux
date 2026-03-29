import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { HealthCheckSettingsSectionProps } from "../../types";

export function InvocationCountPolicyCard({ localSettings, updateLocalSettings }: HealthCheckSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>计入调用策略</CardTitle>
        <CardDescription>控制健康检测结果是否参与成功自增或失败衰减</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label className="text-sm md:text-base font-medium" htmlFor="count-health-check-as-success">
              健康检测计入成功调用
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，模型自动健康检测的成功结果也会触发权重/优先级自增。
            </p>
          </div>
          <Switch
            id="count-health-check-as-success"
            checked={localSettings?.count_health_check_as_success ?? true}
            onCheckedChange={(checked) => updateLocalSettings({ count_health_check_as_success: checked })}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label className="text-sm md:text-base font-medium" htmlFor="count-health-check-as-failure">
              健康检测计入失败调用衰减
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
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
  );
}

