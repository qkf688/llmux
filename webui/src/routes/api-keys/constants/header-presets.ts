/**
 * #6 key 级自定义出站头的预设种子。
 *
 * 头名清单抄自 new-api `setting/operation_setting/channel_affinity_setting.go`
 * 的 codex/claude 两组实战清单；**值是示例默认**——预设=填充便利不是运行期魔法
 * （方案 §2 定案），每行可删可改；渠道实测偏差只改本文件一处。
 * 值为空的行是给动态标识（session/thread/request id）留的占位：由用户按实际
 * 客户端行为填写，未填会原样发出空头（与关联级 customer_headers 显式通道同权）。
 * 适用渠道：Anthropic-Beta/Version 两行在**原生 anthropic 供应商**会被 setAnthropicBeta
 * 清除/被 provider 配置覆写（providers 权威层最高），预设主要对 openai 兼容中转生效。
 */
import type { HeaderRow } from "../utils/header-fields";

export const CODEX_HEADER_PRESET: HeaderRow[] = [
  { key: "Originator", value: "codex_cli_rs" },
  { key: "User-Agent", value: "codex_cli_rs/0.1 (Windows 11; x86_64)" },
  { key: "Session_id", value: "" },
  { key: "Thread_id", value: "" },
  { key: "X-Client-Request-Id", value: "" },
  { key: "X-Codex-Turn-State", value: "" },
  { key: "X-Codex-Window-Id", value: "" },
  { key: "X-OpenAI-Internal-Codex-Responses-Lite", value: "" },
];

export const CLAUDE_CODE_HEADER_PRESET: HeaderRow[] = [
  { key: "User-Agent", value: "claude-cli/1.0.70 (external, cli)" },
  { key: "X-App", value: "cli" },
  { key: "Anthropic-Beta", value: "claude-code-20250219" },
  { key: "Anthropic-Version", value: "2023-06-01" },
  { key: "X-Stainless-Arch", value: "x64" },
  { key: "X-Stainless-Lang", value: "js" },
  { key: "X-Stainless-Os", value: "windows" },
  { key: "X-Stainless-Package-Version", value: "0.65.0" },
  { key: "X-Stainless-Retry-Count", value: "0" },
  { key: "X-Stainless-Runtime", value: "node" },
  { key: "X-Stainless-Runtime-Version", value: "v24.3.0" },
  { key: "X-Stainless-Timeout", value: "600000" },
];

/** 全体预设单源清单：弹窗据此算「互斥的其它预设」（applyPresetRows），新增预设只在这里多挂一行 */
export const ALL_HEADER_PRESETS: HeaderRow[][] = [CODEX_HEADER_PRESET, CLAUDE_CODE_HEADER_PRESET];
