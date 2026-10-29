const tokenCache = new Map<string, { token: string; expiresAt: number }>();

export function contentUrl(fileId: string, mediaToken: string) {
  return `/api/v1/files/${fileId}/content?mt=${encodeURIComponent(mediaToken)}`;
}

export function versionContentUrl(fileId: string, versionId: string, mediaToken: string) {
  return `/api/v1/files/${fileId}/versions/${versionId}/content?mt=${encodeURIComponent(mediaToken)}`;
}

export function thumbnailUrl(fileId: string, mediaToken: string) {
  return `/api/v1/files/${fileId}/thumbnail?mt=${encodeURIComponent(mediaToken)}`;
}

export function officePreviewUrl(fileId: string, mediaToken: string) {
  return `/api/v1/files/${fileId}/office-preview?mt=${encodeURIComponent(mediaToken)}`;
}

export async function getMediaToken(
  fileId: string,
  fetchToken: (fileId: string) => Promise<{ token: string; expires_at: string }>,
): Promise<string> {
  const cacheKey = `file:${fileId}`;
  const cached = tokenCache.get(cacheKey);
  const now = Date.now();
  if (cached && cached.expiresAt - 60_000 > now) {
    return cached.token;
  }
  const res = await fetchToken(fileId);
  tokenCache.set(cacheKey, { token: res.token, expiresAt: Date.parse(res.expires_at) });
  return res.token;
}

export async function getVersionMediaToken(
  fileId: string,
  versionId: string,
  fetchToken: (fileId: string, versionId: string) => Promise<{ token: string; expires_at: string }>,
): Promise<string> {
  const cacheKey = `ver:${fileId}:${versionId}`;
  const cached = tokenCache.get(cacheKey);
  const now = Date.now();
  if (cached && cached.expiresAt - 60_000 > now) {
    return cached.token;
  }
  const res = await fetchToken(fileId, versionId);
  tokenCache.set(cacheKey, { token: res.token, expiresAt: Date.parse(res.expires_at) });
  return res.token;
}
