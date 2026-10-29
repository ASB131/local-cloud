type Props = {
  variant?: "grid" | "inline";
};

/** Folder PNG in the bundle was mislabeled; use emoji until a real folder asset is added. */
export function ExplorerFolderIcon({ variant = "grid" }: Props) {
  const className =
    variant === "grid" ? "explorer-tile-icon explorer-folder-icon" : "explorer-inline-emoji explorer-folder-icon";
  return (
    <span className={className} aria-hidden>
      📁
    </span>
  );
}
