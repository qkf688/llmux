import type { UseFormReturn } from "react-hook-form";
import { Button } from "@/components/ui/button";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { Model, VirtualModelMapping } from "@/lib/api";
import type { MappingFormValues } from "../../../schemas/forms";

interface MappingFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  form: UseFormReturn<MappingFormValues>;
  editingMapping: VirtualModelMapping | null;
  realModels: Model[];
  onSubmit: (values: MappingFormValues) => void;
}

const parseInteger = (value: string) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
};

export function MappingFormDialog({
  open,
  onOpenChange,
  form,
  editingMapping,
  realModels,
  onSubmit,
}: MappingFormDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editingMapping ? "编辑映射" : "添加映射"}</DialogTitle>
          <DialogDescription>选择真实模型，并配置优先级、权重与启用状态</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((values) => {
              onSubmit(values);
            })}
            className="space-y-4"
          >
            <FormField
              control={form.control}
              name="real_model_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>真实模型</FormLabel>
                  <Select
                    value={field.value > 0 ? String(field.value) : undefined}
                    onValueChange={(value) => field.onChange(parseInteger(value))}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择真实模型" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {realModels.map((model) => (
                        <SelectItem key={model.ID} value={String(model.ID)}>
                          {model.Name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid grid-cols-2 gap-4">
              <FormField
                control={form.control}
                name="priority"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>优先级</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        value={field.value}
                        onChange={(event) => field.onChange(parseInteger(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="weight"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>权重</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        value={field.value}
                        onChange={(event) => field.onChange(parseInteger(event.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name="enabled"
              render={({ field }) => (
                <FormItem className="flex items-center gap-2">
                  <FormLabel>启用</FormLabel>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit">{editingMapping ? "更新" : "添加"}</Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
