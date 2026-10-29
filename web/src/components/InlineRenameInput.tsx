import { useEffect, useRef } from "react";

type Props = {
  value: string;
  onChange: (value: string) => void;
  onCommit: () => void;
  onCancel: () => void;
  className?: string;
};

export function InlineRenameInput({ value, onChange, onCommit, onCancel, className }: Props) {
  const ref = useRef<HTMLInputElement>(null);
  const cancelRef = useRef(false);

  useEffect(() => {
    const el = ref.current;
    el?.focus();
    el?.select();
  }, []);

  return (
    <input
      ref={ref}
      type="text"
      className={className}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onClick={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === "Enter") {
          e.preventDefault();
          onCommit();
        }
        if (e.key === "Escape") {
          e.preventDefault();
          cancelRef.current = true;
          onCancel();
        }
      }}
      onBlur={() => {
        if (cancelRef.current) {
          cancelRef.current = false;
          return;
        }
        onCommit();
      }}
    />
  );
}
