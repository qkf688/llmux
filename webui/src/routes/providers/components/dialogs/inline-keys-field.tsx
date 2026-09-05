/**
 * 内联 key 输入/显示（替换原「单块裸明文 textarea」，2026-09-05 优化）：
 * - 逐行 key 列表：每行一条，可独立编辑/删除，避免整块重排；
 * - 掩码显示 + 眼睛切换（默认掩码，仅 count>0 时有可隐藏内容才出现眼睛；无内容时直接可输入）；
 * - 实时计数徽标：共 N 条 / 重复 M 条（重复只是提示，去重仍由提交时的后端 normalize 兜底）；
 * - 批量粘贴口：换行/逗号/空格/制表自适应拆分（splitInlineKeys），所见即所得防静默并 key；
 * - 空行警示条：后端解密失败回空串（join 后占一行）与用户手动空行在保存时行为一致（被忽略），
 *   同一条派生统计兼作警示依据——整组全空提示「需重新填写才能保存」，部分空行提示将忽略条数。
 *
 * 表单形态契约不变：value 恒为换行分隔 string，与后端 InlineKeys 解密回填 / normalizeInlineKeys 兼容。
 */
import { ClipboardPaste, Eye, Plus, Trash2, EyeOff } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { FormLabel } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { deriveInlineKeyStats, maskKey, splitInlineKeys } from "../../utils/inline-keys";

type InlineKeysFieldProps = {
  /** 表单值（换行分隔）。由外层 FormField 注入，外部 reset 回填时经 effect 同步内部行数组 */
  value: string;
  onChange: (next: string) => void;
  /**
   * 该组有凭据但全部无法解密显示的条数（后端各条回空串，仅回填时由 detailToFormValues 填充）。
   * 必须走 prop 而非从 value 派生：N=1 时 [""] join 退化为空串，与「全新空组」无法区分。
   */
  failedCount?: number;
};

export function InlineKeysField({ value, onChange, failedCount = 0 }: InlineKeysFieldProps) {
  // 行数组是与表单值同步的唯一编辑源：外部 reset（回填）→ value 变 → effect 重建；
  // 用户编辑 → 改 rows + onChange(join) → value 变 → effect 幂等同步，不丢更新。
  const [rows, setRows] = useState<string[]>(() => value.split("\n"));
  const [visible, setVisible] = useState(false);
  const [pasting, setPasting] = useState(false);
  const [pasteText, setPasteText] = useState("");

  useEffect(() => {
    setRows(value.split("\n"));
  }, [value]);

  const stats = useMemo(() => deriveInlineKeyStats(value), [value]);
  const parsedPaste = useMemo(() => (pasting ? splitInlineKeys(pasteText) : []), [pasting, pasteText]);
  // 无内容（空组）时没有可隐藏的东西，直接可输入；有内容后默认掩码，点眼睛看明文
  const effectiveVisible = stats.count === 0 || visible;

  const applyRows = (next: string[]) => {
    setRows(next);
    onChange(next.join("\n"));
  };

  // 编辑会话语义：任何编辑动作（输入/添加）进入明文态并保持，直到用户主动点「隐藏」——
  // 否则全新空组键入首字符后 count 由 0→1，effectiveVisible 翻转回掩码，正在聚焦的输入框
  // 被掩码 div 替换导致失焦无法继续输入（review C1）。
  const enterEditMode = () => setVisible(true);

  const updateRow = (index: number, text: string) => {
    enterEditMode();
    applyRows(rows.map((r, i) => (i === index ? text : r)));
  };

  /** 行 input 失焦时：若行内容含多个分隔符分隔的 key（用户从其它来源直接粘进单行），
   *  就地拆成多行——与批量粘贴同一词法器，保住「所见即所得」，避免带分隔符的非法 key 落库。 */
  const splitRowOnBlur = (row: string, index: number) => {
    const parts = splitInlineKeys(row);
    if (parts.length > 1) {
      applyRows([...rows.slice(0, index), ...parts, ...rows.slice(index + 1)]);
    }
  };

  const removeRow = (index: number) => {
    applyRows(rows.filter((_, i) => i !== index));
  };

  const appendRow = () => {
    enterEditMode();
    applyRows([...rows, ""]);
  };

  const mergePaste = () => {
    const keys = splitInlineKeys(pasteText);
    if (keys.length === 0) {
      return;
    }
    applyRows([...rows, ...keys]);
    setPasteText("");
    setPasting(false);
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
          {stats.count > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground"
              onClick={() => setVisible((v) => !v)}
              aria-label={visible ? "隐藏 API Keys" : "显示 API Keys"}
            >
              {visible ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              {visible ? "隐藏" : "显示"}
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
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={appendRow}
          >
            <Plus className="size-3.5" />
            添加 key
          </Button>
        </div>
      </div>

      {/* 警示优先级：后端已知的整组解密失败（failedCount，回填时无法从 value 派生） >
          > 部分空行（忽略提示）> 全空（手动空行形态，需重填）。三态递进不叠加。 */}
      {failedCount > 0 ? (
        <p className="text-xs text-destructive" role="alert">
          {failedCount} 条凭据无法解密显示（可能是后端加密密钥已轮换）——需重新填写这些 key 才能保存
        </p>
      ) : stats.count > 0 && stats.hasBlankLine ? (
        <p className="text-xs text-amber-600" role="note">
          {stats.totalLines - stats.count} 条空白条目保存时将被忽略
        </p>
      ) : stats.allBlank ? (
        <p className="text-xs text-destructive" role="alert">
          该组全部为空白条目——需填写至少一个 key 才能保存
        </p>
      ) : null}

      <div className="space-y-1.5">
        {rows.map((row, index) => (
          <div key={index} className="group flex items-center gap-2">
            {effectiveVisible ? (
              <Input
                value={row}
                onChange={(e) => updateRow(index, e.target.value)}
                onBlur={() => splitRowOnBlur(row, index)}
                placeholder="sk-..."
                aria-label={`第 ${index + 1} 条 key`}
                className="h-8 font-mono text-xs"
              />
            ) : (
              <div
                className="flex-1 truncate rounded-md border border-border/60 bg-muted/30 px-3 py-1.5 font-mono text-xs"
                title="点右上「显示」查看明文"
              >
                {maskKey(row)}
              </div>
            )}
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
        ))}
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