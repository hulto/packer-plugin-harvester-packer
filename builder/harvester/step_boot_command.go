// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"

	hvclient "github.com/hashicorp/packer-plugin-scaffolding/builder/harvester/client"
)

// StepBootCommand sends keyboard input to a VM via the KubeVirt VNC WebSocket
// subresource. It implements the boot_command feature analogous to the QEMU
// or VirtualBox packer builders.
//
// The VNC WebSocket URL is: /apis/subresources.kubevirt.io/v1/namespaces/{ns}/
// virtualmachineinstances/{name}/vnc
//
// The KubeVirt VNC proxy forwards the raw RFB (VNC) protocol over WebSocket
// binary frames. This step implements a minimal subset of RFB needed to send
// key events.
type StepBootCommand struct {
	Config *Config
}

// Run sends boot commands to the VM via VNC.
func (s *StepBootCommand) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if len(s.Config.BootCommand) == 0 {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	vmName, ok := state.Get("vm_name").(string)
	if !ok || vmName == "" {
		ui.Error("vm_name not found in state")
		state.Put("error", fmt.Errorf("vm_name missing from state"))
		return multistep.ActionHalt
	}

	// Wait the configured boot time.
	ui.Say(fmt.Sprintf("Waiting %s before sending boot commands...", s.Config.BootWait))
	select {
	case <-time.After(s.Config.BootWait):
	case <-ctx.Done():
		state.Put("error", ctx.Err())
		return multistep.ActionHalt
	}

	// Interpolate boot commands (supports {{.HTTPIP}}, {{.HTTPPort}} etc.).
	httpIP := stateString(state, "http_ip")
	httpPort := stateString(state, "http_port")
	if usesHTTPTemplateVars(s.Config.BootCommand) {
		if httpPort == "" || httpPort == "0" {
			ui.Error("http_directory is configured but HTTP server is not running (http_port missing)")
			state.Put("error", fmt.Errorf("boot_command requires HTTP server variables, but http_port is missing"))
			return multistep.ActionHalt
		}
		if httpIP == "" {
			vmIP := stateString(state, "vm_ip")
			resolvedIP, err := resolveHTTPIP(client.BaseURL(), vmIP)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to resolve HTTPIP for VM reachability: %s", err))
				state.Put("error", fmt.Errorf("resolve HTTPIP for boot_command: %w", err))
				return multistep.ActionHalt
			}
			httpIP = resolvedIP
			state.Put("http_ip", resolvedIP)
			ui.Say(fmt.Sprintf("Using HTTP server address %s:%s for boot command templates", httpIP, httpPort))
		}
	}

	renderCtx := &interpolate.Context{
		Data: &bootCommandTemplateData{
			HTTPIP:   httpIP,
			HTTPPort: httpPort,
		},
	}

	conn, err := s.connectAndHandshakeVNCWithRetry(ctx, client, vmName, s.Config.WaitForInstanceTimeout)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to connect to VNC for VM %q: %s", vmName, err))
		state.Put("error", err)
		return multistep.ActionHalt
	}
	defer conn.Close()

	ui.Say(fmt.Sprintf("Connected to VNC console of VM %q; sending %d boot command(s)...",
		vmName, len(s.Config.BootCommand)))

	// Send each command line.
	for _, cmd := range s.Config.BootCommand {
		rendered, err := interpolate.Render(cmd, renderCtx)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to interpolate boot command %q: %s", cmd, err))
			state.Put("error", err)
			return multistep.ActionHalt
		}
		const maxAttempts = 3
		sent := false
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			err = sendBootCommand(conn, rendered)
			if err == nil {
				sent = true
				break
			}
			if !isRecoverableVNCError(err) || attempt == maxAttempts {
				break
			}

			ui.Say(fmt.Sprintf("VNC connection dropped while sending boot command; reconnecting (attempt %d/%d): %s", attempt+1, maxAttempts, err))
			_ = conn.Close()
			conn, err = s.connectAndHandshakeVNC(client, vmName)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to reconnect VNC for VM %q: %s", vmName, err))
				state.Put("error", err)
				return multistep.ActionHalt
			}
		}

		if !sent {
			ui.Error(fmt.Sprintf("Failed to send boot command: %s", err))
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	ui.Say("Boot commands sent successfully.")
	return multistep.ActionContinue
}

// Cleanup is a no-op.
func (s *StepBootCommand) Cleanup(_ multistep.StateBag) {}

