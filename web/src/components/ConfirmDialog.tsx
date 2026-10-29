type Props = {
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmDialog({
  title,
  message,
  confirmLabel = "Confirm",
  danger = false,
  onCancel,
  onConfirm,
}: Props) {
  return (
    <div className="explorer-modal-backdrop" role="presentation" onClick={onCancel}>
      <div className="explorer-modal" role="alertdialog" onClick={(e) => e.stopPropagation()}>
        <h2 className="explorer-modal-title">{title}</h2>
        <p className="explorer-modal-message">{message}</p>
        <div className="explorer-modal-actions">
          <button type="button" className="explorer-btn" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            className={`explorer-btn ${danger ? "explorer-btn-danger" : "explorer-btn-primary"}`}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
