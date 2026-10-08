import type {
  CurrentUser,
  GetAuthCsrfResponse,
  Portfolio,
  PortfolioList,
  User,
} from "../../src/api";

export interface SessionFixture {
  currentUser: CurrentUser;
  portfolios: PortfolioList;
  csrf: GetAuthCsrfResponse;
}

/** Each scenario receives independent, contract-checked session responses. */
export function sessionFixture(
  options: {
    portfolio?: Partial<Portfolio>;
    user?: Partial<User>;
    extraPortfolios?: Portfolio[];
  } = {},
): SessionFixture {
  const portfolio: Portfolio = {
    id: "00000000-0000-7000-8000-000000000001",
    name: "Browser check",
    timezone: "UTC",
    collectionIntervalSeconds: 3600,
    undeletable: true,
    version: 1,
    createdAt: "2026-10-03T00:00:00Z",
    updatedAt: "2026-10-03T00:00:00Z",
    ...options.portfolio,
  };
  return {
    currentUser: {
      user: { id: "user", username: "browser-check", status: "active", ...options.user },
      sessionCount: 1,
    },
    portfolios: { items: [portfolio, ...(options.extraPortfolios ?? [])] },
    csrf: { token: "offline-check" },
  };
}