func (s *StepBootCommand) connectAndHandshakeVNC(client *hvclient.HarvesterClient, vmName string) (*websocket.Conn, error) {
	conn, err := s.dialVNC(client, vmName)
	if err != nil {
		return nil, err
	}
	if err := rfbHandshake(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("VNC handshake failed: %w", err)
	}
	return conn, nil
}

func (s *StepBootCommand) connectAndHandshakeVNCWithRetry(ctx context.Context, client *hvclient.HarvesterClient, vmName string, timeout time.Duration) (*websocket.Conn, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		conn, err := s.connectAndHandshakeVNC(client, vmName)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for VNC")
	}
	return nil, lastErr
}

// dialVNC opens a WebSocket connection to the KubeVirt VNC subresource.
func (s *StepBootCommand) dialVNC(client *hvclient.HarvesterClient, vmName string) (*websocket.Conn, error) {
	baseURL := client.BaseURL()
	token := client.VNCToken()

	// Convert https://host to wss://host (or http to ws).
	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += client.VNCWebSocketURL(vmName)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: s.Config.SkipTLSVerify, //nolint:gosec
		},
		Subprotocols: []string{"binary"},
		HandshakeTimeout: 30 * time.Second,
	}

	headers := http.Header{}
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("dial VNC WebSocket %s: %w", wsURL, err)
	}
	return conn, nil
}

// rfbHandshake performs the minimal RFB 3.8 handshake required before sending
// key events. The KubeVirt VNC proxy handles auth on the server side, so we
// only need to exchange version strings and acknowledge the auth result.
func rfbHandshake(conn *websocket.Conn) error {
	// 1. Read server version (12 bytes: "RFB XXX.YYY\n").
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read server version: %w", err)
	}
	if len(msg) < 12 || !strings.HasPrefix(string(msg), "RFB ") {
		return fmt.Errorf("unexpected server version greeting: %q", string(msg))
	}

	// 2. Send client version (3.8).
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("RFB 003.008\n")); err != nil {
		return fmt.Errorf("send client version: %w", err)
	}

	// 3. Read security types.
	_, secTypes, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read security types: %w", err)
	}

	if len(secTypes) == 0 {
		return fmt.Errorf("server sent empty security types")
	}

	// Choose security type 1 (None) if offered; otherwise pick the first.
	chosenSec := secTypes[0]
	for i := 1; i < len(secTypes); i++ {
		if secTypes[i] == 1 {
			chosenSec = 1
			break
		}
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{chosenSec}); err != nil {
		return fmt.Errorf("send security type: %w", err)
	}

	// 4. If security type != 1 (None), read a challenge and send 16 zero bytes
	//    (VNC password auth with empty password – the proxy handles real auth).
	if chosenSec != 1 {
		_, _, err = conn.ReadMessage() // challenge
		if err != nil {
			return fmt.Errorf("read VNC challenge: %w", err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, make([]byte, 16)); err != nil {
			return fmt.Errorf("send VNC response: %w", err)
		}
	}

	// 5. Read security result (4 bytes, 0 = OK).
	_, result, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read security result: %w", err)
	}
	if len(result) >= 4 && binary.BigEndian.Uint32(result[:4]) != 0 {
		return fmt.Errorf("VNC authentication failed (result=%d)", binary.BigEndian.Uint32(result[:4]))
	}

	// 6. Send ClientInit (1 = shared desktop).
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{1}); err != nil {
		return fmt.Errorf("send ClientInit: %w", err)
	}

	// 7. Read ServerInit and discard.
	if _, _, err := conn.ReadMessage(); err != nil {
		return fmt.Errorf("read ServerInit: %w", err)
	}

	return nil
}

