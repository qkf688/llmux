import type { Model } from "@/lib/api";
import type { BatchUpdateValues, ModelFormValues } from "../schemas/forms";

export const defaultModelFormValues: ModelFormValues = {
  name: "",
  remark: "",
  max_retry: 10,
  time_out: 60,
  io_log: false,
  auto_associate: true,
};

export const defaultBatchUpdateValues: BatchUpdateValues = {
  enableMaxRetry: true,
  enableTimeOut: true,
  max_retry: 10,
  time_out: 60,
};

export const toModelFormValues = (model: Model): ModelFormValues => ({
  name: model.Name,
  remark: model.Remark,
  max_retry: model.MaxRetry,
  time_out: model.TimeOut,
  io_log: model.IOLog,
  auto_associate: model.auto_associate,
});
