/**
 * Mock implementations for isolated UI work (VITE_MOCK=true).
 *
 * Same signatures as the live client; backed by in-memory state in
 * `./mock`. Production builds never call these (see `index.ts`).
 */

import type {
  ApiMeta,
  BrowseResult,
  FileContentResult,
  PackPayload,
  PackResult,
  RecentProject,
  SettingsData,
  SmartSelectResult,
  TreeResult,
} from './types';
import { loginError } from './live';
import {
  mockAuthenticated,
  mockLockoutUntil,
  mockRecents,
  mockSettings,
  mockTreeFiles,
  recordMockAttempt,
  resetMockAuth,
  setMockAuthenticated,
  setMockLockout,
  setMockRecents,
  setMockSettings,
  stamp,
} from './mock';

const delay = (ms = 45) => new Promise((r) => setTimeout(r, ms));

export async function getMeta(): Promise<ApiMeta> {
  await delay(30);
  return {
    version: 'v1.4.0-dev',
    styles: ['xml', 'markdown', 'plain'],
    mode: 'local',
    tls: false,
    roots: ['/Users/developer/code', '/Users/developer/work'],
    authKind: 'token',
    authenticated: mockAuthenticated,
  };
}

export async function login(password: string): Promise<void> {
  await delay(70);
  const now = Date.now();
  if (mockLockoutUntil > now) {
    const remaining = Math.ceil((mockLockoutUntil - now) / 1000);
    throw loginError(`Too many attempts, try again in ${remaining}s`, remaining);
  }

  // Mock rule: any password with length >= 4 is accepted unless "wrong"
  if (password === 'wrong') {
    const attempts = recordMockAttempt();
    if (attempts >= 3) {
      setMockLockout(Date.now() + 15000);
      throw loginError('Too many attempts, try again in 15s', 15);
    }
    throw new Error('Wrong password');
  }

  resetMockAuth();
  setMockAuthenticated(true);
}

export async function loginWithToken(token: string): Promise<void> {
  await delay(70);
  if (!token) throw new Error('Missing token');
  setMockAuthenticated(true);
}

export async function logout(): Promise<void> {
  await delay(20);
  setMockAuthenticated(false);
}

export async function getRecents(): Promise<RecentProject[]> {
  await delay(35);
  return [...mockRecents];
}

export async function recordRecent(root: string): Promise<void> {
  await delay(20);
}

export async function deleteRecent(root: string): Promise<void> {
  await delay(30);
  setMockRecents(mockRecents.filter((r) => r.root !== root));
}

export async function browse(pathQuery: string, hidden = false): Promise<BrowseResult> {
  await delay(45);
  const cleanPath = pathQuery.replace(/\/+$/, '') || '/Users/developer/code';

  // Simulate 403 outside allowed roots
  const allowed = ['/Users/developer/code', '/Users/developer/work'];
  const isAllowed = allowed.some((a) => cleanPath === a || cleanPath.startsWith(a + '/'));
  if (!isAllowed && cleanPath !== '/') {
    throw new Error("Couldn't read that folder: outside allowed roots");
  }

  // Mock directories
  if (cleanPath === '/Users/developer/code') {
    return {
      path: '/Users/developer/code',
      parent: '/Users/developer',
      entries: stamp([
        { name: 'sift', isDir: true, isGitRepo: true },
        { name: 'astral-uv', isDir: true, isGitRepo: true },
        { name: 'conc', isDir: true, isGitRepo: true },
        { name: 'tools', isDir: true, isGitRepo: false },
        { name: 'scratchpad', isDir: true, isGitRepo: false },
        { name: 'notes.md', isDir: false, isGitRepo: false },
      ]),
    };
  }

  if (cleanPath === '/Users/developer/code/sift') {
    return {
      path: '/Users/developer/code/sift',
      parent: '/Users/developer/code',
      entries: stamp([
        { name: 'cmd', isDir: true, isGitRepo: false },
        { name: 'pkg', isDir: true, isGitRepo: false },
        { name: 'internal', isDir: true, isGitRepo: false },
        { name: 'web', isDir: true, isGitRepo: false },
        { name: 'docs', isDir: true, isGitRepo: false },
        { name: 'scripts', isDir: true, isGitRepo: false },
        { name: 'go.mod', isDir: false, isGitRepo: false },
      ]),
    };
  }

  // Default directory representation
  return {
    path: cleanPath,
    parent: cleanPath.split('/').slice(0, -1).join('/') || '/',
    entries: stamp([
      { name: 'src', isDir: true, isGitRepo: false },
      { name: 'pkg', isDir: true, isGitRepo: false },
      { name: 'tests', isDir: true, isGitRepo: false },
      { name: '.git', isDir: true, isGitRepo: true },
      { name: 'README.md', isDir: false, isGitRepo: false },
    ]),
  };
}

