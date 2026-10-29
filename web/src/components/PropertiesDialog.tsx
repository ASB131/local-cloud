import type { CloudFile, Folder } from "@/api/types";

type Props = {
  target: { kind: "folder"; folder: Folder } | { kind: "file"; file: CloudFile };
  folderPathLabel: string;
  onClose: () => void;
};

export function PropertiesDialog({ target, folderPathLabel, onClose }: Props) {
  const isFile = target.kind === "file";
  const name = isFile ? target.file.name : target.folder.name;
  const rows: [string, string][] = [
    ["Name", name],
    ["Type", isFile ? target.file.mime_type || "File" : "Folder"],
  ];
  if (isFile) {
    rows.push(["Size", `${target.file.size_bytes.toLocaleString()} bytes`]);
    rows.push(["Modified", new Date(target.file.updated_at).toLocaleString()]);
  } else {
    rows.push(["Created", new Date(target.folder.created_at).toLocaleString()]);
  }
  rows.push(["Location", folderPathLabel]);

  return (
    <div className="explorer-modal-backdrop" role="presentation" onClick={onClose}>
      <div className="explorer-modal" role="dialog" onClick={(e) => e.stopPropagation()}>
        <h2 className="explorer-modal-title">Properties</h2>
        <dl className="explorer-props">
          {rows.map(([k, v]) => (
            <div key={k} className="explorer-props-row">
              <dt>{k}</dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>
        <div className="explorer-modal-actions">
          <button type="button" className="explorer-btn explorer-btn-primary" onClick={onClose}>
            OK
          </button>
        </div>
      </div>
    </div>
  );
}
