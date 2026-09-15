import { useEffect, useState } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { AlertTriangle, Copy, Eye, EyeOff, Info, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { copySecretWithToast } from "@/lib/clipboard";
import type { ApiKeyItem } from "@/lib/api";
import { CODEX_HEADER_PRESET, CLAUDE_CODE_HEADER_PRESET } from "../../constants/header-presets";
import { headersToRows, mergePresetRows, rowsToHeaders } from "../../utils/header-fields";

/** handler/apikeys 解密失败降级值（与 card-list / credentialcrypto.MaskStored 同口径） */
const MASKED_FALLBACK = "****";

/**
 * 调用方 key 前缀：仅用于创建表单的展示拼接（octopus 式内嵌前缀输入框），
 * 校验/补全的单一来源仍是后端 NormalizeCallerKeyValue——用户粘贴含前缀的值
 * 在 onChange 被剥成后缀，提交时统一拼回，杜绝「sk-llmux- sk-llmux-…」双前缀。
 */
const CALLER_KEY_PREFIX = "sk-llmux-";

function stripKeyPrefix(input: string): string {
  return input.startsWith(CALLER_KEY_PREFIX)
    ? input.slice(CALLER_KEY_PREFIX.length)
    : input;
}

/**
 * 掩码短形：合法 key 最短 16，对短值做「前12…后4」截断几乎不遮，
 * 故 <24 一律全遮——掩码态 DOM 永不出现完整明文（沿已删除 key-cell 的纪律）。
 */
function maskedForm(plain: string): string {
  if (plain.length < 24) return "****";
  return `${plain.slice(0, 12)}…${plain.slice(-4)}`;
}

// 弹窗内复制失败的兜底：明文可在本弹窗内显式展开后手动选中，引导自包含不指向别处
const DIALOG_COPY_FAILURE_HINT =
  "复制失败：浏览器限制了自动复制。可点「显示明文」后手动选中复制（建议使用 HTTPS/域名访问）。";

// key 值规则（补前缀、≥16）单一来源在后端 NormalizeCallerKeyValue——本地只钉
// 「名称必填（先 trim 再判，防空格名绕过）」，不复制长度校验（AGENTS：勿扩大前端硬编码缺口）。
// headers 是编辑行数组（键值对可增删改）；提交前经 rowsToHeaders 折成 Record，
// 校验只到形状层（空名行由转换跳过，不当错误——留半截输入不打断保存）。
const apiKeyFormSchema = z.object({
  name: z.string().trim().min(1, { message: "名称不能为空" }),
  value: z.string().trim().optional(),
  headers: z.array(z.object({ key: z.string(), value: z.string() })),
});

type ApiKeyFormValues = z.infer<typeof apiKeyFormSchema>;

type ApiKeyFormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** null = 新建；非 null = 编辑（改名 + 查看/复制明文 + key 级头——key 值不可改，轮换 = 新建 + 停用旧） */
  apiKey: ApiKeyItem | null;
  onSaved: (values: { name: string; value?: string; headers?: Record<string, string> }) => void | Promise<void>;
  isSaving?: boolean;
};

