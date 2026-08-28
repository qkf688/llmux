import { Eye, EyeOff } from "lucide-react";
import type { UseFormReturn } from "react-hook-form";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { Provider, ProviderTemplate } from "@/lib/api";
import type { ProviderFormValues } from "../../form-schema";
import { getProviderExtraFields } from "../../form-fields";
import { EndpointsSection, GroupsSection, SupportTypesField } from "./provider-schedule-fields";

interface ProviderFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingProvider: Provider | null;
  form: UseFormReturn<ProviderFormValues>;
  providerTemplates: ProviderTemplate[];
  watchedType: string;
  showApiKey: boolean;
  toggleShowApiKey: () => void;
  onSubmit: (values: ProviderFormValues) => void | Promise<void>;
}

export function ProviderFormDialog({
  open,
  onOpenChange,
  editingProvider,
  form,
  providerTemplates,
  watchedType,
  showApiKey,
  toggleShowApiKey,
  onSubmit,
}: ProviderFormDialogProps) {
  const extraFields = getProviderExtraFields(watchedType);

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

              {/* 支持类型（原「类型」位置）：多选勾选，第一个 = 主类型 */}
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

              <FormField
                control={form.control}
                name="api_key"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>API Key</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Input
                          {...field}
                          type={showApiKey ? "text" : "password"}
                          autoComplete="new-password"
                          placeholder="sk-..."
                          className="pr-10"
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="absolute right-1.5 top-1/2 -translate-y-1/2 h-8 w-8"
                          onClick={toggleShowApiKey}
                          aria-label={showApiKey ? "隐藏 API Key" : "显示 API Key"}
                        >
                          {showApiKey ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* type schema 驱动的额外字段（如 anthropic version/beta/auth_type） */}
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
                <GroupsSection />
              </div>
            </DialogBody>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit">{editingProvider ? "更新" : "创建"}</Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}