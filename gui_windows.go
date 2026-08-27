//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	guiClassName            = "EsurfingGoWindow"
	guiWindowTitle          = "EsurfingGo"
	guiButtonStart          = 1001
	guiButtonStop           = 1002
	guiButtonInterfaces     = 1003
	guiCheckCredentials     = 1004
	guiCheckAutostart       = 1005
	guiEditUser             = 1101
	guiEditPassword         = 1102
	guiComboInterface       = 1201
	guiStaticStatus         = 1301
	guiEditLog              = 1401
	guiWMLog                = 0x0400 + 1
	guiWMClientDone         = 0x0400 + 2
	guiWMInterfacesDone     = 0x0400 + 3
	guiWMNetworkStatus      = 0x0400 + 4
	guiWMClientError        = 0x0400 + 5
	guiWMNCCreate           = 0x0081
	guiWMCreate             = 0x0001
	guiWMClose              = 0x0010
	guiWMDestroy            = 0x0002
	guiWMNCDestroy          = 0x0082
	guiWMSize               = 0x0005
	guiWMCommand            = 0x0111
	guiWMNCPaint            = 0x0085
	guiWMSetFont            = 0x0030
	guiWMAppTray            = 0x8000 + 1
	guiTaskbarCreated       = 0x8000 + 2
	guiWMLButtonUp          = 0x0202
	guiWMLButtonDoubleClick = 0x0203
	guiWMRButtonUp          = 0x0205
	guiWMSizeMinimized      = 1
	guiSWHide               = 0
	guiSWShow               = 5
	guiMFString             = 0x0000
	guiMFSeparator          = 0x0800
	guiTPMRightButton       = 0x0002
	guiTPMReturnCommand     = 0x0100
	guiSWRestore            = 9
	guiTrayShow             = 2001
	guiTrayExit             = 2002
	guiNIFMessage           = 0x00000001
	guiNIFIcon              = 0x00000002
	guiNIFTip               = 0x00000004
	guiNIMAdd               = 0x00000000
	guiNIMDelete            = 0x00000002
	guiBMGetCheck           = 0x00F0
	guiBMSetCheck           = 0x00F1
	guiBSTChecked           = 1
	guiCBResetContent       = 0x014B
	guiCBAddString          = 0x0143
	guiCBSetCurrent         = 0x014E
	guiCBGetCurrent         = 0x0147
	guiEMSetSel             = 0x00B1
	guiWSClipSiblings       = 0x04000000
	guiWSClipChildren       = 0x02000000
	guiWSVScroll            = 0x00200000
	guiCBSDropdownList      = 0x0003
	guiEditExClientEdge     = 0x00000200
	guiESMultiline          = 0x0004
	guiESAutoVScroll        = 0x0040
	guiESReadOnly           = 0x0800
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	gdi32                     = syscall.NewLazyDLL("gdi32.dll")
	procRegisterClassEx       = user32.NewProc("RegisterClassExW")
	procCreateWindowEx        = user32.NewProc("CreateWindowExW")
	procDefWindowProc         = user32.NewProc("DefWindowProcW")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procDispatchMessage       = user32.NewProc("DispatchMessageW")
	procGetClientRect         = user32.NewProc("GetClientRect")
	procGetMessage            = user32.NewProc("GetMessageW")
	procGetWindowLongPtr      = user32.NewProc("GetWindowLongPtrW")
	procGetModuleHandle       = kernel32.NewProc("GetModuleHandleW")
	procGetStockObject        = gdi32.NewProc("GetStockObject")
	procLoadCursor            = user32.NewProc("LoadCursorW")
	procMessageBox            = user32.NewProc("MessageBoxW")
	procMoveWindow            = user32.NewProc("MoveWindow")
	procPostMessage           = user32.NewProc("PostMessageW")
	procPostQuitMessage       = user32.NewProc("PostQuitMessage")
	procSendMessage           = user32.NewProc("SendMessageW")
	procSetWindowText         = user32.NewProc("SetWindowTextW")
	procSetWindowLongPtr      = user32.NewProc("SetWindowLongPtrW")
	procShowWindow            = user32.NewProc("ShowWindow")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procUpdateWindow          = user32.NewProc("UpdateWindow")
	procCreateFont            = gdi32.NewProc("CreateFontW")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procLoadIcon              = user32.NewProc("LoadIconW")
	procLoadImage             = user32.NewProc("LoadImageW")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	procAppendMenu            = user32.NewProc("AppendMenuW")
	procTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	procSetForeground         = user32.NewProc("SetForegroundWindow")
	procDestroyMenu           = user32.NewProc("DestroyMenu")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
)

var (
	guiWindowRegistryMu sync.RWMutex
	guiWindowRegistry   = make(map[uintptr]*guiWindow)
	guiNextWindowID     uintptr
	guiStringMu         sync.Mutex
	guiStrings          = make(map[uintptr]guiStringPayload)
	guiInterfaceResults = make(map[uintptr]guiInterfacePayload)
	guiNextStringID     uintptr
)

