import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { HealthCheckSettingsSectionProps } from "../../types";

export function BasicSettingsCard({ localSettings, updateLocalSettings }: HealthCheckSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>基本设置</CardTitle>
        <CardDescription>配置健康检测的基本参数</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="health-check-enabled" className="text-sm md:text-base font-medium">
              启用健康检测
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">开启后，系统会定时检测所有模型提供商的可用性。</p>
          </div>
          <Switch
            id="health-check-enabled"
            checked={localSettings?.enabled ?? false}
            onCheckedChange={(checked) => updateLocalSettings({ enabled: checked })}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="health-check-interval" className="text-sm md:text-base font-medium">
            检测间隔（分钟）
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">每隔多少分钟执行一次健康检测。</p>
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

        <div className="space-y-1.5">
          <Label htmlFor="health-check-log-retention-count" className="text-sm md:text-base font-medium">
            健康检测日志保留条数
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">
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

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="health-check-disabled-only" className="text-sm md:text-base font-medium">
              只检测停用的模型
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，健康检测只会检查已停用的模型提供商。适合配合&quot;检测成功自动启用&quot;功能，实现停用模型的自动恢复。
            </p>
          </div>
          <Switch
            id="health-check-disabled-only"
            checked={localSettings?.check_disabled_only ?? false}
            onCheckedChange={(checked) => updateLocalSettings({ check_disabled_only: checked })}
          />
        </div>
      </CardContent>
    </Card>
  );
}

