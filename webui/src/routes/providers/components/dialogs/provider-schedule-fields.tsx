/**
 * 供应商表单「协议与调度」分区（S0 原型）：
 * 支持类型勾选（= 出站协议，第一个勾选为主类型）/ 协议端点区（跟随勾选，URL 继承/覆盖）/
 * 凭据分组区（价格权重 + 白名单 + 凭据来源二选一；关联号池可就地打开号池详情弹窗）。
 * 依赖 ProviderFormDialog 的 <Form> context（useFormContext）。
 * 正式实现（S6）时这些字段改为独立 DTO 提交，本组件结构保留。
 */
import type { ProviderTemplate } from "@/lib/api";
import { Eye, Trash2 } from "lucide-react";
import { useState } from "react";
import { useFieldArray, useFormContext, useWatch } from "react-hook-form";
import { applyProviderTemplateDefaults } from "../../utils/template-defaults";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { usePoolOptions } from "../../hooks/use-pool-options";
import type { ProviderFormGroup, ProviderFormValues } from "../../form-schema";

/** 支持类型 = provider type 多选；每个类型对应一种出站协议 */
const SUPPORTED_TYPE_OPTIONS = [
  { type: "openai", protocol: "openai", label: "OpenAI（chat/completions）" },
  { type: "openai-res", protocol: "responses", label: "OpenAI Responses" },
  { type: "anthropic", protocol: "anthropic", label: "Anthropic" },
] as const;

const PROTOCOL_LABEL: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  responses: "OpenAI Responses",
};

/**
 * 支持类型（checkbox 组，替代原「类型」下拉 +「出站协议」勾选）：
 * 勾选类型 → 生成对应协议端点行；第一个勾选 = 主类型（写回 schema 的 type 字段，
 * 决定 providers.New 分发 / 模板默认值 / 模型同步）；取消勾选 → 移除该协议端点行。
 */
export function SupportTypesField({ providerTemplates }: { providerTemplates: ProviderTemplate[] }) {
  const form = useFormContext<ProviderFormValues>();
  const { control, getValues, setValue } = form;
  const protocols = useWatch({ control, name: "protocols" }) ?? [];
  const { fields, append, remove } = useFieldArray({ control, name: "endpoints" });

  const toggleType = (type: string, checked: boolean) => {
    const option = SUPPORTED_TYPE_OPTIONS.find((o) => o.type === type);
    const protocol = option?.protocol ?? "openai";
    const current = getValues("protocols") ?? [];

    if (checked) {
      const next = [...new Set([...current, protocol])];
      setValue("protocols", next, { shouldDirty: true });
      if (current.length === 0) {
        // 第一个勾选 = 主类型：写回 type 并套模板默认值
        setValue("type", type, { shouldDirty: true });
        applyProviderTemplateDefaults(type, providerTemplates, form);
      }
      if (!fields.some((f) => f.protocol === protocol)) {
        append({ protocol, url: "", enabled: true });
      }
      return;
    }

    const next = current.filter((p) => p !== protocol);
    setValue("protocols", next, { shouldDirty: true });
    const indices = fields.map((f, i) => (f.protocol === protocol ? i : -1)).filter((i) => i >= 0);
    if (indices.length > 0) {
      remove(indices);
    }
    if (next.length > 0) {
      // 主类型切换到剩余第一个
      const first = SUPPORTED_TYPE_OPTIONS.find((o) => o.protocol === next[0]);
      if (first) {
        setValue("type", first.type, { shouldDirty: true });
      }
    }
  };

  return (
    <div className="space-y-2">
      <div>
        <FormLabel>支持类型</FormLabel>
        <p className="text-xs text-muted-foreground">
          可多选；第一个勾选为主类型。勾选即生成对应协议端点，请求按入站协议匹配端点（同协议透传）
        </p>
      </div>
      <div className="flex flex-wrap gap-2">
        {SUPPORTED_TYPE_OPTIONS.map((opt) => (
          <label
            key={opt.type}
            className="flex cursor-pointer items-center gap-2 rounded-lg border p-2.5 hover:bg-muted/50"
          >
            <Checkbox
              checked={protocols.includes(opt.protocol)}
              onCheckedChange={(checked) => toggleType(opt.type, checked === true)}
            />
            <span className="text-sm">{opt.label}</span>
          </label>
        ))}
      </div>
      {protocols.length === 0 ? (
        <FormMessage>至少勾选一个支持类型</FormMessage>
      ) : (
        <p className="text-xs text-muted-foreground">主类型：{PROTOCOL_LABEL[protocols[0]] ?? protocols[0]}</p>
      )}
    </div>
  );
}

