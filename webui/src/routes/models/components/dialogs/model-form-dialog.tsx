import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import type { Model, Provider } from "@/lib/api";
import type { UseFormReturn } from "react-hook-form";
import type { ModelFormValues } from "../../schemas/forms";

interface ModelFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingModel: Model | null;
  form: UseFormReturn<ModelFormValues>;
  providers: Provider[];
  selectedProviderId: string;
  onSelectedProviderIdChange: (value: string) => void;
  loadingProviderModels: boolean;
  hasProviderModels: boolean;
  onOpenModelPicker: () => void;
  onCreate: (values: ModelFormValues) => void | Promise<void>;
  onUpdate: (values: ModelFormValues) => void | Promise<void>;
}

export function ModelFormDialog({
  open,
  onOpenChange,
  editingModel,
  form,
  providers,
  selectedProviderId,
  onSelectedProviderIdChange,
  loadingProviderModels,
  hasProviderModels,
  onOpenModelPicker,
  onCreate,
  onUpdate,
}: ModelFormDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editingModel ? "编辑模型" : "添加模型"}</DialogTitle>
          <DialogDescription>{editingModel ? "修改模型信息" : "添加一个新的模型"}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(editingModel ? onUpdate : onCreate)} className="space-y-4">
            {!editingModel && (
              <div className="space-y-2">
                <label className="text-sm font-medium">从提供商选择（默认全部）</label>
                <div className="flex gap-2">
                  <Select value={selectedProviderId} onValueChange={onSelectedProviderIdChange}>
                    <SelectTrigger className="flex-1">
                      <SelectValue placeholder="选择供应商" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">全部</SelectItem>
                      {providers.map((provider) => (
                        <SelectItem key={provider.ID} value={provider.ID.toString()}>
                          {provider.Name} ({provider.Type})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={onOpenModelPicker}
                    disabled={loadingProviderModels || !hasProviderModels}
                  >
                    {loadingProviderModels ? (
                      <>
                        <Spinner className="h-4 w-4 mr-2" />
                        加载中
                      </>
                    ) : (
                      "选择模型"
                    )}
                  </Button>
                </div>
              </div>
            )}

            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="remark"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={3} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid grid-cols-2 gap-4">
              <FormField
                control={form.control}
                name="max_retry"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>重试次数限制</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        {...field}
                        onChange={(event) => field.onChange(Number(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="time_out"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>超时时间(秒)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        {...field}
                        onChange={(event) => field.onChange(Number(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name="io_log"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                  <div className="space-y-0.5">
                    <FormLabel className="text-base">IO 记录</FormLabel>
                    <div className="text-sm text-muted-foreground">是否记录输入输出日志</div>
                  </div>
                  <FormControl>
                    <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="auto_associate"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                  <div className="space-y-0.5">
                    <FormLabel className="text-base">自动关联</FormLabel>
                    <div className="text-sm text-muted-foreground">是否允许该模型被自动关联触发</div>
                  </div>
                  <FormControl>
                    <Checkbox
                      checked={field.value !== false}
                      onCheckedChange={(checked) => field.onChange(checked)}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit">{editingModel ? "更新" : "创建"}</Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
