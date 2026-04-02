<div>
	<div align="center">
		<img alt="logo" width=200 src="https://raw.githubusercontent.com/NicheHardware/DAS-app/refs/heads/dev/logo.png"/>
	</div>
	<div align="center">
		<img alt="title" src="https://capsule-render.vercel.app/api?type=transparent&fontColor=dbdbdb&text=DAS%20Console&height=80&fontSize=48"/>
	</div>
</div>

**DAS** is a Low-power Distributed Acquisition System. This is the repo of the console of this system.

- DAS Layout: https://github.com/NicheHardware/DAS-layout
- DAS Console: https://github.com/NicheHardware/DAS-app
- DAS Firmware(Private): https://github.com/NicheHardware/DAS-Firmware

## Screenshots

<div align="center">
	<img width="42%" alt="Screenshot-grid" src="https://github.com/user-attachments/assets/b97991ca-ebf0-4cf8-b29f-ae6aebaf5974" />
	<img width="42%" alt="Screenshot-list" src="https://github.com/user-attachments/assets/6e8c98c1-155a-4519-a0f1-c981fbc5abfb" />
</div>

On this console, you can view data from all distributed nodes in real time and export data from any node.

## Build

### One-command multi-platform build

This repository provides a unified build entry at `scripts/build-all.sh`.

It will:

- read the version from the current git tag
- fall back to `dev` when the current commit is not exactly on a tag
- build one or more Wails targets
- place renamed artifacts in `dist/`

Output naming format:

`DAS-Console_<version>_<os>_<arch>`

Examples:

- `DAS-Console_v0.1.0_windows_amd64.exe`
- `DAS-Console_v0.1.0_linux_amd64`
- `DAS-Console_dev_windows_amd64.exe`

### Usage

Build default targets:

```bash
bash scripts/build-all.sh
```

Build specific targets:

```bash
bash scripts/build-all.sh --targets linux/amd64,windows/amd64
```

Build and also generate the Windows NSIS installer:

```bash
bash scripts/build-all.sh --targets windows/amd64 --nsis
```

### Notes

- Default targets are `linux/amd64` and `windows/amd64`.
- Cross-compiling depends on your local toolchain. If the current machine cannot build a target, the script will print a clear error and continue with the remaining targets.
- macOS targets usually require an appropriate macOS build environment/toolchain.
- The raw Wails build output remains in `build/bin/`, while normalized deliverables are collected into `dist/`.
