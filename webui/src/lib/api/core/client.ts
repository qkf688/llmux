import { authStore, getAuthToken } from "@/stores/auth";
import { setLoginExpiryNoticeOnce } from "@/stores/auth/login-expiry-notice";

export const API_BASE = "/api";

/** 管理端统一信封（后端 httpresp.Response）：成功恒 code=200、数据在 data；失败可能走
 *  真实 HTTP 状态码（409/401/500），也可能走 HTTP 200 + 信封 code（400/404，全局惯例）。
 *  故「一次请求成不成功」不能只看状态码，两个信号都要读——本文件是唯一的读取处。 */
type ApiResponseEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

/** 失败信封可能的可展示文案来源：冲突/校验类给 message，500 的 message 与 error 同值。 */
type EnvelopeFailure = { code?: number; message?: string; error?: string };

/** 报障编号头：后端 RequestID 中间件恒回写，与 500 日志里的 request_id 同号
 *  （Stage 3 定号 + Stage 17 来源标记：值自带 gw: 自生成 / in: 沿用前缀，
 *  信封 [req=] 内嵌同值——「信封已内嵌同号则不重复」的判断对带前缀值同样成立）。
 *  管理员据此让运维在服务端日志精确定位这一次请求。 */
const REQUEST_ID_HEADER = "X-Request-Id";

function buildAuthHeaders(headers: HeadersInit = {}): HeadersInit {
  const token = getAuthToken();
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...headers,
  };
}

/** 401 / 无凭证的单一失效出口：写一次性过期提示 → 清会话 → 跳登录页。
 *
 *  提示键兼作去重标记：并发多请求同时撞 401 时只有首个调用方触发整套处置
 *  （已设标记不再跳/不再写），其余只抛错终止调用方，避免重复跳转/重复写提示。
 *  清会话复用 authStore.clearSession（401/登出单一 owner），此处不另造清理逻辑。 */
function handleUnauthorized(): never {
  if (setLoginExpiryNoticeOnce()) {
    authStore.getState().clearSession();
    window.location.href = "/login";
  }
  throw new Error("Unauthorized");
}

/**
 * 发一次 /api 请求并处置 401。
 *
 * `withAuth=false` 只给登录接口用：它的 401 语义是「密码不对」，若照「登录过期」处理会把
 * 刚写好的提示与输入一起刷掉，而那时本就没有登录态可清。改密码的 401 仍是「登录过期」
 * （旧密码错误在后端已改判 400，见 handler/auth/password.go），因此不需要维护接口名单。
 */
async function send(
  endpoint: string,
  options: RequestInit & { withAuth?: boolean } = {}
): Promise<Response> {
  const { withAuth = true, headers, ...rest } = options;
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...rest,
    headers: withAuth ? buildAuthHeaders(headers) : headers,
  });

  if (withAuth && response.status === 401) {
    handleUnauthorized();
  }

  return response;
}

/** JSON 信封请求的请求头：管理端请求体恒为 JSON（文件上传走 apiRequestForm，不注入此项）。 */
function jsonHeaders(headers?: HeadersInit): HeadersInit {
  return { "Content-Type": "application/json", ...headers };
}

/** 失败响应 → 可展示 Error。
 *
 *  文案优先级：信封 message → 信封 error → 中文兜底。500 额外补报障编号（取自响应头，
 *  信封已带同一编号则不重复拼）；其余失败（校验/冲突/未找到）是「改一下就能过」的提示，
 *  不带编号。解析不出 JSON 时（网关/代理错误页、纯文本）保留兜底文案，不把解析报错原文抛给用户。 */
async function failureError(response: Response): Promise<Error> {
  let payload: EnvelopeFailure = {};
  try {
    // JSON 合法但不是对象（字面量 null / 字符串 / 数字）也要退兜底：下面按对象取值，
    // 否则会抛 TypeError 原文上屏，与「不透传内部错误原文」的约定冲突。
    const parsed: unknown = await response.json();
    if (parsed !== null && typeof parsed === "object") {
      payload = parsed as EnvelopeFailure;
    }
  } catch {
    // 见上：非 JSON 失败体走兜底文案
  }

  const detail = textOrEmpty(payload.message) || textOrEmpty(payload.error) || "";
  const rid = response.headers.get(REQUEST_ID_HEADER);

  if (response.status === 500) {
    const base = detail || "服务器内部错误";
    return new Error(rid && !base.includes(rid) ? `${base} [req=${rid}]` : base);
  }

  return new Error(detail || `请求失败（错误码 ${response.status}）`);
}

