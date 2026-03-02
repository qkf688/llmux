import { Button } from "@/components/ui/button";

interface VirtualModelsHeaderProps {
  onCreate: () => void;
}

export function VirtualModelsHeader({ onCreate }: VirtualModelsHeaderProps) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <h1 className="text-2xl font-bold">虚拟模型管理</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          将多个真实模型组合成虚拟模型，支持多种路由策略
        </p>
      </div>
      <Button onClick={onCreate}>创建虚拟模型</Button>
    </div>
  );
}
