import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Suspense, lazy } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { ThemeProvider } from "@/components/theme-provider"
import { ErrorBoundary } from "@/components/error-boundary"
import { queryClient } from "@/lib/query-client"
import Loading from "@/components/loading"
import { Toaster } from './components/ui/sonner';

// 懒加载路由组件
const Layout = lazy(() => import('./routes/layout'));
const Home = lazy(() => import('./routes/home'));
const ProvidersPage = lazy(() => import('./routes/providers'));
const ModelsPage = lazy(() => import('./routes/models'));
const ModelProvidersPage = lazy(() => import('./routes/model-providers'));
const LogsPage = lazy(() => import('./routes/logs'));
const LogChatPage = lazy(() => import('./routes/log-chat'));
const LoginPage = lazy(() => import('./routes/login'));
const SettingsPage = lazy(() => import('./routes/settings'));
const HealthCheckLogsPage = lazy(() => import('./routes/health-check-logs'));
const ModelSyncLogsPage = lazy(() => import('./routes/model-sync-logs'));
const DatabasePage = lazy(() => import('./routes/database'));
const VirtualModelsPage = lazy(() => import('./routes/virtual-models'));

// 简单的加载组件
const PageLoader = () => (
  <div className="flex items-center justify-center min-h-screen">
    <Loading message="加载中..." />
  </div>
);

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="system" storageKey="vite-ui-theme">
        <ErrorBoundary>
          <Router>
            <Routes>
              <Route
                path="/login"
                element={
                  <Suspense fallback={<PageLoader />}>
                    <LoginPage />
                  </Suspense>
                }
              />
              <Route
                path="/"
                element={
                  <Suspense fallback={<PageLoader />}>
                    <Layout />
                  </Suspense>
                }
              >
                <Route index element={<Home />} />
                <Route path="providers" element={<ProvidersPage />} />
                <Route path="models" element={<ModelsPage />} />
                <Route path="model-providers" element={<ModelProvidersPage />} />
                <Route path="virtual-models" element={<VirtualModelsPage />} />
                <Route path="logs" element={<LogsPage />} />
                <Route path="logs/:logId/chat-io" element={<LogChatPage />} />
                <Route path="health-check-logs" element={<HealthCheckLogsPage />} />
                <Route path="model-sync-logs" element={<ModelSyncLogsPage />} />
                <Route path="database" element={<DatabasePage />} />
                <Route path="settings" element={<SettingsPage />} />
              </Route>
            </Routes>
          </Router>
        </ErrorBoundary>
        <Toaster richColors position='top-center' />
      </ThemeProvider>
      {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
    </QueryClientProvider>
  );
}

export default App;
 
