export function safeHttpURL(value?: string): string | undefined {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : undefined;
  } catch {
    return undefined;
  }
}

export function safeURLHost(value?: string): string | undefined {
  const safeURL = safeHttpURL(value);
  if (!safeURL) return undefined;
  return new URL(safeURL).host;
}
