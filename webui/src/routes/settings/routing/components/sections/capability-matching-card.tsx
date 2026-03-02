import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { RoutingSettingsSectionProps } from "../../types";

export function CapabilityMatchingCard({
  localSettings,
  updateLocalSettings,
}: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>能力匹配</CardTitle>
        <CardDescription>配置请求路由时的能力匹配策略</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="strict-capability-match" className="text-sm md:text-base font-medium">
              严格能力匹配
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统会根据请求的能力需求（工具调用、结构化输出、图片处理）筛选供应商。
              <br />
              关闭后，系统将忽略能力匹配条件，允许请求发送到任何启用的供应商。
            </p>
          </div>
          <Switch
            id="strict-capability-match"
            checked={localSettings?.strict_capability_match ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ strict_capability_match: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}
