import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { TagInput } from "@/components/ui/tag-input";
import type { RoutingSettingsSectionProps } from "../../types";

const parseInputNumber = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? fallback : parsed;
};

export function ModelSyncCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>模型自动同步</CardTitle>
        <CardDescription>配置上游模型自动同步选项</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="model-sync-enabled" className="text-sm md:text-base font-medium">
              启用自动同步
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统将定期自动同步启用模型端点的提供商的上游模型列表
            </p>
          </div>
          <Switch
            id="model-sync-enabled"
            checked={localSettings?.model_sync_enabled ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ model_sync_enabled: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="model-sync-interval" className="text-sm">
            同步间隔（小时）
          </Label>
          <Input
            id="model-sync-interval"
            type="number"
            min="1"
            value={localSettings?.model_sync_interval ?? 12}
            onChange={(event) => {
              updateLocalSettings({
                model_sync_interval: parseInputNumber(event.target.value, 12),
              });
            }}
          />
          <p className="text-xs md:text-sm text-muted-foreground">默认12小时同步一次</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="model-sync-log-retention-count" className="text-sm">
            日志保留条数
          </Label>
          <Input
            id="model-sync-log-retention-count"
            type="number"
            min="0"
            value={localSettings?.model_sync_log_retention_count ?? 100}
            onChange={(event) => {
              updateLocalSettings({
                model_sync_log_retention_count: parseInputNumber(event.target.value, 100),
              });
            }}
          />
          <p className="text-xs md:text-sm text-muted-foreground">默认保留100条，0表示不限制</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="model-sync-log-retention-days" className="text-sm">
            日志保留天数
          </Label>
          <Input
            id="model-sync-log-retention-days"
            type="number"
            min="0"
            value={localSettings?.model_sync_log_retention_days ?? 7}
            onChange={(event) => {
              updateLocalSettings({
                model_sync_log_retention_days: parseInputNumber(event.target.value, 7),
              });
            }}
          />
          <p className="text-xs md:text-sm text-muted-foreground">默认保留7天，0表示不限制</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="model-sync-filter-rules" className="text-sm">
            模型过滤规则
          </Label>
          <TagInput
            value={localSettings?.model_sync_filter_rules ?? []}
            onChange={(rules) => {
              updateLocalSettings({ model_sync_filter_rules: rules });
            }}
            placeholder="输入规则后按回车，如 :free 或 -free"
          />
          <p className="text-xs md:text-sm text-muted-foreground">
            只同步包含这些规则的模型。点击标签上的 × 可删除，按 Backspace 删除最后一个
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