/**
 * 协议端点区：每个勾选协议一条端点（不做同协议多 URL）。
 * URL 默认继承 Base URL（不显示输入框）；需要覆盖时点「覆盖 URL」才出现输入框，
 * 输入后显示覆盖值，可「恢复继承」清空。
 */
export function EndpointsSection() {
  const { control } = useFormContext<ProviderFormValues>();
  const protocols = useWatch({ control, name: "protocols" }) ?? [];
  const { fields } = useFieldArray({ control, name: "endpoints" });
  const [covering, setCovering] = useState<Record<string, boolean>>({});

  // covering 以端点行 id 为键：取消勾选移除行后状态随行消失，重新勾选生成新 id → 回到默认「继承」形态
  const rows = fields.map((f, i) => ({ f, i })).filter(({ f }) => protocols.includes(f.protocol));

  return (
    <div className="space-y-3">
      <div>
        <FormLabel>协议端点</FormLabel>
        <p className="text-xs text-muted-foreground">
          跟随「支持类型」勾选自动生成，不可增删（取消勾选即移除）；URL 默认继承 Base URL，需要覆盖时填写（如 …/claude/v1）
        </p>
      </div>
      {protocols.length === 0 && <p className="text-xs text-muted-foreground">请先勾选至少一个出站协议</p>}
      {rows.map(({ f, i }) => (
        <div key={f.id} className="flex items-center gap-2 rounded-lg border p-3">
          <span className="w-28 shrink-0 text-sm font-medium">{PROTOCOL_LABEL[f.protocol] ?? f.protocol}</span>
          <FormField
            control={control}
            name={`endpoints.${i}.url`}
            render={({ field }) => (
              <FormItem className="flex-1 space-y-0">
                <FormControl>
                  {covering[f.id] || field.value ? (
                    <div className="flex items-center gap-2">
                      <Input
                        {...field}
                        placeholder="覆盖 URL（如 https://…/claude/v1）"
                        className="font-mono text-xs"
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-8 shrink-0 px-2 text-xs text-muted-foreground"
                        onClick={() => {
                          field.onChange("");
                          setCovering((prev) => ({ ...prev, [f.id]: false }));
                        }}
                      >
                        恢复继承
                      </Button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <Badge variant="outline" className="text-[10px]">
                        继承 Base URL
                      </Badge>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 px-2 text-xs"
                        onClick={() => setCovering((prev) => ({ ...prev, [f.id]: true }))}
                      >
                        覆盖 URL
                      </Button>
                    </div>
                  )}
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      ))}
    </div>
  );
}

function defaultGroup(): ProviderFormGroup {
  return { name: "", weight: 1, models: "", source: "inline", inlineKeys: "", poolId: "" };
}

/** 单个分组卡片：名称/价格权重/模型白名单/凭据来源二选一；关联号池时可查看池详情 */
function GroupCard({
  index,
  onViewPoolDetail,
}: {
  index: number;
  /** 打开号池详情弹窗（由 ProviderFormDialog 持 state 并渲染共享 PoolDetailDialog） */
  onViewPoolDetail?: (poolId: string) => void;
}) {
  const { control } = useFormContext<ProviderFormValues>();
  const source = useWatch({ control, name: `groups.${index}.source` });
  const pools = usePoolOptions();

  return (
    <div className="space-y-3 rounded-lg border p-3">
      <div className="flex items-end gap-2">
        <FormField
          control={control}
          name={`groups.${index}.name`}
          render={({ field }) => (
            <FormItem className="flex-1 space-y-1">
              <FormLabel>分组名称</FormLabel>
              <FormControl>
                <Input {...field} placeholder="如：低价组 / 走量组" />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={`groups.${index}.weight`}
          render={({ field }) => (
            <FormItem className="w-28 space-y-1">
              <FormLabel>价格权重</FormLabel>
              <FormControl>
                <Input
                  type="number"
                  min={0}
                  max={100}
                  value={field.value}
                  onChange={(e) => {
                    const v = e.target.valueAsNumber;
                    // 清空输入时 valueAsNumber 为 NaN，落成 0 避免 React 受控 value 报警
                    field.onChange(Number.isNaN(v) ? 0 : v);
                  }}
                  title="价格导向权重：便宜的分组权重高 → 选中概率大"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={control}
        name={`groups.${index}.models`}
        render={({ field }) => (
          <FormItem className="space-y-1">
            <FormLabel>模型白名单</FormLabel>
            <FormControl>
              <Input {...field} placeholder="gpt-4o, gpt-4o-mini（逗号分隔，留空 = 不限）" />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={control}
        name={`groups.${index}.source`}
        render={({ field }) => (
          <FormItem className="space-y-1">
            <FormLabel>凭据来源</FormLabel>
            <FormControl>
              <RadioGroup value={field.value} onValueChange={field.onChange} className="flex gap-4">
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="inline" id={`source-inline-${index}`} />
                  <Label htmlFor={`source-inline-${index}`} className="text-sm">
                    内联 Key
                  </Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="pool" id={`source-pool-${index}`} />
                  <Label htmlFor={`source-pool-${index}`} className="text-sm">
                    关联号池
                  </Label>
                </div>
              </RadioGroup>
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {source === "inline" ? (
        <FormField
          control={control}
          name={`groups.${index}.inlineKeys`}
          render={({ field }) => (
            <FormItem className="space-y-1">
              <FormLabel>API Keys（每行一个，可批量粘贴）</FormLabel>
              <FormControl>
                <Textarea {...field} rows={2} placeholder={"sk-...\nsk-..."} className="font-mono text-xs" />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      ) : (
        <FormField
          control={control}
          name={`groups.${index}.poolId`}
          render={({ field }) => (
            <FormItem className="space-y-1">
              <FormLabel>关联号池</FormLabel>
              <div className="flex items-center gap-2">
                <FormControl>
                  <Select value={field.value || undefined} onValueChange={field.onChange}>
                    <SelectTrigger className="min-w-0 flex-1">
                      <SelectValue placeholder="选择号池（可在号池页管理凭据）" />
                    </SelectTrigger>
                    <SelectContent>
                      {pools.map((pool) => (
                        <SelectItem key={pool.id} value={String(pool.id)}>
                          {pool.name}（{pool.keyCount} keys）
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormControl>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-9 shrink-0 px-2.5 text-xs"
                  disabled={!field.value}
                  onClick={() => onViewPoolDetail?.(field.value)}
                  title="查看号池详情（凭据列表 / 状态分布）"
                >
                  <Eye className="size-3.5" />
                  查看详情
                </Button>
              </div>
              <FormMessage />
            </FormItem>
          )}
        />
      )}
    </div>
  );
}

/** 凭据分组区：分组列表 + 添加分组 */
export function GroupsSection({
  onViewPoolDetail,
}: {
  /** 透传给各分组卡片：关联号池时打开该池的详情弹窗 */
  onViewPoolDetail?: (poolId: string) => void;
}) {
  const { control } = useFormContext<ProviderFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "groups" });

  return (
    <div className="space-y-3">
      <div>
        <FormLabel>凭据分组</FormLabel>
        <p className="text-xs text-muted-foreground">
          请求按分组白名单过滤 + 价格权重加权随机；分组内凭据轮询 + 冷却剔除
        </p>
      </div>
      {fields.map((field, index) => (
        <div key={field.id} className="space-y-2">
          <GroupCard index={index} onViewPoolDetail={onViewPoolDetail} />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-destructive hover:text-destructive"
            onClick={() => remove(index)}
          >
            <Trash2 className="size-3.5" />
            删除该分组
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => append(defaultGroup())}>
        + 添加分组
      </Button>
    </div>
  );
}