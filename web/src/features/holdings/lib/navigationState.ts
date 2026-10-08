export function holdingsStateKey(userId: string | undefined, pathname: string): string | null {
  const route = pathname.match(/^\/p\/([^/]+)\/holdings\/(balances|futures|earn|dual-investment)$/);
  return userId && route ? JSON.stringify([userId, route[1], route[2]]) : null;
}
