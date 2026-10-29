export type GridSize = "sm" | "md" | "lg";

const KEY = "localcloud.gridSize";
const ORDER: GridSize[] = ["sm", "md", "lg"];

export function stepGridSize(current: GridSize, direction: 1 | -1): GridSize {
  const i = ORDER.indexOf(current);
  const next = Math.max(0, Math.min(ORDER.length - 1, i + direction));
  return ORDER[next]!;
}

export function loadGridSize(): GridSize {
  try {
    const v = sessionStorage.getItem(KEY);
    if (v === "sm" || v === "md" || v === "lg") return v;
  } catch {
    /* ignore */
  }
  return "md";
}

export function saveGridSize(size: GridSize) {
  sessionStorage.setItem(KEY, size);
}
