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

  it("登录失败：展示中文文案且不透传后端 message（后端回的是英文技术文案）", async () => {
    // 登录接口原样回传的 message：不得上屏（同 client.ts UnauthorizedError 的理由——
    // 拿后端文案当信息通道，文案一改判据就漂；500 路径更会带 [req=xxx] 内部编号）
    vi.mocked(loginApi).mockRejectedValue(new Error("invalid password"));

    renderPage();
    fireEvent.change(screen.getByLabelText("密码"), { target: { value: "wrong" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe("登录失败，请重试");
    // 反向断言：后端 message 确实没漏到界面上（删掉不透传的实现这里必须变红）
    expect(alert.textContent).not.toContain("invalid password");
    expect(screen.queryByText(/invalid password/)).toBeNull();
    expect(screen.queryByText(/\[req=/)).toBeNull();
    // 不走过期提示通道：密码错不写过期标记
    expect(screen.queryByRole("status")).toBeNull();
    expect(hasLoginExpiryNotice()).toBe(false);
  });

  it("登录失败：输入框进入错误态并与提示建立无障碍关联，重新输入后提示消失", async () => {
    vi.mocked(loginApi).mockRejectedValue(new Error("invalid password"));

    renderPage();
    const input = screen.getByLabelText("密码") as HTMLInputElement;
    expect(input).not.toHaveAttribute("aria-invalid");
    expect(input).not.toHaveAttribute("aria-describedby");

    fireEvent.change(input, { target: { value: "wrong" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));

    const alert = await screen.findByRole("alert");
    await waitFor(() => expect(input).toHaveAttribute("aria-invalid", "true"));
    // 读屏靠 describedby 把输入框和提示绑在一起，不能只靠视觉位置
    expect(input.getAttribute("aria-describedby")).toBe(alert.id);
    expect(alert.id).not.toBe("");

    // 重新输入即清错误（用户改密码的当下就该看到「刚才那条作废」）
    fireEvent.change(input, { target: { value: "another-try" } });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(input).not.toHaveAttribute("aria-invalid");
  });

  it("没有过期提示时不渲染提示区（正常打开登录页不显示多余文案）", () => {
    renderPage();

    expect(screen.queryByRole("status")).toBeNull();
  });
});
