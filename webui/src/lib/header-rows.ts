/**
 * 出站自定义头编辑区纯函数（无 React、无 IO，副作用留在弹窗）。
 * Record ↔ 行数组双向转换 + 预设互斥应用（按名覆盖合并）。
 *
 * 两个域共用（api-keys 弹窗与 model-providers 关联弹窗）：**折叠语义单源**——
 * 空名行跳过、同名（大小写不敏感）后者赢且键名取后者，两弹窗不再各折各的。
 * HTTP 头名大小写不敏感（RFC 9110），三处判等统一 toLowerCase——
 * 与后端 chatcore.BuildHeaders 的 header.Set 规范化行为同口径。
 * 最终防线在后端写入闸 common/outboundheaders.go（SanitizeOutboundHeaders，
 * 语法关 + 规范化 Title-Case + 撞名拒绝），此处折叠与去重只优化 UI 语义：
 * 前端故意只校验到形状层（半截输入不打断保存），语法错误由服务端 400 回显。
 */

export type HeaderRow = { key: string; value: string };

/** API 对象 → 编辑行（null/undefined → 空表；键序即 Object.keys 序）。 */
export function headersToRows(headers: Record<string, string> | null | undefined): HeaderRow[] {
  if (!headers) return [];
  return Object.entries(headers).map(([key, value]) => ({ key, value }));
}

/** 编辑行 → API 对象：空名行跳过；同名（大小写不敏感）后者赢且键名取后者。 */
export function rowsToHeaders(rows: HeaderRow[]): Record<string, string> {
  const record: Record<string, string> = {};
  for (const row of rows) {
    const name = row.key.trim();
    if (!name) continue;
    const lower = name.toLowerCase();
    for (const existing of Object.keys(record)) {
      if (existing.toLowerCase() === lower) delete record[existing];
    }
    record[name] = row.value;
  }
  return record;
}

/**
 * 预设应用（Rulings 2 修订 2026-09-16：预设间互斥，换伪装对象 = 换整套头族）：
 * 1. 先移除命中 foreignPresets（其它预设）头名的行——点 Codex 再点 Claude 不残留
 *    Codex 独有头；用户手配行只要不撞任何预设清单就不受影响。
 * 2. 再按名覆盖合并当前预设：撞名行换成预设的名与值（不重复堆行），预设独有项追加。
 *    同一预设重复点幂等；想连手配行一起清掉用编辑区的「清空」按钮。
 */
export function applyPresetRows(
  current: HeaderRow[],
  preset: HeaderRow[],
  foreignPresets: HeaderRow[][],
): HeaderRow[] {
  const presetKeys = new Set(preset.map((item) => item.key.trim().toLowerCase()));
  const foreignKeys = new Set<string>();
  for (const foreign of foreignPresets) {
    for (const item of foreign) {
      const lower = item.key.trim().toLowerCase();
      if (!presetKeys.has(lower)) foreignKeys.add(lower);
    }
  }
  const merged = current
    .filter((row) => !foreignKeys.has(row.key.trim().toLowerCase()))
    .map((row) => {
      const hit = preset.find((p) => p.key.toLowerCase() === row.key.trim().toLowerCase());
      return hit ? { key: hit.key, value: hit.value } : row;
    });
  for (const item of preset) {
    const lower = item.key.toLowerCase();
    if (!merged.some((row) => row.key.trim().toLowerCase() === lower)) {
      merged.push({ key: item.key, value: item.value });
    }
  }
  return merged;
}
