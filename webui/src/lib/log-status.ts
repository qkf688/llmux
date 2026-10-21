/**
 * 状态值域：与后端 `models.ChatLogStatus*` 常量逐字对齐。
 *
 * 不从 `@/lib/api` 的 `ChatLog["status"]` 取——那个字段是裸 `string`（后端
 * `ChatLog.Status string`），取不出联合类型；字面量联合在本文件单点声明，
 * 后端新增状态时靠「三张 Record 都缺该键」让 tsc 报红。
 */
export type ChatLogStatus = "processing" | "success" | "error" | "canceled" | "interrupted" | "unsent";

/**
 * 请求日志状态的文案与配色单源（值域与后端 `models.ChatLogStatus*` 常量同源，
 * 2026-10-10 起五态：`processing` / `success` / `error` / `canceled` / `interrupted`；
 * 2026-10-21 增第六态 `unsent`（未发出：请求压根没发往上游，见下）。
 *
 * 为什么收在一处：状态从两态扩到五态，散落的 `status === "success" ? 绿 : 红` 判定
 * 漏掉新状态不会报错，只会把「用户点了停止」「进程被 kill」静默画成红色失败——
 * 那是冤枉上游的假结论。展示面按整张 Record 取用后，新增第六个状态会让每个面一起
 * 编译报错，而不是等到线上看错色。
 *
 * 放在 lib 而非 routes/logs 内部：`stores/logs/filters.ts` 的筛选摘要与日志页各渲染面
 * 同用一份，而 stores 不能反向依赖 routes。
 */
const LOG_STATUS_LABELS: Record<ChatLogStatus, string> = {
  processing: "进行中",
  success: "成功",
  error: "失败",
  canceled: "已取消",
  interrupted: "已中断",
  unsent: "未发出",
};

/**
 * 纯文字展示面（表格单元格、详情弹窗）的语义色。
 *
 * 刻意让四个非 success 状态彼此可辨：`canceled`（用户主动撤走）是用户的正常
 * 操作、走中性灰；`interrupted`（进程先走了）意味着服务可能被强杀或崩溃、走
 * warning 橙需要警惕；两者都不是上游的错，都不与真正的 `error` 红色抢同一权重；
 * `processing` 走 info 蓝表示「在途、尚无结论」。
 */
export const LOG_STATUS_TEXT_CLASS: Record<ChatLogStatus, string> = {
  processing: "text-info",
  success: "text-success",
  error: "text-destructive",
  canceled: "text-muted-foreground",
  interrupted: "text-warning",
  unsent: "text-notice",
};

/**
 * 徽标（带底色）展示面的语义色：口径同 TEXT_CLASS——canceled 刻意低对比（用户自己
 * 撤的，无需警惕），interrupted 用 warning 提示「进程侧出了问题」，均不与上游失败
 * 抢同一视觉权重。
 */
export const LOG_STATUS_BADGE_CLASS: Record<ChatLogStatus, string> = {
  processing: "bg-info-tint text-info-foreground",
  success: "bg-success-tint text-success-foreground",
  error: "bg-destructive-tint text-destructive-tint-foreground",
  canceled: "bg-muted text-muted-foreground",
  interrupted: "bg-warning-tint text-warning-foreground",
  unsent: "bg-notice-tint text-notice-foreground",
};

/** 状态清单（筛选下拉按此顺序铺开）：从文案表派生，新增状态不会漏进下拉 */
export const LOG_STATUSES = Object.keys(LOG_STATUS_LABELS) as ChatLogStatus[];

/**
 * 悬停说明（表格状态列的 title）：讲「这个状态意味着什么」，文案本身放不下。
 * 与文案表同源，缺该键时消费面按 undefined 退化成无 title，不显示悬停说明。
 */
const LOG_STATUS_HINTS: Record<ChatLogStatus, string> = {
  processing: "请求已发出，尚未收尾",
  success: "请求已完成",
  error: "失败：上游故障、协议错误或体积超限等",
  canceled: "已取消：调用方主动撤走（客户端断开或点了停止），与上游无关",
  interrupted: "已中断：进程先走了（被强杀或崩溃），请求没有收尾",
  unsent: "未发出：没有可用密钥/端点/分组，请求未发往上游；不是该提供商的问题",
};

export function getLogStatusHint(status: string): string | undefined {
  return LOG_STATUS_HINTS[status as ChatLogStatus];
}

export function getLogStatusLabel(status: string): string {
  return LOG_STATUS_LABELS[status as ChatLogStatus] ?? status;
}

export function getLogStatusTextClass(status: string): string {
  return LOG_STATUS_TEXT_CLASS[status as ChatLogStatus] ?? "text-muted-foreground";
}

export function getLogStatusBadgeClass(status: string): string {
  return LOG_STATUS_BADGE_CLASS[status as ChatLogStatus] ?? "bg-muted text-muted-foreground";
}
