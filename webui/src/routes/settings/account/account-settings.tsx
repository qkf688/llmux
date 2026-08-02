import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { selectAuthUser, useAuthStore } from "@/stores/auth";
import { rotateAPIKey, changePassword } from "@/lib/api/modules/auth/auth";

export function AccountSettings() {
  const user = useAuthStore(selectAuthUser);
  const [newAPIKey, setNewAPIKey] = useState<string | null>(null);
  const [keyError, setKeyError] = useState("");
  const [oldPwd, setOldPwd] = useState("");
  const [newPwd, setNewPwd] = useState("");
  const [pwdMsg, setPwdMsg] = useState("");
  const [pwdError, setPwdError] = useState("");

  const rotateMutation = useMutation({
    mutationFn: () => rotateAPIKey(),
    onSuccess: (res) => {
      setNewAPIKey(res.api_key);
      setKeyError("");
    },
    onError: (err) => {
      setKeyError(err instanceof Error ? err.message : "轮换失败");
      setNewAPIKey(null);
    },
  });

  const pwdMutation = useMutation({
    mutationFn: () => changePassword(oldPwd, newPwd),
    onSuccess: () => {
      setPwdMsg("密码已更新");
      setPwdError("");
      setOldPwd("");
      setNewPwd("");
    },
    onError: (err) => {
      setPwdError(err instanceof Error ? err.message : "修改失败");
      setPwdMsg("");
    },
  });

  const handleRotate = () => {
    if (!confirm("确定轮换 API key？旧 key 将立即失效。")) return;
    setNewAPIKey(null);
    rotateMutation.mutate();
  };

  const handleChangePassword = (e: React.FormEvent) => {
    e.preventDefault();
    if (!oldPwd.trim() || !newPwd.trim()) return;
    pwdMutation.mutate();
  };

  return (
    <div className="space-y-4 md:space-y-6">
      <div>
        <h2 className="text-lg md:text-xl font-semibold">账户</h2>
        <p className="text-xs md:text-sm text-muted-foreground">管理登录账户与 API key</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>账户信息</CardTitle>
          <CardDescription>当前登录用户</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div>用户名：<strong>{user?.username ?? "admin"}</strong></div>
          <div>角色：<strong>{user?.role ?? "admin"}</strong></div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>/v1 API Key</CardTitle>
          <CardDescription>
            用于调用代理 API（/v1）的鉴权 key。轮换后旧 key 立即失效，新 key 仅显示一次。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {newAPIKey ? (
            <div className="space-y-2">
              <Label className="text-sm font-medium">新 API Key（请立即保存）</Label>
              <Input readOnly value={newAPIKey} className="font-mono text-xs" />
            </div>
          ) : (
            <p className="text-xs md:text-sm text-muted-foreground">
              当前 key 已隐藏。点击下方按钮轮换以获取新 key。
            </p>
          )}
          {keyError && <p className="text-sm text-destructive">{keyError}</p>}
          <Button onClick={handleRotate} disabled={rotateMutation.isPending}>
            {rotateMutation.isPending ? "轮换中..." : "轮换 API Key"}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>修改密码</CardTitle>
          <CardDescription>更新管理员登录密码</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="space-y-4 max-w-sm">
            <div className="space-y-1.5">
              <Label htmlFor="old-pwd" className="text-sm font-medium">旧密码</Label>
              <Input
                id="old-pwd"
                type="password"
                value={oldPwd}
                onChange={(e) => setOldPwd(e.target.value)}
                disabled={pwdMutation.isPending}
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-pwd" className="text-sm font-medium">新密码</Label>
              <Input
                id="new-pwd"
                type="password"
                value={newPwd}
                onChange={(e) => setNewPwd(e.target.value)}
                disabled={pwdMutation.isPending}
              />
            </div>
            {pwdMsg && <p className="text-sm text-primary">{pwdMsg}</p>}
            {pwdError && <p className="text-sm text-destructive">{pwdError}</p>}
            <Button type="submit" disabled={pwdMutation.isPending || !oldPwd.trim() || !newPwd.trim()}>
              {pwdMutation.isPending ? "更新中..." : "更新密码"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
