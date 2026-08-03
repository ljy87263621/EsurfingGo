package main

import "testing"

func TestGUIConfigPathUsesExecutableDirectory(t *testing.T) {
	got := guiConfigPath(`C:\Tools\EsurfingGo\esurfing.exe`)
	want := `C:\Tools\EsurfingGo\esurfing.local.json`
	if got != want {
		t.Fatalf("guiConfigPath() = %q, want %q", got, want)
	}
}

func TestGUIRunMessageMatchesCurrentRun(t *testing.T) {
	if !guiRunMessageMatches(8, 8) {
		t.Fatal("current run should accept its completion message")
	}
	if guiRunMessageMatches(7, 8) {
		t.Fatal("stale run should not accept its completion message")
	}
}

func TestGUIMessageWindowMustMatchCallbackHandle(t *testing.T) {
	if !guiMessageWindowMatches(0x101, 0x101) {
		t.Fatal("message for the associated window should be accepted")
	}
	if guiMessageWindowMatches(0x101, 0x202) {
		t.Fatal("message for a different window should be rejected")
	}
	if guiMessageWindowMatches(0, 0) {
		t.Fatal("zero handles should not identify a window")
	}
}

func TestGUIRefreshMessageRejectsStaleOrClosingWindow(t *testing.T) {
	if !guiRefreshMessageMatches(false, 4, 4) {
		t.Fatal("current refresh should be accepted")
	}
	if guiRefreshMessageMatches(false, 3, 4) {
		t.Fatal("stale refresh should be rejected")
	}
	if guiRefreshMessageMatches(true, 4, 4) {
		t.Fatal("refresh for a closing window should be rejected")
	}
}

func TestGUIInterfaceSelectionAllowsAutomaticRouting(t *testing.T) {
	interfaces := []NetworkInterface{{Index: 1, Name: "Ethernet"}}

	iface, ok := guiInterfaceSelection(0, interfaces)
	if !ok || iface != nil {
		t.Fatalf("automatic selection = (%v, %t), want (nil, true)", iface, ok)
	}

	iface, ok = guiInterfaceSelection(1, interfaces)
	if !ok || iface == nil || iface.Name != "Ethernet" {
		t.Fatalf("explicit selection = (%v, %t), want Ethernet and true", iface, ok)
	}

	if _, ok = guiInterfaceSelection(2, interfaces); ok {
		t.Fatal("out-of-range selection should be rejected")
	}
}

func TestGUIConfiguredInterfaceSelectionUsesComboIndex(t *testing.T) {
	if got := guiConfiguredInterfaceSelection(2, 3); got != 2 {
		t.Fatalf("configured network 2 = combo index %d, want 2", got)
	}
	if got := guiConfiguredInterfaceSelection(0, 3); got != 0 {
		t.Fatalf("automatic network = combo index %d, want 0", got)
	}
	if got := guiConfiguredInterfaceSelection(4, 3); got != 0 {
		t.Fatalf("out-of-range network = combo index %d, want 0", got)
	}
}

func TestGUIEditStylesRenderAndScrollText(t *testing.T) {
	style := guiEditStyle(false)
	if style&guiWSBorder == 0 {
		t.Fatal("account edit should have a visible border")
	}
	if style&guiESAutoHScroll == 0 {
		t.Fatal("account edit should scroll long text horizontally")
	}

	passwordStyle := guiEditStyle(true)
	if passwordStyle&guiESPassword == 0 {
		t.Fatal("password edit should mask entered text")
	}
}

func TestGUICheckboxStyleUsesAutomaticCheckState(t *testing.T) {
	style := guiCheckboxStyle()
	if style&guiWSVisible == 0 || style&guiWSTabStop == 0 {
		t.Fatalf("checkbox style = %#x, want visible tab-stop control", style)
	}
	if style&guiBSAutoCheckbox == 0 {
		t.Fatalf("checkbox style = %#x, want automatic checkbox behavior", style)
	}
}

func TestGUIAutostartRequiresSavedCredentials(t *testing.T) {
	if !guiAutostartAllowed(true, "user", "password") {
		t.Fatal("saved non-empty credentials should allow autostart")
	}
	if guiAutostartAllowed(false, "user", "password") {
		t.Fatal("autostart should require the save-credentials option")
	}
	if guiAutostartAllowed(true, "", "password") || guiAutostartAllowed(true, "user", "") {
		t.Fatal("autostart should reject incomplete credentials")
	}
}

func TestGUIFontOptionsKeepFaceNameInCreateFontCall(t *testing.T) {
	options := defaultGUIFontOptions()
	if options.height >= 0 {
		t.Fatalf("font height = %d, want a negative character height", options.height)
	}
	if options.weight != 400 {
		t.Fatalf("font weight = %d, want 400", options.weight)
	}
	if options.face != "Microsoft YaHei UI" {
		t.Fatalf("font face = %q, want Microsoft YaHei UI", options.face)
	}
}

func TestGUIUnavailableSMSCodeProviderReturnsWithoutWaiting(t *testing.T) {
	code, ok := guiNoSMSCodeProvider{}.Wait()
	if code != "" || ok {
		t.Fatalf("GUI SMS provider = (%q, %t), want empty code and false", code, ok)
	}
}

func TestSMSCodeWaiterReturnsSubmittedCode(t *testing.T) {
	waiter := newSMSCodeWaiter(nil)
	result := make(chan struct {
		code string
		ok   bool
	}, 1)
	go func() {
		code, ok := waiter.Wait()
		result <- struct {
			code string
			ok   bool
		}{code: code, ok: ok}
	}()

	if !waiter.Submit(" 123456 ") {
		t.Fatal("Submit should accept a non-empty code")
	}
	got := <-result
	if got.code != "123456" || !got.ok {
		t.Fatalf("Wait() = (%q, %t), want (123456, true)", got.code, got.ok)
	}
}

func TestSMSCodeWaiterCanBeCancelled(t *testing.T) {
	waiter := newSMSCodeWaiter(nil)
	result := make(chan struct {
		code string
		ok   bool
	}, 1)
	go func() {
		code, ok := waiter.Wait()
		result <- struct {
			code string
			ok   bool
		}{code: code, ok: ok}
	}()

	waiter.Cancel()
	got := <-result
	if got.code != "" || got.ok {
		t.Fatalf("Wait() = (%q, %t), want empty code and false", got.code, got.ok)
	}
}
