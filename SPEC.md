# Comic Reader — Specification

A small, cross-platform desktop reader for comic book archives. **Keep it simple:** implement only what this spec asks for, and prefer the standard library and plain code over frameworks and abstractions. The app name is `CeeBee`.

## 1. Platform and stack

- **Framework:** Wails v3 (beta). The v3 API has changed recently, so check the current docs at https://v3.wails.io before writing Wails-specific code. Do not rely on memory of v2 or early v3 alphas.
- **Backend:** Go.
- **Frontend:** the Wails v3 vanilla TypeScript template. No UI framework (no React, Vue or Svelte), no CSS framework.
- **Targets:** macOS, Windows, Linux.
- **Dependencies:** keep them to a minimum. Pure-Go libraries only (no cgo), so cross-compiling stays simple.
  - CBZ: `archive/zip` (stdlib)
  - CBT: `archive/tar` (stdlib)
  - CBR: `github.com/nwaples/rardecode/v2`
  - CB7: `github.com/bodgit/sevenzip`

## 2. Scope

### In scope
- Opening one book at a time in a single window.
- Formats: `.cbz`, `.cbr`, `.cb7`, `.cbt`. Detect the format by file content (magic bytes) first and fall back to the extension, because mislabelled files (for example a `.cbr` that is really a zip) are common.
- Reading `ComicInfo.xml` for reading direction, cover page and title.
- One-page and two-page layouts.
- Cover always shown as a single page.
- Keyboard navigation.

### Out of scope (do not build)
Library management, bookmarks or reading progress, settings persistence, zoom and pan, thumbnails, metadata editing, auto-update, file associations, multiple windows or tabs.

## 3. Opening a book

A book can be opened three ways:
1. A toolbar **Open** button that shows the native file dialog, filtered to the supported extensions.
2. Dragging and dropping a file onto the window.
3. A file path passed as the first command-line argument.

Opening a new book replaces the current one and closes its archive.

## 4. Archive handling (Go)

### Page list
- Pages are the archive entries with image extensions: `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.avif`, `.bmp`. Match case-insensitively.
- Ignore directories, `__MACOSX/` entries, and any entry whose file name starts with `.`.
- Images inside subfolders count as pages. Sort by full path.
- Sort with a **natural, case-insensitive sort**, so `page2.jpg` comes before `page10.jpg`.
- An archive with no pages is an error (see §9).

### Access strategy
- **CBZ:** keep the zip open and read entries on demand (random access).
- **CBR, CB7, CBT:** these are often solid or sequential-only, so read all page images into memory once when the book opens. This is fine for typical comic sizes. Do not add a cache layer.

### Serving pages to the frontend
- Pass image bytes through **asset handler / middleware URLs**, not through JS↔Go bindings or base64.
- URL shape: `/book/{bookID}/page/{index}`. `index` is the 0-based position in the sorted page list. `bookID` changes every time a book is opened, so the webview never shows cached images from a previous book.
- Set `Content-Type` from the file extension. An unknown `bookID` or an out-of-range index returns 404.

## 5. ComicInfo.xml

- Find an entry named `ComicInfo.xml` (case-insensitive). Prefer the archive root; otherwise use the first one found.
- If it is missing or fails to parse, use the defaults below silently. Never fail to open a book because of ComicInfo.

| Purpose | Source | Rule |
|---|---|---|
| Reading direction | `<Manga>` | `YesAndRightToLeft` → **RTL**. Anything else, or missing → **LTR**. |
| Cover page | `<Pages><Page Image="n" Type="FrontCover"/>` | `Image` is a 0-based index into the sorted page list. Use the first `FrontCover` entry with a valid in-range index. Otherwise the cover is page 0. |
| Title | `<Series>`, `<Number>`, `<Title>` | If `Series` is present: `"{Series} #{Number}"`, adding `" – {Title}"` when `Title` is present too. Else `Title`. Else the file name without its extension. |

The user can also flip the reading direction manually with a toolbar button (§7). The override lasts until another book is opened.

## 6. Layout

### One-page mode
Each page is shown on its own.

### Two-page mode
Pages are grouped into **spreads**, built in reading order:
1. The **cover page is always a spread on its own**.
2. Pages before the cover, if any, are paired in order: (0,1), (2,3)… An odd page left over before the cover is shown alone.
3. Pages after the cover are paired in order, starting with the page right after the cover. A final leftover page is shown alone.

