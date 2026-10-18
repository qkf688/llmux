/**
 * 协议展示名的**单一来源**：供应商表单的协议勾选/端点区与模型测试的协议矩阵共用一张表。
 *
 * 为什么下沉到 lib 而不是各路由各写一份：这两处读的是同一批协议值
 * （openai / responses / anthropic），一旦某处加协议另一处漏改，用户会看到「勾了但测不到」
 * 或「测出一行没名字的协议」，而没有任何检查会红。展示名不是业务规则，但它是**契约**：
 * 后端回包给的是协议键，前端各处都靠这张表翻译。
 *
 * 取值沿用供应商表单既有文案（openai 显示「OpenAI」），本次不借机改任何页面的展示文字。
 */
export const PROTOCOL_LABEL: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  responses: "OpenAI Responses",
};

/** 协议键 → 展示名；未知键原样返回（后端加新协议时这里不至于显示成空白）。 */
export function protocolLabel(protocol: string): string {
  return PROTOCOL_LABEL[protocol] ?? protocol;
}
