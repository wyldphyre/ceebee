# CeeBee

A small, fast desktop reader for comic book archives, built with [Wails v3](https://v3.wails.io), Go and plain TypeScript.

CeeBee opens one book at a time and focuses on reading: one- or two-page spreads, manga right-to-left support, and keyboard navigation that steps you through pages that don't fit on screen.

## Why another comic archive reader

Because I've long wanted a simple, effective reader that looked nice (enough), was available on Windows, Mac, and Linux, and had the features I wanted. There are plenty of good apps out there, but most were lacking something I wanted or were doing things I really didn't want (too much UI, trying to be a "library" app etc).

The two apps that have come the closest over the years were CDisplay on Windows and Simple Comic on the Mac. CDisplay refused to add reading direction detection from the metadata, but was othewise everthing I needed on Windows. And Simple Comic is pretty much perfect for what I want, but only available on a Mac.

So I made CeeBee. It takes inspiration from my favourite apps for reading comic archives. For it's initial implementation I like to think it has just enough of everything that I might want in a reader of this type, but the feature set might grow over time as I think of things I would actually use.

## AI?

Yes, this was written with Claude. No, I don't feel especially proud of that. I used AI because, while I could have done it myself, eventually, and with vastly greater time spent, I'm lazy enough and time poor enough that it wouldn't have happened otherwise.

## Features

- **Formats:** CBZ, CBR, CB7 and CBT. The format is detected from the file's contents, so mislabelled files (such as a `.cbr` that is really a zip) still open.
- **ComicInfo.xml:** the title, cover page and reading direction come from the archive's metadata when it is present.
- **Layouts:** one page or two-page spreads. The cover is always shown on its own, and so are wide pages (for example, double-page artwork).
- **Reading direction:** left-to-right or right-to-left, set automatically from metadata and switchable from the toolbar.
- **Scaling:** scale to window, scale to width, or original size.
- **Window sizing:** the window resizes to fit each view without blank space around the pages, as big as it needs to be but no bigger than the screen. It changes size only when the content's shape does, such as moving between a cover and a two-page spread. Full-screen and maximised windows are left alone.
- **Auto-scrolling:** when a view doesn't fit on screen, Space steps through it in reading order before turning the page.
- **Progress bar:** a slim bar along the bottom of the window, which can be toggled off.
- **Metadata panel:** a side panel listing the book's ComicInfo.xml fields, such as writer, publisher and summary. Open it with the Info button, View › Show Info or Cmd+I (Ctrl+I).
- **Remembers your settings:** page layout, scaling and the progress bar are restored at startup, and with File › Remember Reading Position on (the default) each book reopens where you left off.
- **Opening books:** use the Open button or File › Open…, pick from File › Open Recent (the last 10 books), drag a file onto the window, open a comic from Finder or Explorer, or pass a path on the command line. CeeBee can be set as the default app for comic files.
- **Show in Finder/Explorer:** File › Show in Finder (Show in Explorer on Windows, Show in File Manager on Linux) reveals the open book's file.
- **Context menu:** right-click the page for the Go and View menus' items, such as Next Page, First Page, Show Toolbar and Show Info.
- **Hideable toolbar:** View › Show Toolbar hides the toolbar for distraction-free reading. It always comes back the next time CeeBee starts.

## Using CeeBee

### Making CeeBee the default comic reader

CeeBee registers itself for `.cbz`, `.cbr`, `.cb7` and `.cbt` files.

- **macOS:** open the app once (or move it to `/Applications`) so macOS learns which files it handles. Then select a comic file in Finder, choose File › Get Info, pick CeeBee under "Open with", and click **Change All…**.
- **Windows:** the installer registers the file types. The standalone `.exe` doesn't, but you can still choose it with "Open with".
- **Linux:** the `.deb`, `.rpm` and AppImage packages declare the comic file types, so CeeBee appears in your desktop's "Open with" list.

### Where settings are kept

Settings and reading positions are saved in `settings.json` in your user settings folder:

| Platform | Location |
|---|---|
| macOS | `~/Library/Application Support/CeeBee/` |
| Windows | `%AppData%\CeeBee\` |
| Linux | `~/.config/CeeBee/` |

Reading positions are kept for the 500 most recently read books, looked up by file path, so a book that is moved or renamed starts from the beginning again. Reaching the end of a book resets its position, so it opens at the start next time. Turning off File › Remember Reading Position also forgets all saved positions.

## Keyboard shortcuts

Every shortcut also has a menu item: reading keys are in the Go menu, and the others are in the File and View menus.

| Key | Left-to-right | Right-to-left |
|---|---|---|
| Space | Next (scrolls first if needed) | Next (scrolls first if needed) |
| Shift+Space | Previous (scrolls first if needed) | Previous (scrolls first if needed) |
| → | Next view | Previous view |
| ← | Previous view | Next view |
| Home, or Cmd+↑ on a Mac | First view | First view |
| End, or Cmd+↓ on a Mac | Last view | Last view |

The arrow keys follow the page visually: in right-to-left mode, ← moves forward. They always turn the page straight away, without scrolling.

| Key | Action |
|---|---|
| Cmd+O (Ctrl+O on Windows and Linux) | Open a comic |
| Cmd+I (Ctrl+I) | Show or hide the metadata panel |
| Option+Cmd+T (Ctrl+Shift+T) | Show or hide the toolbar |

## Building

### Requirements

- Go 1.26 or later
- Node.js 20.19 or 22.12 or later, with npm
- The Wails v3 CLI, matching the version in `go.mod`:

  ```sh
  go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.27
  ```

`go install` puts `wails3` in `~/go/bin`. If your shell can't find it, add that folder to your `PATH`, for example in `~/.zshrc`:

```sh
export PATH="$HOME/go/bin:$PATH"
```

Run `wails3 doctor` to check that your system has everything it needs. On Linux this includes the GTK and WebKitGTK development packages.

### Commands

```sh
wails3 dev        # run with live reload
wails3 build      # build bin/CeeBee
wails3 package    # build a distributable package (for example bin/CeeBee.app on macOS)
```

On macOS, two more packaging tasks are available:

```sh
wails3 task darwin:package:universal    # one app for both Apple Silicon and Intel Macs
wails3 task darwin:package:dmg          # the app in a .dmg disk image
```

The macOS app is only ad-hoc signed. Other Macs will warn that it's from an unidentified developer until it is signed with an Apple Developer ID and notarized.

To open a book directly:

```sh
bin/CeeBee path/to/book.cbz
```

### Platforms

CeeBee targets macOS, Windows and Linux. So far it has been tested on macOS and Windows.

Windows builds can be cross-compiled from a Mac:

```sh
wails3 build GOOS=windows GOARCH=amd64    # bin/CeeBee.exe for x64 PCs
wails3 build GOOS=windows GOARCH=arm64    # for ARM Windows devices
```

Without `GOARCH`, the build uses the architecture of the machine you build on. `wails3 package GOOS=windows` builds an installer instead, which also registers the comic file types; it needs `makensis` (`brew install makensis` on macOS). Windows needs Microsoft's WebView2 runtime, which comes with Windows 11 and current versions of Windows 10.

### Versions

CeeBee follows [semantic versioning](https://semver.org). The version is set in one place, `info.version` in `build/config.yml`:

- The app reads it from there; it's built into the binary.
- Each `wails3 build` or `wails3 package` copies it into the platform files (`Info.plist`, the Windows version information and installer script, the Linux package), using `wails3 update build-assets`. This only happens when `config.yml` has changed. The same goes for the other product details in `config.yml`, such as the name and copyright.

### Release builds

On a Mac, `scripts/release.sh` builds CeeBee for macOS and Windows, on both x64 and arm64, and zips each build into `dist/`:

```
dist/CeeBee-1.0.0-macos-arm64.zip     CeeBee.app
dist/CeeBee-1.0.0-macos-x64.zip       CeeBee.app
dist/CeeBee-1.0.0-windows-arm64.zip   CeeBee.exe
dist/CeeBee-1.0.0-windows-x64.zip     CeeBee.exe
```

The version in the file names comes from `info.version` in `build/config.yml`.

The script needs the same tools as a normal build, plus Xcode's command line tools for the macOS builds. It finds `wails3` in `~/go/bin` even if that folder isn't on your `PATH`. Linux isn't included, because Linux builds need the GTK libraries and have to be made on Linux (or in Docker).

The macOS apps are only ad-hoc signed, so on another Mac they have to be opened the first time by right-clicking the app and choosing Open, or by allowing them under System Settings › Privacy & Security.

## Testing

Run the Go unit tests with:

```sh
wails3 task test
```

This builds the frontend first, because the app embeds it and won't compile without it. Once it has been built, `go test ./...` works directly. The `book` and `settings` packages, which hold the archive, metadata, layout and settings logic, can be tested without a frontend build: `go test ./book ./settings`.

The CBR and CB7 test fixtures in `book/testdata` are generated by `make_fixtures.py`. The script builds the RAR file by hand, since no free tool can create RAR archives, and needs the `7z` command for the CB7 file.

## Project structure

```
main.go              Wails app setup, file drop and file-open events,
                     icon middleware
version.go           Reads the version from build/config.yml
menu.go              The application and context menus, including Open Recent
reveal*.go           Show in Finder/Explorer/File Manager for each platform
service.go           Methods called from the frontend: opening books, settings,
                     reading positions; page-serving middleware
book/
  archive.go         Format detection, opening archives, page list and bytes
  comicinfo.go       ComicInfo.xml parsing
  spreads.go         Grouping pages into two-page spreads
  wide.go            Reading image sizes and detecting wide pages
settings/
  settings.go        Saved settings and reading positions
frontend/
  index.html
  src/main.ts        Reader state, rendering, scrolling, keyboard, toolbar
  src/style.css
build/               Wails build configuration, icons, platform packaging and
                     file associations; build/config.yml holds the version
scripts/
  release.sh         Builds and zips the macOS and Windows releases
SPEC.md              The original specification
```

Page images are served to the webview through Wails asset middleware at `/book/{bookId}/page/{index}`, not through JavaScript bindings.

## License

[MIT](LICENSE) © 2026 Craig Reynolds
