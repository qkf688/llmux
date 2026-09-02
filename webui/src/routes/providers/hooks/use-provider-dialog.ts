import { useEffect, useRef } from "react";
import { toast } from "sonner";
import type { UseFormReturn } from "react-hook-form";
import type { Provider } from "@/lib/api";
import { getProvider } from "@/lib/api";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";
import { detailToFormValues } from "../utils/config";

type UseProviderDialogInput = {
  /** 弹窗开合（store 驱动）：关闭时作废在飞的详情请求 */
  open: boolean;
  form: UseFormReturn<ProviderFormValues>;
  setOpen: (open: boolean) => void;
  setEditingProvider: (provider: Provider | null) => void;
  setDetailLoading: (loading: boolean) => void;
};

export function useProviderDialog({ open, form, setOpen, setEditingProvider, setDetailLoading }: UseProviderDialogInput) {
  // 守卫 token：单调递增 id，发起时捕获、响应侧与最新值比对，不匹配则整轮作废
  // （不回填、不 toast、不关弹窗、不动 loading）。id 变化来源＝用户最新意图：另一次
  // 打开（编辑/新建）或弹窗关闭。用 id 而非 provider 引用比较——同引用双击、列表
  // 刷新后引用更换都在守卫语义内，且 token 语义与 editingProvider 生命周期解耦。
  const requestIdRef = useRef(0);
  const prevOpenRef = useRef(open);

  // 弹窗关闭（含取消/X/失败自动关）：作废在飞请求。没有这一步时，晚到的失败会在
  // 弹窗关闭后弹 toast、晚到的成功会回填已关闭的表单，且 detailLoading 只能靠
  // 晚到响应隐式复位——请求永挂则 store 残留 true。
  useEffect(() => {
    if (prevOpenRef.current && !open) {
      requestIdRef.current += 1;
      setDetailLoading(false);
    }
    prevOpenRef.current = open;
  }, [open, setDetailLoading]);

  /**
   * 打开编辑弹窗并异步加载详情回填。
   * 列表项只带端点/分组计数（GET /providers），不含展开的 Endpoints/Groups，
   * 因此编辑必须按 id 拉 GET /providers/:id 详情，再用 detailToFormValues 映射回表单。
   */
  const openEditDialog = async (provider: Provider) => {
    const requestId = ++requestIdRef.current;
    setEditingProvider(provider);
    form.reset({ ...defaultProviderFormValues });
    setOpen(true);
    setDetailLoading(true);

    // 成功/失败归一为 outcome，守卫单点判断后再分发——两条路径的作废语义必须一致
    const outcome = await getProvider(provider.ID)
      .then((detail) => ({ ok: true as const, detail }))
      .catch((err: unknown) => ({ ok: false as const, err }));

    if (requestIdRef.current !== requestId) {
      return;
    }
    setDetailLoading(false);

    if (outcome.ok) {
      form.reset(detailToFormValues(outcome.detail));
      return;
    }
    const message = outcome.err instanceof Error ? outcome.err.message : String(outcome.err);
    toast.error(`加载供应商详情失败: ${message}`);
    console.error(outcome.err);
    setOpen(false);
    setEditingProvider(null);
  };

  const openCreateDialog = () => {
    // 新建取代任何在飞的编辑请求：晚到的旧详情响应会被守卫丢弃
    requestIdRef.current += 1;
    setEditingProvider(null);
    setDetailLoading(false);
    // 默认预勾选 OpenAI（defaultProviderFormValues.protocols），type 必须同步，否则 zod 校验静默拦截且错误无处渲染
    form.reset({ ...defaultProviderFormValues, type: "openai" });
    setOpen(true);
  };

  return { openEditDialog, openCreateDialog };
}
