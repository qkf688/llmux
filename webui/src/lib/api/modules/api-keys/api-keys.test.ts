import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authStore } from "@/stores/auth";
import { listApiKeys, createApiKey, updateApiKey, deleteApiKey } from "./api-keys";

// fetch 是本模块唯一系统边界（HTTP）；auth store 用真实现 setState 喂 token，
// 不 mock——mock 越少，测出的越接近 client.ts 的真实行为。
const fetchMock = vi.fn();

function jsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Error",
    json: async () => body,
  };
}

// 期望值独立来源：handler/apikeys/crud.go 的 apiKeyResponse 实测形状
// （{id,name,api_key,enabled,created_at,headers} snake_case），非由实现反推。
const ROW_ACTIVE = {
  id: 1,
  name: "codex 专用",
  api_key: "sk-llmux-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  enabled: true,
  created_at: "2026-09-15T10:00:00Z",
  headers: null,
};
const ROW_DISABLED_MASKED = {
  id: 2,
  name: "普通应用",
  api_key: "****",
  enabled: false,
  created_at: "2026-09-14T08:30:00Z",
  headers: { "User-Agent": "claude-cli/1.0.70", "X-App": "cli" },
};

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  authStore.setState({ token: "test-jwt", user: null });
});

afterEach(() => {
  vi.unstubAllGlobals();
  authStore.setState({ token: null, user: null });
});

describe("listApiKeys", () => {
  it("GET /api/api-keys 带管理端 JWT，返回信封 data 且逐字段透出（AC-1）", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { code: 200, message: "success", data: [ROW_ACTIVE, ROW_DISABLED_MASKED] }),
    );

    const keys = await listApiKeys();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/api-keys");
    expect(init.headers).toMatchObject({ Authorization: "Bearer test-jwt" });
    // 含 disabled 与降级 "****" 行原样透出：前端拿到全量列表，不在这层丢行丢键
    expect(keys).toEqual([ROW_ACTIVE, ROW_DISABLED_MASKED]);
  });
});

describe("createApiKey", () => {
  it("POST 创建成功返回 data 行；value 留空按空串提交（后端生成，AC-3）", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { code: 200, message: "success", data: ROW_ACTIVE }));

    const created = await createApiKey({ name: "codex 专用" });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/api-keys");
    expect(init.method).toBe("POST");
    expect(init.headers).toMatchObject({ Authorization: "Bearer test-jwt" });
    expect(JSON.parse(String(init.body))).toEqual({ name: "codex 专用", value: "" });
    expect(created).toEqual(ROW_ACTIVE);
  });

  it("真实 HTTP 409 时抛响应体信封的 message，而非 statusText（AC-4 核心：apiRequest 会丢 body）", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 409,
      statusText: "Conflict",
      json: async () => ({ code: 409, message: "api key value already exists" }),
    });

    await expect(createApiKey({ name: "dup", value: "sk-llmux-duplicate-value-0001" })).rejects.toThrow(
      "api key value already exists",
    );
  });

  it("HTTP 200 + 信封 code 400（值过短）时抛信封 message，错误面与 409 同通道", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        code: 400,
        message: "api key value invalid: total length must be at least 16 characters after sk-llmux- prefix is added",
      }),
    );

    await expect(createApiKey({ name: "short", value: "abc" })).rejects.toThrow(
      "api key value invalid: total length must be at least 16 characters after sk-llmux- prefix is added",
    );
  });

  it("2xx 但 body 非 JSON 时抛泛化可展示文案（不透传 SyntaxError 原文）", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      statusText: "OK",
      json: async () => {
        throw new Error("Unexpected token < in JSON");
      },
    });

    await expect(createApiKey({ name: "x" })).rejects.toThrow(
      "API request failed: invalid success response body",
    );
  });

  // #6：create 支持可选 headers（表单/编排层共用单源，键缺席=后端不写该列）
  it("#6 携带 headers 时 POST body 含 headers 对象；未带时该键缺席", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { code: 200, message: "success", data: ROW_DISABLED_MASKED }));

    await createApiKey({ name: "codex", value: "", headers: { "User-Agent": "codex_cli_rs/0.1" } });
    expect(JSON.parse(String(fetchMock.mock.calls[0][1].body))).toEqual({
      name: "codex",
      value: "",
      headers: { "User-Agent": "codex_cli_rs/0.1" },
    });

    await createApiKey({ name: "no-headers" });
    expect(JSON.parse(String(fetchMock.mock.calls[1][1].body))).not.toHaveProperty("headers");
  });
});

describe("updateApiKey", () => {
  it("PATCH 部分更新：只发传入字段（启停切换不带 name），返回更新后的明文行（AC-5/AC-6）", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { code: 200, message: "success", data: ROW_DISABLED_MASKED }),
    );

    const updated = await updateApiKey(2, { enabled: false });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/api-keys/2");
    expect(init.method).toBe("PATCH");
    // 三态指针语义：未传字段整个键缺席，后端才不会误改
    expect(JSON.parse(String(init.body))).toEqual({ enabled: false });
    expect(updated).toEqual(ROW_DISABLED_MASKED);
  });

  it("改名提交 {name} 单字段", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { code: 200, message: "success", data: ROW_ACTIVE }));

    await updateApiKey(1, { name: "改名后" });

    const [, init] = fetchMock.mock.calls[0];
    expect(JSON.parse(String(init.body))).toEqual({ name: "改名后" });
  });

  // #6 三态：headers 键随传入出现（{}=清空 是有效载荷必须可表达）；不传则缺席
  it("#6 PATCH 携带 headers（含空对象=清空）时键在 body；未传时缺席", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { code: 200, message: "success", data: ROW_ACTIVE }));

    await updateApiKey(1, { headers: { "X-App": "cli" } });
    expect(JSON.parse(String(fetchMock.mock.calls[0][1].body))).toEqual({ headers: { "X-App": "cli" } });

    await updateApiKey(1, { headers: {} });
    expect(JSON.parse(String(fetchMock.mock.calls[1][1].body))).toEqual({ headers: {} });

    await updateApiKey(1, { name: "仅改名" });
    expect(JSON.parse(String(fetchMock.mock.calls[2][1].body))).not.toHaveProperty("headers");
  });
});

describe("deleteApiKey", () => {
  it("DELETE /api/api-keys/:id（AC-7）", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { code: 200, message: "success", data: { deleted: 1 } }));

    await deleteApiKey(5);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/api-keys/5");
    expect(init.method).toBe("DELETE");
  });
});
