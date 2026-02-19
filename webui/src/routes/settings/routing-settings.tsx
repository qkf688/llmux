import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TagInput } from "@/components/ui/tag-input";
import { toast } from "sonner";
import { updateSettings } from "@/lib/api";
import type { Settings } from "@/lib/api";
import { Spinner } from "@/components/ui/spinner";

interface RoutingSettingsProps {
  settings: Settings | null;
  onSettingsChange: (settings: Settings) => void;
}

export function RoutingSettings({ settings, onSettingsChange }: RoutingSettingsProps) {
  const [saving, setSaving] = useState(false);
  const [localSettings, setLocalSettings] = useState(settings);
  const [hasChanges, setHasChanges] = useState(false);

  const handleStrictCapabilityMatchChange = (checked: boolean) => {
    if (localSettings) {
      const newSettings = { ...localSettings, strict_capability_match: checked };
      setLocalSettings(newSettings);
      setHasChanges(true);
    }
  };

  const handleSave = async () => {
    if (!localSettings) return;

    try {
      setSaving(true);
      // 清理模型过滤规则：去除空行和首尾空格
      const cleanedSettings = {
        ...localSettings,
        model_sync_filter_rules: localSettings.model_sync_filter_rules
          ?.map(r => r.trim())
          .filter(Boolean) ?? []
      };
      const updated = await updateSettings(cleanedSettings);
      setLocalSettings(updated);
      onSettingsChange(updated);
      setHasChanges(false);
      toast.success("通用设置保存成功");
    } catch (error) {
      toast.error("保存设置失败: " + (error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setLocalSettings(settings);
    setHasChanges(false);
  };

  return (
    <div className="space-y-4 md:space-y-6">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h2 className="text-lg md:text-xl font-semibold">通用设置</h2>
          <p className="text-xs md:text-sm text-muted-foreground">配置系统通用选项</p>
        </div>
        <div className="flex gap-1.5 md:gap-2">
          <Button
            variant="outline"
            onClick={handleReset}
            disabled={!hasChanges || saving}
          >
            重置
          </Button>
          <Button
            onClick={handleSave}
            disabled={!hasChanges || saving}
          >
            {saving ? <Spinner className="w-4 h-4 mr-2" /> : null}
            保存
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>能力匹配</CardTitle>
          <CardDescription>
            配置请求路由时的能力匹配策略
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="strict-capability-match" className="text-sm md:text-base font-medium">
                严格能力匹配
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统会根据请求的能力需求（工具调用、结构化输出、图片处理）筛选供应商。
                <br />
                关闭后，系统将忽略能力匹配条件，允许请求发送到任何启用的供应商。
              </p>
            </div>
            <Switch
              id="strict-capability-match"
              checked={localSettings?.strict_capability_match ?? true}
              onCheckedChange={handleStrictCapabilityMatchChange}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>模型自动同步</CardTitle>
          <CardDescription>
            配置上游模型自动同步选项
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="model-sync-enabled" className="text-sm md:text-base font-medium">
                启用自动同步
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统将定期自动同步启用模型端点的提供商的上游模型列表
              </p>
            </div>
            <Switch
              id="model-sync-enabled"
              checked={localSettings?.model_sync_enabled ?? false}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, model_sync_enabled: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="model-sync-interval" className="text-sm">同步间隔（小时）</Label>
            <Input
              id="model-sync-interval"
              type="number"
              min="1"
              value={localSettings?.model_sync_interval ?? 12}
              onChange={(e) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, model_sync_interval: parseInt(e.target.value) || 12 });
                  setHasChanges(true);
                }
              }}
            />
            <p className="text-xs md:text-sm text-muted-foreground">默认12小时同步一次</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="model-sync-log-retention-count" className="text-sm">日志保留条数</Label>
            <Input
              id="model-sync-log-retention-count"
              type="number"
              min="0"
              value={localSettings?.model_sync_log_retention_count ?? 100}
              onChange={(e) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, model_sync_log_retention_count: parseInt(e.target.value) ?? 100 });
                  setHasChanges(true);
                }
              }}
            />
            <p className="text-xs md:text-sm text-muted-foreground">默认保留100条，0表示不限制</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="model-sync-log-retention-days" className="text-sm">日志保留天数</Label>
            <Input
              id="model-sync-log-retention-days"
              type="number"
              min="0"
              value={localSettings?.model_sync_log_retention_days ?? 7}
              onChange={(e) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, model_sync_log_retention_days: parseInt(e.target.value) ?? 7 });
                  setHasChanges(true);
                }
              }}
            />
            <p className="text-xs md:text-sm text-muted-foreground">默认保留7天，0表示不限制</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="model-sync-filter-rules" className="text-sm">模型过滤规则</Label>
            <TagInput
              value={localSettings?.model_sync_filter_rules ?? []}
              onChange={(rules) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, model_sync_filter_rules: rules });
                  setHasChanges(true);
                }
              }}
              placeholder="输入规则后按回车，如 :free 或 -free"
            />
            <p className="text-xs md:text-sm text-muted-foreground">
              只同步包含这些规则的模型。点击标签上的 × 可删除，按 Backspace 删除最后一个
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>模板模糊匹配</CardTitle>
          <CardDescription>
            配置模板匹配时的模糊规则，允许 gpt-5 匹配 gpt-5:free 等变体
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="template-fuzzy-match-enabled" className="text-sm md:text-base font-medium">
                启用模糊匹配
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，模板名称可以匹配带后缀的模型（如 gpt-5 匹配 gpt-5:free）
              </p>
            </div>
            <Switch
              id="template-fuzzy-match-enabled"
              checked={localSettings?.template_fuzzy_match_enabled ?? false}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, template_fuzzy_match_enabled: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="template-fuzzy-match-separators" className="text-sm">分隔符</Label>
            <TagInput
              value={localSettings?.template_fuzzy_match_separators ?? []}
              onChange={(separators) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, template_fuzzy_match_separators: separators });
                  setHasChanges(true);
                }
              }}
              placeholder="输入分隔符后按回车，如 : 或 -"
            />
            <p className="text-xs md:text-sm text-muted-foreground">
              允许的分隔符（如 : 和 -）。点击标签上的 × 可删除，按 Backspace 删除最后一个
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="template-fuzzy-match-suffixes" className="text-sm">后缀关键词</Label>
            <TagInput
              value={localSettings?.template_fuzzy_match_suffixes ?? []}
              onChange={(suffixes) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, template_fuzzy_match_suffixes: suffixes });
                  setHasChanges(true);
                }
              }}
              placeholder="输入关键词后按回车，如 free 或 beta"
            />
            <p className="text-xs md:text-sm text-muted-foreground">
              允许的后缀关键词（如 free、preview、beta）。点击标签上的 × 可删除，按 Backspace 删除最后一个
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>模型关联自动化</CardTitle>
          <CardDescription>
            配置模型关联的自动添加和清理功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="auto-associate-on-add" className="text-sm md:text-base font-medium">
                添加时自动关联
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                新增提供商添加"全部模型"时，或提供商增加模型时，自动关联到模板匹配的模型（包含 Model.Name、既有关联 ProviderModel 与手动模板项）
              </p>
            </div>
            <Switch
              id="auto-associate-on-add"
              checked={localSettings?.auto_associate_on_add ?? false}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, auto_associate_on_add: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="auto-clean-on-delete" className="text-sm md:text-base font-medium">
                删除时自动清理
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                提供商被删除或模型减少时，自动清除无效的模型关联
              </p>
            </div>
            <Switch
              id="auto-clean-on-delete"
              checked={localSettings?.auto_clean_on_delete ?? false}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, auto_clean_on_delete: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="auto-save-template-on-associate" className="text-sm md:text-base font-medium">
                关联模型时自动保存到模板
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                创建模型关联时，自动将 ProviderModel 保存到模板项中，方便后续自动关联和模型管理
              </p>
            </div>
            <Switch
              id="auto-save-template-on-associate"
              checked={localSettings?.auto_save_template_on_associate ?? false}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, auto_save_template_on_associate: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>格式转换</CardTitle>
          <CardDescription>
            配置 API 格式转换功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="enable-format-conversion" className="text-sm md:text-base font-medium">
                启用格式转换
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统允许在不同 API 格式间转换（如 OpenAI ↔ Anthropic）。
                <br />
                关闭后只能使用与提供商类型匹配的格式，可减少转换开销，提升性能。
              </p>
            </div>
            <Switch
              id="enable-format-conversion"
              checked={localSettings?.enable_format_conversion ?? true}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, enable_format_conversion: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
