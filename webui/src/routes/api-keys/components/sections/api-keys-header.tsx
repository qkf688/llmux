import { Key, Plus } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

type ApiKeysHeaderProps = {
  /** 当前 key 数量，用于页头副标题 */
  keyCount: number;
  onCreate: () => void;
};

/**
 * API Keys 页头。文案口径：本页管理「调用方 → 网关」的 /v1 访问密钥，
 * 与号池的上游凭据 key 是相反方向的两域（方案 §2），措辞不共用「凭据」。
 */
export function ApiKeysHeader({ keyCount, onCreate }: ApiKeysHeaderProps) {
  return (
    <PageHeader
      icon={Key}
      title="API Keys"
      subtitle={`共 ${keyCount} 把 · 调用方访问 /v1 接口的密钥`}
      actions={
        <Button size="sm" onClick={onCreate}>
          <Plus className="size-4" />
          新建 Key
        </Button>
      }
    />
  );
}
