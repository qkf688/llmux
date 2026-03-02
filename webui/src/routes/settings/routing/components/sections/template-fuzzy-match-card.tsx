import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { TagInput } from "@/components/ui/tag-input";
import type { RoutingSettingsSectionProps } from "../../types";

export function TemplateFuzzyMatchCard({
  localSettings,
  updateLocalSettings,
}: RoutingSettingsSectionProps) {
  return (
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
              updateLocalSettings({ template_fuzzy_match_enabled: checked });
            }}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="template-fuzzy-match-separators" className="text-sm">
            分隔符
          </Label>
          <TagInput
            value={localSettings?.template_fuzzy_match_separators ?? []}
            onChange={(separators) => {
              updateLocalSettings({ template_fuzzy_match_separators: separators });
            }}
            placeholder="输入分隔符后按回车，如 : 或 -"
          />
          <p className="text-xs md:text-sm text-muted-foreground">
            允许的分隔符（如 : 和 -）。点击标签上的 × 可删除，按 Backspace 删除最后一个
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="template-fuzzy-match-suffixes" className="text-sm">
            后缀关键词
          </Label>
          <TagInput
            value={localSettings?.template_fuzzy_match_suffixes ?? []}
            onChange={(suffixes) => {
              updateLocalSettings({ template_fuzzy_match_suffixes: suffixes });
            }}
            placeholder="输入关键词后按回车，如 free 或 beta"
          />
          <p className="text-xs md:text-sm text-muted-foreground">
            允许的后缀关键词（如 free、preview、beta）。点击标签上的 × 可删除，按 Backspace 删除最后一个
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
