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
};

export function useProviderDialog({ form, setOpen, setEditingProvider }: UseProviderDialogInput) {
  /**
   * 打开编辑弹窗并异步加载详情回填。
   * 列表项只带端点/分组计数（GET /providers），不含展开的 Endpoints/Groups，
   * 因此编辑必须按 id 拉 GET /providers/:id 详情，再用 detailToFormValues 映射回表单。
   */
  const openEditDialog = async (provider: Provider) => {
    setEditingProvider(provider);
    form.reset({ ...defaultProviderFormValues });
    setOpen(true);

    try {
      const detail = await getProvider(provider.ID);
      form.reset(detailToFormValues(detail));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`加载供应商详情失败: ${message}`);
      console.error(err);
      setOpen(false);
      setEditingProvider(null);
    }
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    // 默认预勾选 OpenAI（defaultProviderFormValues.protocols），type 必须同步，否则 zod 校验静默拦截且错误无处渲染
    form.reset({ ...defaultProviderFormValues, type: "openai" });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}