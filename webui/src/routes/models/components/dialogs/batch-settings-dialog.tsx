import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
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
import type { UseFormReturn } from "react-hook-form";
import type { BatchUpdateValues } from "../../schemas/forms";
import type { ValueRange } from "../../types";

interface BatchSettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedCount: number;
  maxRetryRange: ValueRange;
  timeOutRange: ValueRange;
  form: UseFormReturn<BatchUpdateValues>;
  updating: boolean;
  onSubmit: (values: BatchUpdateValues) => void | Promise<void>;
}

export function BatchSettingsDialog({
  open,
  onOpenChange,
  selectedCount,
  maxRetryRange,
  timeOutRange,
  form,
  updating,
  onSubmit,
}: BatchSettingsDialogProps) {
  const enableMaxRetry = form.watch("enableMaxRetry");
  const enableTimeOut = form.watch("enableTimeOut");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>批量设置模型参数</DialogTitle>
          <DialogDescription>为选中的 {selectedCount} 个模型统一设置重试次数和超时时间</DialogDescription>
        </DialogHeader>

        <div className="rounded-lg border bg-muted/50 p-4 space-y-2">
          <div className="text-sm">
            <span className="text-muted-foreground">已选中：</span>
            <span className="font-medium">{selectedCount} 个模型</span>
          </div>
          <div className="text-sm">
            <span className="text-muted-foreground">当前重试次数范围：</span>
            <span className="font-medium">
              {maxRetryRange.min === maxRetryRange.max
                ? maxRetryRange.min
                : `${maxRetryRange.min} - ${maxRetryRange.max}`}
            </span>
          </div>
          <div className="text-sm">
            <span className="text-muted-foreground">当前超时时间范围：</span>
            <span className="font-medium">
              {timeOutRange.min === timeOutRange.max
                ? `${timeOutRange.min} 秒`
                : `${timeOutRange.min} - ${timeOutRange.max} 秒`}
            </span>
          </div>
        </div>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <div className="flex items-center gap-4">
              <FormField
                control={form.control}
                name="enableMaxRetry"
                render={({ field }) => (
                  <FormItem className="flex items-center space-x-2 space-y-0">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="max_retry"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>重试次数限制</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        {...field}
                        onChange={(event) => field.onChange(Number(event.target.value))}
                        disabled={!enableMaxRetry}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className="flex items-center gap-4">
              <FormField
                control={form.control}
                name="enableTimeOut"
                render={({ field }) => (
                  <FormItem className="flex items-center space-x-2 space-y-0">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="time_out"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>超时时间(秒)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        {...field}
                        onChange={(event) => field.onChange(Number(event.target.value))}
                        disabled={!enableTimeOut}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <p className="text-xs text-muted-foreground">只有勾选的字段才会被更新</p>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={updating}>
                取消
              </Button>
              <Button type="submit" disabled={updating || (!enableMaxRetry && !enableTimeOut)}>
                {updating ? "更新中..." : "确认设置"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
