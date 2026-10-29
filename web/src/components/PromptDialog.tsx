import { useEffect, useId, useRef } from "react";

type Props = {
  title: string;
  label: string;
  defaultValue?: string;
  submitLabel?: string;
  onCancel: () => void;
  onSubmit: (value: string) => void;
};

export function PromptDialog({ title, label, defaultValue = "", submitLabel = "OK", onCancel, onSubmit }: Props) {
  const titleId = useId();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, []);

  return (
    <div className="explorer-modal-backdrop" role="presentation" onClick={onCancel}>
      <div
        className="explorer-modal"
        role="dialog"
        aria-labelledby={titleId}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "Escape") onCancel();
        }}
      >
        <h2 id={titleId} className="explorer-modal-title">
          {title}
        </h2>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const v = inputRef.current?.value.trim() ?? "";
            if (v) onSubmit(v);
          }}
        >
          <label className="explorer-modal-label">
            {label}
            <input ref={inputRef} className="explorer-modal-input" type="text" defaultValue={defaultValue} />
          </label>
          <div className="explorer-modal-actions">
            <button type="button" className="explorer-btn" onClick={onCancel}>
              Cancel
            </button>
            <button type="submit" className="explorer-btn explorer-btn-primary">
              {submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
