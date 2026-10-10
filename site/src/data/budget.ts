export interface BudgetFile {
  rank: number;
  path: string;
  full: number;
  sigs: number | null;
}

export const BUDGET_FILES: BudgetFile[] = [
  { rank: 1, path: 'internal/api/handler.go', full: 4200, sigs: 600 },
  { rank: 2, path: 'internal/api/router.go', full: 2100, sigs: 380 },
  { rank: 3, path: 'cmd/api/main.go', full: 1800, sigs: 300 },
  { rank: 4, path: 'internal/api/middleware.go', full: 3400, sigs: 520 },
  { rank: 5, path: 'internal/store/postgres.go', full: 7800, sigs: 900 },
  { rank: 6, path: 'README.md', full: 2600, sigs: null },
  { rank: 7, path: 'internal/store/query.go', full: 5900, sigs: 700 },
  { rank: 8, path: 'internal/legacy/parser.go', full: 11400, sigs: 1300 },
  { rank: 9, path: 'go.mod', full: 400, sigs: null },
  { rank: 10, path: 'internal/config/load.go', full: 3100, sigs: 450 },
];

export type FileBudgetStatus = 'FULL' | 'SIGS' | 'SKIP';

export interface BudgetSimulationRow {
  rank: number;
  path: string;
  status: FileBudgetStatus;
  tokensUsed: number;
}

export function computeBudgetSimulation(budget: number): {
  rows: BudgetSimulationRow[];
  totalUsed: number;
} {
  let remaining = budget;
  let totalUsed = 0;

  const rows: BudgetSimulationRow[] = BUDGET_FILES.map((file) => {
    if (file.full <= remaining) {
      remaining -= file.full;
      totalUsed += file.full;
      return { rank: file.rank, path: file.path, status: 'FULL', tokensUsed: file.full };
    } else if (file.sigs !== null && file.sigs <= remaining) {
      remaining -= file.sigs;
      totalUsed += file.sigs;
      return { rank: file.rank, path: file.path, status: 'SIGS', tokensUsed: file.sigs };
    }
    return { rank: file.rank, path: file.path, status: 'SKIP', tokensUsed: 0 };
  });

  return { rows, totalUsed };
}
