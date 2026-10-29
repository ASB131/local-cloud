/** Explorer label: `notes.txt` → `notes`. Leaves `.gitignore`-style names unchanged. */
export function fileDisplayName(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return name;
  return name.slice(0, i);
}

export function fileExtensionWithDot(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i);
}

/** Initial value when renaming a file in the explorer (extension hidden). */
export function fileRenameEditValue(fullName: string): string {
  return fileDisplayName(fullName);
}

/** Turn rename field text back into a stored file name, keeping the original extension unless the user typed a new one. */
export function fileNameFromRenameEdit(edited: string, originalFullName: string): string {
  const trimmed = edited.trim();
  if (!trimmed) return originalFullName;
  const dot = trimmed.lastIndexOf(".");
  if (dot > 0) return trimmed;
  const ext = fileExtensionWithDot(originalFullName);
  return trimmed + ext;
}
