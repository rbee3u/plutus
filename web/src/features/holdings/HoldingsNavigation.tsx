import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import { Link, useLocation, useNavigate, type LinkProps } from "react-router";
import { useSessionScope } from "@/shared/lib/session";

export function holdingsStateKey(userId: string | undefined, pathname: string): string | null {
  const route = pathname.match(/^\/p\/([^/]+)\/holdings\/(balances|futures|dual-investment)$/);
  return userId && route ? JSON.stringify([userId, route[1], route[2]]) : null;
}

type Prepare = { pathname: string; search: () => string };
type Navigation = {
  href: (to: string) => string;
  go: (to: string) => void;
  register: (prepare: Prepare) => () => void;
};
const NavigationContext = createContext<Navigation | null>(null);

/** Memory belongs to this signed-in app session; the current URL always wins on entry. */
export function HoldingsNavigationProvider({ children }: { children: ReactNode }) {
  const { userId } = useSessionScope();
  const location = useLocation();
  const navigate = useNavigate();
  const searches = useRef(new Map<string, string>());
  const active = useRef<Prepare | null>(null);
  useEffect(() => {
    const key = holdingsStateKey(userId, location.pathname);
    if (key) searches.current.set(key, location.search);
  }, [userId, location.pathname, location.search]);
  const register = useCallback((prepare: Prepare) => {
    active.current = prepare;
    return () => {
      if (active.current === prepare) active.current = null;
    };
  }, []);
  const href = useCallback(
    (to: string) => {
      if (to.includes("?") || to.includes("#")) return to;
      const key = holdingsStateKey(userId, to);
      return key ? `${to}${searches.current.get(key) ?? ""}` : to;
    },
    [userId],
  );
  const go = useCallback(
    (to: string) => {
      const prepare = active.current;
      if (prepare?.pathname === location.pathname) {
        const key = holdingsStateKey(userId, prepare.pathname);
        if (key) searches.current.set(key, prepare.search());
      }
      void navigate(href(to));
    },
    [href, location.pathname, navigate, userId],
  );
  const value = useMemo(() => ({ href, go, register }), [href, go, register]);
  return <NavigationContext value={value}>{children}</NavigationContext>;
}

export function useHoldingsNavigation() {
  const navigation = useContext(NavigationContext);
  if (!navigation) throw new Error("Holdings navigation requires the application Shell.");
  return navigation;
}

/** Normal clicks save drafts first; modified clicks retain native link behavior. */
export function SessionLink({ to, onClick, ...props }: LinkProps & { to: string }) {
  const navigation = useHoldingsNavigation();
  return (
    <Link
      {...props}
      to={navigation.href(to)}
      onClick={(event) => {
        onClick?.(event);
        if (
          event.defaultPrevented ||
          event.button !== 0 ||
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey ||
          props.target === "_blank"
        )
          return;
        event.preventDefault();
        navigation.go(to);
      }}
    />
  );
}