// sendBootCommand interprets a packer boot_command string and sends the
// corresponding RFB KeyEvent messages.
//
// Supported special sequences (case-insensitive):
//
//	<enter>  <return>  <bs>  <del>  <esc>  <tab>
//	<up>  <down>  <left>  <right>
//	<f1>–<f12>
//	<spacebar>  <wait>  <wait5>  <wait10>
//	<leftShift>  <rightShift>  <leftCtrl>  <rightCtrl>
//	<leftAlt>  <rightAlt>  <leftSuper>  <rightSuper>
func sendBootCommand(conn *websocket.Conn, cmd string) error {
	i := 0
	for i < len(cmd) {
		if cmd[i] == '<' {
			end := strings.IndexByte(cmd[i:], '>')
			if end == -1 {
				return fmt.Errorf("unclosed '<' in boot command at position %d", i)
			}
			token := strings.ToLower(cmd[i+1 : i+end])
			i += end + 1

			switch token {
			case "wait":
				time.Sleep(1 * time.Second)
			case "wait5":
				time.Sleep(5 * time.Second)
			case "wait10":
				time.Sleep(10 * time.Second)
			default:
				key, ok := specialKeys[token]
				if !ok {
					return fmt.Errorf("unknown special key sequence <%s>", token)
				}
				if err := sendKeyPress(conn, key); err != nil {
					return err
				}
			}
		} else {
			r := rune(cmd[i])
			i++
			keySym, shift := runeToKeySym(r)
			if shift {
				if err := sendKeyDown(conn, rfbKeyLeftShift); err != nil {
					return err
				}
			}
			if err := sendKeyPress(conn, keySym); err != nil {
				return err
			}
			if shift {
				if err := sendKeyUp(conn, rfbKeyLeftShift); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// sendKeyPress sends a key-down then key-up RFB event.
func sendKeyPress(conn *websocket.Conn, keySym uint32) error {
	if err := sendKeyDown(conn, keySym); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return sendKeyUp(conn, keySym)
}

// sendKeyDown sends an RFB KeyEvent (down=1).
func sendKeyDown(conn *websocket.Conn, keySym uint32) error {
	return sendKeyEvent(conn, 1, keySym)
}

// sendKeyUp sends an RFB KeyEvent (down=0).
func sendKeyUp(conn *websocket.Conn, keySym uint32) error {
	return sendKeyEvent(conn, 0, keySym)
}

// sendKeyEvent sends a raw RFB KeyEvent message.
//
// RFB KeyEvent format (8 bytes):
//
//	byte 0  : message-type (4)
//	byte 1  : down-flag (1=down, 0=up)
//	bytes 2-3: padding
//	bytes 4-7: key-sym (big-endian uint32)
func sendKeyEvent(conn *websocket.Conn, downFlag uint8, keySym uint32) error {
	msg := make([]byte, 8)
	msg[0] = 4 // KeyEvent message type
	msg[1] = downFlag
	msg[2] = 0 // padding
	msg[3] = 0 // padding
	binary.BigEndian.PutUint32(msg[4:], keySym)
	return conn.WriteMessage(websocket.BinaryMessage, msg)
}

// runeToKeySym converts a Unicode rune to its X11 keysym value.
// Returns (keysym, needsShift).
func runeToKeySym(r rune) (uint32, bool) {
	// Printable ASCII maps directly to its Unicode value in X11 keysyms.
	if r >= 0x20 && r <= 0x7e {
		// Check if the character requires shift.
		shift := unicode.IsUpper(r) || isShiftSymbol(r)
		return uint32(r), shift
	}
	return uint32(r), false
}

// isShiftSymbol returns true for characters that require the Shift key on a
// standard US keyboard layout.
func isShiftSymbol(r rune) bool {
	const shiftChars = "~!@#$%^&*()_+{}|:\"<>?"
	return strings.ContainsRune(shiftChars, r)
}

// X11 keysym constants for special keys.
const (
	rfbKeyBackSpace   uint32 = 0xff08
	rfbKeyTab         uint32 = 0xff09
	rfbKeyReturn      uint32 = 0xff0d
	rfbKeyEscape      uint32 = 0xff1b
	rfbKeyDelete      uint32 = 0xffff
	rfbKeyHome        uint32 = 0xff50
	rfbKeyLeft        uint32 = 0xff51
	rfbKeyUp          uint32 = 0xff52
	rfbKeyRight       uint32 = 0xff53
	rfbKeyDown        uint32 = 0xff54
	rfbKeyPageUp      uint32 = 0xff55
	rfbKeyPageDown    uint32 = 0xff56
	rfbKeyEnd         uint32 = 0xff57
	rfbKeyInsert      uint32 = 0xff63
	rfbKeyF1          uint32 = 0xffbe
	rfbKeyF2          uint32 = 0xffbf
	rfbKeyF3          uint32 = 0xffc0
	rfbKeyF4          uint32 = 0xffc1
	rfbKeyF5          uint32 = 0xffc2
	rfbKeyF6          uint32 = 0xffc3
	rfbKeyF7          uint32 = 0xffc4
	rfbKeyF8          uint32 = 0xffc5
	rfbKeyF9          uint32 = 0xffc6
	rfbKeyF10         uint32 = 0xffc7
	rfbKeyF11         uint32 = 0xffc8
	rfbKeyF12         uint32 = 0xffc9
	rfbKeyLeftShift   uint32 = 0xffe1
	rfbKeyRightShift  uint32 = 0xffe2
	rfbKeyLeftCtrl    uint32 = 0xffe3
	rfbKeyRightCtrl   uint32 = 0xffe4
	rfbKeyLeftAlt     uint32 = 0xffe9
	rfbKeyRightAlt    uint32 = 0xffea
	rfbKeyLeftSuper   uint32 = 0xffeb
	rfbKeyRightSuper  uint32 = 0xffec
	rfbKeySpacebar    uint32 = 0x0020
)

// specialKeys maps packer boot_command token strings to X11 keysyms.
var specialKeys = map[string]uint32{
	"bs":          rfbKeyBackSpace,
	"backspace":   rfbKeyBackSpace,
	"del":         rfbKeyDelete,
	"delete":      rfbKeyDelete,
	"enter":       rfbKeyReturn,
	"return":      rfbKeyReturn,
	"esc":         rfbKeyEscape,
	"escape":      rfbKeyEscape,
	"tab":         rfbKeyTab,
	"up":          rfbKeyUp,
	"down":        rfbKeyDown,
	"left":        rfbKeyLeft,
	"right":       rfbKeyRight,
	"home":        rfbKeyHome,
	"end":         rfbKeyEnd,
	"pageup":      rfbKeyPageUp,
	"pagedown":    rfbKeyPageDown,
	"insert":      rfbKeyInsert,
	"spacebar":    rfbKeySpacebar,
	"f1":          rfbKeyF1,
	"f2":          rfbKeyF2,
	"f3":          rfbKeyF3,
	"f4":          rfbKeyF4,
	"f5":          rfbKeyF5,
	"f6":          rfbKeyF6,
	"f7":          rfbKeyF7,
	"f8":          rfbKeyF8,
	"f9":          rfbKeyF9,
	"f10":         rfbKeyF10,
	"f11":         rfbKeyF11,
	"f12":         rfbKeyF12,
	"leftshift":   rfbKeyLeftShift,
	"rightshift":  rfbKeyRightShift,
	"leftctrl":    rfbKeyLeftCtrl,
	"rightctrl":   rfbKeyRightCtrl,
	"leftalt":     rfbKeyLeftAlt,
	"rightalt":    rfbKeyRightAlt,
	"leftsuper":   rfbKeyLeftSuper,
	"rightsuper":  rfbKeyRightSuper,
}

// bootCommandTemplateData is passed to the interpolation engine for boot commands.
type bootCommandTemplateData struct {
	HTTPIP   string
	HTTPPort string
}

func usesHTTPTemplateVars(commands []string) bool {
	for _, cmd := range commands {
		if strings.Contains(cmd, "{{.HTTPIP}}") || strings.Contains(cmd, "{{ .HTTPIP }}") ||
			strings.Contains(cmd, "{{.HTTPPort}}") || strings.Contains(cmd, "{{ .HTTPPort }}") {
			return true
		}
	}
	return false
}

func stateString(state multistep.StateBag, key string) string {
	v, ok := state.GetOk(key)
	if !ok || v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" {
		return ""
	}
	return s
}

// resolveHTTPIP determines a reachable IP for the HTTP server that guests can
// use in boot command templates. It prefers the API server host (which is
// usually routable from the VM network), then falls back to the local source
// IP used to reach the VM.
func resolveHTTPIP(baseURL, vmIP string) (string, error) {
	if ip, err := httpIPFromBaseURL(baseURL); err == nil && ip != "" {
		return ip, nil
	}
	if vmIP == "" {
		return "", fmt.Errorf("no vm_ip available and API host was not usable")
	}
	return localIPForTarget(vmIP)
}

func httpIPFromBaseURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("base URL %q has no host", baseURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	if len(ips) > 0 {
		return ips[0].String(), nil
	}
	return "", fmt.Errorf("no IP addresses resolved for %q", host)
}

// localIPForTarget finds the local source IP that would be used to reach the target.
func localIPForTarget(targetIP string) (string, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(targetIP, "80"))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return "", fmt.Errorf("unexpected local address %T", conn.LocalAddr())
	}
	return addr.IP.String(), nil
}

func isRecoverableVNCError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "close sent")
}
