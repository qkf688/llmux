import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import type { UseFormReturn } from "react-hook-form";
import { PoolDetailDialog } from "@/components/number-pools/pool-detail-dialog";
import { usePools } from "@/hooks/api";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { Provider, ProviderTemplate } from "@/lib/api";
import type { ProviderFormValues } from "../../form-schema";
import { getActiveProviderExtras } from "../../form-fields";
import { EndpointsSection, GroupsSection, SupportTypesField } from "./provider-schedule-fields";

interface ProviderFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingProvider: Provider | null;
  /** 编辑详情加载中：禁用提交，防止默认 children 全量覆盖真实子表（新建路径恒 false） */
  detailLoading: boolean;
  form: UseFormReturn<ProviderFormValues>;
  providerTemplates: ProviderTemplate[];
  watchedType: string;
  /** 已勾选的出站协议：与主类型共同决定 adapter 额外字段的激活（如主类型 openai + 勾选 anthropic） */
  watchedProtocols: string[] | undefined;
  onSubmit: (values: ProviderFormValues) => void | Promise<void>;
}

export function ProviderFormDialog({
  open,
  onOpenChange,
  editingProvider,
  detailLoading,
  form,
  providerTemplates,
  watchedType,
  watchedProtocols,
  onSubmit,
}: ProviderFormDialogProps) {
  const extraFields = getActiveProviderExtras(watchedType, watchedProtocols);

  // 号池详情子弹窗：state 持在表单层（生命周期跟表单走，不经 page）；
  // 嵌套模式照 ImportCredentialsDialog——子弹窗渲染在 Dialog 根内兄弟位置，父关闭时重置
  const [poolDetail, setPoolDetail] = useState<{ open: boolean; poolId: string | null }>({
    open: false,
    poolId: null,
  });
  const { data: pools = [] } = usePools();
  const poolDetailTarget = poolDetail.poolId
    ? (pools.find((p) => String(p.ID) === poolDetail.poolId) ?? null)
    : null;

  useEffect(() => {
    if (!open) {
      setPoolDetail({ open: false, poolId: null });
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{editingProvider ? "编辑提供商" : "添加提供商"}</DialogTitle>
          <DialogDescription>{editingProvider ? "修改提供商信息" : "添加一个新的提供商"}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="flex min-h-0 flex-1 flex-col gap-4"
          >
            <DialogBody className="-mx-1 min-w-0 space-y-4 px-1">
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

              {/* 支持类型（原「类型」位置）：□ 勾选支持协议；○ 显式指定主类型 */}
              <SupportTypesField providerTemplates={providerTemplates} />

              <FormField
                control={form.control}
                name="base_url"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Base URL</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        autoComplete="url"
                        inputMode="url"
                        placeholder="https://api.openai.com/v1"
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* 顶层 API Key 已废除（S6）：凭据在分组内联 keys / 关联号池中维护 */}

              {/* 主类型 / 已勾选协议激活的 adapter 额外字段（如 anthropic version/beta/auth_type）。
                  协议端点按端点协议实例化 provider，只要勾选 anthropic 协议，这些字段就对
                  anthropic 端点生效——即使主类型是 openai（config 字段为 provider 级共享）。 */}
              {extraFields.length > 0 && (
                <div className="space-y-3">
                  <div>
                    <h3 className="text-sm font-semibold">Anthropic 协议配置</h3>
                    <p className="text-xs text-muted-foreground">
                      作用于 Anthropic 协议端点（版本 / Beta / 认证方式）
                    </p>
                  </div>
                  {extraFields.map((extra) => (
                    <FormField
                      key={extra.name}
                      control={form.control}
                      name={extra.name}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{extra.label}</FormLabel>
                          {extra.kind === "select" ? (
                            <Select onValueChange={field.onChange} value={field.value}>
                              <FormControl>
                                <SelectTrigger>
                                  <SelectValue placeholder="请选择" />
                                </SelectTrigger>
                              </FormControl>
                              <SelectContent>
                                {(extra.options ?? []).map((opt) => (
                                  <SelectItem key={opt.value} value={opt.value}>
                                    {opt.label}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          ) : (
                            <FormControl>
                              <Input {...field} placeholder={extra.placeholder} />
                            </FormControl>
                          )}
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  ))}
                </div>
              )}

              <FormField
                control={form.control}
                name="console"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>控制台地址（可选）</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="https://example.com/console" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="proxy"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>代理地址（可选）</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="http://user:pass@host:port" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="model_endpoint"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border p-3">
                    <div className="space-y-0.5">
                      <FormLabel>模型端点</FormLabel>
                      <div className="text-sm text-muted-foreground">是否支持从上游获取模型列表</div>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="model_filter_enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border p-3">
                    <div className="space-y-0.5">
                      <FormLabel>启用模型过滤</FormLabel>
                      <div className="text-sm text-muted-foreground">同步时只保留符合过滤规则的模型</div>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />

              {/* S0 原型：协议勾选 + 协议端点 + 凭据分组（S6 起改独立 DTO 提交） */}
              <div className="space-y-4 border-t pt-4">
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold">协议端点与凭据分组</h3>
                  <p className="text-xs text-muted-foreground">
                    协议端点（URL 继承/覆盖）与凭据分组（价格权重 / 白名单 / 凭据来源）
                  </p>
                </div>
                <EndpointsSection />
                <GroupsSection onViewPoolDetail={(poolId) => setPoolDetail({ open: true, poolId })} />
              </div>
            </DialogBody>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit" disabled={detailLoading}>
                {detailLoading ? "加载中…" : editingProvider ? "更新" : "创建"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
      <PoolDetailDialog
        open={poolDetail.open}
        onOpenChange={(next) => setPoolDetail((prev) => ({ ...prev, open: next }))}
        pool={poolDetailTarget}
      />
    </Dialog>
  );
}