const (
	guiGWLPUserData   = ^uintptr(20)
	guiDefaultGUIFont = 17
)

type guiPoint struct {
	X int32
	Y int32
}

type guiRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type guiMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      guiPoint
	Private uint32
}

type guiInterfaceResult struct {
	interfaces []NetworkInterface
	err        error
}

type guiStringPayload struct {
	owner uintptr
	text  string
}

type guiInterfacePayload struct {
	owner  uintptr
	result guiInterfaceResult
}

type guiWndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type guiCreateStruct struct {
	CreateParams uintptr
	Instance     uintptr
	Menu         uintptr
	Parent       uintptr
	Height       int32
	Width        int32
	Y            int32
	X            int32
	Style        int32
	Name         uintptr
	Class        uintptr
	ExStyle      uint32
}

type guiNotifyIconData struct {
	Size             uint32
	Window           uintptr
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUID             [16]byte
	BalloonIcon      uintptr
}

type guiWindow struct {
	hwnd               uintptr
	user               uintptr
	password           uintptr
	interfaces         uintptr
	start              uintptr
	stop               uintptr
	status             uintptr
	log                uintptr
	saveCredentials    uintptr
	autostart          uintptr
	font               uintptr
	fontOwned          bool
	selected           []NetworkInterface
	configuredNetwork  int
	client             *Client
	states             *States
	session            *Session
	logWriter          *guiLogWriter
	mu                 sync.Mutex
	stopping           bool
	closing            bool
	destroyed          bool
	runID              uint64
	compatibilityRunID uint64
	refreshID          uint64
	registryToken      uintptr
	trayIcon           uintptr
	trayAdded          bool
	autostartRequested bool
	autostartPending   bool
	configPath         string
	config             FileConfig
	taskbarCreated     uint32
}

type guiLogWriter struct {
	window *guiWindow
	file   *os.File
}

func (w *guiLogWriter) Write(p []byte) (int, error) {
	if w == nil || w.window == nil {
		return len(p), nil
	}
	text := strings.TrimSpace(string(p))
	if text == "" {
		return len(p), nil
	}
	if w.file != nil {
		_, _ = w.file.Write(p)
	}
	w.window.postGUIString(guiWMLog, text)
	return len(p), nil
}

