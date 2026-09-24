/* eslint-disable react-refresh/only-export-components */
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { selectIsAuthenticated, useAuthStore } from "@/stores/auth";
import { redirectToLogin } from "@/lib/api/core/client";

// Redirect to login if not authenticated (except for login page)
const ProtectedApp = () => {
  const isAuthenticated = useAuthStore(selectIsAuthenticated);
  const path = window.location.pathname;

  // Allow access to login page without authentication
  if (path === '/login') {
    return <App />;
  }

  // Redirect to login if not authenticated
  if (!isAuthenticated) {
    // 跳登录页走失效处置的单一 owner（清会话 + 跳转）：此处是「未登录直入」，
    // 不写过期提示；禁止再手写 window.location.href = '/login' 另开第二处跳转 owner
    redirectToLogin();
    return null;
  }

  return <App />;
};

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ProtectedApp />
  </StrictMode>,
) 
