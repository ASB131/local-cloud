import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { TransferPanel } from "@/components/TransferPanel";
import { CloudLayout } from "@/layout/CloudLayout";
import { AdminShellPage } from "@/pages/AdminShellPage";
import { FavoritesPage } from "@/pages/FavoritesPage";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { TrashPage } from "@/pages/TrashPage";
import { CloudFolderPage } from "@/pages/CloudFolderPage";
import { CloudCategoryPage } from "@/pages/CloudCategoryPage";
import { CloudSearchPage } from "@/pages/CloudSearchPage";
import { CloudHomePage } from "@/pages/CloudHomePage";
import { AuthProvider, useAuth } from "@/state/auth";
import { TransfersProvider } from "@/state/transfersContext";
import { connectCloudWs } from "@/ws/cloud";

const queryClient = new QueryClient();

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center text-zinc-400">Loading…</div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RequireGuest({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return null;
  if (user) return <Navigate to="/cloud" replace />;
  return <>{children}</>;
}

function CloudWsBridge() {
  const { accessToken } = useAuth();
  useEffect(() => {
    if (!accessToken) return;
    const client = connectCloudWs(accessToken, queryClient);
    return () => client.close();
  }, [accessToken]);
  return null;
}

function RootRedirect() {
  const { user, loading } = useAuth();
  if (loading) return null;
  return <Navigate to={user ? "/cloud" : "/login"} replace />;
}

export function App() {
  const qc = useMemo(() => queryClient, []);
  return (
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <TransfersProvider>
          <BrowserRouter>
            <CloudWsBridge />
            <Routes>
              <Route path="/" element={<RootRedirect />} />
              <Route path="/login" element={<RequireGuest><LoginPage /></RequireGuest>} />
              <Route path="/register" element={<RequireGuest><RegisterPage /></RequireGuest>} />
              <Route
                path="/cloud"
                element={
                  <RequireAuth>
                    <CloudLayout />
                  </RequireAuth>
                }
              >
                <Route index element={<CloudHomePage />} />
                <Route path="folder/:folderId" element={<CloudFolderPage />} />
                <Route path="search" element={<CloudSearchPage />} />
                <Route path="types/:categoryId" element={<CloudCategoryPage />} />
                <Route path="favorites" element={<FavoritesPage />} />
                <Route path="trash" element={<TrashPage />} />
              </Route>
              <Route
                path="/settings"
                element={
                  <RequireAuth>
                    <SettingsPage />
                  </RequireAuth>
                }
              />
              <Route
                path="/admin/*"
                element={
                  <RequireAuth>
                    <AdminShellPage />
                  </RequireAuth>
                }
              />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
            <TransferPanel />
          </BrowserRouter>
        </TransfersProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}
