import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ThemeProvider } from "@/components/theme-provider";
import { login as loginApi } from "@/lib/api/modules/auth/auth";
import {
  LOGIN_EXPIRY_NOTICE_STORAGE_KEY,
  LOGIN_EXPIRY_NOTICE_TEXT,
  setLoginExpiryNoticeOnce,
} from "@/stores/auth/login-expiry-notice";
import LoginPage from "./login";

vi.mock("@/lib/api/modules/auth/auth", () => ({
  login: vi.fn(),
  changePassword: vi.fn(),
}));

function renderPage() {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <LoginPage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
  vi.mocked(loginApi).mockReset();
  // jsdom 没有 matchMedia，ThemeProvider 解析系统主题时会调用：给个最小实现
  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("登录过期一次性提示", () => {
  it("401 跳转后展示提示，读后即焚且只展示一次（文案精确等值，不携带内部错误文本）", () => {
    setLoginExpiryNoticeOnce();

    const { unmount } = renderPage();

    // 展示的文案精确等于静态常量——断言文本本体而非子串，任何内部错误文本混入都会判红
    expect(screen.getByRole("status").textContent).toBe(LOGIN_EXPIRY_NOTICE_TEXT);
    // 读后清除：存储里不再残留
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBeNull();

    // 只展示一次：整页刷新后重新挂载登录页不再出现
    unmount();
    renderPage();
    expect(screen.queryByText(LOGIN_EXPIRY_NOTICE_TEXT)).toBeNull();
  });

  it("登录密码错误：展示错误文案，不走过期提示通道", async () => {
    vi.mocked(loginApi).mockRejectedValue(new Error("密码错误"));

    renderPage();
    fireEvent.change(screen.getByLabelText("密码"), { target: { value: "wrong" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("密码错误"));
    expect(screen.queryByText(LOGIN_EXPIRY_NOTICE_TEXT)).toBeNull();
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBeNull();
  });

  it("没有过期提示时不渲染提示区（正常打开登录页不显示多余文案）", () => {
    renderPage();

    expect(screen.queryByRole("status")).toBeNull();
  });
});
