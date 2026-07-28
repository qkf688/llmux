import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { BalancerSettingsSectionProps } from "../../types";

export function AutoPriorityDecayCard({ localSettings, updateLocalSettings }: BalancerSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>自动优先级衰减</CardTitle>
        <CardDescription>配置调用失败时自动降低供应商优先级的行为</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="auto-priority-decay" className="text-sm md:text-base font-medium">
              启用自动优先级衰减
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，每次调用失败时，系统会自动减少对应供应商关联的优先级。
            </p>
          </div>
          <Switch
            id="auto-priority-decay"
            checked={localSettings?.auto_priority_decay ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ auto_priority_decay: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-priority-decay-default" className="text-sm md:text-base font-medium">
            默认优先级值
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">
            新创建关联的默认优先级值，也是重置优先级时使用的值。
          </p>
          <Input
            id="auto-priority-decay-default"
            type="number"
            min={1}
            max={1000}
            value={localSettings?.auto_priority_decay_default ?? 100}
            onChange={(e) => {
              updateLocalSettings({ auto_priority_decay_default: parseInt(e.target.value) || 100 });
            }}
            className="w-32"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="auto-priority-decay-step" className="text-sm md:text-base font-medium">
            衰减步长
          </Label>
          <p className="text-xs md:text-sm text-muted-foreground">每次调用失败时减少的优先级值。</p>
          <Input
            id="auto-priority-decay-step"
            type="number"
            min={1}
            max={100}
            value={localSettings?.auto_priority_decay_step ?? 1}
            onChange={(e) => {
              updateLocalSettings({ auto_priority_decay_step: parseInt(e.target.value) || 1 });
            }}
            className="w-32"
          />
        </div>

        <div className="space-y-3 md:space-y-4">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="auto-priority-decay-disable-enabled" className="text-sm md:text-base font-medium">
                启用自动禁用
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，当优先级降到禁用阈值时，自动禁用该供应商关联。
              </p>
            </div>
            <Switch
              id="auto-priority-decay-disable-enabled"
              checked={localSettings?.auto_priority_decay_disable_enabled ?? true}
              onCheckedChange={(checked) => {
                updateLocalSettings({ auto_priority_decay_disable_enabled: checked });
              }}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="auto-priority-decay-threshold" className="text-sm md:text-base font-medium">
              禁用阈值
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              当优先级降到此值或以下时的处理策略，由上方开关控制是否自动禁用。
            </p>
            <Input
              id="auto-priority-decay-threshold"
              type="number"
              min={0}
              max={100}
              value={localSettings?.auto_priority_decay_threshold ?? 90}
              onChange={(e) => {
                updateLocalSettings({ auto_priority_decay_threshold: parseInt(e.target.value) || 90 });
              }}
              className="w-32"
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

