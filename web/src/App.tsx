import { Routes, Route, Navigate } from "react-router";
import { useAuth } from "@/lib/auth";
import { Layout } from "@/components/Layout";
import { PageSpinner } from "@/components/ui/Spinner";
import LoginPage from "@/pages/LoginPage";
import LibraryPage from "@/pages/LibraryPage";
import CreateMangaPage from "@/pages/CreateMangaPage";
import EditMangaPage from "@/pages/EditMangaPage";
import MangaDetailPage from "@/pages/MangaDetailPage";
import CreateChapterPage from "@/pages/CreateChapterPage";
import EditChapterPage from "@/pages/EditChapterPage";
import ViewsManagerPage from "@/pages/ViewsManagerPage";
import SettingsPage from "@/pages/SettingsPage";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  if (isLoading) return <PageSpinner />;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<LibraryPage />} />
        <Route path="manga/new" element={<CreateMangaPage />} />
        <Route path="manga/:id" element={<MangaDetailPage />} />
        <Route path="manga/:id/edit" element={<EditMangaPage />} />
        <Route path="manga/:id/chapters/new" element={<CreateChapterPage />} />
        <Route path="manga/:id/chapters/:chapterId/edit" element={<EditChapterPage />} />
        <Route path="paperback" element={<ViewsManagerPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
