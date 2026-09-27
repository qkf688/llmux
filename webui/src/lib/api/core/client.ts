import { authStore, getAuthToken } from "@/stores/auth";
import { markLoginExpiredOnce } from "@/stores/auth/login-expiry-notice";

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

/** 跳登录页的单一 owner（未登录直入与 401 失效共用）：清会话 + 整页跳转，不写过期提示。 */
export function redirectToLogin(): void {
  authStore.getState().clearSession();
  window.location.href = "/login";
}

/** 401 失效出口抛出的错误类型。message 恒为 `Unauthorized`（既有断言依赖此文案）。
 *  存在的理由：自持连接通道（SSE）的失败呈现归调用方，需要一个结构化判据区分
 *  「登录过期终止」与「真失败」——用 message 文案比对应当避免（文案一改判据就漂）。 */
export class UnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "UnauthorizedError";
  }
}

/** 401 失效出口：标记过期（兼作去重）→ 清会话 → 跳登录页。
 *
 *  同一去重窗口内并发多请求撞 401，只有首个调用方触发整套处置（已设标记不再跳/不再写），
 *  其余只抛错终止调用方；窗口时长、写失败兜底与处置被打断的残余边界见
 *  `stores/auth/login-expiry-notice.ts` 模块注释。未登录直入（发起时本就无凭据）**不走
 *  本出口**：那是「未登录」不是「登录过期」，调用点直接 `redirectToLogin` 静默跳转。
 *  清会话复用 authStore.clearSession（401/登出单一 owner），此处不另造清理逻辑。 */
export function handleUnauthorized(): never {
  if (markLoginExpiredOnce()) {
    redirectToLogin();
  }
  throw new UnauthorizedError();
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
 *  管连接与取消，拿不到 Response 也就无法走信封拆包——故它只借这里的鉴权头与「无凭证
 *  直入」处置：发起时无凭证则清会话 + 跳登录页（未登录场景，不写过期提示）。
 *  连接建立时收到的 401（发起时有 token、到达时已失效）由 `fetchHandlingUnauthorized`
 *  经同一失效出口处置。事件内容与错误呈现归调用方。新增长连接端点若也要走这条通道，
 *  必须复用本函数（鉴权）+ `fetchHandlingUnauthorized`（401 处置）而不是自拼 Authorization。 */
export function requireAuthTokenOrRedirect(): string {
  const token = getAuthToken();
  if (!token) {
    redirectToLogin();
    throw new UnauthorizedError();
  }
  return token;
}

/** 自持连接通道的 fetch 替身（`fetchEventSource` 的 `fetch` 选项）：响应 401 走统一
 *  失效出口（`handleUnauthorized`），与信封通道同一 owner，调用点不得另造处置。
 *  鉴权头仍由调用方经 `requireAuthTokenOrRedirect` 拿 token 后携带（流的 headers 属流配置）。
 *  SSE 的 HTTP 状态只存在于连接建立的那次响应（流内事件不是 HTTP 响应），故判响应层
 *  即覆盖「发起后 token 才失效」的窗口；调用方另需把 `UnauthorizedError` 当非失败终止
 *  收场，不得写成「测试失败」。 */
export function fetchHandlingUnauthorized(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<Response> {
  return fetch(input, init).then((response) => {
    if (response.status === 401) {
      handleUnauthorized();
    }
    return response;
  });
}
