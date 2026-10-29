export type LocalFileEntry = {
  file: File;
  relativePath: string;
};

export async function walkFileTree(entry: FileSystemEntry, prefix = ""): Promise<LocalFileEntry[]> {
  if (entry.isFile) {
    const fe = entry as FileSystemFileEntry;
    const file = await new Promise<File>((resolve, reject) => {
      fe.file(resolve, reject);
    });
    const path = prefix ? `${prefix}/${file.name}` : file.name;
    return [{ file, relativePath: path }];
  }
  if (!entry.isDirectory) return [];
  const dir = entry as FileSystemDirectoryEntry;
  const reader = dir.createReader();
  const out: LocalFileEntry[] = [];
  const readBatch = (): Promise<FileSystemEntry[]> =>
    new Promise((resolve, reject) => reader.readEntries(resolve, reject));
  let batch: FileSystemEntry[];
  do {
    batch = await readBatch();
    for (const child of batch) {
      const childPrefix = prefix ? `${prefix}/${child.name}` : child.name;
      const nested = await walkFileTree(child, child.isDirectory ? childPrefix : prefix);
      out.push(...nested);
    }
  } while (batch.length > 0);
  return out;
}

export function filesFromInputList(fileList: FileList): LocalFileEntry[] {
  return Array.from(fileList).map((file) => ({
    file,
    relativePath: (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name,
  }));
}