export function ApiKeyFormDialog({
  open,
  onOpenChange,
  apiKey,
  onSaved,
  isSaving = false,
}: ApiKeyFormDialogProps) {
  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(apiKeyFormSchema),
    defaultValues: { name: "", value: "", headers: [] },
  });
  const headerRows = useFieldArray({ control: form.control, name: "headers" });
  // 明文显示态是弹窗本地一次性状态：每次打开回落掩码（明文只在显式动作后可见）
  const [revealed, setRevealed] = useState(false);

  useEffect(() => {
    if (open) {
      form.reset({
        name: apiKey?.name ?? "",
        value: "",
        // 编辑回显既有 headers（round-trip 读侧）；新建从空表起
        headers: headersToRows(apiKey?.headers),
      });
      setRevealed(false);
    }
  }, [open, apiKey, form]);

  const applyPreset = (preset: typeof CODEX_HEADER_PRESET) => {
    headerRows.replace(mergePresetRows(form.getValues("headers"), preset));
  };

  const onSubmit = (values: ApiKeyFormValues) => {
    const suffix = stripKeyPrefix((values.value ?? "").trim());
    void onSaved({
      name: values.name.trim(),
      // 编辑模式无 value 字段（后端 PATCH 也不接受改值），undefined 保证不带上；
      // 创建模式输入框只存后缀，拼回前缀提交（留空 = undefined，交后端自动生成）
      value: apiKey ? undefined : suffix ? `${CALLER_KEY_PREFIX}${suffix}` : undefined,
      // 表单即真相：弹窗恒提交 headers 键（空区折成 {}=清空，Rulings 5）
      headers: rowsToHeaders(values.headers),
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{apiKey ? "编辑 API Key" : "新建 API Key"}</DialogTitle>
          <DialogDescription>
            {apiKey
              ? "改名与查看/复制密钥值；key 值不可修改，轮换 = 新建新 key + 停用/删除旧 key"
              : "调用方持此 key 访问 /v1 接口；与号池的上游凭据无关"}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col gap-4">
            <DialogBody className="-mx-1 min-w-0 space-y-4 px-1">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="如：Codex 专用" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              {apiKey && (
                <div className="space-y-1.5">
                  <FormLabel>Key 值</FormLabel>
                  {apiKey.api_key === MASKED_FALLBACK ? (
                    <div className="flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-xs text-amber-600 dark:text-amber-400">
                      <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                      <span>
                        明文不可用：该行解密失败，无法显示或复制。请新建 key 并停用/删除本条。
                      </span>
                    </div>
                  ) : (
                    <>
                      {/* 明文容器定宽换行：弹窗宽度固定，不存在行内展开撑破布局的问题 */}
                      <div className="min-w-0 rounded-lg bg-muted/40 px-3 py-2">
                        {revealed ? (
                          <span
                            translate="no"
                            className="block break-all font-mono text-sm select-all"
                          >
                            {apiKey.api_key}
                          </span>
                        ) : (
                          <span className="block truncate font-mono text-sm">
                            {maskedForm(apiKey.api_key)}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1">
                        {/* 纯图标按钮：语义走 aria-label（沿密集操作区不设 title 的决定） */}
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 text-muted-foreground hover:bg-muted hover:text-foreground"
                          onClick={() => setRevealed((current) => !current)}
                          aria-label={revealed ? "隐藏明文" : "显示明文"}
                        >
                          {revealed ? (
                            <EyeOff className="size-4" />
                          ) : (
                            <Eye className="size-4" />
                          )}
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 text-muted-foreground hover:bg-muted hover:text-foreground"
                          onClick={() =>
                            void copySecretWithToast(
                              apiKey.api_key,
                              `已复制密钥 ${apiKey.name}`,
                              DIALOG_COPY_FAILURE_HINT,
                            )
                          }
                          aria-label={`复制密钥 ${apiKey.name}`}
                        >
                          <Copy className="size-4" />
                        </Button>
                      </div>
                    </>
                  )}
                </div>
              )}
              {!apiKey && (
                <FormField
                  control={form.control}
                  name="value"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        <span className="flex items-center gap-1.5">
                          自定义密钥
                          <span className="text-[10px] font-normal text-muted-foreground/70">
                            （可选）
                          </span>
                          {/* 规则提示收进 ⓘ（octopus 版式）：说明文字不再占输入框下方一行 */}
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                className="inline-flex size-4 cursor-help items-center justify-center text-muted-foreground hover:text-foreground"
                                aria-label="密钥值规则说明"
                              >
                                <Info className="size-3.5" />
                              </button>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              留空自动生成 sk-llmux- + 32 位随机 hex；自定义值缺
                              sk-llmux- 前缀会自动补全，补全后总长须 ≥16 字符
                            </TooltipContent>
                          </Tooltip>
                        </span>
                      </FormLabel>
                      {/* octopus 式内嵌前缀输入框：前缀是展示件不可编辑，框内只存后缀 */}
                      <div className="flex items-center overflow-hidden rounded-md border border-input bg-muted/20 transition-colors focus-within:border-primary/40">
                        <span className="shrink-0 select-none px-3 font-mono text-sm text-muted-foreground">
                          {CALLER_KEY_PREFIX}
                        </span>
                        <FormControl>
                          <Input
                            {...field}
                            value={field.value ?? ""}
                            onChange={(event) =>
                              field.onChange(stripKeyPrefix(event.target.value))
                            }
                            placeholder="留空则自动生成"
                            autoComplete="off"
                            className="rounded-none border-0 bg-transparent shadow-none focus-visible:ring-0"
                          />
                        </FormControl>
                      </div>
                      {/* 规则说明只保留在 ⓘ tooltip 一处（octopus 版式），不再于框下重复占行 */}
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
              {/* #6 key 级自定义出站头：键值行编辑 + 一键预设（创建/编辑两模式同现，Rulings 1） */}
              <div className="space-y-3 rounded-md border p-4">
                <div className="flex items-center justify-between gap-2">
                  <FormLabel className="text-sm">自定义出站头</FormLabel>
                  <div className="flex gap-2">
                    <Button type="button" variant="outline" size="sm" onClick={() => applyPreset(CODEX_HEADER_PRESET)}>
                      填充 Codex 预设
                    </Button>
                    <Button type="button" variant="outline" size="sm" onClick={() => applyPreset(CLAUDE_CODE_HEADER_PRESET)}>
                      填充 Claude Code 预设
                    </Button>
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">
                  网关转发 /v1 请求时为该 key 注入下列头（伪装 Codex/Claude Code 客户端头族）。
                  不参与诊断/连通测试，验证靠真实流量与日志归因；撞同名时关联级自定义头优先，
                  Authorization 恒被供应商凭据覆写。
                </p>
                {headerRows.fields.map((rowField, index) => (
                  <div key={rowField.id} className="flex items-center gap-2">
                    <Input
                      placeholder="Header 名"
                      className="min-w-0 flex-1 font-mono text-sm"
                      {...form.register(`headers.${index}.key` as const)}
                    />
                    <Input
                      placeholder="值"
                      className="min-w-0 flex-1 font-mono text-sm"
                      {...form.register(`headers.${index}.value` as const)}
                    />
                    {/* 纯图标按钮：语义走 aria-label（沿密集操作区惯例） */}
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 shrink-0 text-muted-foreground hover:bg-muted hover:text-foreground"
                      onClick={() => headerRows.remove(index)}
                      aria-label="删除出站头"
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ))}
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => headerRows.append({ key: "", value: "" })}
                >
                  <Plus className="size-4" />
                  添加出站头
                </Button>
              </div>
            </DialogBody>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
                取消
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? "保存中…" : apiKey ? "保存" : "创建"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
