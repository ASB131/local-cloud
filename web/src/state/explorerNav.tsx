import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from "react";

type ExplorerNavContextValue = {
  canBack: boolean;
  canForward: boolean;
  push: (folderId: string) => void;
  back: () => string | null;
  forward: () => string | null;
};

const ExplorerNavContext = createContext<ExplorerNavContextValue | null>(null);

export function ExplorerNavProvider({ children }: { children: ReactNode }) {
  const navRef = useRef({ stack: [] as string[], index: -1, skipPush: false });
  const [version, setVersion] = useState(0);
  const bump = () => setVersion((v) => v + 1);

  const push = useCallback((folderId: string) => {
    const nav = navRef.current;
    if (nav.skipPush) {
      nav.skipPush = false;
      bump();
      return;
    }
    if (nav.stack[nav.index] === folderId) return;
    nav.stack = nav.stack.slice(0, nav.index + 1);
    nav.stack.push(folderId);
    nav.index = nav.stack.length - 1;
    bump();
  }, []);

  const back = useCallback(() => {
    const nav = navRef.current;
    if (nav.index <= 0) return null;
    nav.index -= 1;
    nav.skipPush = true;
    bump();
    return nav.stack[nav.index] ?? null;
  }, []);

  const forward = useCallback(() => {
    const nav = navRef.current;
    if (nav.index >= nav.stack.length - 1) return null;
    nav.index += 1;
    nav.skipPush = true;
    bump();
    return nav.stack[nav.index] ?? null;
  }, []);

  const value = useMemo(() => {
    void version;
    const nav = navRef.current;
    return {
      canBack: nav.index > 0,
      canForward: nav.index >= 0 && nav.index < nav.stack.length - 1,
      push,
      back,
      forward,
    };
  }, [version, push, back, forward]);

  return <ExplorerNavContext.Provider value={value}>{children}</ExplorerNavContext.Provider>;
}

export function useExplorerNav() {
  const ctx = useContext(ExplorerNavContext);
  if (!ctx) throw new Error("useExplorerNav requires ExplorerNavProvider");
  return ctx;
}
