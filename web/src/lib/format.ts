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