export async function getTree(root: string): Promise<TreeResult> {
  await delay(60);

  // If opening a new root, add to recents list
  const existing = mockRecents.find((r) => r.root === root);
  const shortName = root.split('/').filter(Boolean).pop() || 'project';
  if (!existing) {
    mockRecents.unshift({
      root,
      name: shortName,
      lastOpened: 'Just now',
      branch: 'main',
    });
  } else {
    existing.lastOpened = 'Just now';
  }

  return {
    root,
    files: mockTreeFiles,
    budget: 64000,
    budgetSource: 'default',
  };
}

export async function getFile(root: string, path: string, mode: 'full' | 'sigs'): Promise<FileContentResult> {
  await delay(40);

  const fileMeta = mockTreeFiles.find((f) => f.path === path);
  const lang = fileMeta?.language || 'go';
  const baseTokens = fileMeta?.tokens || 500;
  const tokens = mode === 'sigs' ? Math.max(25, Math.floor(baseTokens * 0.22)) : baseTokens;

  // Generate realistic file content based on mode and language
  if (mode === 'sigs') {
    const sigsContent = `// Package ${path.split('/')[1] || 'main'} signatures extracted by sift
package ${path.split('/')[1] || 'main'}

import (
\t"context"
\t"io"
\t"time"
)

// Types and exported interfaces
type Config struct {
\tRootPath string
\tMaxTokens int
\tRedactSecrets bool
}

type Service interface {
\tExecute(ctx context.Context, input io.Reader) (*Result, error)
\tClose() error
}

func NewService(cfg Config) (Service, error)
func ExtractSignatures(source []byte, lang string) ([]byte, int, error)
func Validate(ctx context.Context) error
`;
    return {
      content: sigsContent,
      tokens,
      language: lang,
      truncated: false,
    };
  }

  // Full file content simulation
  const fullContent = `// Code generated and maintained by sift developers.
// Path: ${path}
package ${path.split('/')[1] || 'main'}

import (
\t"context"
\t"fmt"
\t"io"
\t"strings"
\t"time"
)

// APIKeyHeader for authentication
const APIKeyHeader = "X-Sift-Key"

// Secrets for testing redaction rules
var testAPISecret = "MOCK-SECRET-KEY-0000-FAKE"

type Options struct {
\tRoot       string \`json:"root"\`
\tMaxTokens  int    \`json:"max_tokens"\`
\tRedact     bool   \`json:"redact"\`
\tStyle      string \`json:"style"\`
}

type Collector struct {
\topts    Options
\tstarted time.Time
\tcount   int
}

func NewCollector(opts Options) (*Collector, error) {
\tif opts.Root == "" {
\t\treturn nil, fmt.Errorf("root cannot be empty")
\t}
\treturn &Collector{
\t\topts:    opts,
\t\tstarted: time.Now(),
\t\tcount:   0,
\t}, nil
}

func (c *Collector) Process(ctx context.Context, r io.Reader) error {
\tselect {
\tcase <-ctx.Done():
\t\treturn ctx.Err()
\tdefault:
\t\tc.count++
\t\treturn nil
\t}
}
`;

  return {
    content: fullContent,
    tokens,
    language: lang,
    truncated: false,
  };
}

