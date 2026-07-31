import type { Model, ModelWithProvider, Provider } from "./api";

/**
 * 稳定空数组，仅作只读 fallback，避免 `data = []` 在 loading 时每次 render 产生新引用触发 effect 循环。
 * 多个页面共享同一引用，Object.freeze 冻结防止误 mutate 污染所有消费方；
 * `as unknown as` 将 freeze 的 readonly 返回刻意放宽为可变数组类型，以兼容下游 `Model[]` 签名。
 */
export const EMPTY_MODELS: Model[] = Object.freeze([]) as unknown as Model[];
export const EMPTY_PROVIDERS: Provider[] = Object.freeze([]) as unknown as Provider[];
export const EMPTY_MODEL_PROVIDERS: ModelWithProvider[] = Object.freeze([]) as unknown as ModelWithProvider[];
