import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAppSelector } from "@/store/hooks";
import { ROUTES } from "@/lib/constants";
import { getAuthToken } from "@/lib/authToken";

export default function AuthGuard() {
  const { isAuthenticated } = useAppSelector((state) => state.user);
  const location = useLocation();
  const token = getAuthToken();

  if (!token || !isAuthenticated) {
    return <Navigate to={ROUTES.auth} state={{ from: location }} replace />;
  }

  return <Outlet />;
}
