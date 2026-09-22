import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { RoutingSettingsSectionProps } from "../../types";

export function CacheAffinityCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>缓存亲和</CardTitle>
        <CardDescription>同一对话优先走同一供应商密钥，提高提示词缓存命中率</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="cache-affinity-enabled" className="text-sm md:text-base font-medium">
              启用缓存亲和
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，同一对话的请求会优先钉住上次成功的模型、供应商、分组与密钥（供应商的提示词缓存按账号分槽，
              命中可大幅降低输入成本）；钉住的目标失效时自动转移，转移成功后改钉新目标。
              <br />
              关闭后，恢复按权重与轮询随机分配供应商密钥。
            </p>
          </div>
          <Switch
            id="cache-affinity-enabled"
            checked={localSettings?.cache_affinity_enabled ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ cache_affinity_enabled: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}
