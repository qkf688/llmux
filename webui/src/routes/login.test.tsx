import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StrictMode } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ThemeProvider } from "@/components/theme-provider";
import { login as loginApi } from "@/lib/api/modules/auth/auth";
import {
  LOGIN_EXPIRY_NOTICE_STORAGE_KEY,
  consumeLoginExpiryNotice,
  hasLoginExpiryNotice,
  markLoginExpiredOnce,
} from "@/stores/auth/login-expiry-notice";
import LoginPage from "./login";

vi.mock("@/lib/api/modules/auth/auth", () => ({
  login: vi.fn(),
  changePassword: vi.fn(),
}));

// StrictMode 对齐 main.tsx 的真实挂载形态：双渲染/双 effect 下提示仍须只展示一次且不丢失
function renderPage() {
  return render(
    <StrictMode>
      <ThemeProvider>
        <MemoryRouter initialEntries={["/login"]}>
          <LoginPage />
        </MemoryRouter>
      </ThemeProvider>
    </StrictMode>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
  consumeLoginExpiryNotice();
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
  it("401 跳转后展示提示，读后即焚且只展示一次（文案钉死字面量，不携带内部错误文本）", () => {
    markLoginExpiredOnce();

    const { unmount } = renderPage();

    // 文案精确等于字面量（非子串、也非对着实现常量比较）：任何动态文本混入都会判红
    expect(screen.getByRole("status").textContent).toBe("登录已过期，请重新登录");
    // 读后清除：存储与内存标记都不再残留
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBeNull();
    expect(hasLoginExpiryNotice()).toBe(false);

    // 只展示一次：整页刷新后重新挂载登录页不再出现
    unmount();
    renderPage();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("登录密码错误：展示错误文案，不走过期提示通道", async () => {
    vi.mocked(loginApi).mockRejectedValue(new Error("密码错误"));

    renderPage();
    fireEvent.change(screen.getByLabelText("密码"), { target: { value: "wrong" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("密码错误"));
    expect(screen.queryByRole("status")).toBeNull();
    expect(hasLoginExpiryNotice()).toBe(false);
  });

  it("没有过期提示时不渲染提示区（正常打开登录页不显示多余文案）", () => {
    renderPage();

    expect(screen.queryByRole("status")).toBeNull();
  });
});
