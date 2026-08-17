import { Link2 } from "lucide-react";

import { PageHeader } from "@/components/page-header";

type ModelProvidersHeaderProps = {
  /** 当前筛选后的关联数量，用于页头副标题 */
  associationCount: number;
};

export function ModelProvidersHeader({ associationCount }: ModelProvidersHeaderProps) {
  return (
    <PageHeader
      icon={Link2}
      title="模型提供商关联"
      subtitle={`当前 ${associationCount} 条关联`}
    />
  );
}
