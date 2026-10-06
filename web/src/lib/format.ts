/**
 * Formatting utilities for tokens, sizes, and paths.
 */

export function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(1)}M`;
  }
  if (tokens >= 1000) {
    const k = tokens / 1000;
    return k >= 10 ? `${Math.round(k)}k` : `${k.toFixed(1)}k`;
  }
  return tokens.toLocaleString();
}

export function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${bytes} B`;
}

export function truncateMiddle(text: string, maxLength = 36): string {
  if (text.length <= maxLength) return text;
  const part = Math.floor((maxLength - 3) / 2);
  return `${text.slice(0, part)}...${text.slice(-part)}`;
}

/**
 * Human-friendly relative time for recents ("just now", "3h ago", "Mar 4").
 * Passes through strings that are not parseable dates (mock data already
 * ships friendly labels like "10 minutes ago").
 */
export function formatRelativeTime(input: string): string {
  const when = Date.parse(input);
  if (Number.isNaN(when)) return input;
  const diffMs = Date.now() - when;
  if (diffMs < 0) return 'just now';
  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;
  const date = new Date(when);
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
