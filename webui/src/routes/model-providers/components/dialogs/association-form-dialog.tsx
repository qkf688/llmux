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
import { PROTOCOL_OPTIONS } from "@/lib/protocols";

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

// 能力总控 + 明细勾选（三项共用一个组件）。
//
// 为什么是「总控 + 勾选」而不是三项各自三态下拉：
//  ① 「继承」与「显式等于模型真值」在 COALESCE 解析后完全等价，UI 区分它俩只是添麻烦；
//  ② 绝大多数链路要么整体跟随、要么整体弱于模型，一个动作就能覆盖主场景；
//  ③ 总控下拉把"这条链路要不要单独管"表达成一个明确动作，不必让用户对三件平级的事各决策一次。
//
// 取值域（inherit / custom）与下方 thinking 白名单的 thinking_levels_mode 刻意一致：
// 两块在同一弹窗内，枚举名与措辞统一才不需要用户学两套。
function CapabilitySection({
  form,
  modelCaps,
}: {
  form: UseFormReturn<FormValues>;
  modelCaps: { tool_call: boolean; structured_output: boolean; image: boolean };
}) {
  const mode = useWatch({ control: form.control, name: "capabilities_mode" });
  const caps: { name: "tool_call" | "structured_output" | "image"; label: string; modelKey: keyof typeof modelCaps }[] = [
    { name: "tool_call", label: "工具调用", modelKey: "tool_call" },
    { name: "structured_output", label: "结构化输出", modelKey: "structured_output" },
    { name: "image", label: "视觉", modelKey: "image" },
  ];
  const inheritedSummary = caps
    .map((c) => `${c.label} ${modelCaps[c.modelKey] ? "启用" : "禁用"}`)
    .join(" · ");

  return (
    <>
      <FormField
        control={form.control}
        name="capabilities_mode"
        render={({ field }) => (
          <FormItem className="rounded-md border p-4 space-y-3">
            <div className="space-y-1">
              <FormLabel>模型能力</FormLabel>
              <div className="text-sm text-muted-foreground">
                默认跟随模型设置（当前：{inheritedSummary}）。仅当这条链路的能力与模型不一致时才自定义。
              </div>
            </div>
            <FormControl>
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inherit">继承（跟随模型设置）</SelectItem>
                  <SelectItem value="custom">自定义</SelectItem>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {/* 明细仅在「自定义」下出现：继承时摆一排勾选框只会让人以为它们生效。 */}
      {mode === "custom" && (
        <FormItem className="rounded-md border p-4 space-y-3">
          <div className="space-y-1">
            <FormLabel>自定义</FormLabel>
            <div className="text-sm text-muted-foreground">
              勾选这条链路实际支持的能力。取消勾选表示该链路不支持；保存后这三项不再跟随模型设置。
            </div>
          </div>
          <FormControl>
            <div className="flex flex-wrap gap-x-6 gap-y-2 pt-1">
              {caps.map((c) => (
                <FormField
                  key={c.name}
                  control={form.control}
                  name={c.name}
                  render={({ field: capField }) => (
                    <label className="flex items-center gap-2 cursor-pointer text-sm">
                      <Checkbox
                        checked={capField.value}
                        onCheckedChange={(checked) => capField.onChange(checked === true)}
                      />
                      {c.label}
                    </label>
                  )}
                />
              ))}
            </div>
          </FormControl>
        </FormItem>
      )}
    </>
  );
}

// 勾选组（value/label 列表）：协议与思考档位两处共用同一段 map JSX，
// 避免「同一组件抄两遍」——两处的增删逻辑曾是各写一份，改动时容易只改一处。
function CheckboxOptionList({
  options,
  value,
  onChange,
  onUncheck,
}: {
  options: { value: string; label: string }[];
  value: string[];
  onChange: (next: string[]) => void;
  onUncheck?: (unchecked: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-3 pt-1">
      {options.map((opt) => {
        const checked = value.includes(opt.value);
        return (
          <label key={opt.value} className="flex items-center gap-2 cursor-pointer text-sm">
            <Checkbox
              checked={checked}
              onCheckedChange={(c) => {
                const current = [...value];
                const idx = current.indexOf(opt.value);
                if (c) {
                  if (idx < 0) current.push(opt.value);
                } else {
                  if (idx >= 0) current.splice(idx, 1);
                  onUncheck?.(opt.value);
                }
                onChange(current);
              }}
            />
            {opt.label}
          </label>
        );
      })}
    </div>
  );
}

// 协议覆盖（集合 + 主格式）。
//
// 为什么两项分开：集合表达「这个模型能走哪几条」（客户端发这些格式就直通），主格式表达
// 「需要转换时落哪条」。合成一个下拉会让「两条都支持、但改写优先用第二条」无法表达。
// 默认整块继承供应商：绝大多数模型与供应商一致，只有混合协议网关才需要单独收窄。
function ProtocolSection({ form }: { form: UseFormReturn<FormValues> }) {
  const mode = useWatch({ control: form.control, name: "protocols_mode" });
  const selected = useWatch({ control: form.control, name: "protocols" }) ?? [];

  return (
    <>
      <FormField
        control={form.control}
        name="protocols_mode"
        render={({ field }) => (
          <FormItem className="rounded-md border p-4 space-y-3">
            <div className="space-y-1">
              <FormLabel>协议</FormLabel>
              <div className="text-sm text-muted-foreground">
                默认跟随供应商（供应商已启用的协议都可走）。仅当这个模型支持的协议与供应商不同时才自定义。
              </div>
            </div>
            <FormControl>
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inherit">继承（跟随供应商）</SelectItem>
                  <SelectItem value="custom">自定义</SelectItem>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {mode === "custom" && (
        <FormField
          control={form.control}
          name="protocols"
          render={({ field }) => (
            <FormItem className="rounded-md border p-4 space-y-3">
              <div className="space-y-1">
                <FormLabel>允许走的协议</FormLabel>
                <div className="text-sm text-muted-foreground">
                  勾选这个模型实际支持的协议；客户端发来不在勾选里的格式，会被转换成主格式。
                  供应商没配过的协议也能勾——地址按供应商的 Base URL 拼出来。
                  <span className="block pt-1">
                    这里勾选的协议不受供应商页端点启停影响（那是供应商级默认，被关联覆盖）。
                  </span>
                </div>
              </div>
              <FormControl>
                <CheckboxOptionList
                  options={PROTOCOL_OPTIONS}
                  value={field.value ?? []}
                  onChange={field.onChange}
                  // 主格式被取消勾选时清回继承：否则提交一个必然被后端拒绝的组合。
                  onUncheck={(unchecked) => {
                    if (form.getValues("primary_protocol") === unchecked) {
                      form.setValue("primary_protocol", "");
                    }
                  }}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* 主格式独立于集合：集合继承时也能单独指定（后端要求它落在供应商已启用的协议里）。
          始终渲染是为了不把「只设了主格式」的存量配置藏起来——藏起来等于下次保存时静默清掉。 */}
      <FormField
        control={form.control}
        name="primary_protocol"
        render={({ field }) => (
          <FormItem className="rounded-md border p-4 space-y-3">
            <div className="space-y-1">
              <FormLabel>主格式</FormLabel>
              <div className="text-sm text-muted-foreground">
                客户端格式不在勾选里时转成这条。「继承」用供应商的主格式（它不在勾选里时取固定先后第一条）。
              </div>
            </div>
            <FormControl>
              <Select
                value={field.value === "" ? "inherit" : field.value}
                onValueChange={(value) => field.onChange(value === "inherit" ? "" : value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inherit">继承供应商主格式</SelectItem>
                  {(mode === "custom"
                    ? PROTOCOL_OPTIONS.filter((opt) => selected.includes(opt.value))
                    : PROTOCOL_OPTIONS
                  ).map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
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
  // 当前选中模型的固有能力真值，供能力总控展示「继承时的实际取值」，
  // 也供下方 supports_thinking / thinking_levels 复用（避免每处重复 find）。
  const selectedModel = models.find((m) => m.ID === selectedModelId);
  // 订阅明细卡片的显隐条件：thinking 白名单只在「自定义」时渲染。
  const thinkingMode = useWatch({ control: form.control, name: "thinking_levels_mode" });

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
                <p className="text-sm text-muted-foreground">
                  请点上方「模型列表」选择模型。目录里没有的名字，请先到供应商页的「全部模型」里「添加模型」。
                </p>
              )}

              <FormLabel>模型能力</FormLabel>
              <CapabilitySection
                form={form}
                modelCaps={{
                  tool_call: selectedModel?.tool_call ?? false,
                  structured_output: selectedModel?.structured_output ?? false,
                  image: selectedModel?.image ?? false,
                }}
              />

              <FormLabel>协议</FormLabel>
              <ProtocolSection form={form} />

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
                          不在白名单的档位会被就近钳制到最接近的档位；空列表=不约束。当前继承值：{inheritedLabel}
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
              {/* 明细仅在「自定义」下出现：继承时摆一排勾选框只会让人以为它们生效。 */}
              {thinkingMode === "custom" && (
                <FormField
                  control={form.control}
                  name="thinking_levels_custom"
                  render={({ field }) => (
                    <FormItem className="rounded-md border p-4 space-y-3">
                      <div className="space-y-1">
                        <FormLabel>自定义白名单</FormLabel>
                        <div className="text-sm text-muted-foreground">
                          勾选允许的 reasoning_effort 档位；全部不勾=不约束（任意档位透传）
                        </div>
                      </div>
                      <FormControl>
                        <CheckboxOptionList
                          options={THINKING_LEVEL_OPTIONS}
                          value={field.value ?? []}
                          onChange={field.onChange}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
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