Examples (cover = 0, 7 pages): `[0] [1,2] [3,4] [5,6]`
(cover = 0, 6 pages): `[0] [1,2] [3,4] [5]`
(cover = 2, 7 pages): `[0,1] [2] [3,4] [5,6]`

The cover rule also applies in one-page mode, where it has no visible effect.

Implement spread building in Go as a pure function, `BuildSpreads(pageCount, coverIndex int) [][]int`, and return the result to the frontend when the book opens.

### Display
- Scale each spread to fit the window (equivalent to `object-fit: contain`), keep the aspect ratio, and centre it. Never crop.
- In a two-page spread the two images sit side by side with no gap and are scaled to the same height.
- **LTR:** the first page of the spread is on the left. **RTL:** the first page is on the right.
- Background: neutral dark grey.

### Switching layouts
- Toggle with a toolbar button. Default: two-page mode.
- After switching, show the view that contains the page the reader was on. In two-page → one-page, that is the first page of the current spread.

### Preloading
After a view is shown, preload the images of the next view (create `Image` objects in the frontend) so page turns are instant.

## 7. UI

A slim toolbar at the top containing:
- **Open** button
- Book title
- Page indicator, for example `12–13 / 48` or `1 / 48`. Use 1-based numbers.
- **1 page / 2 pages** layout toggle
- **LTR / RTL** direction toggle (shows the current direction)

Before a book is opened, the reading area shows a short message: "Open a comic or drop a file here".

Window title: `{book title} — Comic Reader`, or just `Comic Reader` when no book is open.

## 8. Keyboard

| Key | LTR | RTL |
|---|---|---|
| → Right arrow | Next view | Previous view |
| ← Left arrow | Previous view | Next view |
| Space | Next view | Next view |

- "View" means one page in one-page mode and one spread in two-page mode.
- Arrow keys follow the visual direction: in RTL, pressing Left moves toward the left side of the page, which is forward.
- At the first or last view, keys that would go past the end do nothing.
- Prevent Space's default scrolling behaviour.

## 9. Errors

Show errors in a simple dismissible message in the reading area. Do not crash. Cases:
- Unsupported or unreadable file
- Archive contains no images
- An individual page that fails to decode or load: show a placeholder ("Page could not be loaded") in its slot and keep going

## 10. Suggested structure

```
/
├─ main.go              # Wails app setup, CLI arg, asset middleware registration
├─ book/
│  ├─ archive.go        # format detection, open, page list, page bytes
│  ├─ comicinfo.go      # ComicInfo.xml parsing
│  ├─ spreads.go        # BuildSpreads
│  └─ *_test.go
├─ service.go           # bound methods: OpenDialog(), OpenPath(path) → BookInfo
└─ frontend/
   ├─ index.html
   ├─ src/main.ts       # state, rendering, keyboard, toolbar
   └─ src/style.css
```

`BookInfo` returned to the frontend:

```go
type BookInfo struct {
    BookID     string  `json:"bookId"`
    Title      string  `json:"title"`
    PageCount  int     `json:"pageCount"`
    CoverIndex int     `json:"coverIndex"`
    RTL        bool    `json:"rtl"`
    Spreads    [][]int `json:"spreads"` // two-page mode spreads
}
```

## 11. Tests

Go unit tests (stdlib `testing` only):
- Natural sort ordering
- Filtering of non-image, hidden and `__MACOSX` entries
- ComicInfo parsing: each `Manga` value, a `FrontCover` page, an out-of-range cover index, missing or malformed XML
- `BuildSpreads`: the three examples in §6, plus cover = last page, a 1-page book and a 2-page book
- Format detection using small fixture archives created in the test (at least CBZ and CBT; for CBR and CB7, use tiny checked-in fixtures)

No frontend test framework.

## 12. Acceptance criteria

1. Builds and runs on macOS, Windows and Linux using the Wails v3 build tooling.
2. Opens CBZ, CBR, CB7 and CBT files via the dialog, drag and drop, and the command line.
3. A book with `<Manga>YesAndRightToLeft</Manga>` opens in RTL. Others open in LTR.
4. The cover (the ComicInfo `FrontCover`, otherwise the first page) is always shown alone, in both layouts.
5. Two-page spreads follow §6, including correct left/right placement for RTL.
6. Left, Right and Space navigate as in §8.
7. Page turns feel instant on a typical CBZ.
8. All Go tests pass.
