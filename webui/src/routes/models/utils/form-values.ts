import type { Model } from "@/lib/api";
import type { ModelFormValues } from "../schemas/forms";

export const defaultModelFormValues: ModelFormValues = {
  name: "",
  remark: "",
  io_log: false,
  auto_associate: true,
  supports_thinking: false,
  thinking_levels: [],
};

export const toModelFormValues = (model: Model): ModelFormValues => ({
  name: model.Name,
  remark: model.Remark,
  io_log: model.IOLog,
  auto_associate: model.auto_associate,
  supports_thinking: model.supports_thinking ?? false,
  thinking_levels: model.thinking_levels ?? [],
});
