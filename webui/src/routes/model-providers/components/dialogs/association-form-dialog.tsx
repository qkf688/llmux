import { useWatch } from "react-hook-form";
import type { FieldArrayWithId, UseFormReturn } from "react-hook-form";
import type { Model, ModelWithProvider, Provider } from "@/lib/api";
import type { FormValues } from "../../form-schema";
import type { ProviderModelSelection } from "../../types";
import { buildSelectionKey } from "../../utils/selection";
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
  FormDescription,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { THINKING_LEVEL_OPTIONS } from "@/lib/constants/thinking-levels";

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

// 能力三态字段（三项共用）："inherit" 跟随模型真值，true/false 为本链路的 override。
// 绝大多数第三方与官方能力一致，无需逐条标注，故默认态是「继承」而非勾选框。
// modelValue 仅用于把"继承时的实际取值"显示出来，避免用户选了继承却看不出最终能力。
function CapabilityTriStateField({
  form,
  name,
  label,
  modelValue,
}: {
  form: UseFormReturn<FormValues>;
  name: "tool_call" | "structured_output" | "image";
  label: string;
  modelValue: boolean;
}) {
  const inheritedText = modelValue ? "启用" : "禁用";
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem className="rounded-md border p-4">
          <div className="space-y-1">
            <FormLabel>{label}</FormLabel>
            <div className="text-sm text-muted-foreground">
              覆盖模型级能力；"继承"跟随模型设置（当前：{inheritedText}）
            </div>
          </div>
          <FormControl>
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger>
                <SelectValue placeholder="选择" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="inherit">继承（当前：{inheritedText}）</SelectItem>
                <SelectItem value="true">启用</SelectItem>
                <SelectItem value="false">禁用</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  );
}

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
  // 仅订阅 model_id 字段变化（比 form.watch 每渲染全表单订阅更小）
  const selectedModelId = useWatch({ control: form.control, name: "model_id" });
  // 当前选中模型的固有能力真值，供能力三态字段展示「继承时的实际取值」，
  // 也供下方 supports_thinking / thinking_levels 复用（避免每处重复 find）。
  const selectedModel = models.find((m) => m.ID === selectedModelId);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{editingAssociation ? "编辑关联" : "添加关联"}</DialogTitle>
          <DialogDescription>
            {editingAssociation ? "修改模型提供商关联" : "添加一个新的模型提供商关联"}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleSubmit)} className="flex min-h-0 flex-1 flex-col gap-4">
            <DialogBody className="-mx-1 space-y-4 px-1">
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
                  name="provider_model"
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
              <CapabilityTriStateField
                form={form}
                name="tool_call"
                label="工具调用"
                modelValue={selectedModel?.tool_call ?? false}
              />
              <CapabilityTriStateField
                form={form}
                name="structured_output"
                label="结构化输出"
                modelValue={selectedModel?.structured_output ?? false}
              />
              <CapabilityTriStateField
                form={form}
                name="image"
                label="视觉"
                modelValue={selectedModel?.image ?? false}
              />

              <FormLabel>参数配置</FormLabel>
              <FormField
                control={form.control}
                name="supports_thinking"
                render={({ field }) => {
                  const inherited = selectedModel ? selectedModel.supports_thinking : false;
                  return (
                    <FormItem className="rounded-md border p-4">
                      <div className="space-y-1">
                        <FormLabel>支持 thinking</FormLabel>
                        <div className="text-sm text-muted-foreground">
                          覆盖模型级的 thinking 能力；"继承"跟随模型设置（当前：{inherited ? "启用" : "禁用"}）
                        </div>
                      </div>
                      <FormControl>
                        <Select value={field.value} onValueChange={field.onChange}>
                          <SelectTrigger>
                            <SelectValue placeholder="选择" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="inherit">继承（当前：{inherited ? "启用" : "禁用"}）</SelectItem>
                            <SelectItem value="true">启用</SelectItem>
                            <SelectItem value="false">禁用</SelectItem>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  );
                }}
              />
              <FormField
                control={form.control}
                name="thinking_levels_mode"
                render={({ field }) => {
                  const inheritedLevels = selectedModel?.thinking_levels;
                  const inheritedLabel =
                    inheritedLevels === undefined || inheritedLevels === null
                      ? "未设置（不约束）"
                      : inheritedLevels.length === 0
                        ? "不约束（空）"
                        : inheritedLevels.join(", ");
                  return (
                    <FormItem className="rounded-md border p-4 space-y-3">
                      <div className="space-y-1">
                        <FormLabel>思考档位白名单</FormLabel>
                        <div className="text-sm text-muted-foreground">
                          覆盖模型级的思考档位白名单；"继承"跟随模型设置（当前：{inheritedLabel}）。
                          自定义模式下勾选允许的档位，空列表=不约束
                        </div>
                      </div>
                      <FormControl>
                        <Select value={field.value} onValueChange={field.onChange}>
                          <SelectTrigger>
                            <SelectValue placeholder="选择" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="inherit">继承（当前：{inheritedLabel}）</SelectItem>
                            <SelectItem value="custom">自定义</SelectItem>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  );
                }}
              />
              <FormField
                control={form.control}
                name="thinking_levels_custom"
                render={({ field }) => (
                  <FormItem className="rounded-md border p-4 space-y-3">
                    <div className="space-y-1">
                      <FormLabel>自定义白名单</FormLabel>
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
                      <FormDescription>
                        开启后，生产转发与诊断/连通测试均只透传调试性请求头（X-* 自定义头、User-Agent、协议 beta/版本头）给上游；
                        Authorization、Cookie 等会话信息不会外发。需要额外头部请用下方自定义头部。
                      </FormDescription>
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
                        {headerFields.map((header, index) => (
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
                          </div>
                        ))}
                        <p className="text-sm text-muted-foreground">{"优先级: 提供商配置 > 自定义请求头 > 透传请求头"}</p>
                        <p className="text-sm text-muted-foreground">
                          只差大小写的同名头按「后填者赢」合并为一条；头名/头值非法会被保存拦截。
                          凭据头（Authorization / x-api-key）归供应商认证配置专属，配了也不生效
                          （被供应商凭据覆写或按其认证方式自动删除）。
                        </p>
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
            </DialogBody>

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

