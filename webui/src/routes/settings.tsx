import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { HealthCheckSettings, Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";
import { RoutingSettings } from "./settings/routing-settings";
import { BalancerSettings } from "./settings/balancer-settings";
import { LogsSettings } from "./settings/logs-settings";
import { HealthCheckSettingsTab } from "./settings/health-check-settings";
import { useSettings, useHealthCheckSettingsQuery, settingsKeys, healthCheckSettingsKeys } from "@/hooks/api/use-providers";

export default function SettingsPage() {
  const queryClient = useQueryClient();
  const { data: settings = null, isLoading: settingsLoading } = useSettings();
  const { data: healthCheckSettings = null, isLoading: healthCheckLoading } = useHealthCheckSettingsQuery();

  const loading = settingsLoading || healthCheckLoading;

  const handleSettingsChange = useCallback((_newSettings: Settings) => {
    void queryClient.invalidateQueries({ queryKey: settingsKeys.all });
  }, [queryClient]);

  const handleHealthCheckSettingsChange = useCallback((_newSettings: HealthCheckSettings) => {
    void queryClient.invalidateQueries({ queryKey: healthCheckSettingsKeys.all });
  }, [queryClient]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="container mx-auto py-4 md:py-6 space-y-4 md:space-y-6 max-w-5xl px-4">
        <div>
          <h1 className="text-xl md:text-2xl font-bold">系统设置</h1>
          <p className="text-sm text-muted-foreground">管理系统全局配置</p>
        </div>

        <Tabs defaultValue="general" className="w-full">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="general">通用</TabsTrigger>
            <TabsTrigger value="balancer">负载均衡</TabsTrigger>
            <TabsTrigger value="logs">日志</TabsTrigger>
            <TabsTrigger value="health-check">健康检测</TabsTrigger>
            <TabsTrigger value="about">关于</TabsTrigger>
          </TabsList>

          <TabsContent value="general" className="mt-4 md:mt-6">
            <RoutingSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="balancer" className="mt-4 md:mt-6">
            <BalancerSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="logs" className="mt-4 md:mt-6">
            <LogsSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="health-check" className="mt-4 md:mt-6">
            <HealthCheckSettingsTab
              healthCheckSettings={healthCheckSettings}
              onHealthCheckSettingsChange={handleHealthCheckSettingsChange}
            />
          </TabsContent>

          <TabsContent value="about" className="mt-4 md:mt-6">
            <div className="space-y-4 md:space-y-6">
              <div>
                <h2 className="text-lg md:text-xl font-semibold">关于</h2>
                <p className="text-xs md:text-sm text-muted-foreground">系统信息</p>
              </div>
              <Card>
                <CardContent className="pt-6">
                  <div className="space-y-2 text-sm text-muted-foreground">
                    <p><strong>LLMUX</strong> - LLM 代理网关</p>
                    <p>支持多供应商负载均衡、请求路由和监控</p>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