func runGUI(autostartRequested bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, _ := procGetModuleHandle.Call(0)
	taskbarCreated := registerGUIWindowMessage("TaskbarCreated")
	className, _ := syscall.UTF16PtrFromString(guiClassName)
	cursor, _, _ := procLoadCursor.Call(0, uintptr(32512))
	icon := loadGUIAppIcon(instance)
	wndProc := syscall.NewCallback(guiWndProc)
	class := guiWndClassEx{
		Size:       uint32(unsafe.Sizeof(guiWndClassEx{})),
		WndProc:    wndProc,
		Instance:   instance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 6,
		ClassName:  className,
		IconSm:     icon,
	}
	if result, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); result == 0 && err != syscall.Errno(1410) {
		return fmt.Errorf("register window class: %w", err)
	}

	window := &guiWindow{autostartRequested: autostartRequested, taskbarCreated: taskbarCreated}
	token := registerGUIWindow(window)
	title, _ := syscall.UTF16PtrFromString(guiWindowTitle)
	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		0x00CF0000,
		0x80000000,
		0x80000000,
		620,
		500,
		0,
		0,
		instance,
		token,
	)
	if hwnd == 0 {
		unregisterGUIWindow(token)
		return fmt.Errorf("create window: %w", err)
	}

	window.hwnd = hwnd
	setGUIText(hwnd, guiWindowTitle)
	if autostartRequested {
		procShowWindow.Call(hwnd, guiSWHide)
	} else {
		procShowWindow.Call(hwnd, guiSWShow)
	}
	procUpdateWindow.Call(hwnd)

	var msg guiMsg
	for {
		result, _, getErr := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if result == 0 {
			break
		}
		if result == ^uintptr(0) {
			return fmt.Errorf("get message: %w", getErr)
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}

func guiWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == guiWMNCCreate {
		if lParam == 0 {
			return 0
		}
		create := guiCreateStructFromLParam(lParam)
		window := guiWindowForToken(create.CreateParams)
		if window == nil {
			return 0
		}
		window.mu.Lock()
		window.hwnd = hwnd
		window.mu.Unlock()
		procSetWindowLongPtr.Call(hwnd, guiGWLPUserData, create.CreateParams)
		return 1
	}

	window := guiWindowForHWND(hwnd)
	if window == nil {
		return defWindowProc(hwnd, message, wParam, lParam)
	}
	if message == window.taskbarCreated {
		window.mu.Lock()
		window.trayAdded = false
		window.mu.Unlock()
		window.ensureTrayIcon()
		return 0
	}

	switch message {
	case guiWMCreate:
		window.createControls()
		return 0
	case guiWMClose:
		if window.requestClose() {
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case guiWMDestroy:
		procPostQuitMessage.Call(0)
		return 0
	case guiWMSize:
		window.layoutControls()
		if wParam == guiWMSizeMinimized {
			window.hideToTray()
		}
	case guiWMCommand:
		commandID := uint32(wParam & 0xffff)
		notificationCode := uint32((wParam >> 16) & 0xffff)
		if notificationCode == 0 && commandID == guiButtonStart {
			window.startClient()
			return 0
		} else if notificationCode == 0 && commandID == guiButtonStop {
			window.stopClient()
			return 0
		} else if notificationCode == 0 && commandID == guiButtonInterfaces {
			window.refreshInterfaces()
			return 0
		} else if notificationCode == 0 && (commandID == guiCheckCredentials || commandID == guiCheckAutostart) {
			window.saveSettings()
			return 0
		}
	case guiWMAppTray:
		switch uint32(lParam) {
		case guiWMLButtonUp, guiWMLButtonDoubleClick:
			window.showFromTray()
		case guiWMRButtonUp:
			window.showTrayMenu()
		}
		return 0
	case guiWMLog:
		payload, ok := takeGUIString(lParam)
		if !ok || payload.owner != window.registryToken {
			return 0
		}
		window.appendLog(payload.text)
	case guiWMClientDone:
		if lParam != window.registryToken {
			return 0
		}
		window.mu.Lock()
		currentRun := window.runID
		window.mu.Unlock()
		if !guiRunMessageMatches(uint64(wParam), currentRun) {
			return 0
		}
		window.setStatus("已停止")
		window.mu.Lock()
		closing := window.closing
		window.mu.Unlock()
		if closing {
			procDestroyWindow.Call(hwnd)
		}
	case guiWMClientError:
		payload, ok := takeGUIString(lParam)
		if ok && payload.owner == window.registryToken {
			messageBox(window.hwnd, payload.text, guiWindowTitle)
		}
	case guiWMInterfacesDone:
		payload, ok := takeGUIInterfaceResult(lParam)
		if !ok || payload.owner != window.registryToken {
			return 0
		}
		window.mu.Lock()
		currentRefresh := window.refreshID
		closing := window.closing
		window.mu.Unlock()
		if !guiRefreshMessageMatches(closing, uint64(wParam), currentRefresh) {
			return 0
		}
		window.applyInterfaceResult(payload.result)
	case guiWMNetworkStatus:
		payload, ok := takeGUIString(lParam)
		if !ok || payload.owner != window.registryToken {
			return 0
		}
		window.mu.Lock()
		currentRun := window.runID
		compatibilityRun := window.compatibilityRunID
		closing := window.closing
		window.mu.Unlock()
		if !guiNetworkStatusMessageMatches(closing, uint64(wParam), currentRun, compatibilityRun) {
			return 0
		}
		window.setStatus(payload.text)
	case guiWMNCDestroy:
		window.removeTrayIcon()
		window.mu.Lock()
		font := window.font
		fontOwned := window.fontOwned
		window.font = 0
		window.fontOwned = false
		window.destroyed = true
		window.hwnd = 0
		token := window.registryToken
		window.mu.Unlock()
		if fontOwned && font != 0 {
			procDeleteObject.Call(font)
		}
		procSetWindowLongPtr.Call(hwnd, guiGWLPUserData, 0)
		unregisterGUIWindow(token)
		removeGUIPayloads(token)
	}
	return defWindowProc(hwnd, message, wParam, lParam)
}

func registerGUIWindow(window *guiWindow) uintptr {
	guiWindowRegistryMu.Lock()
	defer guiWindowRegistryMu.Unlock()
	guiNextWindowID++
	if guiNextWindowID == 0 {
		guiNextWindowID++
	}
	token := guiNextWindowID
	window.registryToken = token
	guiWindowRegistry[token] = window
	return token
}

func unregisterGUIWindow(token uintptr) {
	if token == 0 {
		return
	}
	guiWindowRegistryMu.Lock()
	delete(guiWindowRegistry, token)
	guiWindowRegistryMu.Unlock()
}

func guiWindowForToken(token uintptr) *guiWindow {
	if token == 0 {
		return nil
	}
	guiWindowRegistryMu.RLock()
	window := guiWindowRegistry[token]
	guiWindowRegistryMu.RUnlock()
	return window
}

func guiWindowForHWND(hwnd uintptr) *guiWindow {
	if hwnd == 0 {
		return nil
	}
	token, _, _ := procGetWindowLongPtr.Call(hwnd, guiGWLPUserData)
	return guiWindowForToken(token)
}

// guiCreateStructFromLParam reads the CREATESTRUCTW supplied by Win32 during
// WM_NCCREATE. The pointer is valid only for the duration of this callback.
func guiCreateStructFromLParam(lParam uintptr) *guiCreateStruct {
	return (*guiCreateStruct)(unsafe.Add(unsafe.Pointer(nil), lParam))
}

func (w *guiWindow) createControls() {
	w.font = createGUIFont()
	w.fontOwned = w.font != 0 && w.font != guiDefaultGUIFont
	labels := []uintptr{
		createGUIControl(0, "STATIC", "账号", guiStaticStyle(), 18, 22, 100, 24, w.hwnd, 0),
		createGUIControl(0, "STATIC", "密码", guiStaticStyle(), 18, 56, 100, 24, w.hwnd, 0),
		createGUIControl(0, "STATIC", "网卡", guiStaticStyle(), 18, 90, 100, 24, w.hwnd, 0),
	}
	w.user = createGUIControl(guiEditExClientEdge, "EDIT", "", guiEditControlStyle(false), 150, 18, 420, 26, w.hwnd, guiEditUser)
	w.password = createGUIControl(guiEditExClientEdge, "EDIT", "", guiEditControlStyle(true), 150, 52, 420, 26, w.hwnd, guiEditPassword)
	w.interfaces = createGUIControl(0, "COMBOBOX", "", guiComboStyle(), 150, 86, 300, 180, w.hwnd, guiComboInterface)
	w.start = createGUIButton("登录并保持", guiButtonStart, 150, 124, 120, 32, w.hwnd)
	w.stop = createGUIButton("停止/注销", guiButtonStop, 280, 124, 120, 32, w.hwnd)
	refresh := createGUIButton("刷新网卡", guiButtonInterfaces, 410, 124, 100, 32, w.hwnd)
	w.saveCredentials = createGUIControl(0, "BUTTON", "保存账号密码", guiCheckboxStyle(), 150, 166, 150, 24, w.hwnd, guiCheckCredentials)
	w.autostart = createGUIControl(0, "BUTTON", "开机自动启动并认证", guiCheckboxStyle(), 310, 166, 210, 24, w.hwnd, guiCheckAutostart)
	w.status = createGUIControl(0, "STATIC", "状态：未启动", guiStaticStyle(), 150, 198, 420, 24, w.hwnd, guiStaticStatus)
	w.log = createGUIControl(guiEditExClientEdge, "EDIT", "", guiLogStyle(), 18, 233, 570, 202, w.hwnd, guiEditLog)
	controls := append(labels, w.user, w.password, w.interfaces, w.start, w.stop, refresh, w.saveCredentials, w.autostart, w.status, w.log)
	for _, handle := range controls {
		setGUIFont(handle, w.font)
	}
	w.loadConfig()
	w.refreshInterfaces()
	persistentLog, persistentPath, persistentErr := setupPersistentLog()
	if persistentErr != nil {
		messageBox(w.hwnd, "无法创建本地日志文件："+persistentErr.Error(), guiWindowTitle)
	}
	w.logWriter = &guiLogWriter{window: w, file: persistentLog}
	log.SetOutput(w.logWriter)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if persistentPath != "" {
		log.Printf("[GUI] Persistent log: %s", persistentPath)
	}
	w.layoutControls()
	w.ensureTrayIcon()
}

func guiStaticStyle() uintptr {
	return guiWSChild | guiWSVisible
}

func guiEditControlStyle(password bool) uintptr {
	return guiWSChild | guiWSVisible | guiWSTabStop | guiEditStyle(password)
}

func guiComboStyle() uintptr {
	return guiWSChild | guiWSVisible | guiWSTabStop | guiCBSDropdownList | guiWSVScroll
}

func guiLogStyle() uintptr {
	return guiWSChild | guiWSVisible | guiWSVScroll | guiESMultiline | guiESAutoVScroll | guiESReadOnly
}

func (w *guiWindow) layoutControls() {
	var rect guiRect
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rect)))
	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	if w.log == 0 {
		return
	}
	logWidth := width - 36
	logHeight := height - 251
	if logWidth < 1 {
		logWidth = 1
	}
	if logHeight < 1 {
		logHeight = 1
	}
	moveGUIControl(w.log, 18, 233, logWidth, logHeight)
}

