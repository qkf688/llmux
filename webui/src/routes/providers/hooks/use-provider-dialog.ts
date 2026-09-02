import { useRef } from "react";
import { toast } from "sonner";
import type { UseFormReturn } from "react-hook-form";
import type { Provider } from "@/lib/api";
import { getProvider } from "@/lib/api";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { detailToFormValues } from "../utils/config";

type UseProviderDialogInput = {
  form: UseFormReturn<ProviderFormValues>;
  setOpen: (open: boolean) => void;
  setEditingProvider: (provider: Provider | null) => void;
  setDetailLoading: (loading: boolean) => void;
};

export function useProviderDialog({ form, setOpen, setEditingProvider, setDetailLoading }: UseProviderDialogInput) {
  // 最新打开的编辑目标：await 返回后若已被另一次打开（编辑其他供应商 / 新建）取代，
  // 本轮响应整体作废——不回填、不关弹窗、不动 loading（loading 归最新发起者所有）。
  // 没有该守卫时，晚到的旧响应会把 A 的数据回填进 B 的表单（提交走全量 DTO，属数据破坏）。
  const latestEditRef = useRef<Provider | null>(null);

  /**
   * 打开编辑弹窗并异步加载详情回填。
   * 列表项只带端点/分组计数（GET /providers），不含展开的 Endpoints/Groups，
   * 因此编辑必须按 id 拉 GET /providers/:id 详情，再用 detailToFormValues 映射回表单。
   */
  const openEditDialog = async (provider: Provider) => {
    latestEditRef.current = provider;
    setEditingProvider(provider);
    form.reset({ ...defaultProviderFormValues });
    setOpen(true);
    setDetailLoading(true);

    try {
      const detail = await getProvider(provider.ID);
      if (latestEditRef.current !== provider) {
        return;
      }
      form.reset(detailToFormValues(detail));
      setDetailLoading(false);
    } catch (err) {
      if (latestEditRef.current !== provider) {
        return;
      }
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`加载供应商详情失败: ${message}`);
      console.error(err);
      setOpen(false);
      setEditingProvider(null);
      setDetailLoading(false);
    }
  };

  const openCreateDialog = () => {
    // 新建取代任何在飞的编辑请求：晚到的旧详情响应会被守卫丢弃
    latestEditRef.current = null;
    setEditingProvider(null);
    setDetailLoading(false);
    // 默认预勾选 OpenAI（defaultProviderFormValues.protocols），type 必须同步，否则 zod 校验静默拦截且错误无处渲染
    form.reset({ ...defaultProviderFormValues, type: "openai" });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}
