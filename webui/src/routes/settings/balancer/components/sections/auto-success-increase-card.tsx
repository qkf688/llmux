import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { BalancerSettingsSectionProps } from "../../types";

export function AutoSuccessIncreaseCard({ localSettings, updateLocalSettings }: BalancerSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>成功调用自增</CardTitle>
        <CardDescription>配置成功调用后自动提升权重与优先级的策略</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="auto-success-increase" className="text-sm md:text-base font-medium">
              启用成功自增
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              关闭后，成功调用不会自动提升权重或优先级。
            </p>
          </div>
          <Switch
            id="auto-success-increase"
            checked={localSettings?.auto_success_increase ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ auto_success_increase: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-weight-increase-step" className="text-sm md:text-base font-medium">
            权重增加步长
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">每次调用成功后增加的权重值。</p>
          <Input
            id="auto-weight-increase-step"
            type="number"
            min={1}
            max={100}
            value={localSettings?.auto_weight_increase_step ?? 1}
            onChange={(e) => {
              updateLocalSettings({ auto_weight_increase_step: parseInt(e.target.value) || 1 });
            }}
            className="w-32"
            disabled={!localSettings?.auto_success_increase}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-weight-increase-max" className="text-sm md:text-base font-medium">
            权重增加上限
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">成功自增后的最大权重值，防止无限增长。</p>
          <Input
            id="auto-weight-increase-max"
            type="number"
            min={1}
            max={10000}
            value={localSettings?.auto_weight_increase_max ?? 5}
            onChange={(e) => {
              updateLocalSettings({ auto_weight_increase_max: parseInt(e.target.value) || 5 });
            }}
            className="w-32"
            disabled={!localSettings?.auto_success_increase}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-priority-increase-step" className="text-sm md:text-base font-medium">
            优先级增加步长
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">每次调用成功后增加的优先级值。</p>
          <Input
            id="auto-priority-increase-step"
            type="number"
            min={1}
            max={100}
            value={localSettings?.auto_priority_increase_step ?? 1}
            onChange={(e) => {
              updateLocalSettings({ auto_priority_increase_step: parseInt(e.target.value) || 1 });
            }}
            className="w-32"
            disabled={!localSettings?.auto_success_increase}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-priority-increase-max" className="text-sm md:text-base font-medium">
            优先级增加上限
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">成功自增后的最大优先级值。</p>
          <Input
            id="auto-priority-increase-max"
            type="number"
            min={0}
            max={10000}
            value={localSettings?.auto_priority_increase_max ?? 10}
            onChange={(e) => {
              updateLocalSettings({ auto_priority_increase_max: parseInt(e.target.value) || 10 });
            }}
            className="w-32"
            disabled={!localSettings?.auto_success_increase}
          />
        </div>
      </CardContent>
    </Card>
  );
}