func (w *guiWindow) loadConfig() {
	executable, err := os.Executable()
	if err != nil {
		if w.autostartRequested {
			w.showFromTray()
			w.setStatus("无法获取程序路径，未自动认证")
		}
		return
	}
	w.configPath = guiConfigPath(executable)
	cfg, err := loadFileConfig(w.configPath)
	if err != nil {
		if w.autostartRequested {
			w.showFromTray()
			w.setStatus("读取配置失败，未自动认证")
		}
		return
	}
	w.config = cfg
	setGUIText(w.user, cfg.User)
	setGUIText(w.password, cfg.Password)
	credentialsAvailable := (cfg.SaveCredentials || (cfg.User != "" && cfg.Password != "")) && guiAutostartAllowed(true, cfg.User, cfg.Password)
	if credentialsAvailable {
		setGUIChecked(w.saveCredentials, true)
	}
	w.configuredNetwork = cfg.Network
	startWithWindows := cfg.StartWithWindows
	if enabled, err := isAutostartEnabled(executable); err == nil {
		startWithWindows = enabled && credentialsAvailable
		if enabled && !startWithWindows {
			_ = setAutostart(executable, false)
		}
	}
	setGUIChecked(w.autostart, startWithWindows)
	if w.autostartRequested {
		w.autostartPending = startWithWindows
		if !w.autostartPending {
			w.showFromTray()
		}
	}
	if len(w.selected) > 0 {
		setGUIComboSelection(w.interfaces, guiConfiguredInterfaceSelection(w.configuredNetwork, len(w.selected)))
	}
}

