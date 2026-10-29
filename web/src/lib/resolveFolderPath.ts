import { getFolderChildren } from "@/api/files";

/** Resolve `Cloud › Photos › 2024` style paths from the user's root folder. */
export async function resolveFolderPath(rootFolderId: string, rawPath: string): Promise<string | null> {
  const normalized = rawPath.replace(/\\/g, "/").trim();
  if (!normalized) return rootFolderId;

  const parts = normalized
    .split(/[/›>]/)
    .map((p) => p.trim())
    .filter((p) => p.length > 0 && p.toLowerCase() !== "cloud" && p.toLowerCase() !== "root");

  let currentId = rootFolderId;
  for (const segment of parts) {
    const { folders } = await getFolderChildren(currentId);
    const match = folders.find((f) => f.name.localeCompare(segment, undefined, { sensitivity: "base" }) === 0);
    if (!match) return null;
    currentId = match.id;
  }
  return currentId;
}
