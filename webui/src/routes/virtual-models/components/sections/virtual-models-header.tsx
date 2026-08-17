import { Layers } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

interface VirtualModelsHeaderProps {
  onCreate: () => void;
}

export function VirtualModelsHeader({ onCreate }: VirtualModelsHeaderProps) {
  return (
    <PageHeader
      icon={Layers}
      title="虚拟模型管理"
      subtitle="将多个真实模型组合成虚拟模型，支持多种路由策略"
      actions={<Button onClick={onCreate}>创建虚拟模型</Button>}
    />
  );
}
