/**
 * 供应商表单「协议与调度」分区（S0 原型）：
 * 主类型（○ 单选）× 支持类型勾选（= 出站协议）/ 协议端点区（跟随勾选，URL 继承/覆盖）/
 * 凭据分组区（价格权重 + 关联号池下拉，可就地打开号池详情弹窗）。
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { usePoolOptions } from "../../hooks/use-pool-options";
import { PROTOCOL_LABEL, SUPPORTED_TYPE_OPTIONS, type ProviderFormGroup, type ProviderFormValues } from "../../form-schema";

/**
 * 主类型 / 支持类型（S0 原型）：主类型为 Select 下拉（紧凑，决定 providers.New 分发 / 模板默认值 / 模型同步），
 * 支持类型为 □ checkbox 纵列表选出站协议（每协议一行，生成对应端点行）。主类型必须 ∈ 已勾选协议：
 * 下拉只列出已勾选项；取消勾选主类型时自动落到剩余第一个（schema superRefine 兜底）。
 * 首个勾选 = 初始主类型（下拉无候选，此时自动写入并套模板默认值；此后显式改选不覆盖已填字段）。
 */
export function SupportTypesField({ providerTemplates }: { providerTemplates: ProviderTemplate[] }) {
  const form = useFormContext<ProviderFormValues>();
  const { control, getValues, setValue } = form;
  const protocols = useWatch({ control, name: "protocols" }) ?? [];

  // 端点严格跟随协议 1:1：勾选补一条空端点、取消勾选移除对应端点。这里用 getValues/setValue
  // 直接改 endpoints，不引入 useFieldArray——EndpointsSection 以 useWatch 读同一份表单值即可。
  // 两处若各起一个同名 useFieldArray，RHF 不会让第二个实例随第一个的 append/remove 更新，
  // 端点显示会卡在初始数量（这正是「勾选多个类型端点不增加」的根因）。
  const syncEndpoint = (protocol: string, add: boolean) => {
    const endpoints = getValues("endpoints") ?? [];
    if (add) {
      if (!endpoints.some((e) => e.protocol === protocol)) {
        setValue("endpoints", [...endpoints, { protocol, url: "", enabled: true }], { shouldDirty: true });
      }
      return;
    }
    setValue("endpoints", endpoints.filter((e) => e.protocol !== protocol), { shouldDirty: true });
  };

  const toggleType = (type: string, checked: boolean) => {
    const option = SUPPORTED_TYPE_OPTIONS.find((o) => o.type === type);
    const protocol = option?.protocol ?? "openai";
    const current = getValues("protocols") ?? [];

    if (checked) {
      const next = [...new Set([...current, protocol])];
      setValue("protocols", next, { shouldDirty: true });
      if (current.length === 0) {
        // 首个勾选 = 初始主类型：写回 type 并套模板默认值
        setValue("type", type, { shouldDirty: true });
        applyProviderTemplateDefaults(type, providerTemplates, form);
      }
      syncEndpoint(protocol, true);
      return;
    }

    const next = current.filter((p) => p !== protocol);
    setValue("protocols", next, { shouldDirty: true });
    syncEndpoint(protocol, false);
    // 仅当移除的是主类型本身时，主类型落到剩余第一个（保持 type ∈ protocols）；
    // 移除次要协议不触碰主类型（原实现无条件切第一个，会把用户显式选的主类型悄悄改掉）
    if (getValues("type") === type && next.length > 0) {
      const first = SUPPORTED_TYPE_OPTIONS.find((o) => o.protocol === next[0]);
      if (first) {
        setValue("type", first.type, { shouldDirty: true });
      }
    }
  };

  return (
    <div className="space-y-3">
      <FormField
        control={control}
        name="type"
        render={({ field }) => (
          <FormItem className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <FormLabel className="shrink-0">主类型</FormLabel>
              <FormControl>
                <Select onValueChange={field.onChange} value={field.value || undefined}>
                  <SelectTrigger className="h-8 min-w-0 flex-1">
                    <SelectValue placeholder="选择主类型" />
                  </SelectTrigger>
                  <SelectContent>
                    {SUPPORTED_TYPE_OPTIONS.filter((o) => protocols.includes(o.protocol)).map((opt) => (
                      <SelectItem key={opt.type} value={opt.type}>
                        {opt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormControl>
              <p className="shrink-0 text-xs text-muted-foreground">决定后端实现 / 模板默认值 / 模型同步</p>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className="space-y-1.5">
        <Tooltip>
          <TooltipTrigger asChild>
            <FormLabel className="w-fit cursor-default">
              支持协议
            </FormLabel>
          </TooltipTrigger>
          <TooltipContent>勾选即生成对应协议端点，请求按入站协议匹配端点（同协议透传）</TooltipContent>
        </Tooltip>
        <div className="space-y-1.5">
          {SUPPORTED_TYPE_OPTIONS.map((opt) => {
            const supported = protocols.includes(opt.protocol);
            const lastStanding = protocols.length === 1 && supported;
            return (
              <Tooltip key={opt.type}>
                <TooltipTrigger asChild>
                  <label
                    className="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border/60 px-3 py-2 text-sm transition-colors hover:bg-muted/50"
                  >
                    <Checkbox
                      checked={supported}
                      // 最后一个勾选禁取消：至少保留一个出站协议/端点（zod 数组 min(1) 兜底）
                      disabled={lastStanding}
                      onCheckedChange={(checked) => toggleType(opt.type, checked === true)}
                    />
                    <span>{opt.label}</span>
                  </label>
                </TooltipTrigger>
                {lastStanding && <TooltipContent>至少保留一个支持协议</TooltipContent>}
              </Tooltip>
            );
          })}
        </div>
        {protocols.length === 0 && <p className="text-sm font-medium text-destructive">至少勾选一个支持协议</p>}
      </div>
    </div>
  );
}

/**
 * 单条协议端点行：URL 默认继承 Base URL（不显示输入框），点「覆盖 URL」才出现输入框，
 * 「恢复继承」清空回到继承态。covering 是行内局部状态——行以 protocol 为 key，取消勾选即
 * 卸载、状态随之消失，重新勾选是全新行（默认继承态），与旧的「行 id 键」语义一致。
 */
function EndpointRow({ index, protocol }: { index: number; protocol: string }) {
  const { control } = useFormContext<ProviderFormValues>();
  const [covering, setCovering] = useState(false);

  return (
    <div className="flex items-center gap-2 rounded-lg border p-3">
      <span className="w-28 shrink-0 text-sm font-medium">{PROTOCOL_LABEL[protocol] ?? protocol}</span>
      <FormField
        control={control}
        name={`endpoints.${index}.url`}
        render={({ field }) => (
          <FormItem className="flex-1 space-y-0">
            <FormControl>
              {covering || field.value ? (
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
                      setCovering(false);
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
                    onClick={() => setCovering(true)}
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
  // endpoints 由 SupportTypesField 独家增删，本组件只读：用 useWatch 读同一份表单值，
  // 不能另起同名 useFieldArray（RHF 双实例不互相同步，端点行会卡在初始数量）。
  const endpoints = useWatch({ control, name: "endpoints" }) ?? [];

  // index 取 endpoints 数组的绝对下标，供 EndpointRow 定位 endpoints.{index}.url；
  // 只渲染已勾选协议的端点（取消勾选的行随即消失）
  const rows = endpoints
    .map((endpoint, index) => ({ endpoint, index }))
    .filter(({ endpoint }) => protocols.includes(endpoint.protocol));

  return (
    <div className="space-y-3">
      <div>
        <FormLabel>协议端点</FormLabel>
        <p className="text-xs text-muted-foreground">
          跟随「支持类型」勾选自动生成，不可增删（取消勾选即移除）；URL 默认继承 Base URL，需要覆盖时填写（如 …/claude/v1）
        </p>
      </div>
      {protocols.length === 0 && <p className="text-xs text-muted-foreground">请先勾选至少一个出站协议</p>}
      {rows.map(({ endpoint, index }) => (
        <EndpointRow key={endpoint.protocol} index={index} protocol={endpoint.protocol} />
      ))}
    </div>
  );
}

function defaultGroup(): ProviderFormGroup {
  return { name: "", weight: 1, poolId: "" };
}

/** 单个分组卡片：名称/价格权重/关联号池；凭据只存在于号池，可查看池详情 */
function GroupCard({
  index,
  onViewPoolDetail,
}: {
  index: number;
  /** 打开号池详情弹窗（由 ProviderFormDialog 持 state 并渲染共享 PoolDetailDialog） */
  onViewPoolDetail?: (poolId: string) => void;
}) {
  const { control } = useFormContext<ProviderFormValues>();
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
                <Tooltip>
                  <TooltipTrigger asChild>
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
                    />
                  </TooltipTrigger>
                  <TooltipContent>价格导向权重：便宜的分组权重高 → 选中概率大</TooltipContent>
                </Tooltip>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

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
                    <SelectValue placeholder="选择号池（凭据在号池页管理）" />
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
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-9 shrink-0 px-2.5 text-xs"
                      disabled={!field.value}
                      onClick={() => onViewPoolDetail?.(field.value)}
                    >
                      <Eye className="size-3.5" />
                      查看详情
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>查看号池详情（凭据列表 / 状态分布）</TooltipContent>
              </Tooltip>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />
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
          请求按模型可用性（分组白名单由同步维护）+ 价格权重加权随机；号池内凭据轮询 + 冷却剔除
        </p>
      </div>
      {fields.map((field, index) => (
        <div key={field.id} className="space-y-2">
          <GroupCard index={index} onViewPoolDetail={onViewPoolDetail} />
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                  // 至少保留一个分组：删到最后一个时禁用（zod 数组 min(1) 兜底）
                  disabled={fields.length <= 1}
                  onClick={() => remove(index)}
                >
                  <Trash2 className="size-3.5" />
                  删除该分组
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>{fields.length <= 1 ? "至少保留一个凭据分组" : "删除该分组"}</TooltipContent>
          </Tooltip>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => append(defaultGroup())}>
        + 添加分组
      </Button>
    </div>
  );
}