export async function smartSelect(root: string, budget: number): Promise<SmartSelectResult> {
  await delay(75);

  // Algorithm: rank by score, include top scored files as FULL,
  // secondary files as SIGS, skipping low-relevance or huge files to fit budget.
  const sorted = [...mockTreeFiles].sort((a, b) => b.score - a.score);
  const selections: Record<string, 'full' | 'sigs' | 'skip'> = {};
  let usedTokens = 0;

  for (const file of sorted) {
    // Very large files like tables.go or vocab.json or score < 0.35 get skipped or sigs
    if (file.tokens > 15000 || file.score < 0.35) {
      selections[file.path] = 'skip';
      continue;
    }

    if (usedTokens + file.tokens <= budget * 0.82 && file.score >= 0.75) {
      selections[file.path] = 'full';
      usedTokens += file.tokens;
    } else if (usedTokens + Math.floor(file.tokens * 0.22) <= budget && file.score >= 0.5) {
      selections[file.path] = 'sigs';
      usedTokens += Math.floor(file.tokens * 0.22);
    } else {
      selections[file.path] = 'skip';
    }
  }

  return { selections };
}

export async function pack(payload: PackPayload): Promise<PackResult> {
  await delay(90);

  const { selections, budget, style, prompt, redact } = payload;
  let totalTokens = 0;
  let fileCount = 0;
  let redactionsCount = 0;
  const skipped: string[] = [];
  const formattedBlocks: string[] = [];

  // Prompt header
  if (prompt && prompt.trim()) {
    if (style === 'xml') {
      formattedBlocks.push(`<task_prompt>\n${prompt.trim()}\n</task_prompt>\n`);
    } else if (style === 'markdown') {
      formattedBlocks.push(`## Task Prompt\n\n${prompt.trim()}\n\n---\n`);
    } else {
      formattedBlocks.push(`TASK PROMPT:\n${prompt.trim()}\n\n========================================\n`);
    }
    totalTokens += Math.ceil(prompt.length / 4);
  }

  // Process selected files
  for (const [path, mode] of Object.entries(selections)) {
    if (mode === 'skip') {
      skipped.push(path);
      continue;
    }

    const file = mockTreeFiles.find((f) => f.path === path);
    if (!file) continue;

    const fileTokens = mode === 'sigs' ? Math.max(25, Math.floor(file.tokens * 0.22)) : file.tokens;

    // Check if exceeds budget
    if (totalTokens + fileTokens > budget) {
      skipped.push(path);
      continue;
    }

    fileCount++;
    totalTokens += fileTokens;

    let codeSnippet = `// File: ${path} (${mode.toUpperCase()})\npackage ${path.split('/')[1] || 'main'}\n\nvar secretKey = "MOCK-SECRET-KEY-0000-FAKE"\n\nfunc Run() error {\n\treturn nil\n}`;
    if (mode === 'sigs') {
      codeSnippet = `// Signatures: ${path}\npackage ${path.split('/')[1] || 'main'}\n\nfunc Run() error`;
    }

    if (redact) {
      if (codeSnippet.includes('MOCK-SECRET-KEY-')) {
        codeSnippet = codeSnippet.replace(/MOCK-SECRET-KEY-[A-Z0-9-]+/g, '[REDACTED_SECRET_KEY]');
        redactionsCount++;
      }
    }

    if (style === 'xml') {
      formattedBlocks.push(
        `<file path="${path}" mode="${mode}" tokens="${fileTokens}">\n${codeSnippet}\n</file>`,
      );
    } else if (style === 'markdown') {
      formattedBlocks.push(
        `### ${path} (${mode.toUpperCase()})\n\`\`\`${file.language || 'text'}\n${codeSnippet}\n\`\`\`\n`,
      );
    } else {
      formattedBlocks.push(
        `--- FILE: ${path} [${mode.toUpperCase()}] ---\n${codeSnippet}\n`,
      );
    }
  }

  const document = formattedBlocks.join('\n\n');

  return {
    document,
    tokens: totalTokens,
    fileCount,
    redactions: redact ? redactionsCount : 0,
    skipped,
  };
}

export async function getSettings(): Promise<SettingsData> {
  await delay(25);
  return { ...mockSettings };
}

export async function saveSettings(settings: SettingsData): Promise<void> {
  await delay(40);
  setMockSettings({ ...settings });
}
