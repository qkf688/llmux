import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// @/lib/api 与 sonner 是页面 hook 的边界（HTTP 门面 / 全局 toast）；
// fetch 级契约已在 lib/api/modules/api-keys/api-keys.test.ts 钉过，这里只测编排行为
const { listApiKeysMock, createApiKeyMock, updateApiKeyMock, deleteApiKeyMock } = vi.hoisted(() => ({
  listApiKeysMock: vi.fn(),
  createApiKeyMock: vi.fn(),
  updateApiKeyMock: vi.fn(),
  deleteApiKeyMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  listApiKeys: listApiKeysMock,
  createApiKey: createApiKeyMock,
  updateApiKey: updateApiKeyMock,
  deleteApiKey: deleteApiKeyMock,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

import { toast } from "sonner";
import { useApiKeysPage } from "./use-api-keys-page";

function createWrapper() {
  // 每用例独立 QueryClient，避免 react-query 缓存跨用例污染（associations-data 测试先例）
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

beforeEach(() => {
  listApiKeysMock.mockReset();
  createApiKeyMock.mockReset();
  updateApiKeyMock.mockReset();
  deleteApiKeyMock.mockReset();
  listApiKeysMock.mockResolvedValue([]);
  vi.mocked(toast.error).mockClear();
  vi.mocked(toast.success).mockClear();
});

describe("useApiKeysPage 创建", () => {
  it("成功：POST 后关弹窗、toast 成功并刷新列表（AC-3）", async () => {
    createApiKeyMock.mockResolvedValue({
      id: 1,
      name: "新 key",
      api_key: "sk-llmux-generated0000000000000000ab",
      enabled: true,
      created_at: "2026-09-15T10:00:00Z",
    });
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.openCreateForm());
    await act(async () => {
      await result.current.handleApiKeySaved({ name: "新 key" });
    });

    expect(createApiKeyMock).toHaveBeenCalledWith({ name: "新 key" });
    expect(result.current.formOpen).toBe(false);
    expect(toast.success).toHaveBeenCalled();
    await waitFor(() => expect(listApiKeysMock).toHaveBeenCalledTimes(2));
  });

  it("409 冲突：toast 展示响应体的具体冲突文案，弹窗保持打开（AC-4）", async () => {
    createApiKeyMock.mockRejectedValue(new Error("api key value already exists"));
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.openCreateForm());
    await act(async () => {
      await result.current.handleApiKeySaved({ name: "dup", value: "sk-llmux-same-value-000001" });
    });

    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("api key value already exists"));
    expect(result.current.formOpen).toBe(true);
    expect(listApiKeysMock).toHaveBeenCalledTimes(1); // 失败不触发列表刷新
  });
});

const EXISTING_KEY = {
  id: 3,
  name: "旧名字",
  api_key: "sk-llmux-existing000000000000abcdef01",
  enabled: true,
  created_at: "2026-09-15T09:00:00Z",
};

describe("useApiKeysPage 改名 / 启停 / 删除", () => {
  it("编辑模式保存：PATCH {name} 单字段并关弹窗（AC-5）", async () => {
    updateApiKeyMock.mockResolvedValue({ ...EXISTING_KEY, name: "新名字" });
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.openEditForm(EXISTING_KEY));
    await act(async () => {
      await result.current.handleApiKeySaved({ name: "新名字" });
    });

    expect(updateApiKeyMock).toHaveBeenCalledWith(EXISTING_KEY.id, { name: "新名字" });
    expect(createApiKeyMock).not.toHaveBeenCalled();
    expect(result.current.formOpen).toBe(false);
    expect(toast.success).toHaveBeenCalled();
  });

  // #6：弹窗提交的 headers 原样进 PATCH（三态：未传不带键=后端不改）
  it("#6 编辑保存带 headers：PATCH 同时提交 name 与 headers", async () => {
    updateApiKeyMock.mockResolvedValue({ ...EXISTING_KEY, name: "新名字" });
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.openEditForm(EXISTING_KEY));
    await act(async () => {
      await result.current.handleApiKeySaved({ name: "新名字", headers: { "X-App": "cli" } });
    });

    expect(updateApiKeyMock).toHaveBeenCalledWith(EXISTING_KEY.id, {
      name: "新名字",
      headers: { "X-App": "cli" },
    });
  });

  // #6：创建路径 headers 原样进 POST body（lib 层负责缺席语义）
  it("#6 创建保存带 headers：createApiKey 收到 headers", async () => {
    createApiKeyMock.mockResolvedValue({ ...EXISTING_KEY, name: "新 key" });
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.openCreateForm());
    await act(async () => {
      await result.current.handleApiKeySaved({ name: "新 key", headers: {} });
    });

    expect(createApiKeyMock).toHaveBeenCalledWith({ name: "新 key", headers: {} });
  });

  it("启停切换：PATCH {enabled} 取反值，成功后列表失效（AC-6）", async () => {
    updateApiKeyMock.mockResolvedValue({ ...EXISTING_KEY, enabled: false });
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    await act(async () => {
      await result.current.toggleApiKey(EXISTING_KEY);
    });

    expect(updateApiKeyMock).toHaveBeenCalledWith(EXISTING_KEY.id, { enabled: false });
    await waitFor(() => expect(listApiKeysMock).toHaveBeenCalledTimes(2));
  });

  it("启停失败：toast 展示后端文案，不刷新列表（AC-6 异常分支）", async () => {
    updateApiKeyMock.mockRejectedValue(new Error("api key not found"));
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    await act(async () => {
      await result.current.toggleApiKey(EXISTING_KEY);
    });

    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("api key not found"));
    expect(listApiKeysMock).toHaveBeenCalledTimes(1);
  });

  it("删除：确认后置 deleteOpen=false，DELETE 按 id 调用并刷新列表（AC-7）", async () => {
    deleteApiKeyMock.mockResolvedValue(undefined);
    const { result } = renderHook(() => useApiKeysPage(), { wrapper: createWrapper() });

    act(() => result.current.requestDelete(EXISTING_KEY));
    expect(result.current.deleteOpen).toBe(true);
    await act(async () => {
      await result.current.handleDeleteConfirm();
    });

    expect(deleteApiKeyMock).toHaveBeenCalledWith(EXISTING_KEY.id);
    expect(result.current.deleteOpen).toBe(false);
    expect(toast.success).toHaveBeenCalled();
    await waitFor(() => expect(listApiKeysMock).toHaveBeenCalledTimes(2));
  });
});
