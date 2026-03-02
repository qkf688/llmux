import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";

interface RoutingSettingsHeaderProps {
  saving: boolean;
  hasChanges: boolean;
  onReset: () => void;
  onSave: () => void;
}

export function RoutingSettingsHeader({
  saving,
  hasChanges,
  onReset,
  onSave,
}: RoutingSettingsHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2">
      <div>
        <h2 className="text-lg md:text-xl font-semibold">通用设置</h2>
        <p className="text-xs md:text-sm text-muted-foreground">配置系统通用选项</p>
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
