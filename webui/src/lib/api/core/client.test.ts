import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authStore } from "@/stores/auth";
import {
  apiRequest,
  apiRequestAnonymous,
  apiRequestFile,
  apiRequestForm,
} from "./client";

// fetch 是本模块唯一的系统边界（HTTP），其余（auth store、location）用真实现，
// 使断言覆盖到 401 清会话/跳转这类副作用，而不是只测了 mock。
const fetchMock = vi.fn();

type ResponseSpec = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
  /** 需要模拟「body 不是 JSON」时置 true（json() 抛错） */
  invalidJson?: boolean;
};

function response(spec: ResponseSpec) {
  return {
    ok: spec.status >= 200 && spec.status < 300,
    status: spec.status,
    statusText: String(spec.status),
    headers: new Headers(spec.headers ?? { "Content-Type": "application/json" }),
    json: async () => {
      if (spec.invalidJson) {
        throw new SyntaxError("Unexpected token < in JSON at position 0");
      }
      return spec.body;
    },
  };
}

/** 后端成功信封 */
function envelope(data: unknown, code = 200, message = "success") {
  return { code, message, data };
}

/** 捕获抛出的 Error 本体：500 文案要断言精确等值（子串断言盖不住「编号重复拼接」）。 */
async function catchError(promise: Promise<unknown>): Promise<Error> {
  try {
    await promise;
  } catch (e) {
    return e as Error;
  }
  throw new Error("expected promise to reject");
}

let locationAssignments: string[];

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  authStore.setState({ token: "test-jwt", user: { id: 1, username: "admin", role: "admin" } });

  // jsdom 的 location.href 赋值会触发「navigation not implemented」告警，也无法断言；
  // 换成可写对象，让「跳登录页」这一副作用可被检查。
  locationAssignments = [];
  Object.defineProperty(window, "location", {
    configurable: true,
    value: {
      get href() {
        return locationAssignments.at(-1) ?? "";
      },
      set href(value: string) {
        locationAssignments.push(value);
      },
    },
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  authStore.setState({ token: null, user: null });
});

describe("apiRequest 成功路径", () => {
  it("2xx + 信封 code 200 时返回 data，并带 JWT 与 JSON 头", async () => {
    fetchMock.mockResolvedValue(response({ status: 200, body: envelope({ ok: 1 }) }));

    const data = await apiRequest<{ ok: number }>("/probe");

    expect(data).toEqual({ ok: 1 });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/probe");
    expect(init.headers).toMatchObject({
      Authorization: "Bearer test-jwt",
      "Content-Type": "application/json",
    });
  });

  it("HTTP 200 但信封 code 非 200（400/404 惯例）时抛信封 message", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 200, body: envelope(null, 400, "Provider not found") }),
    );

    await expect(apiRequest("/probe")).rejects.toThrow("Provider not found");
  });

  it("2xx 但 body 非 JSON 时抛固定人话，不透传解析报错原文", async () => {
    fetchMock.mockResolvedValue(response({ status: 200, invalidJson: true }));

    await expect(apiRequest("/probe")).rejects.toThrow("请求失败：响应内容格式不对");
  });

  it("2xx 但 body 是字面量 null 时同样给人话，不抛 TypeError 原文", async () => {
    fetchMock.mockResolvedValue(response({ status: 200, body: null }));

    await expect(apiRequest("/probe")).rejects.toThrow("请求失败：响应内容格式不对");
  });
});

