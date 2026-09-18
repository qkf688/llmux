import { authStore, getAuthToken } from "@/stores/auth";

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
 *  （Stage 3）。管理员据此让运维在服务端日志精确定位这一次请求。 */
const REQUEST_ID_HEADER = "X-Request-Id";

function buildAuthHeaders(headers: HeadersInit = {}): HeadersInit {
  const token = getAuthToken();
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...headers,
  };
}

function handleUnauthorized(): never {
  authStore.getState().clearSession();
  window.location.href = "/login";
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
    payload = (await response.json()) as EnvelopeFailure;
  } catch {
    // 见上：非 JSON 失败体走兜底文案
  }

  const detail = payload.message?.trim() || payload.error?.trim() || "";
  const rid = response.headers.get(REQUEST_ID_HEADER);

  if (response.status === 500) {
    const base = detail || "服务器内部错误";
    return new Error(rid && !base.includes(rid) ? `${base} [req=${rid}]` : base);
  }

  return new Error(detail || `请求失败（错误码 ${response.status}）`);
}

/** 2xx → 校验信封 code 并取 data；body 非 JSON 时给固定人话（不透传 SyntaxError 原文）。 */
async function unwrapEnvelope<T>(response: Response): Promise<T> {
  let payload: ApiResponseEnvelope<T>;
  try {
    payload = (await response.json()) as ApiResponseEnvelope<T>;
  } catch {
    throw new Error("请求失败：响应内容格式不对");
  }

  if (payload.code !== 200) {
    throw new Error(payload.message?.trim() || `请求失败（响应码 ${payload.code}）`);
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