# Windows Theme Switcher

Tray app to toggle Windows light/dark mode, with optional start-on-boot.

## Requirements

- Windows 10/11
- Go 1.21+ and a C compiler (CGO) if building locally

## Build

```
go mod tidy
go build -ldflags "-H=windowsgui -s -w" -o ThemeSwitcher.exe .
```

## Usage

Run `ThemeSwitcher.exe`. Right-click the tray icon:

- **Light Mode** / **Dark Mode** — switch theme
- **Start with Windows** — toggle run-at-startup (registry `Run` key)
- **Exit**
