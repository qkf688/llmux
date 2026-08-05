import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Settings } from "@/lib/api";
import type { RoutingSettingsSectionProps } from "../../types";

export function ParameterMappingCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>参数映射</CardTitle>
        <CardDescription>配置请求参数的自动映射与规范化</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="reasoning-effort-mapping-enabled" className="text-sm md:text-base font-medium">
              reasoning_effort 参数映射
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              自动归一化 reasoning_effort 参数，处理无效档位
            </p>
          </div>
          <Switch
            id="reasoning-effort-mapping-enabled"
            checked={localSettings?.reasoning_effort_mapping_enabled ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ reasoning_effort_mapping_enabled: checked });
            }}
          />
        </div>

        {localSettings?.reasoning_effort_mapping_enabled && (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="reasoning-effort-default-value" className="text-sm">
                默认值
              </Label>
              <Select
                value={localSettings.reasoning_effort_default_value}
                onValueChange={(value) => {
                  updateLocalSettings({
                    reasoning_effort_default_value: value as Settings["reasoning_effort_default_value"],
                  });
                }}
              >
                <SelectTrigger id="reasoning-effort-default-value">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="minimal">minimal（极低推理强度）</SelectItem>
                  <SelectItem value="low">low（低推理强度）</SelectItem>
                  <SelectItem value="medium">medium（中等推理强度）</SelectItem>
                  <SelectItem value="high">high（高推理强度）</SelectItem>
                  <SelectItem value="xhigh">xhigh（超高推理强度）</SelectItem>
                  <SelectItem value="max">max（最大推理强度）</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs md:text-sm text-muted-foreground">
                auto 兜底 / 未知档位 clamp_to_default 时使用的默认值
              </p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="reasoning-effort-unknown-strategy" className="text-sm">
                未知档位处理策略
              </Label>
              <Select
                value={localSettings.reasoning_effort_unknown_strategy ?? "clamp_to_default"}
                onValueChange={(value) => {
                  updateLocalSettings({
                    reasoning_effort_unknown_strategy: value as Settings["reasoning_effort_unknown_strategy"],
                  });
                }}
              >
                <SelectTrigger id="reasoning-effort-unknown-strategy">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="clamp_to_default">clamp_to_default（钳制到默认值）</SelectItem>
                  <SelectItem value="passthrough">passthrough（原样透传）</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs md:text-sm text-muted-foreground">
                请求中的 reasoning_effort 不在模型白名单时的处理方式
              </p>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
