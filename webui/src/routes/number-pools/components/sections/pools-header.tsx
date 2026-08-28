import { Plus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type PoolsHeaderProps = {
  onCreate: () => void;
};

export function PoolsHeader({ onCreate }: PoolsHeaderProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold tracking-tight">号池</h1>
          <Badge variant="outline" className="text-[10px]">
            S0 原型 · Mock 数据
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">
          凭据池统一管理：批量导入 / 冷却与错误状态可视化 / 分组引用（设计定案 · 号池维度）
        </p>
      </div>
      <Button onClick={onCreate}>
        <Plus className="size-4" />
        新建号池
      </Button>
    </div>
  );
}