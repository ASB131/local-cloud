# Icons

All PNGs in `file-types/` are copied from the icon set you provided (project `assets/`). They are **not** fetched from Flaticon/CDN in the build.

Some extensions reuse the closest icon from that set (for example `.doc` → `docx.png`, other code → `js.png`). Add a `python.png` (or similar) to `file-types/` and extend `fileIconAsset.ts` if you want a dedicated `.py` label.
