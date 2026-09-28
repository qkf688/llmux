/**
 * 内联 key 输入/显示（掩码化安全浏览，2026-09-06 对齐号池 KeyMasked 体系）：
 * - 详情回填 = 服务端掩码行（masked=true，只读，保存按 id 保留，明文永不出常规响应）；
 * - 点「显示」经受控 raw 端点拉取该组明文，整组切换为可编辑明文行（masked=false）；
 * - 新增/粘贴的行直接是明文态；掩码行不可单独编辑（要改必须先显示）；
 * - 明文行可单条复制（copyTextWithToast，提示只回显行号不回显明文）；掩码行无复制入口，
 *   要复制先点「显示」——供应商侧只有整组 raw 端点，没有单条明文出口；
 * - 实时计数徽标：共 N 条 / 明文行重复 M 条（重复仅提示，提交时后端 normalize 显式 400 拒绝）；
 * - 批量粘贴口：换行/逗号/空格/制表自适应拆分（splitInlineKeys），所见即所得防静默并 key；
 * - 空明文行警示条：保存时被忽略（对齐后端 normalizeInlineKeys 空条目兜底）；
 * - 一律不设原生 title（动作按钮与掩码行文本都不设，与号池 CredentialKeyCell 一致）：hover
 *   提示在密集表单里频繁弹出干扰阅读，语义由可见文案 / aria-label 承载。
 *
 * 行形态契约（ProviderFormInlineKeyRow）：value 恒为该数组；onChange 吐整个数组，
 * 保存时 buildProviderPayload 把 masked 行映射为 {id, key:""}（保留）、明文行映射为 {id, key}。
 * 状态（status/cooldownUntil/failCount）是凭据属性、由详情回填，不参与提交。
 *
 * 内联 key 的人工恢复：被判停（temp_unsched）或冷却中的 key 在这里就能看状态并一键启用
 * ——状态读写走凭据域 `/api/credentials`（启停与号池页同一套接口），不再需要删行重加。
 */
