/**
 * InlineKeysField：内联 key 的状态徽标与人工恢复入口。
 *
 * 为什么这几条要单独锁：内联 key 被判停（auth_fail → temp_unsched）或停用后，此前
 * 弹窗里看不到状态、也没有恢复入口，只能删行重加。这里锁住「看得到 + 一键启用」
 * 的行为契约（调凭据域批量端点，成功后按最新行回写状态）。
 *
 * 边界 mock：网络（@/lib/api）、toast（sonner）、FormLabel（需要 FormField 上下文，
 * 本组件是受控 value/onChange 组件，测试不铺 RHF 表单）。
 */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { batchUpdateCredentialStatusMock, getProviderGroupInlineKeysRawMock, toastErrorMock } =
  vi.hoisted(() => ({
    batchUpdateCredentialStatusMock: vi.fn(),
    getProviderGroupInlineKeysRawMock: vi.fn(),
    toastErrorMock: vi.fn(),
  }));

vi.mock("@/lib/api", () => ({
  batchUpdateCredentialStatus: batchUpdateCredentialStatusMock,
  getProviderGroupInlineKeysRaw: getProviderGroupInlineKeysRawMock,
}));

vi.mock("sonner", () => ({
  toast: { error: toastErrorMock, success: vi.fn(), info: vi.fn() },
}));

vi.mock("@/components/ui/form", () => ({
  FormLabel: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}));

import { InlineKeysField } from "./inline-keys-field";
import type { ProviderFormInlineKeyRow } from "../../form-schema";

function row(p: Partial<ProviderFormInlineKeyRow> & { key: string }): ProviderFormInlineKeyRow {
  return { masked: true, ...p };
}

const onChange = vi.fn();

function Harness({ initial }: { initial: ProviderFormInlineKeyRow[] }) {
  const [rows, setRows] = useState(initial);
  return (
    <InlineKeysField
      value={rows}
      onChange={(next) => {
        onChange(next);
        setRows(next);
      }}
      providerId={1}
      groupId={23}
    />
  );
}

function lastRows(): ProviderFormInlineKeyRow[] {
  const calls = onChange.mock.calls;
  return calls[calls.length - 1]?.[0] ?? [];
}

beforeEach(() => {
  onChange.mockReset();
  batchUpdateCredentialStatusMock.mockReset();
  getProviderGroupInlineKeysRawMock.mockReset();
  toastErrorMock.mockReset();
});

describe("InlineKeysField 状态徽标与启停", () => {
  it("判停行显示状态徽标与「启用」，点击走凭据域批量端点并清掉连败计数", async () => {
    batchUpdateCredentialStatusMock.mockResolvedValue({ updated: 1 });
    render(<Harness initial={[row({ id: 103, key: "sk-****ab12", status: "temp_unsched", failCount: 3 })]} />);

    expect(screen.getByText("临时停调度")).toBeTruthy();
    fireEvent.click(screen.getByLabelText("启用第 1 条 key"));

    await waitFor(() => {
      expect(batchUpdateCredentialStatusMock).toHaveBeenCalledWith({ ids: [103], status: "active" });
    });
    const rows = lastRows();
    expect(rows[0]).toMatchObject({ id: 103, status: "active", failCount: 0 });
    // 徽标随状态刷新（受控 value 回写后渲染层同步）
    await waitFor(() => expect(screen.getByText("健康")).toBeTruthy());
  });

  it("健康行显示「停用」，点击切 disabled", async () => {
    batchUpdateCredentialStatusMock.mockResolvedValue({ updated: 1 });
    render(<Harness initial={[row({ id: 101, key: "sk-****ab12", status: "active" })]} />);

    fireEvent.click(screen.getByLabelText("停用第 1 条 key"));

    await waitFor(() => {
      expect(batchUpdateCredentialStatusMock).toHaveBeenCalledWith({ ids: [101], status: "disabled" });
    });
    expect(lastRows()[0]).toMatchObject({ id: 101, status: "disabled" });
  });

  it("组级「全部启用」只带非健康行（含无状态的新增行不入列）", async () => {
    batchUpdateCredentialStatusMock.mockResolvedValue({ updated: 2 });
    render(
      <Harness
        initial={[
          row({ id: 101, key: "sk-****1", status: "active" }),
          row({ id: 102, key: "sk-****2", status: "temp_unsched", failCount: 3 }),
          row({ id: 103, key: "sk-****3", status: "disabled" }),
          row({ key: "", status: undefined, id: undefined }),
        ]}
      />
    );

    fireEvent.click(screen.getByLabelText("启用全部 2 条不可用 key"));

    await waitFor(() => {
      expect(batchUpdateCredentialStatusMock).toHaveBeenCalledWith({ ids: [102, 103], status: "active" });
    });
    expect(lastRows().slice(0, 3)).toMatchObject([
      { id: 101, status: "active" },
      { id: 102, status: "active" },
      { id: 103, status: "active" },
    ]);
  });

  it("失败时提示原因且不回写状态（徽标保持判停）", async () => {
    batchUpdateCredentialStatusMock.mockRejectedValue(new Error("boom"));
    render(<Harness initial={[row({ id: 103, key: "sk-****ab12", status: "temp_unsched" })]} />);

    fireEvent.click(screen.getByLabelText("启用第 1 条 key"));

    await waitFor(() => expect(toastErrorMock).toHaveBeenCalled());
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("临时停调度")).toBeTruthy();
  });

  it("「显示」明文后仍保留状态与凭据 ID（启停入口不因显示而消失）", async () => {
    getProviderGroupInlineKeysRawMock.mockResolvedValue({ keys: [{ id: 103, key: "sk-plain-ab12" }] });
    render(<Harness initial={[row({ id: 103, key: "sk-****ab12", status: "temp_unsched", failCount: 2 })]} />);

    fireEvent.click(screen.getByText("显示"));

    await waitFor(() => expect(lastRows().length).toBe(1));
    expect(lastRows()[0]).toMatchObject({
      id: 103,
      key: "sk-plain-ab12",
      masked: false,
      status: "temp_unsched",
      failCount: 2,
    });
    expect(screen.getByLabelText("启用第 1 条 key")).toBeTruthy();
  });

  it("新增行（无 id/状态）不显示状态徽标与启停按钮", () => {
    render(<Harness initial={[row({ key: "sk-new", masked: false, id: undefined, status: undefined })]} />);

    expect(screen.queryByLabelText("启用第 1 条 key")).toBeNull();
    expect(screen.queryByLabelText("停用第 1 条 key")).toBeNull();
  });
});