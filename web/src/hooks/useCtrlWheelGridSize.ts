import { useEffect, useRef, type RefObject } from "react";
import { saveGridSize, stepGridSize, type GridSize } from "@/state/explorerGridSize";

/** Ctrl+wheel adjusts icon grid size (Windows Explorer style). Scroll up = larger. */
export function useCtrlWheelGridSize(
  containerRef: RefObject<HTMLElement | null>,
  enabled: boolean,
  gridSize: GridSize,
  setGridSize: (size: GridSize) => void,
) {
  const sizeRef = useRef(gridSize);
  sizeRef.current = gridSize;

  useEffect(() => {
    const el = containerRef.current;
    if (!el || !enabled) return;

    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      const direction: 1 | -1 = e.deltaY < 0 ? 1 : -1;
      const next = stepGridSize(sizeRef.current, direction);
      if (next === sizeRef.current) return;
      sizeRef.current = next;
      setGridSize(next);
      saveGridSize(next);
    };

    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [containerRef, enabled, setGridSize]);
}
