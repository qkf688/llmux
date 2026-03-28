import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

type CapabilityMode = "keep" | "enable" | "disable";

type BatchCapabilitiesDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedCount: number;
  updating: boolean;
  onConfirm: (capabilities: { tool_call?: boolean; structured_output?: boolean; image?: boolean }) => Promise<void>;
};

function modeToOptionalBool(mode: CapabilityMode): boolean | undefined {
  if (mode === "keep") return undefined;
  return mode === "enable";
}

export function BatchCapabilitiesDialog({
  open,
  onOpenChange,
  selectedCount,
  updating,
  onConfirm,
}: BatchCapabilitiesDialogProps) {
  const [toolCall, setToolCall] = useState<CapabilityMode>("keep");
  const [structuredOutput, setStructuredOutput] = useState<CapabilityMode>("keep");
  const [image, setImage] = useState<CapabilityMode>("keep");

  useEffect(() => {
    if (!open) return;
    setToolCall("keep");
    setStructuredOutput("keep");
    setImage("keep");
  }, [open]);

  const payload = useMemo(
    () => ({
      tool_call: modeToOptionalBool(toolCall),
      structured_output: modeToOptionalBool(structuredOutput),
      image: modeToOptionalBool(image),
    }),
    [image, structuredOutput, toolCall]
  );

  const hasAnyChange = payload.tool_call !== undefined || payload.structured_output !== undefined || payload.image !== undefined;
  const submitDisabled = updating || selectedCount === 0 || !hasAnyChange;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>批量设置能力</DialogTitle>
          <DialogDescription>
            对选中的 {selectedCount} 个关联批量更新能力字段，“保持不变”不会修改现有值
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="rounded-md border p-4 space-y-3">
            <div className="text-sm font-medium">工具调用</div>
            <RadioGroup value={toolCall} onValueChange={(value) => setToolCall(value as CapabilityMode)} className="flex gap-4">
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="keep" id="tool-call-keep" />
                <Label htmlFor="tool-call-keep">保持不变</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="enable" id="tool-call-enable" />
                <Label htmlFor="tool-call-enable">启用</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="disable" id="tool-call-disable" />
                <Label htmlFor="tool-call-disable">停用</Label>
              </div>
            </RadioGroup>
          </div>

          <div className="rounded-md border p-4 space-y-3">
            <div className="text-sm font-medium">结构化输出</div>
            <RadioGroup
              value={structuredOutput}
              onValueChange={(value) => setStructuredOutput(value as CapabilityMode)}
              className="flex gap-4"
            >
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="keep" id="structured-output-keep" />
                <Label htmlFor="structured-output-keep">保持不变</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="enable" id="structured-output-enable" />
                <Label htmlFor="structured-output-enable">启用</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="disable" id="structured-output-disable" />
                <Label htmlFor="structured-output-disable">停用</Label>
              </div>
            </RadioGroup>
          </div>

          <div className="rounded-md border p-4 space-y-3">
            <div className="text-sm font-medium">视觉</div>
            <RadioGroup value={image} onValueChange={(value) => setImage(value as CapabilityMode)} className="flex gap-4">
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="keep" id="image-keep" />
                <Label htmlFor="image-keep">保持不变</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="enable" id="image-enable" />
                <Label htmlFor="image-enable">启用</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="disable" id="image-disable" />
                <Label htmlFor="image-disable">停用</Label>
              </div>
            </RadioGroup>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={updating}>
            取消
          </Button>
          <Button
            type="button"
            disabled={submitDisabled}
            onClick={async () => {
              await onConfirm(payload);
            }}
          >
            {updating ? "提交中..." : "确认"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

