import type { Model } from "@/lib/api";

export const buildModelUpdatePayload = (model: Model) => ({
  name: model.Name,
  remark: model.Remark,
  io_log: model.IOLog,
  auto_associate: model.auto_associate,
  supports_thinking: model.supports_thinking,
  thinking_levels: model.thinking_levels ?? [],
});
