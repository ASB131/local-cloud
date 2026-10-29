import { describe, expect, it } from "vitest";
import { fileDisplayName, fileNameFromRenameEdit, fileRenameEditValue } from "./fileDisplayName";

describe("fileDisplayName", () => {
  it("strips the last extension", () => {
    expect(fileDisplayName("notes.txt")).toBe("notes");
    expect(fileDisplayName("archive.tar.gz")).toBe("archive.tar");
  });

  it("keeps dotfiles without an extension segment", () => {
    expect(fileDisplayName(".gitignore")).toBe(".gitignore");
  });
});

describe("fileNameFromRenameEdit", () => {
  it("preserves extension when the user renames the base only", () => {
    expect(fileNameFromRenameEdit("notes", "notes.txt")).toBe("notes.txt");
    expect(fileRenameEditValue("notes.txt")).toBe("notes");
  });

  it("allows changing the extension when the user types one", () => {
    expect(fileNameFromRenameEdit("readme.md", "notes.txt")).toBe("readme.md");
  });
});
