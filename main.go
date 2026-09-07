package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows/registry"
)

const (
	appName = "Windows Theme Switcher"
	regPath = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

const (
	hwndBroadcast   = 0xFFFF
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
)

func getTheme() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	if err != nil {
		return "unknown"
	}
	defer k.Close()

	apps, _, err1 := k.GetIntegerValue("AppsUseLightTheme")
	system, _, err2 := k.GetIntegerValue("SystemUsesLightTheme")
	if err1 != nil || err2 != nil {
		return "unknown"
	}

	if apps == 0 && system == 0 {
		return "dark"
	}
	if apps == 1 && system == 1 {
		return "light"
	}
	return "mixed"
}

func setTheme(theme string) bool {
	if theme != "light" && theme != "dark" {
		return false
	}

	var value uint32
	if theme == "light" {
		value = 1
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	if err := k.SetDWordValue("AppsUseLightTheme", value); err != nil {
		return false
	}
	if err := k.SetDWordValue("SystemUsesLightTheme", value); err != nil {
		return false
	}

	notifyThemeChanged()
	return true
}

func notifyThemeChanged() {
	for _, s := range []string{"ImmersiveColorSet", "WindowsThemeElement"} {
		ptr, err := syscall.UTF16PtrFromString(s)
		if err != nil {
			continue
		}
		procSendMessageTimeoutW.Call(
			uintptr(hwndBroadcast),
			uintptr(wmSettingChange),
			0,
			uintptr(unsafe.Pointer(ptr)),
			uintptr(smtoAbortIfHung),
			1000,
			0,
		)
	}
}

func startupEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(appName)
	return err == nil
}

func enableStartup() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	return k.SetStringValue(appName, fmt.Sprintf(`"%s"`, exePath)) == nil
}

func disableStartup() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	return k.DeleteValue(appName) == nil
}

func createIconImage(theme string) *image.RGBA {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	if theme == "dark" {
		drawMoon(img, size)
	} else {
		drawSun(img, size)
	}

	return img
}

func drawMoon(img *image.RGBA, size int) {
	moonColor := color.RGBA{245, 200, 70, 255}
	cx1, cy1, r1 := 32, 31, 23
	cx2, cy2, r2 := 43, 21, 18

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx1 := float64(x - cx1)
			dy1 := float64(y - cy1)
			inOuter := dx1*dx1+dy1*dy1 <= float64(r1*r1)

			dx2 := float64(x - cx2)
			dy2 := float64(y - cy2)
			inCut := dx2*dx2+dy2*dy2 <= float64(r2*r2)

			if inOuter && !inCut {
				img.Set(x, y, moonColor)
			}
		}
	}
}

func drawSun(img *image.RGBA, size int) {
	sunColor := color.RGBA{255, 190, 40, 255}
	cx, cy, r := 32, 32, 14

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			if dx*dx+dy*dy <= float64(r*r) {
				img.Set(x, y, sunColor)
			}
		}
	}

	for angle := 0; angle < 360; angle += 45 {
		rad := float64(angle) * math.Pi / 180
		x1 := cx + int(math.Cos(rad)*21)
		y1 := cy + int(math.Sin(rad)*21)
		x2 := cx + int(math.Cos(rad)*29)
		y2 := cy + int(math.Sin(rad)*29)
		drawThickLine(img, x1, y1, x2, y2, 4, sunColor)
	}
}

func drawThickLine(img *image.RGBA, x1, y1, x2, y2, width int, c color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	steps := int(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy))))
	if steps == 0 {
		steps = 1
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := x1 + int(float64(dx)*t)
		y := y1 + int(float64(dy)*t)
		drawDisc(img, x, y, width/2, c)
	}
}

func drawDisc(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				img.Set(cx+x, cy+y, c)
			}
		}
	}
}

func encodeICO(img image.Image) []byte {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil
	}
	pngBytes := pngBuf.Bytes()

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	widthByte := byte(width)
	if width >= 256 {
		widthByte = 0
	}
	heightByte := byte(height)
	if height >= 256 {
		heightByte = 0
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))

	buf.WriteByte(widthByte)
	buf.WriteByte(heightByte)
	buf.WriteByte(0)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(32))
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes)))
	binary.Write(&buf, binary.LittleEndian, uint32(22))

	buf.Write(pngBytes)

	return buf.Bytes()
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

var (
	mLight   *systray.MenuItem
	mDark    *systray.MenuItem
	mStartup *systray.MenuItem
	mQuit    *systray.MenuItem
)

func startupLabel() string {
	if startupEnabled() {
		return "✓  Start with Windows"
	}
	return "Start with Windows"
}

func updateChecks(theme string) {
	switch theme {
	case "light":
		mLight.Check()
		mDark.Uncheck()
	case "dark":
		mDark.Check()
		mLight.Uncheck()
	default:
		mLight.Uncheck()
		mDark.Uncheck()
	}
}

func refreshIcon() {
	theme := getTheme()
	systray.SetIcon(encodeICO(createIconImage(theme)))
	systray.SetTooltip(fmt.Sprintf("%s - %s Mode", appName, titleCase(theme)))
	updateChecks(theme)
}

func onReady() {
	theme := getTheme()
	systray.SetIcon(encodeICO(createIconImage(theme)))
	systray.SetTitle("")
	systray.SetTooltip(fmt.Sprintf("%s - %s Mode", appName, titleCase(theme)))

	mLight = systray.AddMenuItem("☀  Light Mode", "Switch to light mode")
	mDark = systray.AddMenuItem("☾  Dark Mode", "Switch to dark mode")
	systray.AddSeparator()
	mStartup = systray.AddMenuItem(startupLabel(), "Run at Windows startup")
	systray.AddSeparator()
	mQuit = systray.AddMenuItem("✕  Exit", "Exit application")

	updateChecks(theme)

	go handleEvents()
}

func handleEvents() {
	for {
		select {
		case <-mLight.ClickedCh:
			setTheme("light")
			refreshIcon()
		case <-mDark.ClickedCh:
			setTheme("dark")
			refreshIcon()
		case <-mStartup.ClickedCh:
			if startupEnabled() {
				disableStartup()
			} else {
				enableStartup()
			}
			mStartup.SetTitle(startupLabel())
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func onExit() {}

func main() {
	if runtime.GOOS != "windows" {
		fmt.Println("This application only works on Windows.")
		os.Exit(1)
	}
	systray.Run(onReady, onExit)
}
