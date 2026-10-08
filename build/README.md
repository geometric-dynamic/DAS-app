# Wails 3 build assets

`Taskfile.yml` at the project root defines development, builds and packaging.
`config.yml` holds application metadata and the Go/Vite development watcher.

- `appicon.png`: source for generated Windows and macOS icons.
- `darwin/Info.plist` and `Info.dev.plist`: macOS bundle metadata.
- `windows/info.json` and `wails.exe.manifest`: Windows executable resources.
- `windows/nsis/`: Windows installer templates, requiring `makensis`.

Run `wails3 task icons` to regenerate icons. Run `wails3 build` to produce a
binary in `bin/`, or `wails3 package` on Windows/macOS to create a distributable.
