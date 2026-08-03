package main

import (
	"strings"
	"sync"
)

const (
	guiWSBorder       uintptr = 0x00800000
	guiESAutoHScroll  uintptr = 0x0080
	guiESPassword     uintptr = 0x0020
	guiBSAutoCheckbox uintptr = 0x0003
)

func guiConfigPath(executablePath string) string {
	separator := strings.LastIndexAny(executablePath, `\/`)
	if separator == -1 {
		return "esurfing.local.json"
	}
	return executablePath[:separator+1] + "esurfing.local.json"
}

func guiRunMessageMatches(messageRun, currentRun uint64) bool {
	return messageRun == currentRun
}

func guiMessageWindowMatches(messageWindow, callbackWindow uintptr) bool {
	return messageWindow != 0 && messageWindow == callbackWindow
}

func guiRefreshMessageMatches(closing bool, messageRefresh, currentRefresh uint64) bool {
	return !closing && messageRefresh == currentRefresh
}

// guiInterfaceSelection maps the combo-box selection to an optional interface.
// Selection zero means that the operating system should choose the route.
func guiInterfaceSelection(selection int, interfaces []NetworkInterface) (*NetworkInterface, bool) {
	if selection == 0 {
		return nil, true
	}
	if selection < 1 || selection > len(interfaces) {
		return nil, false
	}
	return &interfaces[selection-1], true
}

// guiConfiguredInterfaceSelection converts the config's 1-based interface
// number to the combo-box index, where zero means automatic routing.
func guiConfiguredInterfaceSelection(network, interfaceCount int) int {
	if network < 1 || network > interfaceCount {
		return 0
	}
	return network
}

func guiEditStyle(password bool) uintptr {
	style := guiWSBorder | guiESAutoHScroll
	if password {
		style |= guiESPassword
	}
	return style
}

func guiCheckboxStyle() uintptr {
	return guiWSChild | guiWSVisible | guiWSTabStop | guiBSAutoCheckbox
}

func guiAutostartAllowed(saveCredentials bool, user, password string) bool {
	return saveCredentials && strings.TrimSpace(user) != "" && strings.TrimSpace(password) != ""
}

type guiFontOptions struct {
	height int32
	weight int32
	face   string
}

func defaultGUIFontOptions() guiFontOptions {
	return guiFontOptions{
		height: -16,
		weight: 400,
		face:   "Microsoft YaHei UI",
	}
}

// guiNoSMSCodeProvider keeps the GUI from blocking on os.Stdin. The normal
// campus login path has no user-entered SMS step; CLI callers retain the
// compatibility provider and pre-entered SMSCode options.
type guiNoSMSCodeProvider struct{}

func (guiNoSMSCodeProvider) Wait() (string, bool) {
	return "", false
}

// SMSCodeProvider supplies a code after the server requests SMS verification.
type SMSCodeProvider interface {
	Wait() (string, bool)
}

type smsCodeWaiter struct {
	mu        sync.Mutex
	signal    chan struct{}
	code      string
	cancelled bool
	signaled  bool
	requested bool
	onRequest func()
}

func newSMSCodeWaiter(onRequest func()) *smsCodeWaiter {
	return &smsCodeWaiter{
		signal:    make(chan struct{}),
		onRequest: onRequest,
	}
}

func (w *smsCodeWaiter) Wait() (string, bool) {
	w.mu.Lock()
	if w.code != "" {
		code := w.code
		w.mu.Unlock()
		return code, true
	}
	if w.cancelled {
		w.mu.Unlock()
		return "", false
	}
	var onRequest func()
	if !w.requested {
		w.requested = true
		onRequest = w.onRequest
	}
	signal := w.signal
	w.mu.Unlock()

	if onRequest != nil {
		onRequest()
	}
	<-signal

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancelled || w.code == "" {
		return "", false
	}
	return w.code, true
}

func (w *smsCodeWaiter) Submit(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancelled || w.code != "" {
		return false
	}
	w.code = code
	w.signalOnceLocked()
	return true
}

func (w *smsCodeWaiter) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancelled = true
	w.signalOnceLocked()
}

func (w *smsCodeWaiter) signalOnceLocked() {
	if !w.signaled {
		w.signaled = true
		close(w.signal)
	}
}
