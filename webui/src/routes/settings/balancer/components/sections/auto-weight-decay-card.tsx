import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { BalancerSettingsSectionProps } from "../../types";

export function AutoWeightDecayCard({ localSettings, updateLocalSettings }: BalancerSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>自动权重衰减</CardTitle>
        <CardDescription>配置调用失败时自动降低供应商权重的行为</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="auto-weight-decay" className="text-sm md:text-base font-medium">
              启用自动权重衰减
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，每次调用失败时，系统会自动减少对应供应商关联的权重。
            </p>
          </div>
          <Switch
            id="auto-weight-decay"
            checked={localSettings?.auto_weight_decay ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ auto_weight_decay: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-weight-decay-default" className="text-sm md:text-base font-medium">
            默认权重值
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">
            重置权重时使用的默认值，也是新创建关联的推荐权重值。
          </p>
          <Input
            id="auto-weight-decay-default"
            type="number"
            min={1}
            max={1000}
            value={localSettings?.auto_weight_decay_default ?? 100}
            onChange={(e) => {
              updateLocalSettings({ auto_weight_decay_default: parseInt(e.target.value) || 100 });
            }}
            className="w-32"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-weight-decay-step" className="text-sm md:text-base font-medium">
            衰减步长
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">每次调用失败时减少的权重值。</p>
          <Input
            id="auto-weight-decay-step"
            type="number"
            min={1}
            max={100}
            value={localSettings?.auto_weight_decay_step ?? 1}
            onChange={(e) => {
              updateLocalSettings({ auto_weight_decay_step: parseInt(e.target.value) || 1 });
            }}
            className="w-32"
          />
        </div>
      </CardContent>
    </Card>
  );
}

