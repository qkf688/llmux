import type { Model } from "@/lib/api";

export const buildModelUpdatePayload = (model: Model) => ({
  name: model.Name,
  remark: model.Remark,
  io_log: model.IOLog,
  auto_associate: model.auto_associate,
  supports_thinking: model.supports_thinking,
  thinking_levels: model.thinking_levels ?? [],
  // 能力真值：随表单整份提交（后端条件写，不发字段=保留存量）。
  tool_call: model.tool_call,
  structured_output: model.structured_output,
  image: model.image,
});
