import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { THINKING_LEVEL_OPTIONS } from "@/lib/constants/thinking-levels";

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
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{editingModel ? "编辑模型" : "添加模型"}</DialogTitle>
          <DialogDescription>{editingModel ? "修改模型信息" : "添加一个新的模型"}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(editingModel ? onUpdate : onCreate)}
            className="flex min-h-0 flex-1 flex-col gap-4"
          >
            <DialogBody className="-mx-1 space-y-4 px-1">
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

            <div className="rounded-lg border p-4 space-y-3">
              <div className="space-y-0.5">
                <div className="text-base font-medium">行为开关</div>
                <div className="text-sm text-muted-foreground">
                  IO 记录：是否记录输入输出日志；自动关联：是否允许被自动关联触发；支持 thinking：关闭时上游请求中的 thinking/reasoning 字段会被裁剪
                </div>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <FormField
                  control={form.control}
                  name="io_log"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-center gap-2 space-y-0">
                      <FormControl>
                        <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                      </FormControl>
                      <FormLabel className="text-sm">IO 记录</FormLabel>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="auto_associate"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-center gap-2 space-y-0">
                      <FormControl>
                        <Checkbox
                          checked={field.value !== false}
                          onCheckedChange={(checked) => field.onChange(checked)}
                        />
                      </FormControl>
                      <FormLabel className="text-sm">自动关联</FormLabel>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="supports_thinking"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-center gap-2 space-y-0">
                      <FormControl>
                        <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                      </FormControl>
                      <FormLabel className="text-sm">支持 thinking</FormLabel>
                    </FormItem>
                  )}
                />
              </div>
            </div>

            <FormField
              control={form.control}
              name="thinking_levels"
              render={({ field }) => (
                <FormItem className="rounded-lg border p-4 space-y-3">
                  <div className="space-y-0.5">
                    <FormLabel className="text-base">思考档位白名单</FormLabel>
                    <div className="text-sm text-muted-foreground">
                      勾选允许的 reasoning_effort 档位；空列表=不约束（任意档位透传）。
                      不在白名单的档位会被就近钳制到白名单内最接近的档位
                    </div>
                  </div>
                  <FormControl>
                    <div className="flex flex-wrap gap-3 pt-1">
                      {THINKING_LEVEL_OPTIONS.map((opt) => {
                        const checked = (field.value ?? []).includes(opt.value);
                        return (
                          <label
                            key={opt.value}
                            className="flex items-center gap-2 cursor-pointer text-sm"
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={(c) => {
                                const current = field.value ? [...field.value] : [];
                                if (c) {
                                  current.push(opt.value);
                                } else {
                                  const idx = current.indexOf(opt.value);
                                  if (idx >= 0) current.splice(idx, 1);
                                }
                                field.onChange(current);
                              }}
                            />
                            {opt.label}
                          </label>
                        );
                      })}
                    </div>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            </DialogBody>

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
