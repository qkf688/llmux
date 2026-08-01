import type { FieldArrayWithId, UseFormReturn } from "react-hook-form";
import type { Model, ModelWithProvider, Provider } from "@/lib/api";
import type { FormValues } from "../../form-schema";
import type { ProviderModelSelection } from "../../types";
import { buildSelectionKey } from "../../utils/selection";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

type AssociationFormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingAssociation: ModelWithProvider | null;
  form: UseFormReturn<FormValues>;
  models: Model[];
  providers: Provider[];
  selectedProviderModels: ProviderModelSelection[];
  isSubmitting: boolean;
  headerFields: FieldArrayWithId<FormValues, "customer_headers", "id">[];
  appendHeader: (value: { key: string; value: string }) => void;
  removeHeader: (index: number) => void;
  onSubmitCreate: (values: FormValues) => Promise<void>;
  onSubmitUpdate: (values: FormValues) => Promise<void>;
  onOpenModelListDialog: () => void;
  onClearSelectedProviderModels: () => void;
  onRemoveSelectedProviderModel: (selectionKey: string) => void;
  onProviderChange: () => void;
};

export function AssociationFormDialog({
  open,
  onOpenChange,
  editingAssociation,
  form,
  models,
  providers,
  selectedProviderModels,
  isSubmitting,
  headerFields,
  appendHeader,
  removeHeader,
  onSubmitCreate,
  onSubmitUpdate,
  onOpenModelListDialog,
  onClearSelectedProviderModels,
  onRemoveSelectedProviderModel,
  onProviderChange,
}: AssociationFormDialogProps) {
  const handleSubmit = editingAssociation ? onSubmitUpdate : onSubmitCreate;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{editingAssociation ? "编辑关联" : "添加关联"}</DialogTitle>
          <DialogDescription>
            {editingAssociation ? "修改模型提供商关联" : "添加一个新的模型提供商关联"}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-4 flex-1 min-h-0">
            <div className="space-y-4 overflow-y-auto pr-1 sm:pr-2 max-h-[60vh] flex-1 min-h-0">
              <FormField
                control={form.control}
                name="model_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>模型</FormLabel>
                    <Select
                      value={field.value.toString()}
                      onValueChange={(value) => field.onChange(parseInt(value, 10))}
                      disabled={!!editingAssociation}
                    >
                      <FormControl>
                        <SelectTrigger className="form-select">
                          <SelectValue placeholder="选择模型" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {models.map((model) => (
                          <SelectItem key={model.ID} value={model.ID.toString()}>
                            {model.Name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="provider_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>提供商（默认全部）</FormLabel>
                    <Select
                      value={field.value && field.value > 0 ? field.value.toString() : "0"}
                      onValueChange={(value) => {
                        if (value === "0") {
                          field.onChange(0);
                          onProviderChange();
                          return;
                        }
                        field.onChange(parseInt(value, 10));
                        onProviderChange();
                      }}
                    >
                      <FormControl>
                        <SelectTrigger className="form-select">
                          <SelectValue placeholder="选择提供商" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="0">全部</SelectItem>
                        {providers.map((provider) => (
                          <SelectItem key={provider.ID} value={provider.ID.toString()}>
                            {provider.Name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormItem>
                <FormLabel>提供商模型</FormLabel>
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-muted-foreground">已选择 {selectedProviderModels.length} 个模型</span>
                    <div className="flex gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={onOpenModelListDialog}
                        disabled={providers.length === 0}
                      >
                        模型列表
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={onClearSelectedProviderModels}
                        disabled={selectedProviderModels.length === 0}
                      >
                        清空
                      </Button>
                    </div>
                  </div>
                  {selectedProviderModels.length > 0 && (
                    <div className="max-h-32 overflow-y-auto border rounded-md p-2 space-y-1">
                      {selectedProviderModels.map((selection) => {
                        const selectionKey = buildSelectionKey(selection.providerId, selection.modelId);
                        return (
                          <div
                            key={selectionKey}
                            className="flex items-center justify-between text-sm py-1 px-2 bg-muted/50 rounded"
                          >
                            <span className="truncate">
                              {selection.providerName} / {selection.modelId}
                            </span>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-5 w-5 p-0"
                              onClick={() => onRemoveSelectedProviderModel(selectionKey)}
                            >
                              ✕
                            </Button>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </FormItem>

              {selectedProviderModels.length === 0 && (
                <FormField
                  control={form.control}
                  name="provider_name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>手动输入模型名称</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder="输入提供商模型名称" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormLabel>模型能力</FormLabel>
              <FormField
                control={form.control}
                name="tool_call"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>工具调用</FormLabel>
                    </div>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="structured_output"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>结构化输出</FormLabel>
                    </div>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="image"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>视觉</FormLabel>
                    </div>
                  </FormItem>
                )}
              />

              <FormLabel>参数配置</FormLabel>
              <FormField
                control={form.control}
                name="with_header"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>请求头透传</FormLabel>
                    </div>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="customer_headers"
                render={({ field }) => {
                  const headerValues = field.value ?? [];
                  return (
                    <FormItem>
                      <div className="flex items-center justify-between">
                        <FormLabel>自定义请求头</FormLabel>
                        <Button type="button" variant="outline" size="sm" onClick={() => appendHeader({ key: "", value: "" })}>
                          添加
                        </Button>
                      </div>
                      <div className="space-y-2">
                        {headerFields.map((header, index) => {
                          const errorMsg = form.formState.errors.customer_headers?.[index]?.key?.message;
                          return (
                            <div key={header.id} className="space-y-1">
                              <div className="flex gap-2 items-center">
                                <div className="flex-1">
                                  <Input
                                    placeholder="Header Key"
                                    value={headerValues[index]?.key ?? ""}
                                    onChange={(event) => {
                                      const next = [...headerValues];
                                      next[index] = { ...next[index], key: event.target.value };
                                      field.onChange(next);
                                    }}
                                  />
                                </div>
                                <div className="flex-1">
                                  <Input
                                    placeholder="Header Value"
                                    value={headerValues[index]?.value ?? ""}
                                    onChange={(event) => {
                                      const next = [...headerValues];
                                      next[index] = { ...next[index], value: event.target.value };
                                      field.onChange(next);
                                    }}
                                  />
                                </div>
                                <Button type="button" size="sm" variant="destructive" onClick={() => removeHeader(index)}>
                                  删除
                                </Button>
                              </div>
                              {errorMsg && <p className="text-sm text-red-500">{errorMsg}</p>}
                            </div>
                          );
                        })}
                        <p className="text-sm text-muted-foreground">{"优先级: 提供商配置 > 自定义请求头 > 透传请求头"}</p>
                      </div>
                    </FormItem>
                  );
                }}
              />

              <FormField
                control={form.control}
                name="weight"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>权重 (必须大于0)</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type="number"
                        min="1"
                        onChange={(event) => field.onChange(parseInt(event.target.value, 10) || 0)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="priority"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>优先级 (优先级高的优先选择，相同优先级按权重随机)</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type="number"
                        min="0"
                        onChange={(event) => field.onChange(parseInt(event.target.value, 10) || 0)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="max_tokens"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>max_tokens 上限 (0=不限，超过此值会被裁剪)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        min="0"
                        placeholder="0=不限"
                        value={field.value ?? 0}
                        onChange={(event) => field.onChange(parseInt(event.target.value, 10) || 0)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
                取消
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? "提交中..." : editingAssociation ? "更新" : "创建"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

