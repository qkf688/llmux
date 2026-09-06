/**
 * CredentialKeyCell 的 DOM 层契约测试。
 *
 * hook 单测（use-credential-key-reveal.test.ts）把 lib/clipboard 整个 mock 掉了，所以「明文不出现在
 * 可访问性名 / 未显示时的 DOM」这类契约只能在渲染层钉住——这里用真实 hook + 真实组件，
 * 只 mock 网络与剪贴板两个边界。
 */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { getCredentialRawMock, copySecretWithToastMock } = vi.hoisted(() => ({
  getCredentialRawMock: vi.fn(),
  copySecretWithToastMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  getCredentialRaw: getCredentialRawMock,
}));

vi.mock("@/lib/clipboard", () => ({
  copySecretWithToast: copySecretWithToastMock,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

import { CredentialKeyCell } from "./credential-key-cell";
import { useCredentialKeyReveal } from "./use-credential-key-reveal";

const POOL_ID = 7;
const MASKED = "sk-****ab12";
const PLAIN = "sk-plaintext-value-ab12";
const REVEAL_LABEL = `显示 key ${MASKED} 的明文`;
const HIDE_LABEL = `隐藏 key ${MASKED}`;
const COPY_LABEL = `复制 key ${MASKED}`;

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function Harness() {
  const reveal = useCredentialKeyReveal({ poolId: POOL_ID, view: "1|all|" });
  return <CredentialKeyCell credId={11} keyMasked={MASKED} reveal={reveal} />;
}

beforeEach(() => {
  getCredentialRawMock.mockReset();
  copySecretWithToastMock.mockReset();
  copySecretWithToastMock.mockResolvedValue(true);
});

describe("CredentialKeyCell", () => {
  it("默认只渲染服务端掩码，DOM 里没有明文", () => {
    const { container } = render(<Harness />);

    expect(container.textContent).toContain(MASKED);
    expect(container.textContent).not.toContain(PLAIN);
    expect(getCredentialRawMock).not.toHaveBeenCalled();
  });

  it("点「显示」后明文上屏并标 translate=no，按钮的可访问性名仍只用掩码", async () => {
    getCredentialRawMock.mockResolvedValue({ key: PLAIN });
    const { container } = render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: REVEAL_LABEL }));

    await waitFor(() => expect(container.textContent).toContain(PLAIN));
    // 整页翻译会把可见文本上传翻译服务，明文必须排除在外
    expect(container.querySelector("span[translate='no']")?.textContent).toBe(PLAIN);
    // aria-label / title 是掩码之外的第二条扩散通道（读屏、自动化脚本），不得携带明文
    screen
      .getAllByRole("button")
      .forEach((button) => expect(button.getAttribute("aria-label")).not.toContain(PLAIN));
    expect(screen.getByRole("button", { name: HIDE_LABEL })).toBeTruthy();
  });

  it("点「复制」不改变显示态：剪贴板拿到明文，DOM 仍只有掩码", async () => {
    getCredentialRawMock.mockResolvedValue({ key: PLAIN });
    const { container } = render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: COPY_LABEL }));

    await waitFor(() =>
      expect(copySecretWithToastMock).toHaveBeenCalledWith(PLAIN, `已复制 key: ${MASKED}`),
    );
    expect(container.textContent).not.toContain(PLAIN);
    expect(screen.getByRole("button", { name: REVEAL_LABEL })).toBeTruthy();
  });

  it("显示在途时两个按钮都禁用，转圈只落在被点的眼睛按钮上", async () => {
    const deferred = createDeferred<{ key: string }>();
    getCredentialRawMock.mockReturnValue(deferred.promise);
    render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: REVEAL_LABEL }));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: COPY_LABEL })).toBeDisabled(),
    );
    expect(screen.getByRole("button", { name: REVEAL_LABEL }).querySelector("svg.animate-spin")).not.toBeNull();
    // 反馈错位回归：pending 曾让 spinner 出现在眼睛上、被点的复制按钮毫无反应
    expect(screen.getByRole("button", { name: COPY_LABEL }).querySelector("svg.animate-spin")).toBeNull();

    // 放行在途请求，避免测试结束时留下未收尾的 pending 状态
    deferred.resolve({ key: PLAIN });
    await waitFor(() => expect(screen.getByRole("button", { name: HIDE_LABEL })).toBeTruthy());
  });
});
