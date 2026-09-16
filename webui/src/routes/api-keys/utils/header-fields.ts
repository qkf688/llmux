/**
 * headers 编辑区纯函数（无 React、无 IO，副作用留在弹窗）：
 * Record ↔ 行数组双向转换 + 预设按名覆盖合并。
 * HTTP 头名大小写不敏感（RFC 9110），三处判等统一 toLowerCase——
 * 与后端 chatcore.BuildHeaders 的 header.Set 规范化行为同口径。
 * 注：后端 apikeys 写入闸（headers_guard.go）是最终防线，此处去重只优化 UI 语义。
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
 * 预设按名覆盖合并（Rulings 2）：撞名行换成预设的名与值（不重复堆行），
 * 其余手配行保留，预设独有项追加。再点一次幂等——保护用户已改的其它行，
 * 想全量重来用编辑区的「清空」按钮。
 */
export function mergePresetRows(current: HeaderRow[], preset: HeaderRow[]): HeaderRow[] {
  const merged = current.map((row) => {
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
