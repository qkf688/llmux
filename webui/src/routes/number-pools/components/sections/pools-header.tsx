import { KeyRound, Plus } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

type PoolsHeaderProps = {
  /** 当前号池数量，用于页头副标题 */
  poolCount: number;
  onCreate: () => void;
};

/**
 * 号池页头：标题栏 + 新建按钮，统一走 PageHeader 配方（与其他管理页一致）。
 */
export function PoolsHeader({ poolCount, onCreate }: PoolsHeaderProps) {
  return (
    <PageHeader
      icon={KeyRound}
      title="号池管理"
      subtitle={`共 ${poolCount} 个号池`}
      actions={
        <Button size="sm" onClick={onCreate}>
          <Plus className="size-4" />
          新建号池
        </Button>
      }
    />
  );
}
