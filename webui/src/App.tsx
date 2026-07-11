import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import { Suspense, lazy, useMemo } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { ThemeProvider } from "@/components/theme-provider";
import { ErrorBoundary } from "@/components/error-boundary";
import { queryClient } from "@/lib/query-client";
import Loading from "@/components/loading";
import { Toaster } from "./components/ui/sonner";
import {
  appChildPath,
  appRoutes,
  createLazyPages,
  layoutLazy,
} from "./routes/route-config";

const Layout = lazy(layoutLazy);

const PageLoader = () => (
  <div className="flex items-center justify-center min-h-screen">
    <Loading message="加载中..." />
  </div>
);

function App() {
  const lazyPages = useMemo(() => createLazyPages(appRoutes), []);

  const standaloneRoutes = appRoutes.filter((r) => r.layout === "none");
  const appChildRoutes = appRoutes.filter((r) => r.layout === "app");

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="system" storageKey="vite-ui-theme">
        <ErrorBoundary>
          <Router>
            <Routes>
              {standaloneRoutes.map((route) => {
                const Page = lazyPages.get(route.path)!;
                return (
                  <Route
                    key={route.path}
                    path={route.path}
                    element={
                      <Suspense fallback={<PageLoader />}>
                        <Page />
                      </Suspense>
                    }
                  />
                );
              })}
              <Route
                path="/"
                element={
                  <Suspense fallback={<PageLoader />}>
                    <Layout />
                  </Suspense>
                }
              >
                {appChildRoutes.map((route) => {
                  const Page = lazyPages.get(route.path)!;
                  const element = <Page />;
                  if (route.index) {
                    return <Route key={route.path} index element={element} />;
                  }
                  return (
                    <Route
                      key={route.path}
                      path={appChildPath(route)}
                      element={element}
                    />
                  );
                })}
              </Route>
            </Routes>
          </Router>
        </ErrorBoundary>
        <Toaster richColors position="top-center" />
      </ThemeProvider>
      {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
    </QueryClientProvider>
  );
}

export default App;