func (w *guiWindow) refreshInterfaces() {
	w.mu.Lock()
	if w.closing || w.destroyed || w.hwnd == 0 {
		w.mu.Unlock()
		return
	}
	w.refreshID++
	refreshID := w.refreshID
	hwnd := w.hwnd
	token := w.registryToken
	w.mu.Unlock()

	go func() {
		interfaces, err := ListNetworkInterfaces()
		postGUIInterfaceResult(hwnd, token, guiWMInterfacesDone, refreshID, guiInterfaceResult{
			interfaces: interfaces,
			err:        err,
		})
	}()
}

func (w *guiWindow) applyInterfaceResult(result guiInterfaceResult) {
	if result.err != nil {
		w.setStatus("网卡读取失败：" + result.err.Error())
		w.mu.Lock()
		autostartPending := w.autostartPending
		w.autostartPending = false
		w.mu.Unlock()
		if autostartPending {
			w.showFromTray()
			w.setStatus("网卡读取失败，未自动认证")
		}
		return
	}
	w.selected = result.interfaces
	procSendMessage.Call(w.interfaces, guiCBResetContent, 0, 0)
	addGUIComboItem(w.interfaces, "自动选择（系统路由）")
	for _, iface := range result.interfaces {
		addGUIComboItem(w.interfaces, fmt.Sprintf("%d: %s", iface.Index, iface.Name))
	}
	setGUIComboSelection(w.interfaces, guiConfiguredInterfaceSelection(w.configuredNetwork, len(result.interfaces)))
	if w.autostartPending {
		w.autostartPending = false
		if getGUIText(w.user) == "" || getGUIText(w.password) == "" {
			w.showFromTray()
			w.setStatus("未配置可自动认证的账号密码")
			return
		}
		w.startClient()
	}
}