/** 信封文案字段可能是任意 JSON 类型：只认字符串，其余（数字/对象/缺失）当没有。 */
function textOrEmpty(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/** 2xx → 校验信封 code 并取 data；body 非 JSON 时给固定人话（不透传 SyntaxError 原文）。 */
async function unwrapEnvelope<T>(response: Response): Promise<T> {
  let payload: ApiResponseEnvelope<T>;
  try {
    const parsed: unknown = await response.json();
    if (parsed === null || typeof parsed !== "object") {
      throw new Error("not an envelope");
    }
    payload = parsed as ApiResponseEnvelope<T>;
  } catch {
    throw new Error("请求失败：响应内容格式不对");
  }

  if (payload.code !== 200) {
    throw new Error(textOrEmpty(payload.message) || `请求失败（响应码 ${payload.code}）`);
  }

  return payload.data;
}

/**
 * JSON 信封请求（管理端绝大多数端点）：带 JWT，401 视为登录过期（清会话并回登录页）。
 *
 * 失败时抛的 Error.message 已是从响应体取出的可展示文案，页面层直接 `toErrorMessage(err)`
 * 即可——不必也不应在页面层再解一次响应。
 */
export async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await send(endpoint, {
    ...options,
    headers: jsonHeaders(options.headers),
  });

  if (!response.ok) {
    throw await failureError(response);
  }

  return unwrapEnvelope<T>(response);
}

/** 匿名信封请求（仅登录接口）：失败文案同样取自信封，但不触发跳登录页。 */
export async function apiRequestAnonymous<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await send(endpoint, {
    ...options,
    withAuth: false,
    headers: jsonHeaders(options.headers),
  });

  if (!response.ok) {
    throw await failureError(response);
  }

  return unwrapEnvelope<T>(response);
}

/** 自带 Content-Type 的信封请求（FormData 上传）：不注入 application/json——
 *  那会顶掉 multipart 的 boundary，后端解析不到文件。 */
export async function apiRequestForm<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await send(endpoint, options);

  if (!response.ok) {
    throw await failureError(response);
  }

  return unwrapEnvelope<T>(response);
}

/**
 * 文件下载请求（响应体不是信封 data）：成功时把 Response 交给调用方读 blob 与文件名头。
 *
 * 判「是不是真文件」不能只看状态码——导出接口的失败走 HTTP 200 + 信封 code（httpresp
 * 全局惯例），只看 ok 会把写着错误说明的 JSON 当文件下给用户。故以是否有
 * `Content-Disposition` 文件名头为准：两个导出端点成功路径恒设该头、失败路径不设。
 * 新增下载端点必须同样设该头，否则会被这里判成失败信封。
 */
export async function apiRequestFile(
  endpoint: string,
  options: RequestInit = {}
): Promise<Response> {
  const response = await send(endpoint, options);

  if (!response.ok || !response.headers.get("Content-Disposition")) {
    throw await failureError(response);
  }

  return response;
}

/** SSE 流请求（当前仅 ReAct 测试端点 `/api/test/react/:id`）：`fetchEventSource` 自己
 *  管连接与取消，拿不到 Response 也就无法走信封拆包——故它只借这里的鉴权与失效处置：
 *  无凭证时清会话 + 跳登录页（与 `apiRequest` 的 401 处置一致；此前该调用点只跳转不
 *  清会话，会让 store 与 localStorage 留着失效 token）。事件内容与错误呈现归调用方。
 *  新增长连接端点若也要走这条通道，必须复用本函数而不是自拼 Authorization。 */
export function requireAuthTokenOrRedirect(): string | null {
  const token = getAuthToken();
  if (!token) {
    handleUnauthorized();
  }
  return token;
}
