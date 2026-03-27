import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";

interface BalancerSettingsHeaderProps {
  saving: boolean;
  hasChanges: boolean;
  onReset: () => void;
  onSave: () => void;
}

export function BalancerSettingsHeader({
  saving,
  hasChanges,
  onReset,
  onSave,
}: BalancerSettingsHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2">
      <div>
        <h2 className="text-lg md:text-xl font-semibold">负载均衡</h2>
        <p className="text-xs md:text-sm text-muted-foreground">配置权重、优先级和自动调整策略</p>
      </div>
      <div className="flex gap-1.5 md:gap-2">
        <Button variant="outline" onClick={onReset} disabled={!hasChanges || saving}>
          重置
        </Button>
        <Button onClick={onSave} disabled={!hasChanges || saving}>
          {saving ? <Spinner className="w-4 h-4 mr-2" /> : null}
          保存
        </Button>
      </div>
    </div>
  );
}