func (w *guiWindow) startClient() {
	w.mu.Lock()
	if w.client != nil || w.closing {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	user := getGUIText(w.user)
	password := getGUIText(w.password)
	if user == "" || password == "" {
		messageBox(w.hwnd, "请输入账号和密码。", guiWindowTitle)
		return
	}

	selection := getGUIComboSelection(w.interfaces)
	iface, ok := guiInterfaceSelection(selection, w.selected)
	if !ok {
		messageBox(w.hwnd, "请选择有效的网卡。", guiWindowTitle)
		return
	}
	w.saveSettings()

	states := NewStates()
	session := NewSession()
	states.RefreshStates()
	var client *Client
	if iface == nil {
		transport, err := NewTUNAwareHTTPTransport()
		w.setStatus(guiAutomaticTransportStatus(err, transport != nil))
		if err == nil && transport != nil {
			client = NewClient(Options{LoginUser: user, LoginPassword: password, SMSCodeProvider: guiNoSMSCodeProvider{}}, states, session, transport)
		} else {
			client = NewClient(Options{LoginUser: user, LoginPassword: password, SMSCodeProvider: guiNoSMSCodeProvider{}}, states, session)
		}
	} else {
		transport, err := NewBoundHTTPTransport(iface)
		if err != nil {
			messageBox(w.hwnd, err.Error(), guiWindowTitle)
			return
		}
		client = NewClient(Options{LoginUser: user, LoginPassword: password, SMSCodeProvider: guiNoSMSCodeProvider{}}, states, session, transport)
	}
	w.mu.Lock()
	w.client = client
	w.states = states
	w.session = session
	w.stopping = false
	w.runID++
	runID := w.runID
	if iface == nil {
		w.compatibilityRunID = runID
	} else {
		w.compatibilityRunID = 0
	}
	hwnd := w.hwnd
	token := w.registryToken
	w.mu.Unlock()
	statusName := "自动路由"
	if iface != nil {
		statusName = iface.Name
	}
	w.setStatus("运行中：" + statusName)
	if iface == nil {
		w.inspectNetworkCompatibilityForRun(runID)
	}
	go func() {
		client.Run()
		connected := states.IsLogged()
		w.mu.Lock()
		wasStopping := w.stopping
		w.mu.Unlock()
		if session.IsInitialized() && states.IsLogged() {
			client.Term()
			states.SetLogged(false)
		}
		session.Free()
		w.mu.Lock()
		if w.client == client {
			w.client = nil
			w.states = nil
			w.session = nil
			w.stopping = false
			w.compatibilityRunID = 0
		}
		w.mu.Unlock()
		w.postGUIClientDone(hwnd, token, runID)
		if !connected && !wasStopping {
			postGUIString(hwnd, token, guiWMClientError, "连接失败，请查看日志获取详细信息。")
		}
	}()
}

func (w *guiWindow) stopClient() {
	w.mu.Lock()
	client, states := w.client, w.states
	if client != nil {
		w.stopping = true
		w.compatibilityRunID = 0
	}
	w.mu.Unlock()
	if client == nil {
		return
	}
	if states.IsRunning() {
		states.SetRunning(false)
	}
	w.setStatus("正在停止…")
}

func (w *guiWindow) requestClose() bool {
	w.mu.Lock()
	w.closing = true
	client := w.client
	w.mu.Unlock()
	if client == nil {
		return true
	}
	w.stopClient()
	return false
}

func (w *guiWindow) saveSettings() {
	user := getGUIText(w.user)
	password := getGUIText(w.password)
	network := getGUIComboSelection(w.interfaces)
	if network < 0 {
		network = 0
	}
	saveCredentials := isGUIChecked(w.saveCredentials)
	startWithWindows := isGUIChecked(w.autostart)
	if !guiAutostartAllowed(saveCredentials, user, password) {
		if startWithWindows {
			setGUIChecked(w.autostart, false)
			startWithWindows = false
			if user != "" || password != "" {
				messageBox(w.hwnd, "开机自动认证需要同时勾选保存账号密码，并填写完整账密。", guiWindowTitle)
			}
		}
	}
	if startWithWindows {
		executable, err := currentExecutablePath()
		if err != nil {
			setGUIChecked(w.autostart, false)
			startWithWindows = false
			messageBox(w.hwnd, "无法获取程序路径，不能设置开机启动。", guiWindowTitle)
		} else if err := setAutostart(executable, true); err != nil {
			setGUIChecked(w.autostart, false)
			startWithWindows = false
			messageBox(w.hwnd, "设置开机启动失败："+err.Error(), guiWindowTitle)
		}
	} else if executable, err := currentExecutablePath(); err == nil {
		if err := setAutostart(executable, false); err != nil {
			messageBox(w.hwnd, "取消开机启动失败："+err.Error(), guiWindowTitle)
		}
	}
	w.config = guiConfigForSave(w.config, user, password, network, saveCredentials, startWithWindows)
	if err := saveFileConfig(w.configPath, w.config); err != nil {
		messageBox(w.hwnd, "保存设置失败："+err.Error(), guiWindowTitle)
	}
}

func (w *guiWindow) setStatus(text string) {
	setGUIText(w.status, "状态："+text)
}

func (w *guiWindow) inspectNetworkCompatibilityForRun(runID uint64) {
	go func() {
		status, err := inspectNetworkCompatibility()
		if err != nil {
			log.Printf("[GUI] network compatibility inspection unavailable: %v", err)
			return
		}
		if message := networkCompatibilityLogMessage("[GUI]", status); message != "" {
			log.Print(message)
		}
		text := guiNetworkCompatibilityStatusText(status)
		if text == "" {
			return
		}
		w.mu.Lock()
		hwnd, owner, destroyed := w.hwnd, w.registryToken, w.destroyed
		w.mu.Unlock()
		if destroyed || hwnd == 0 || owner == 0 {
			return
		}
		postGUIStringWithWParam(hwnd, owner, guiWMNetworkStatus, uintptr(runID), text)
	}()
}

func (w *guiWindow) appendLog(text string) {
	if w.log == 0 {
		return
	}
	current := getGUIText(w.log)
	if len(current) > 16000 {
		current = current[len(current)-12000:]
	}
	if current != "" {
		current += "\r\n"
	}
	setGUIText(w.log, current+text)
	procSendMessage.Call(w.log, guiEMSetSel, 0, ^uintptr(0))
}

func (w *guiWindow) postGUIString(message uint32, text string) {
	w.mu.Lock()
	hwnd, owner, destroyed := w.hwnd, w.registryToken, w.destroyed
	w.mu.Unlock()
	if destroyed || hwnd == 0 || owner == 0 {
		return
	}
	postGUIString(hwnd, owner, message, text)
}

func postGUIString(hwnd, owner uintptr, message uint32, text string) {
	postGUIStringWithWParam(hwnd, owner, message, 0, text)
}

func postGUIStringWithWParam(hwnd, owner uintptr, message uint32, wParam uintptr, text string) {
	if hwnd == 0 || owner == 0 {
		return
	}
	guiStringMu.Lock()
	guiNextStringID++
	if guiNextStringID == 0 {
		guiNextStringID++
	}
	id := guiNextStringID
	guiStrings[id] = guiStringPayload{owner: owner, text: text}
	guiStringMu.Unlock()
	if result, _, _ := procPostMessage.Call(hwnd, uintptr(message), wParam, id); result == 0 {
		guiStringMu.Lock()
		delete(guiStrings, id)
		guiStringMu.Unlock()
	}
}

func postGUIInterfaceResult(hwnd, owner uintptr, message uint32, refreshID uint64, result guiInterfaceResult) {
	if hwnd == 0 || owner == 0 {
		return
	}
	guiStringMu.Lock()
	guiNextStringID++
	if guiNextStringID == 0 {
		guiNextStringID++
	}
	id := guiNextStringID
	guiInterfaceResults[id] = guiInterfacePayload{owner: owner, result: result}
	guiStringMu.Unlock()
	if posted, _, _ := procPostMessage.Call(hwnd, uintptr(message), uintptr(refreshID), id); posted == 0 {
		guiStringMu.Lock()
		delete(guiInterfaceResults, id)
		guiStringMu.Unlock()
	}
}

func takeGUIString(id uintptr) (guiStringPayload, bool) {
	if id == 0 {
		return guiStringPayload{}, false
	}
	guiStringMu.Lock()
	defer guiStringMu.Unlock()
	text, ok := guiStrings[id]
	if ok {
		delete(guiStrings, id)
	}
	return text, ok
}

func takeGUIInterfaceResult(id uintptr) (guiInterfacePayload, bool) {
	if id == 0 {
		return guiInterfacePayload{}, false
	}
	guiStringMu.Lock()
	defer guiStringMu.Unlock()
	result, ok := guiInterfaceResults[id]
	if ok {
		delete(guiInterfaceResults, id)
	}
	return result, ok
}

func removeGUIPayloads(owner uintptr) {
	if owner == 0 {
		return
	}
	guiStringMu.Lock()
	for id, payload := range guiStrings {
		if payload.owner == owner {
			delete(guiStrings, id)
		}
	}
	for id, payload := range guiInterfaceResults {
		if payload.owner == owner {
			delete(guiInterfaceResults, id)
		}
	}
	guiStringMu.Unlock()
}

func (w *guiWindow) postGUIClientDone(hwnd, owner uintptr, runID uint64) {
	w.mu.Lock()
	destroyed := w.destroyed
	w.mu.Unlock()
	if destroyed || hwnd == 0 || owner == 0 {
		return
	}
	procPostMessage.Call(hwnd, guiWMClientDone, uintptr(runID), owner)
}

func createGUIControl(exStyle uintptr, class, text string, style uintptr, x, y, width, height int, parent uintptr, id int) uintptr {
	classPtr, _ := syscall.UTF16PtrFromString(class)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	instance, _, _ := procGetModuleHandle.Call(0)
	hwnd, _, _ := procCreateWindowEx.Call(exStyle, uintptr(unsafe.Pointer(classPtr)), uintptr(unsafe.Pointer(textPtr)), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, uintptr(id), instance, 0)
	return hwnd
}

func createGUIButton(text string, id, x, y, width, height int, parent uintptr) uintptr {
	return createGUIControl(0, "BUTTON", text, 0x50010000, x, y, width, height, parent, id)
}

func setGUIText(hwnd uintptr, text string) {
	ptr, _ := syscall.UTF16PtrFromString(text)
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
}

func getGUIText(hwnd uintptr) string {
	length, _, _ := procSendMessage.Call(hwnd, 0x000E, 0, 0)
	buffer := make([]uint16, length+1)
	procSendMessage.Call(hwnd, 0x000D, length+1, uintptr(unsafe.Pointer(&buffer[0])))
	return syscall.UTF16ToString(buffer)
}

func setGUIFont(hwnd, font uintptr) {
	if hwnd != 0 && font != 0 {
		procSendMessage.Call(hwnd, guiWMSetFont, font, 1)
	}
}

func createGUIFont() uintptr {
	options := defaultGUIFontOptions()
	face, _ := syscall.UTF16PtrFromString(options.face)
	font, _, _ := procCreateFont.Call(
		uintptr(options.height),
		0,
		0,
		0,
		uintptr(options.weight),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(face)),
	)
	if font != 0 {
		return font
	}
	stockFont, _, _ := procGetStockObject.Call(guiDefaultGUIFont)
	return stockFont
}

