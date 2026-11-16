package phone

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tailshell/adbx"
)

type Tool struct {
	Name        string
	Title       string
	Description string
	Schema      map[string]any
	ReadOnly    bool
	Destructive bool
	OpenWorld   bool
}

type Result struct {
	Text    string
	Image   []byte
	Data    any
	IsError bool
	Status  int
	Reauth  bool
}

type Element struct {
	Index      int    `json:"index"`
	Text       string `json:"text,omitempty"`
	Desc       string `json:"description,omitempty"`
	Kind       string `json:"kind"`
	Clickable  bool   `json:"clickable,omitempty"`
	Scrollable bool   `json:"scrollable,omitempty"`
	Editable   bool   `json:"editable,omitempty"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }

var Tools = []Tool{
	{Name: "phone_screenshot", Title: "Screenshot", ReadOnly: true, Schema: schema(map[string]any{}),
		Description: "Capture the phone screen as a PNG image. Use it to see what is on screen before acting and again after acting to verify the result. Pixel positions in the image are the same coordinates phone_tap and phone_swipe use. Secure screens (some banking apps, password fields) may come out black. Prefer phone_ui when you need exact tap targets or to read text. The screen may show private data (messages, passwords, one-time codes, payment or health details): look only at what the request needs and never repeat or forward secrets."},
	{Name: "phone_ui", Title: "Read screen elements", ReadOnly: true, Schema: schema(map[string]any{}),
		Description: "List the elements currently on screen as text, with the center x,y to tap for each, from Android's accessibility tree. Each line: [n] \"text\" (content description) kind, clickable/scrollable/editable flags, center=(x,y). Find the element by its text or description, then phone_tap its center. Only elements with text, a description, or that are clickable are listed. Empty or sparse for games, some WebViews and secure screens: fall back to phone_screenshot. The screen may show private data (messages, passwords, one-time codes, payment or health details): read only what the request needs and never repeat or forward secrets."},
	{Name: "phone_info", Title: "Phone info", ReadOnly: true, Schema: schema(map[string]any{}),
		Description: "Phone model, Android version, screen size in pixels (width x height, portrait) and the window currently in focus. Call this first; the screen size defines the coordinate space for tap and swipe."},
	{Name: "phone_tap", Destructive: true, OpenWorld: true, Title: "Tap",
		Schema:      schema(map[string]any{"x": num("x pixel, 0 = left edge"), "y": num("y pixel, 0 = top edge")}, "x", "y"),
		Description: "Tap the screen at pixel (x, y). Use the center coordinates from phone_ui or positions read off a phone_screenshot. To type into a field, tap it first, then call phone_type. Side effect: it presses whatever is at that spot, which can send, delete or buy if that is what is there, so only tap what the user asked for."},
	{Name: "phone_swipe", Destructive: true, OpenWorld: true, Title: "Swipe",
		Schema:      schema(map[string]any{"x1": num("start x"), "y1": num("start y"), "x2": num("end x"), "y2": num("end y"), "duration_ms": num("duration in milliseconds, default 300")}, "x1", "y1", "x2", "y2"),
		Description: "Swipe from (x1,y1) to (x2,y2). To scroll content DOWN (see what is below), swipe UP: e.g. x1=540,y1=1700 to x2=540,y2=700 on a 1080x2400 screen. To scroll up, swipe down. Use a longer duration_ms (e.g. 800) for a slow drag; the default 300 is a flick. Side effect: scrolls or drags, and a swipe can also dismiss or reorder items."},
	{Name: "phone_type", Destructive: true, OpenWorld: true, Title: "Type text",
		Schema:      schema(map[string]any{"text": str("text to type")}, "text"),
		Description: "Type text into the focused text field (tap the field first). ASCII only: emoji and non-Latin characters are not supported. Does not press Enter: call phone_key with ENTER to submit or search. Side effect: the text lands in whatever field has focus, which may be a message or a form."},
	{Name: "phone_key", Destructive: true, OpenWorld: true, Title: "Press key",
		Schema:      schema(map[string]any{"key": str("key name such as HOME, or a numeric keycode")}, "key"),
		Description: "Press a hardware/navigation key. Common: HOME, BACK, APP_SWITCH (recent apps), ENTER, DEL (backspace), TAB, WAKEUP (wake the screen), SLEEP (screen off), POWER (toggles the screen), VOLUME_UP, VOLUME_DOWN, MENU, DPAD_UP/DOWN/LEFT/RIGHT/CENTER. The KEYCODE_ prefix is optional; a numeric Android keycode also works. Side effect: HOME and BACK change what is on screen, and POWER can turn the screen off."},
	{Name: "phone_open_app", OpenWorld: true, Title: "Open app",
		Schema:      schema(map[string]any{"package": str("Android package name")}, "package"),
		Description: "Launch an app by Android package name, e.g. com.google.android.gm (Gmail), com.android.chrome, com.whatsapp, com.google.android.apps.maps, com.android.settings. If you do not know the package name, use phone_list_apps with a filter. Side effect: launches the app and brings it to the foreground."},
	{Name: "phone_open_url", OpenWorld: true, Title: "Open URL",
		Schema:      schema(map[string]any{"url": str("https:// or http:// URL")}, "url"),
		Description: "Open an http(s) URL in the phone's default browser or the app registered for that link. Only http and https are accepted. Side effect: opens the page in the phone's browser, and pages can act on their own once loaded."},
	{Name: "phone_list_apps", Title: "List installed apps", ReadOnly: true,
		Schema:      schema(map[string]any{"filter": str("optional substring to match")}),
		Description: "List the package names of user-installed apps, optionally filtered by a case-insensitive substring (e.g. \"google\", \"bank\", \"maps\"). Use it to find the package name for phone_open_app."},
	{Name: "phone_shell", Destructive: true, OpenWorld: true, Title: "Shell command",
		Schema:      schema(map[string]any{"command": str("shell command to run")}, "command"),
		Description: "Run a command in the phone's adb shell (limited shell user, not root). A last resort for things the other tools cannot do, e.g. dumpsys battery, settings get global airplane_mode_on, am start ... . 45s timeout, output capped at 60 KB, no interactive programs. Do not run destructive commands (rm, pm uninstall/clear, settings put, reboot) unless the user explicitly asked. Side effect: whatever the command does to the phone and its files."},
}

var (
	pkgRe    = regexp.MustCompile(`^[A-Za-z0-9_.]{1,200}$`)
	keyRe    = regexp.MustCompile(`^[A-Za-z0-9_]{1,40}$`)
	urlRe    = regexp.MustCompile(`^https?://[^\s'"` + "`" + `$;&|<>\\]{1,1000}$`)
	filterRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{0,60}$`)
	boundsRe = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
)

func fail(status int, msg string) Result { return Result{Text: msg, IsError: true, Status: status} }

func Find(name string) (Tool, bool) {
	for _, t := range Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

func Call(ctx context.Context, token string, resolve adbx.DialResolver, name string, a map[string]any) Result {
	if _, ok := Find(name); !ok {
		return fail(404, "unknown tool: "+name)
	}
	if a == nil {
		a = map[string]any{}
	}
	geti := func(k string) (int, bool) {
		v, ok := a[k].(float64)
		return int(v), ok
	}
	gets := func(k string) string {
		s, _ := a[k].(string)
		return s
	}
	switch name {
	case "phone_tap":
		if _, ok := geti("x"); !ok {
			return fail(400, "x and y are required integers")
		}
		if _, ok := geti("y"); !ok {
			return fail(400, "x and y are required integers")
		}
	case "phone_swipe":
		for _, k := range []string{"x1", "y1", "x2", "y2"} {
			if _, ok := geti(k); !ok {
				return fail(400, "x1, y1, x2, y2 are required integers")
			}
		}
	case "phone_type":
		if gets("text") == "" {
			return fail(400, "text is required")
		}
	case "phone_key":
		if !keyRe.MatchString(gets("key")) {
			return fail(400, "key must be a key name like HOME or a numeric keycode")
		}
	case "phone_open_app":
		if !pkgRe.MatchString(gets("package")) {
			return fail(400, "package must look like com.example.app")
		}
	case "phone_open_url":
		if !urlRe.MatchString(gets("url")) {
			return fail(400, "url must be a plain http(s) URL")
		}
	case "phone_list_apps":
		if !filterRe.MatchString(gets("filter")) {
			return fail(400, "filter must be a short plain substring")
		}
	case "phone_shell":
		if strings.TrimSpace(gets("command")) == "" {
			return fail(400, "command is required")
		}
	}

	c, err := adbx.Attach(ctx, token, resolve)
	if err != nil {
		if errors.Is(err, adbx.ErrReauth) {
			return Result{Text: err.Error(), IsError: true, Status: 401, Reauth: true}
		}
		return fail(502, "cannot reach the phone: "+err.Error()+". The phone may be asleep or offline in Tailscale, wireless debugging may be off, or its port may have changed: ask the user to check those and, if the port changed, update the URL on the connection page.")
	}
	sh := func(cmd string) (string, Result) {
		out, err := c.Shell(ctx, cmd)
		out = strings.TrimSpace(out)
		if len(out) > 60000 {
			out = out[:60000] + "\n…(truncated)"
		}
		if err != nil {
			return out, Result{Text: out + "\n(error: " + err.Error() + ")", IsError: true}
		}
		if out == "" {
			out = "(no output)"
		}
		return out, Result{Text: out}
	}

	switch name {
	case "phone_screenshot":
		png, err := c.Screenshot(ctx)
		if err != nil {
			return fail(502, err.Error())
		}
		if len(png) > 2_800_000 {
			return fail(502, fmt.Sprintf("screenshot is %d bytes, too large to return", len(png)))
		}
		return Result{Image: png, Text: "screenshot"}
	case "phone_ui":
		els, err := dumpUI(ctx, c)
		if err != nil {
			return fail(502, err.Error())
		}
		return Result{Text: formatElements(els), Data: map[string]any{"elements": els}}
	case "phone_info":
		out, r := sh(`echo "model=$(getprop ro.product.model)"; echo "android=$(getprop ro.build.version.release)"; echo "size=$(wm size | tail -1 | sed 's/.*: //')"; echo "focus=$(dumpsys window | grep mCurrentFocus | tail -1 | sed 's/^[^{]*{[^ ]* [^ ]* //; s/}$//')"`)
		if r.IsError {
			return r
		}
		info := map[string]string{}
		for _, l := range strings.Split(out, "\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				info[k] = strings.TrimSpace(v)
			}
		}
		return Result{
			Text: fmt.Sprintf("model: %s\nandroid: %s\nscreen: %s\nforeground: %s", info["model"], info["android"], info["size"], info["focus"]),
			Data: info,
		}
	case "phone_tap":
		x, _ := geti("x")
		y, _ := geti("y")
		_, r := sh(fmt.Sprintf("input tap %d %d && echo tapped", x, y))
		return r
	case "phone_swipe":
		x1, _ := geti("x1")
		y1, _ := geti("y1")
		x2, _ := geti("x2")
		y2, _ := geti("y2")
		d, ok := geti("duration_ms")
		if !ok || d <= 0 {
			d = 300
		}
		_, r := sh(fmt.Sprintf("input swipe %d %d %d %d %d && echo swiped", x1, y1, x2, y2, d))
		return r
	case "phone_type":
		esc := strings.ReplaceAll(strings.ReplaceAll(gets("text"), " ", "%s"), "'", `'\''`)
		_, r := sh("input text '" + esc + "' && echo typed")
		return r
	case "phone_key":
		k := strings.ToUpper(gets("key"))
		if strings.Trim(k, "0123456789") != "" && !strings.HasPrefix(k, "KEYCODE_") {
			k = "KEYCODE_" + k
		}
		_, r := sh("input keyevent " + k + " && echo pressed " + k)
		return r
	case "phone_open_app":
		_, r := sh("monkey -p " + gets("package") + " -c android.intent.category.LAUNCHER 1 2>&1 | grep -E 'Events injected|No activities|aborted' | head -3")
		return r
	case "phone_open_url":
		_, r := sh("am start -a android.intent.action.VIEW -d '" + gets("url") + "' 2>&1 | tail -2")
		return r
	case "phone_list_apps":
		cmd := "cmd package list packages -3 2>/dev/null | sed 's/^package://' | sort"
		if f := gets("filter"); f != "" {
			cmd += " | grep -i -F '" + f + "'"
		}
		out, r := sh(cmd + " | head -200")
		if !r.IsError {
			r.Data = map[string]any{"packages": strings.Fields(out)}
		}
		return r
	default:
		_, r := sh(gets("command"))
		return r
	}
}

type uiNode struct {
	Text      string   `xml:"text,attr"`
	Desc      string   `xml:"content-desc,attr"`
	Class     string   `xml:"class,attr"`
	Clickable string   `xml:"clickable,attr"`
	Scroll    string   `xml:"scrollable,attr"`
	Password  string   `xml:"password,attr"`
	Enabled   string   `xml:"enabled,attr"`
	Bounds    string   `xml:"bounds,attr"`
	Children  []uiNode `xml:"node"`
}

type uiRoot struct {
	Children []uiNode `xml:"node"`
}

func extractXML(s string) string {
	i := strings.Index(s, "<?xml")
	if i < 0 {
		i = strings.Index(s, "<hierarchy")
	}
	j := strings.LastIndex(s, "</hierarchy>")
	if i < 0 || j < 0 {
		return ""
	}
	return s[i : j+len("</hierarchy>")]
}

func dumpUI(ctx context.Context, c *adbx.Client) ([]Element, error) {
	out, err := c.Run(ctx, 30*time.Second, "exec-out", "uiautomator", "dump", "/dev/tty")
	xmlText := extractXML(string(out))
	if err != nil || xmlText == "" {
		if _, err := c.Run(ctx, 30*time.Second, "shell", "uiautomator dump /sdcard/.tailshell_ui.xml >/dev/null 2>&1"); err != nil {
			return nil, err
		}
		out, err = c.Run(ctx, 15*time.Second, "exec-out", "cat", "/sdcard/.tailshell_ui.xml")
		c.Run(ctx, 10*time.Second, "shell", "rm -f /sdcard/.tailshell_ui.xml")
		if xmlText = extractXML(string(out)); err != nil || xmlText == "" {
			return nil, fmt.Errorf("could not read the screen (secure screen, or the screen is off/locked): %s", strings.TrimSpace(string(out)))
		}
	}
	var root uiRoot
	if err := xml.Unmarshal([]byte(xmlText), &root); err != nil {
		return nil, err
	}
	els := []Element{}
	var walk func(n uiNode)
	walk = func(n uiNode) {
		if m := boundsRe.FindStringSubmatch(n.Bounds); m != nil {
			var l, t, r, b int
			fmt.Sscanf(m[1]+" "+m[2]+" "+m[3]+" "+m[4], "%d %d %d %d", &l, &t, &r, &b)
			text, desc := strings.TrimSpace(n.Text), strings.TrimSpace(n.Desc)
			clickable := n.Clickable == "true" && n.Enabled != "false"
			if (text != "" || desc != "" || clickable) && r > l && b > t {
				kind := n.Class[strings.LastIndex(n.Class, ".")+1:]
				if n.Password == "true" {
					text = "(password field)"
				}
				els = append(els, Element{
					Index: len(els), Text: text, Desc: desc, Kind: kind,
					Clickable: clickable, Scrollable: n.Scroll == "true", Editable: strings.Contains(kind, "EditText"),
					X: (l + r) / 2, Y: (t + b) / 2,
				})
			}
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	for _, n := range root.Children {
		walk(n)
	}
	return els, nil
}

func formatElements(els []Element) string {
	if len(els) == 0 {
		return "No elements with text or actions on screen (blank, a game, a secure screen, or the screen is off). Try phone_screenshot."
	}
	var lines []string
	for i, e := range els {
		if i == 200 {
			lines = append(lines, "…(more elements omitted; scroll and read again)")
			break
		}
		line := fmt.Sprintf("[%d]", e.Index)
		if e.Text != "" {
			line += fmt.Sprintf(" %q", e.Text)
		}
		if e.Desc != "" {
			line += fmt.Sprintf(" (%s)", e.Desc)
		}
		line += " " + e.Kind
		var flags []string
		if e.Clickable {
			flags = append(flags, "clickable")
		}
		if e.Scrollable {
			flags = append(flags, "scrollable")
		}
		if e.Editable {
			flags = append(flags, "editable")
		}
		if len(flags) > 0 {
			line += " " + strings.Join(flags, ",")
		}
		lines = append(lines, line+fmt.Sprintf(" center=(%d,%d)", e.X, e.Y))
	}
	return strings.Join(lines, "\n")
}
