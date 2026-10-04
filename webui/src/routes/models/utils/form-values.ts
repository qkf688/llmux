import type { Model } from "@/lib/api";
import type { ModelFormValues } from "../schemas/forms";

export const defaultModelFormValues: ModelFormValues = {
  name: "",
  remark: "",
  io_log: false,
  auto_associate: true,
  supports_thinking: false,
  thinking_levels: [],
  // 能力真值默认 false：与后端 models.Model 的二态零值一致，
  // 新建模型不猜能力，由用户按官方文档勾选。
  tool_call: false,
  structured_output: false,
  image: false,
};

export const toModelFormValues = (model: Model): ModelFormValues => ({
  name: model.Name,
  remark: model.Remark,
  io_log: model.IOLog,
  auto_associate: model.auto_associate,
  supports_thinking: model.supports_thinking ?? false,
  thinking_levels: model.thinking_levels ?? [],
  // ?? false 兜底：键恒在（后端直返实体），但 mock/旧数据缺键时不应崩在表单里。
  tool_call: model.tool_call ?? false,
  structured_output: model.structured_output ?? false,
  image: model.image ?? false,
});