func addGUIComboItem(hwnd uintptr, text string) {
	ptr, _ := syscall.UTF16PtrFromString(text)
	procSendMessage.Call(hwnd, guiCBAddString, 0, uintptr(unsafe.Pointer(ptr)))
}

func setGUIComboSelection(hwnd uintptr, index int) {
	procSendMessage.Call(hwnd, guiCBSetCurrent, uintptr(index), 0)
}

func getGUIComboSelection(hwnd uintptr) int {
	result, _, _ := procSendMessage.Call(hwnd, guiCBGetCurrent, 0, 0)
	return int(result)
}

func moveGUIControl(hwnd uintptr, x, y, width, height int32) {
	procMoveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 1)
}

func messageBox(hwnd uintptr, text, title string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	procMessageBox.Call(hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x10)
}

func setGUIChecked(hwnd uintptr, checked bool) {
	value := uintptr(0)
	if checked {
		value = guiBSTChecked
	}
	procSendMessage.Call(hwnd, guiBMSetCheck, value, 0)
}

func isGUIChecked(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	result, _, _ := procSendMessage.Call(hwnd, guiBMGetCheck, 0, 0)
	return result == guiBSTChecked
}

func registerGUIWindowMessage(name string) uint32 {
	ptr, _ := syscall.UTF16PtrFromString(name)
	result, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(ptr)))
	return uint32(result)
}

