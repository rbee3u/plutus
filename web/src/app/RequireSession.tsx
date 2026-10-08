import { Navigate, Outlet } from "react-router";
import { useCurrentUser } from "@/shared/lib/session";
import { RootRedirect } from "@/app/shell/Shell";
import { RequestRecovery } from "@/shared/ui/QueryRecovery";

/** Authentication gating belongs to route composition, before mounting features. */
export function RequireSession() {
  const user = useCurrentUser();
  if (user.isPending) return <RootRedirect />;
  if (user.error)
    return (
      <RequestRecovery
        error={user.error}
        hasData={false}
        busy={user.isFetching}
        fallback="Plutus could not restore the session."
        onRetry={() => {
          void user.refetch();
        }}
      />
    );
  return user.data ? <Outlet /> : <Navigate to="/login" replace />;
}
