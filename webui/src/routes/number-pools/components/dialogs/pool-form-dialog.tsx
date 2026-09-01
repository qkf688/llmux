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
import type { PoolListItem } from "@/lib/api";

const poolFormSchema = z.object({
  name: z.string().min(1, { message: "号池名称不能为空" }),
  note: z.string().optional(),
});

type PoolFormValues = z.infer<typeof poolFormSchema>;

type PoolFormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** null = 新建 */
  pool: PoolListItem | null;
  onSaved: (values: { name: string; note?: string }) => void | Promise<void>;
  isSaving?: boolean;
};

export function PoolFormDialog({
  open,
  onOpenChange,
  pool,
  onSaved,
  isSaving = false,
}: PoolFormDialogProps) {
  const form = useForm<PoolFormValues>({
    resolver: zodResolver(poolFormSchema),
    defaultValues: { name: "", note: "" },
  });

  useEffect(() => {
    if (open) {
      form.reset({ name: pool?.Name ?? "", note: pool?.Note ?? "" });
    }
  }, [open, pool, form]);

  const onSubmit = (values: PoolFormValues) => {
    void onSaved({
      name: values.name.trim(),
      note: values.note?.trim() || undefined,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{pool ? "编辑号池" : "新建号池"}</DialogTitle>
          <DialogDescription>
            {pool ? "修改号池信息（凭据管理在详情内进行）" : "号池是凭据集合；创建后可批量导入 Key"}
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
                      <Input {...field} placeholder="如：OpenAI 主池" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="note"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>备注（可选）</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="用途 / 负责人等" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </DialogBody>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
                取消
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? "保存中…" : pool ? "更新" : "创建"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
