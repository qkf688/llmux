import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
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
import type { ApiKeyItem } from "@/lib/api";

// key 值规则（补前缀、≥16）单一来源在后端 NormalizeCallerKeyValue——本地只钉
// 「名称必填（先 trim 再判，防空格名绕过）」，不复制长度校验（AGENTS：勿扩大前端硬编码缺口）
const apiKeyFormSchema = z.object({
  name: z.string().trim().min(1, { message: "名称不能为空" }),
  value: z.string().trim().optional(),
});

type ApiKeyFormValues = z.infer<typeof apiKeyFormSchema>;

type ApiKeyFormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** null = 新建；非 null = 编辑（仅改名——key 值不可改，轮换 = 新建 + 停用旧） */
  apiKey: ApiKeyItem | null;
  onSaved: (values: { name: string; value?: string }) => void | Promise<void>;
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
    defaultValues: { name: "", value: "" },
  });

  useEffect(() => {
    if (open) {
      form.reset({ name: apiKey?.name ?? "", value: "" });
    }
  }, [open, apiKey, form]);

  const onSubmit = (values: ApiKeyFormValues) => {
    void onSaved({
      name: values.name.trim(),
      // 编辑模式无 value 字段（后端 PATCH 也不接受改值），undefined 保证不带上
      value: apiKey ? undefined : values.value?.trim() || undefined,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{apiKey ? "重命名 API Key" : "新建 API Key"}</DialogTitle>
          <DialogDescription>
            {apiKey
              ? "仅可改名；key 值不可修改，轮换 = 新建新 key + 停用/删除旧 key"
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
              {!apiKey && (
                <FormField
                  control={form.control}
                  name="value"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Key 值（可选）</FormLabel>
                      <FormControl>
                        {/* autoComplete=off：防浏览器密码管理器把用户手填的机密 key 存进 profile（落盘、不受前端生命周期管辖，与 clipboard 禁用 prompt 同条理由链） */}
                        <Input {...field} placeholder="留空 = 系统自动生成" autoComplete="off" />
                      </FormControl>
                      <p className="text-xs text-muted-foreground">
                        自定义值缺 sk-llmux- 前缀会自动补全，补全后总长须 ≥16 字符
                      </p>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
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
