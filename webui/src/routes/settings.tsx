import { useState, useEffect } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "sonner";
import { getSettings, getHealthCheckSettings } from "@/lib/api";
import type { Settings, HealthCheckSettings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";
import { RoutingSettings } from "./settings/routing-settings";
import { BalancerSettings } from "./settings/balancer-settings";
import { LogsSettings } from "./settings/logs-settings";
import { HealthCheckSettingsTab } from "./settings/health-check-settings";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [healthCheckSettings, setHealthCheckSettings] = useState<HealthCheckSettings | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadSettings();
    loadHealthCheckSettings();
  }, []);

  const loadSettings = async () => {
    try {
      setLoading(true);
      const data = await getSettings();
      setSettings(data);
    } catch (error) {
      toast.error("加载设置失败: " + (error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const loadHealthCheckSettings = async () => {
    try {
      const data = await getHealthCheckSettings();
      setHealthCheckSettings(data);
    } catch (error) {
      toast.error("加载健康检测设置失败: " + (error as Error).message);
    }
  };

  const handleSettingsChange = (newSettings: Settings) => {
    setSettings(newSettings);
  };

  const handleHealthCheckSettingsChange = (newSettings: HealthCheckSettings) => {
    setHealthCheckSettings(newSettings);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="container mx-auto py-6 space-y-6 max-w-5xl">
        <div>
          <h1 className="text-2xl font-bold">系统设置</h1>
          <p className="text-muted-foreground">管理系统全局配置</p>
        </div>

        <Tabs defaultValue="general" className="w-full">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="general">通用</TabsTrigger>
            <TabsTrigger value="balancer">负载均衡</TabsTrigger>
            <TabsTrigger value="logs">日志</TabsTrigger>
            <TabsTrigger value="health-check">健康检测</TabsTrigger>
            <TabsTrigger value="about">关于</TabsTrigger>
          </TabsList>

          <TabsContent value="general" className="mt-6">
            <RoutingSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="balancer" className="mt-6">
            <BalancerSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="logs" className="mt-6">
            <LogsSettings
              settings={settings}
              onSettingsChange={handleSettingsChange}
            />
          </TabsContent>

          <TabsContent value="health-check" className="mt-6">
            <HealthCheckSettingsTab
              healthCheckSettings={healthCheckSettings}
              onHealthCheckSettingsChange={handleHealthCheckSettingsChange}
            />
          </TabsContent>

          <TabsContent value="about" className="mt-6">
            <div className="space-y-6">
              <div>
                <h2 className="text-xl font-semibold">关于</h2>
                <p className="text-sm text-muted-foreground">系统信息</p>
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