describe("apiRequest 失败路径", () => {
  it("真实非 2xx（409）时抛响应体信封的 message", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 409, body: { code: 409, message: "api key value already exists" } }),
    );

    await expect(apiRequest("/probe")).rejects.toThrow("api key value already exists");
  });

  it("非 2xx 且 body 非 JSON 时用中文兜底，并带错误码", async () => {
    fetchMock.mockResolvedValue(response({ status: 409, invalidJson: true }));

    await expect(apiRequest("/probe")).rejects.toThrow("请求失败（错误码 409）");
  });

  it("500 取信封 message 并补响应头里的报障编号", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 500,
        headers: { "Content-Type": "application/json", "X-Request-Id": "8f3a2b1c9d0e" },
        body: { code: 500, message: "internal server error", error: "internal server error" },
      }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("internal server error [req=8f3a2b1c9d0e]");
  });

  it("500 且信封已内嵌同一编号时不重复拼（须精确等值：子串断言盖不住重复拼接）", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 500,
        headers: { "Content-Type": "application/json", "X-Request-Id": "8f3a2b1c9d0e" },
        body: { code: 500, message: "internal server error [req=8f3a2b1c9d0e]" },
      }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("internal server error [req=8f3a2b1c9d0e]");
  });

  it("500 且用静态文案（无编号）时前端补上编号", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 500,
        headers: { "Content-Type": "application/json", "X-Request-Id": "aa11bb22cc33" },
        body: {
          code: 500,
          message: "encryption not configured",
          error: "encryption not configured",
        },
      }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("encryption not configured [req=aa11bb22cc33]");
  });

  it("500 且 body 解析不出文案时退回中文兜底 + 编号", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 500, headers: { "X-Request-Id": "deadbeef0001" }, invalidJson: true }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("服务器内部错误 [req=deadbeef0001]");
  });

  it("失败体是字面量 null（JSON 合法但不是对象）时走兜底，不抛 TypeError 原文", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 500, headers: { "X-Request-Id": "aa11bb22cc33" }, body: null }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("服务器内部错误 [req=aa11bb22cc33]");
  });

  it("信封文案字段不是字符串（数字）时当没有，走兜底", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 409, body: { code: 409, message: 12345 } }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("请求失败（错误码 409）");
  });

  it("校验类失败（400）不带报障编号——它是可直接改正的提示", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 400,
        headers: { "Content-Type": "application/json", "X-Request-Id": "aa11bb22cc33" },
        body: { code: 400, message: "值长度不足" },
      }),
    );

    const err = await catchError(apiRequest("/probe"));
    expect(err.message).toBe("值长度不足");
  });
});

describe("401 处置", () => {
  it("apiRequest 收到 401：清会话、跳登录页、抛 Unauthorized", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 401, body: { code: 401, message: "Invalid or expired token" } }),
    );

    await expect(apiRequest("/probe")).rejects.toThrow("Unauthorized");
    expect(authStore.getState().token).toBeNull();
    expect(locationAssignments).toEqual(["/login"]);
  });

  it("apiRequestAnonymous 收到 401：不跳转、不清会话，抛信封文案（登录密码错场景）", async () => {
    authStore.setState({ token: null, user: null });
    fetchMock.mockResolvedValue(
      response({ status: 401, body: { code: 401, message: "invalid password" } }),
    );

    await expect(apiRequestAnonymous("/auth/login")).rejects.toThrow("invalid password");
    expect(locationAssignments).toEqual([]);
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers).not.toHaveProperty("Authorization");
  });
});

describe("apiRequestForm", () => {
  it("不注入 Content-Type（保住 multipart 边界），仍带 JWT 并返回 data", async () => {
    fetchMock.mockResolvedValue(response({ status: 200, body: envelope({ imported: 2 }) }));

    const body = new FormData();
    body.append("mode", "merge");
    const data = await apiRequestForm<{ imported: number }>("/system/import-config", {
      method: "POST",
      body,
    });

    expect(data).toEqual({ imported: 2 });
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers).toMatchObject({ Authorization: "Bearer test-jwt" });
    expect(init.headers).not.toHaveProperty("Content-Type");
  });

  it("失败（2xx + 信封 code 400）时抛信封 message", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 200, body: envelope(null, 400, "请求体超过上限 20 MiB") }),
    );

    await expect(
      apiRequestForm("/system/import-config", { method: "POST", body: new FormData() }),
    ).rejects.toThrow("请求体超过上限 20 MiB");
  });
});

describe("apiRequestFile", () => {
  it("成功（有 Content-Disposition）时返回 Response 供调用方读 blob", async () => {
    const resp = response({
      status: 200,
      headers: { "Content-Disposition": "attachment; filename=llmux_config_x.json" },
    });
    fetchMock.mockResolvedValue(resp);

    await expect(apiRequestFile("/system/export-config")).resolves.toBe(resp);
  });

  it("导出失败的通行形状（HTTP 200 + 信封错误，无文件名头）按失败抛出，不返回 Response", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 200,
        body: { code: 404, message: "数据库文件不存在" },
      }),
    );

    await expect(apiRequestFile("/system/export-database")).rejects.toThrow("数据库文件不存在");
  });

  it("真实非 2xx 时抛信封 message", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 500, body: { code: 500, message: "无法访问数据库文件" } }),
    );

    await expect(apiRequestFile("/system/export-database")).rejects.toThrow("无法访问数据库文件");
  });
});