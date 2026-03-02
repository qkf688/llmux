import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { RoutingSettingsSectionProps } from "../../types";

export function FormatConversionCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>格式转换</CardTitle>
        <CardDescription>配置 API 格式转换功能</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="enable-format-conversion" className="text-sm md:text-base font-medium">
              启用格式转换
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统允许在不同 API 格式间转换（如 OpenAI ↔ Anthropic）。
              <br />
              关闭后只能使用与提供商类型匹配的格式，可减少转换开销，提升性能。
            </p>
          </div>
          <Switch
            id="enable-format-conversion"
            checked={localSettings?.enable_format_conversion ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ enable_format_conversion: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}
