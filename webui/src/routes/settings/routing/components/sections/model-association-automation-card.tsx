import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { RoutingSettingsSectionProps } from "../../types";

export function ModelAssociationAutomationCard({
  localSettings,
  updateLocalSettings,
}: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>模型关联自动化</CardTitle>
        <CardDescription>配置模型关联的自动添加和清理功能</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="auto-associate-on-add" className="text-sm md:text-base font-medium">
              添加时自动关联
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              新增提供商添加&quot;全部模型&quot;时，或提供商增加模型时，自动关联到模板匹配的模型（包含
              Model.Name、既有关联 ProviderModel 与手动模板项）
            </p>
          </div>
          <Switch
            id="auto-associate-on-add"
            checked={localSettings?.auto_associate_on_add ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ auto_associate_on_add: checked });
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
              updateLocalSettings({ auto_clean_on_delete: checked });
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
              updateLocalSettings({ auto_save_template_on_associate: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}
