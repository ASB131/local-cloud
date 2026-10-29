import { Navigate, useParams, useSearchParams } from "react-router-dom";
import { useAuth } from "@/state/auth";

/** Legacy `/cloud/types/:id` → folder explorer with `?category=` */
export function CloudCategoryPage() {
  const { categoryId = "" } = useParams();
  const { user } = useAuth();
  const [searchParams] = useSearchParams();
  const root = user?.root_folder_id ?? "";
  const preview = searchParams.get("preview");
  const q = new URLSearchParams({ category: categoryId });
  if (preview) q.set("preview", preview);
  return <Navigate to={`/cloud/folder/${root}?${q.toString()}`} replace />;
}