func guiTrayEventRestoresWindow(event uint32) bool {
	return event == guiWMLButtonUp || event == guiWMLButtonDoubleClick
}

func (w *guiWindow) ensureTrayIcon() {
	w.mu.Lock()
	if w.trayAdded || w.hwnd == 0 || w.destroyed {
		w.mu.Unlock()
		return
	}
	hwnd := w.hwnd
	w.mu.Unlock()
	instance, _, _ := procGetModuleHandle.Call(0)
	icon := loadGUIAppIcon(instance)
	var tip [128]uint16
	copy(tip[:], syscall.StringToUTF16(guiWindowTitle))
	data := guiNotifyIconData{
		Size:            uint32(unsafe.Sizeof(guiNotifyIconData{})),
		Window:          hwnd,
		ID:              1,
		Flags:           guiNIFMessage | guiNIFIcon | guiNIFTip,
		CallbackMessage: guiWMAppTray,
		Icon:            icon,
		Tip:             tip,
	}
	if result, _, _ := procShellNotifyIcon.Call(guiNIMAdd, uintptr(unsafe.Pointer(&data))); result != 0 {
		w.mu.Lock()
		w.trayIcon = icon
		w.trayAdded = true
		w.mu.Unlock()
	}
}

func loadGUIAppIcon(instance uintptr) uintptr {
	return loadImageIcon(instance, guiAppIconResourceID, 0, 0)
}

func loadImageIcon(instance, resourceID uintptr, width, height int32) uintptr {
	const (
		guiImageIcon     = 1
		guiLRDefaultSize = 0x00000040
		guiLRShared      = 0x00008000
	)
	icon, _, _ := procLoadImage.Call(
		instance,
		resourceID,
		guiImageIcon,
		uintptr(width),
		uintptr(height),
		guiLRDefaultSize|guiLRShared,
	)
	if icon == 0 {
		icon, _, _ = procLoadIcon.Call(instance, resourceID)
	}
	return icon
}

func (w *guiWindow) removeTrayIcon() {
	w.mu.Lock()
	if !w.trayAdded || w.hwnd == 0 {
		w.mu.Unlock()
		return
	}
	hwnd := w.hwnd
	w.trayAdded = false
	w.trayIcon = 0
	w.mu.Unlock()
	data := guiNotifyIconData{
		Size:   uint32(unsafe.Sizeof(guiNotifyIconData{})),
		Window: hwnd,
		ID:     1,
	}
	procShellNotifyIcon.Call(guiNIMDelete, uintptr(unsafe.Pointer(&data)))
}

func (w *guiWindow) hideToTray() {
	w.ensureTrayIcon()
	procShowWindow.Call(w.hwnd, guiSWHide)
}

func (w *guiWindow) showFromTray() {
	w.ensureTrayIcon()
	procShowWindow.Call(w.hwnd, guiSWShow)
	procShowWindow.Call(w.hwnd, guiSWRestore)
	procSetForeground.Call(w.hwnd)
}

func (w *guiWindow) showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	showText, _ := syscall.UTF16PtrFromString("显示主窗口")
	exitText, _ := syscall.UTF16PtrFromString("退出")
	procAppendMenu.Call(menu, guiMFString, guiTrayShow, uintptr(unsafe.Pointer(showText)))
	procAppendMenu.Call(menu, guiMFSeparator, 0, 0)
	procAppendMenu.Call(menu, guiMFString, guiTrayExit, uintptr(unsafe.Pointer(exitText)))
	procSetForeground.Call(w.hwnd)
	var point guiPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	command, _, _ := procTrackPopupMenu.Call(menu, guiTPMRightButton|guiTPMReturnCommand, uintptr(point.X), uintptr(point.Y), 0, w.hwnd, 0)
	procPostMessage.Call(w.hwnd, 0, 0, 0)
	switch uint32(command) {
	case guiTrayShow:
		w.showFromTray()
	case guiTrayExit:
		if w.requestClose() {
			procDestroyWindow.Call(w.hwnd)
		}
	}
}

func defWindowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}