import { ClipboardPaste, Copy, Eye, Plus, RefreshCw, Trash2, Loader2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { FormLabel } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CredentialStatusBadge } from "@/components/credentials/credential-status-badge";
import { batchUpdateCredentialStatus, getProviderGroupInlineKeysRaw } from "@/lib/api";
import { copySecretWithToast } from "@/lib/clipboard";
import { toErrorMessage } from "@/lib/errors";
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
  // 启停在途的凭据 ID：按钮转圈 + 禁用（同一条只允许一次在飞请求）
  const [statusPending, setStatusPending] = useState<ReadonlySet<number>>(new Set());
  // 在飞期间用户可能继续编辑行；await 回来后按最新行改状态，不用发起时的闭包快照
  // （否则会把窗口期内的编辑整段覆盖回去）。
  const rowsRef = useRef<ProviderFormInlineKeyRow[]>(value);

  useEffect(() => {
    setRows(value);
  }, [value]);

  useEffect(() => {
    rowsRef.current = rows;
  }, [rows]);

  const stats = useMemo(() => deriveInlineKeyStats(value), [value]);
  const parsedPaste = useMemo(() => (pasting ? splitInlineKeys(pasteText) : []), [pasting, pasteText]);
  const hasMasked = rows.some((r) => r.masked);
  const canReveal = hasMasked && providerId != null && groupId != null;
  // 已保存且非健康的行（判停 / 停用 / 冷却中）：组级「全部启用」的恢复对象
  const recoverableIds = rows
    .filter((r) => r.id != null && r.id > 0 && r.status != null && r.status !== "active")
    .map((r) => r.id!);

  const applyRows = (next: ProviderFormInlineKeyRow[]) => {
    setRows(next);
    onChange(next);
  };

  /** 启停既有凭据：走凭据域批量端点（单条也是它），成功后按最新行写回状态。
   *  本地写回而非重拉详情：弹窗表单有未保存编辑，重拉会整段覆盖用户输入。 */
  const updateStatuses = async (ids: number[], status: string) => {
    if (ids.length === 0) return;
    setStatusPending((prev) => new Set([...prev, ...ids]));
    try {
      await batchUpdateCredentialStatus({ ids, status });
      const idSet = new Set(ids);
      applyRows(
        rowsRef.current.map((r) =>
          r.id != null && idSet.has(r.id)
            ? {
                ...r,
                status,
                // 后端恢复 active 时连带清连败计数（models.CredentialRecoveryFields），
                // 本地同步清掉，徽标旁的连败信息才不会留着旧值
                failCount: status === "active" ? 0 : r.failCount,
              }
            : r
        )
      );
      toast.success(status === "active" ? `已启用 ${ids.length} 条 key` : `已停用 ${ids.length} 条 key`);
    } catch (err) {
      toast.error(`更新 key 状态失败: ${toErrorMessage(err)}`);
    } finally {
      setStatusPending((prev) => {
        const next = new Set(prev);
        ids.forEach((id) => next.delete(id));
        return next;
      });
    }
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
          r.masked && r.id != null ? { ...r, key: byId.get(r.id) ?? "", masked: false } : r
        )
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`显示明文失败: ${message}`);
    } finally {
      setRevealing(false);
    }
  };

  /** 复制明文行。走 copySecretWithToast（禁 prompt 兜底——prompt 会把明文整条弹上屏）；
   *  成功提示只回显行号：前端不重建掩码（掩码单源在后端 KeyMasked），更不能把明文写进
   *  任何用户可见文案（契约见 lib/clipboard）。 */
  const copyRowKey = async (index: number, key: string) => {
    await copySecretWithToast(key, `已复制第 ${index + 1} 条 key`);
  };

  /** 既有凭据行的状态徽标 + 启停按钮（新增行无 id/状态 → 返回 null）。
   *  恢复入口的意义：内联 key 被状态机判停后，此前只能删行重加才能回到生产。 */
  const renderStatusControls = (row: ProviderFormInlineKeyRow, index: number) => {
    if (row.id == null || row.id <= 0 || row.status == null) {
      return null;
    }
    const pending = statusPending.has(row.id);
    const isActive = row.status === "active";
    return (
      <>
        <CredentialStatusBadge status={row.status} cooldownUntil={row.cooldownUntil} />
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs text-muted-foreground"
          onClick={() => void updateStatuses([row.id!], isActive ? "disabled" : "active")}
          disabled={pending}
          aria-label={`${isActive ? "停用" : "启用"}第 ${index + 1} 条 key`}
        >
          {pending ? <Loader2 className="size-3.5 animate-spin" /> : null}
          {isActive ? "停用" : "启用"}
        </Button>
      </>
    );
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
          {recoverableIds.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-amber-600"
              onClick={() => void updateStatuses(recoverableIds, "active")}
              aria-label={`启用全部 ${recoverableIds.length} 条不可用 key`}
            >
              <RefreshCw className="size-3.5" />
              全部启用 {recoverableIds.length} 条
            </Button>
          )}
          {hasMasked && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground"
              onClick={revealPlaintext}
              disabled={!canReveal || revealing}
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
              >
                {row.key}
              </div>
              {renderStatusControls(row, index)}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-destructive focus-visible:opacity-100 group-hover:opacity-100"
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
                className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity focus-visible:opacity-100 group-hover:opacity-100"
                onClick={() => void copyRowKey(index, row.key)}
                // 空行没有可复制内容（提交时也被忽略），禁用避免复制空串还提示成功
                disabled={row.key.trim() === ""}
                aria-label={`复制第 ${index + 1} 条 key`}
              >
                <Copy className="size-3.5" />
              </Button>
              {renderStatusControls(row, index)}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-destructive focus-visible:opacity-100 group-hover:opacity-100"
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
