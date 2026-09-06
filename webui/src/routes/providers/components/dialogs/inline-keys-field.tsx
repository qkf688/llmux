/**
 * 内联 key 输入/显示（掩码化安全浏览，2026-09-06 对齐号池 KeyMasked 体系）：
 * - 详情回填 = 服务端掩码行（masked=true，只读，保存按 id 保留，明文永不出常规响应）；
 * - 点「显示」经受控 raw 端点拉取该组明文，整组切换为可编辑明文行（masked=false）；
 * - 新增/粘贴的行直接是明文态；掩码行不可单独编辑（要改必须先显示）；
 * - 实时计数徽标：共 N 条 / 明文行重复 M 条（重复仅提示，提交时后端 normalize 显式 400 拒绝）；
 * - 批量粘贴口：换行/逗号/空格/制表自适应拆分（splitInlineKeys），所见即所得防静默并 key；
 * - 空明文行警示条：保存时被忽略（对齐后端 normalizeInlineKeys 空条目兜底）。
 *
 * 行形态契约（ProviderFormInlineKeyRow）：value 恒为该数组；onChange 吐整个数组，
 * 保存时 buildProviderPayload 把 masked 行映射为 {id, key:""}（保留）、明文行映射为 {id, key}。
 */
import { ClipboardPaste, Eye, Plus, Trash2, Loader2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { FormLabel } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { getProviderGroupInlineKeysRaw } from "@/lib/api";
import type { ProviderFormInlineKeyRow } from "../../form-schema";
import { deriveInlineKeyStats, splitInlineKeys } from "../../utils/inline-keys";

type InlineKeysFieldProps = {
  /** 表单值（行数组）。由外层 FormField 注入，外部 reset 回填时经 effect 同步内部行数组 */
  value: ProviderFormInlineKeyRow[];
  onChange: (next: ProviderFormInlineKeyRow[]) => void;
  /** 供应商 ID：既有分组的「显示」明文拉取定位（新建无既有 key 时为空） */
  providerId?: number;
  /** 既有分组 ID：详情回填时经 groupId 填充，「显示」raw 端点定位用 */
  groupId?: number;
};

export function InlineKeysField({ value, onChange, providerId, groupId }: InlineKeysFieldProps) {
  // 行数组是与表单值同步的唯一编辑源：外部 reset（回填）→ value 变 → effect 重建；
  // 用户编辑 → 改 rows + onChange(数组) → value 变 → effect 幂等同步，不丢更新。
  const [rows, setRows] = useState<ProviderFormInlineKeyRow[]>(value);
  const [revealing, setRevealing] = useState(false);
  const [pasting, setPasting] = useState(false);
  const [pasteText, setPasteText] = useState("");

  useEffect(() => {
    setRows(value);
  }, [value]);

  const stats = useMemo(() => deriveInlineKeyStats(value), [value]);
  const parsedPaste = useMemo(() => (pasting ? splitInlineKeys(pasteText) : []), [pasting, pasteText]);
  const hasMasked = rows.some((r) => r.masked);
  const canReveal = hasMasked && providerId != null && groupId != null;

  const applyRows = (next: ProviderFormInlineKeyRow[]) => {
    setRows(next);
    onChange(next);
  };

  const updateRow = (index: number, text: string) => {
    // 只有明文行会渲染 Input（掩码行是只读 div），masked:false 为幂等保险；
    // 清空文本的既有行在提交层按「删除」处理（见 buildProviderPayload 过滤）
    applyRows(rows.map((r, i) => (i === index ? { ...r, key: text, masked: false } : r)));
  };

  /** 行 input 失焦时：若行内容含多个分隔符分隔的 key（用户从其它来源直接粘进单行），
   *  就地拆成多行——与批量粘贴同一词法器，保住「所见即所得」，避免带分隔符的非法 key 落库。 */
  const splitRowOnBlur = (row: ProviderFormInlineKeyRow, index: number) => {
    const parts = splitInlineKeys(row.key);
    if (parts.length > 1) {
      applyRows([
        ...rows.slice(0, index),
        ...parts.map((key) => ({ id: undefined, key, masked: false })),
        ...rows.slice(index + 1),
      ]);
    }
  };

  const removeRow = (index: number) => {
    applyRows(rows.filter((_, i) => i !== index));
  };

  const appendRow = () => {
    applyRows([...rows, { id: undefined, key: "", masked: false }]);
  };

  const mergePaste = () => {
    const keys = splitInlineKeys(pasteText);
    if (keys.length === 0) {
      return;
    }
    applyRows([...rows, ...keys.map((key) => ({ id: undefined, key, masked: false }))]);
    setPasteText("");
    setPasting(false);
  };

  /** 「显示」= 受控明文拉取：raw 端点返回 `[{id,key}]`，按凭据 id 归位替换本地掩码行——
   *  不依赖数组顺序，删掩码行未保存也能显示剩余；服务端已消失的凭据（并发保存）明确报错，
   *  绝不静默错位。明文此刻起驻留表单行（已过 wire），后续编辑即改即提交。 */
  const revealPlaintext = async () => {
    if (providerId == null || groupId == null) {
      return;
    }
    setRevealing(true);
    try {
      const { keys } = await getProviderGroupInlineKeysRaw(providerId, groupId);
      const byId = new Map(keys.map((k) => [k.id, k.key]));
      const stale = rows.filter((r) => r.masked && (r.id == null || !byId.has(r.id)));
      if (stale.length > 0) {
        toast.error("部分掩码条目在服务端已变更（可能被并发保存删除），请重新打开编辑弹窗");
        return;
      }
      applyRows(
        rows.map((r) =>
          r.masked && r.id != null ? { id: r.id, key: byId.get(r.id) ?? "", masked: false } : r
        )
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`显示明文失败: ${message}`);
    } finally {
      setRevealing(false);
    }
  };

  return (
    <div className="space-y-1.5">
      <FormLabel>API Keys</FormLabel>
      <div className="flex flex-wrap items-center gap-1.5">
        {stats.count > 0 && (
          <Badge variant="secondary" className="text-[10px] font-normal">
            共 {stats.count} 条
          </Badge>
        )}
        {stats.duplicates > 0 && (
          <Badge variant="destructive" className="text-[10px] font-normal">
            重复 {stats.duplicates} 条
          </Badge>
        )}
        <div className="ml-auto flex items-center gap-1">
          {hasMasked && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground"
              onClick={revealPlaintext}
              disabled={!canReveal || revealing}
              title={canReveal ? "拉取该组明文 keys（受控端点，仅当前编辑会话可见）" : "无可显示的既有 key"}
            >
              {revealing ? <Loader2 className="size-3.5 animate-spin" /> : <Eye className="size-3.5" />}
              显示
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground"
            onClick={() => setPasting((p) => !p)}
            aria-expanded={pasting}
          >
            <ClipboardPaste className="size-3.5" />
            批量粘贴
          </Button>
          <Button type="button" variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={appendRow}>
            <Plus className="size-3.5" />
            添加 key
          </Button>
        </div>
      </div>

      {/* 警示：空明文行（保存忽略）> 整组无有效 key（需重填）。掩码行是既有凭据，不算空态。 */}
      {rows.length > 0 && !stats.allBlank && stats.hasBlankLine ? (
        <p className="text-xs text-amber-600" role="note">
          {rows.filter((r) => !r.masked && r.key.trim() === "").length} 条空白条目保存时将被忽略（清空已显示的 key = 删除该条）
        </p>
      ) : rows.length > 0 && stats.allBlank ? (
        <p className="text-xs text-destructive" role="alert">
          该组没有有效 key——需填写至少一个 key 才能保存
        </p>
      ) : null}

      <div className="space-y-1.5">
        {rows.map((row, index) =>
          row.masked ? (
            <div key={index} className="group flex items-center gap-2">
              <div
                className="flex-1 truncate rounded-md border border-border/60 bg-muted/30 px-3 py-1.5 font-mono text-xs"
                title="服务端掩码；点右上「显示」查看明文后可编辑"
              >
                {row.key}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                onClick={() => removeRow(index)}
                aria-label={`删除第 ${index + 1} 条 key（保存后生效）`}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ) : (
            <div key={index} className="group flex items-center gap-2">
              <Input
                value={row.key}
                onChange={(e) => updateRow(index, e.target.value)}
                onBlur={() => splitRowOnBlur(row, index)}
                placeholder="sk-..."
                aria-label={`第 ${index + 1} 条 key`}
                // 拉明文期间禁编辑：reveal 基于发起时的 rows 回填，窗口内键入会被整组覆盖
                disabled={revealing}
                className="h-8 font-mono text-xs"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                onClick={() => removeRow(index)}
                aria-label={`删除第 ${index + 1} 条 key`}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          )
        )}
      </div>

      {pasting && (
        <div className="space-y-1.5 rounded-lg border border-dashed p-3">
          <Textarea
            value={pasteText}
            onChange={(e) => setPasteText(e.target.value)}
            rows={4}
            autoFocus
            aria-label="批量粘贴内容"
            className="min-h-24 font-mono text-xs"
            placeholder={"每行一个，或逗号 / 空格 / 制表符分隔，可混合\n拆分结果将追加到列表尾部（不覆盖现有 key）"}
          />
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setPasting(false);
                setPasteText("");
              }}
            >
              取消
            </Button>
            <Button type="button" size="sm" disabled={parsedPaste.length === 0} onClick={mergePaste}>
              <ClipboardPaste className="size-3.5" />
              追加 {parsedPaste.length > 0 && `${parsedPaste.length} 条`}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
