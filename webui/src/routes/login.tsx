import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTheme } from "@/components/theme-provider";
import { selectSetSession, useAuthStore } from "@/stores/auth";
import { login as loginApi } from "@/lib/api/modules/auth/auth";

/* 主题切换图标 */
const MoonIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="size-[18px]">
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
  </svg>
);
const SunIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="size-[18px]">
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
  </svg>
);

/* 密码显隐图标 */
const EyeIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="size-[18px]">
    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
    <circle cx="12" cy="12" r="3" />
  </svg>
);
const EyeOffIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="size-[18px]">
    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
    <line x1="1" y1="1" x2="23" y2="23" />
  </svg>
);

/* Logo：鼠尾草绿圆角方块 + 白色 L，主色用 currentColor 便于主题适配 */
const Logo = ({ size = 36 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" aria-label="LLMux logo" className="text-primary shrink-0">
    <rect width="36" height="36" rx="8" fill="currentColor" />
    <text x="18" y="25" text-anchor="middle"
          font-family="Inter, sans-serif" font-size="20" font-weight="700"
          className="fill-primary-foreground">L</text>
  </svg>
);

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const setSession = useAuthStore(selectSetSession);
  const { theme, setTheme } = useTheme();

  const isDark = theme === "dark";
  const trimmed = password.trim();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!trimmed || loading) return;

    setError("");
    setLoading(true);
    try {
      const res = await loginApi(trimmed);
      setSession(res.token, res.user);
      navigate("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen bg-background text-foreground transition-colors duration-300">
      {/* 主题切换按钮：右上角固定 */}
      <Button
        type="button"
        variant="outline"
        size="icon"
        onClick={() => setTheme(isDark ? "light" : "dark")}
        aria-label="切换主题"
        className="absolute top-5 right-5 z-10 size-10 rounded-md border-border bg-card text-muted-foreground hover:text-accent hover:border-accent"
      >
        {isDark ? SunIcon : MoonIcon}
      </Button>

      {/* 单栏居中布局：桌面 + 移动端一致 */}
      <div className="flex w-full min-h-screen items-center justify-center px-6 py-10">
        <main className="w-full max-w-[380px]">
          {/* 顶部品牌 */}
          <div className="flex items-center justify-center gap-2.5 mb-8">
            <Logo size={32} />
            <span className="text-xl font-semibold tracking-tight text-foreground">LLMux</span>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground mb-1.5">登录</h1>
            <p className="text-sm text-muted-foreground mb-6">输入管理员密码以访问系统</p>

            <form onSubmit={handleLogin} noValidate>
              <div className="mb-4">
                <Label htmlFor="password" className="block text-[13px] font-medium text-foreground mb-1.5">
                  密码
                </Label>
                <div className="relative">
                  <Input
                    id="password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    value={password}
                    onChange={(e) => {
                      setPassword(e.target.value);
                      if (error) setError("");
                    }}
                    placeholder="请输入密码"
                    autoComplete="current-password"
                    autoFocus
                    disabled={loading}
                    className="h-11 pr-11"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword((v) => !v)}
                    aria-label={showPassword ? "隐藏密码" : "显示密码"}
                    className="absolute top-1/2 right-2.5 -translate-y-1/2 flex items-center justify-center size-7 text-muted-foreground hover:text-foreground transition-colors rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                  >
                    {showPassword ? EyeOffIcon : EyeIcon}
                  </button>
                </div>
              </div>

              {error && (
                <p className="text-[13px] text-destructive -mt-2 mb-4" role="alert">{error}</p>
              )}

              <Button
                type="submit"
                disabled={loading || !trimmed}
                className="w-full h-11 text-[15px] font-semibold tracking-wide"
              >
                {loading ? "登录中..." : "登录"}
              </Button>
            </form>
        </main>
      </div>
    </div>
  );
}
