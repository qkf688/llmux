import { KeyRound, Plus } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

type PoolsHeaderProps = {
  /** 手建号池数量，用于页头副标题 */
  poolCount: number;
  /** 专属号池（供应商分组「添加 Key」自动建出）数量 */
  autoPoolCount: number;
  onCreate: () => void;
};

/**
 * 号池页头：标题栏 + 新建按钮，统一走 PageHeader 配方（与其他管理页一致）。
 * 副标题把手建池与专属池分开计数——两类池的来路不同，混在一个数字里看不出比例。
 */
export function PoolsHeader({ poolCount, autoPoolCount, onCreate }: PoolsHeaderProps) {
  return (
    <PageHeader
      icon={KeyRound}
      title="号池管理"
      subtitle={`共 ${poolCount + autoPoolCount} 个号池（手建 ${poolCount} · 专属 ${autoPoolCount}）`}
      actions={
        <Button size="sm" onClick={onCreate}>
          <Plus className="size-4" />
          新建号池
        </Button>
      }
    />
  );
}
