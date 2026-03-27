import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { LogsSettingsSectionProps } from "../../types";
import { RawRequestResponseSection } from "./raw-request-response-section";

export function LogsConfigurationCard({ localSettings, updateLocalSettings }: LogsSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>日志配置</CardTitle>
        <CardDescription>管理系统日志的保留和记录策略</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="space-y-1.5">
          <Label htmlFor="log-retention-count" className="text-sm md:text-base font-medium">
            日志保留条数
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">
            系统自动保留的最新日志条数。设置为 0 表示不限制。
            <br />
            修改此设置后，超出保留条数的旧日志将被自动删除。
          </p>
          <Input
            id="log-retention-count"
            type="number"
            min={0}
            max={100000}
            value={localSettings?.log_retention_count ?? 100}
            onChange={(e) => {
              updateLocalSettings({ log_retention_count: parseInt(e.target.value) || 0 });
            }}
            className="w-32"
          />
        </div>

        <RawRequestResponseSection
          localSettings={localSettings}
          updateLocalSettings={updateLocalSettings}
        />

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label className="text-sm md:text-base font-medium" htmlFor="disable-all-logs">
              完全关闭日志记录
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统将不记录任何请求日志，可大幅提升性能（提升100-200%）。
              <br />
              建议仅在极致性能要求下使用，关闭后无法在界面查看请求历史。
            </p>
          </div>
          <Switch
            id="disable-all-logs"
            checked={localSettings?.disable_all_logs ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ disable_all_logs: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

