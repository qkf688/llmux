import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authStore } from "@/stores/auth";
import { exportConfig, exportDatabase, importConfig } from "./import-export";

// 三条路径都经 core/client 的统一拆包通道：本用例断言的是「失败不再被当成成功」
// 与「成功路径仍能拿到 / 交出正确的东西」，而不是重复 client 的错误矩阵。
const fetchMock = vi.fn();
const { downloadBlobMock } = vi.hoisted(() => ({ downloadBlobMock: vi.fn() }));

vi.mock("../../core/download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../core/download")>()),
  downloadBlob: downloadBlobMock,
}));

type ResponseSpec = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
};

function response(spec: ResponseSpec) {
  return {
    ok: spec.status >= 200 && spec.status < 300,
    status: spec.status,
    statusText: String(spec.status),
    headers: new Headers(spec.headers ?? {}),
    json: async () => spec.body,
    blob: async () => new Blob(["file-bytes"]),
  };
}

beforeEach(() => {
  fetchMock.mockReset();
  downloadBlobMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  authStore.setState({ token: "test-jwt", user: { id: 1, username: "admin", role: "admin" } });
});

afterEach(() => {
  vi.unstubAllGlobals();
  authStore.setState({ token: null, user: null });
});

describe("exportConfig", () => {
  it("成功（带文件名头）时把响应体当文件下载，文件名取自响应头", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 200,
        headers: { "Content-Disposition": "attachment; filename=llmux_config_20260918.json" },
      }),
    );

    await exportConfig(["providers", "models"]);

    expect(String(fetchMock.mock.calls[0][0])).toBe(
      "/api/system/export-config?types=providers%2Cmodels",
    );
    expect(downloadBlobMock).toHaveBeenCalledTimes(1);
    expect(downloadBlobMock.mock.calls[0][1]).toBe("llmux_config_20260918.json");
  });

  it("后端读配置失败（HTTP 200 + 信封错误）时抛错且不下载——不再把错误说明存成配置文件", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 200, body: { code: 500, message: "读取提供商数据失败" } }),
    );

    await expect(exportConfig(["providers"])).rejects.toThrow("读取提供商数据失败");
    expect(downloadBlobMock).not.toHaveBeenCalled();
  });
});

describe("exportDatabase", () => {
  it("成功（带文件名头）时下载数据库文件", async () => {
    fetchMock.mockResolvedValue(
      response({
        status: 200,
        headers: { "Content-Disposition": "attachment; filename=llmux_backup_20260918.db" },
      }),
    );

    await exportDatabase();

    expect(downloadBlobMock).toHaveBeenCalledTimes(1);
    expect(downloadBlobMock.mock.calls[0][1]).toBe("llmux_backup_20260918.db");
  });

  it("数据库文件不存在（HTTP 200 + 信封 404）时抛出真实原因，不再笼统说「导出数据库失败」", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 200, body: { code: 404, message: "数据库文件不存在" } }),
    );

    await expect(exportDatabase()).rejects.toThrow("数据库文件不存在");
    expect(downloadBlobMock).not.toHaveBeenCalled();
  });
});

describe("importConfig", () => {
  it("成功返回信封 data 的颗粒度计数（与 handler/importexport 的 ImportOutcome 逐键对齐）", async () => {
    const result = {
      providers: { imported: 1, skipped: 0 },
      models: { imported: 2, skipped: 0 },
      associations: { imported: 3, skipped: 0 },
      templates: { imported: 0, skipped: 0 },
      settings: { imported: 0, skipped: 1 },
    };
    fetchMock.mockResolvedValue(response({ status: 200, body: { code: 200, message: "success", data: result } }));

    const data = await importConfig({ mode: "merge", types: ["providers"], file: new File(["{}"], "c.json") });

    expect(data).toEqual(result);
    // 页面按 result.providers.imported 汇总展示：形状错位会让每次成功导入都弹「导入失败」
    expect(data.providers.imported).toBe(1);
    const [, init] = fetchMock.mock.calls[0];
    expect(init.body).toBeInstanceOf(FormData);
    expect(init.headers).not.toHaveProperty("Content-Type");
  });

  it("导入失败（HTTP 200 + 信封 code 400）时抛后端文案", async () => {
    fetchMock.mockResolvedValue(
      response({ status: 200, body: { code: 400, message: "请求体超过上限 20 MiB" } }),
    );

    await expect(
      importConfig({ mode: "replace", types: ["providers"], file: new File(["{}"], "c.json") }),
    ).rejects.toThrow("请求体超过上限 20 MiB");
  });
});