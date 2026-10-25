/** 排行类名次徽章的配色：15% tint 底 + 文字安全深档。
 *
 *  文字色必须用 --chart-N-strong（亮色下的加深档）而非 --chart-N 本体——
 *  本体在 15% tint 底上实测只有 2.5~3.5:1，低于 WCAG 正文 4.5:1，
 *  实测口径见 index.css 的 -strong 注释。第 4 名起退回灰色。 */
export function rankToneClass(rank: number) {
  if (rank === 1) return "bg-[color:var(--chart-7)]/15 text-[color:var(--chart-7-strong)]";
  if (rank === 2) return "bg-[color:var(--chart-6)]/15 text-[color:var(--chart-6-strong)]";
  if (rank === 3) return "bg-[color:var(--chart-8)]/15 text-[color:var(--chart-8-strong)]";
  return "bg-muted/60 text-muted-foreground";
}
