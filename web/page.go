package web

const Page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>tailshell</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<style>
:root{
  --bg:#09090b; --surface:#18181b; --border:#27272a; --border-hi:#3f3f46;
  --text:#fafafa; --muted:#a1a1aa; --subtle:#71717a; --input:#09090b;
  --btn:#fafafa; --btn-text:#18181b; --btn-hover:#e4e4e7; --sel:#27272a;
}
@media (prefers-color-scheme: light){
  :root{
    --bg:#fafafa; --surface:#ffffff; --border:#e4e4e7; --border-hi:#a1a1aa;
    --text:#09090b; --muted:#52525b; --subtle:#71717a; --input:#fafafa;
    --btn:#18181b; --btn-text:#fafafa; --btn-hover:#3f3f46; --sel:#f4f4f5;
  }
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:14px/1.5 -apple-system,BlinkMacSystemFont,"Geist","Inter","Segoe UI",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
main{max-width:600px;margin:0 auto;padding:56px 20px 80px}
h1{font-size:20px;font-weight:600;letter-spacing:-.01em;margin:0}
.mark{vertical-align:-6px;margin-right:9px;border-radius:7px}
.top{display:flex;align-items:center;justify-content:space-between;margin:0 0 4px}
.gh{display:inline-flex;align-items:center;gap:6px;color:var(--muted);text-decoration:none;font-size:13px;font-weight:500;border:1px solid var(--border);border-radius:6px;padding:5px 10px}
.gh:hover{color:var(--text);border-color:var(--border-hi)}
.gh svg{width:16px;height:16px;flex:none}
.foot{margin-top:28px;text-align:center;font-size:12px;color:var(--subtle)}
.foot a{color:var(--muted);text-decoration:none;display:inline-flex;align-items:center;gap:4px;vertical-align:bottom}
.foot a:hover{color:var(--text)}
.foot svg{width:12px;height:12px}
.sub{color:var(--muted);margin:0 0 32px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:8px;padding:20px;margin-bottom:16px}
.card h2{font-size:14px;font-weight:600;margin:0 0 4px}
.card p{margin:0 0 14px;color:var(--muted);font-size:13px}
label{display:block;font-size:13px;font-weight:500;margin:14px 0 6px}
input,textarea,select{width:100%;background:var(--input);color:var(--text);border:1px solid var(--border);border-radius:6px;padding:8px 10px;font:inherit;outline:none}
input:focus,textarea:focus,select:focus{border-color:var(--border-hi);box-shadow:0 0 0 3px color-mix(in srgb,var(--border-hi) 30%,transparent)}
select{appearance:none;-webkit-appearance:none;-moz-appearance:none;padding-right:36px;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%23a1a1aa' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");background-repeat:no-repeat;background-position:right 12px center;background-size:16px 16px;cursor:pointer}
@media (prefers-color-scheme: light){select{background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2352525b' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E")}}
textarea{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;resize:vertical}
.hint{font-size:12px;color:var(--subtle);margin-top:6px}
button{font:inherit;font-weight:500;border-radius:6px;padding:8px 14px;cursor:pointer;border:1px solid var(--border);background:transparent;color:var(--text)}
button:hover{border-color:var(--border-hi)}
button.primary{background:var(--btn);color:var(--btn-text);border-color:var(--btn)}
button.primary:hover{background:var(--btn-hover);border-color:var(--btn-hover)}
button:disabled{opacity:.5;cursor:default}
.row{display:flex;gap:8px;align-items:center;margin-top:16px}
.dev{display:flex;align-items:center;gap:12px;width:100%;text-align:left;padding:10px 12px;margin:0 0 6px;border:1px solid var(--border);border-radius:6px;background:transparent}
.dev:hover{border-color:var(--border-hi)}
.dev.on{border-color:var(--text);background:var(--sel)}
.dot{width:8px;height:8px;border-radius:50%;flex:none;border:1px solid var(--subtle)}
.dot.up{background:var(--text);border-color:var(--text)}
.dev .n{font-weight:500}
.dev .m{color:var(--subtle);font-size:12px;margin-left:auto;font-family:ui-monospace,Menlo,monospace}
.err{margin-top:12px;padding:8px 10px;border:1px solid var(--border-hi);border-radius:6px;font-size:13px;color:var(--text);white-space:pre-wrap;word-break:break-word}
.warn{margin-top:12px;font-size:12px;color:var(--muted)}
.url{white-space:pre-wrap;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;word-break:break-all;background:var(--input);border:1px solid var(--border);border-radius:6px;padding:10px;user-select:all}
code{font-family:ui-monospace,Menlo,monospace;font-size:12px;background:var(--input);border:1px solid var(--border);border-radius:4px;padding:1px 5px}
details{margin-top:14px}
summary{cursor:pointer;color:var(--muted);font-size:13px}
.hide{display:none}
.steps{margin:0;padding-left:18px;color:var(--muted);font-size:13px}
</style>
</head>
<body>
<main>
  <header class="top">
    <h1><img class="mark" src="https://raw.githubusercontent.com/theabbie/tailshell/master/plugin/assets/icon.png" height="25" alt="">tailshell</h1>
    <a class="gh" href="https://github.com/theabbie/tailshell" target="_blank" rel="noopener noreferrer" aria-label="tailshell on GitHub"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4"/><path d="M9 18c-4.51 2-5-2-7-2"/></svg><span>GitHub</span></a>
  </header>
  <p class="sub">Pair an Android phone over your tailnet and get an MCP URL for it.</p>

  <div class="card" id="setup-card">
    <h2>Server</h2>
    <p id="setup-msg">Checking&hellip;</p>
  </div>

  <div class="card hide" id="oauth-card">
    <h2>Authorize access</h2>
    <p id="oauth-msg"></p>
    <div class="row"><button id="oauth-cancel">Cancel</button></div>
  </div>

  <div class="card hide" id="existing-card">
    <h2>Already set up?</h2>
    <p>Paste a tailshell MCP URL, HTTP URL or token you already have, and it is handed back as is. No pairing needed.</p>
    <label for="existing">URL or token</label>
    <input id="existing" type="password" spellcheck="false" autocomplete="off">
    <label for="existing-port">New connect port <span class="hint">(only if wireless debugging was toggled)</span></label>
    <input id="existing-port" inputmode="numeric" placeholder="optional">
    <div class="row"><button class="primary" id="existing-go">Authorize with this</button><span class="hint" id="existing-hint"></span></div>
    <div class="err hide" id="existing-err"></div>
  </div>

  <div class="card" id="join-card">
    <h2>1. Join your tailnet</h2>
    <p>Create a one-time auth key in Tailscale (Settings &rarr; Keys). It is sent once to add a node named <code>tailshell</code> to your tailnet and is never stored.</p>
    <label for="authkey">Tailscale auth key</label>
    <input id="authkey" type="password" autocomplete="off" spellcheck="false" placeholder="tskey-auth-...">
    <details>
      <summary>Already have node state?</summary>
      <label for="statein">Node state</label>
      <textarea id="statein" rows="3" spellcheck="false" placeholder="Paste a node state from an earlier run to skip the auth key"></textarea>
    </details>
    <div class="row"><button class="primary" id="join">Connect</button><span class="hint" id="join-hint"></span></div>
    <div class="err hide" id="join-err"></div>
  </div>

  <div class="card hide" id="dev-card">
    <h2>2. Choose a device</h2>
    <p>Machines on your tailnet.</p>
    <div id="devs"></div>
    <details>
      <summary>Node state</summary>
      <p class="hint" style="margin:8px 0">This is the identity of the node created above. It is already embedded in the MCP URL you will get; keep it private.</p>
      <textarea id="stateout" rows="3" readonly spellcheck="false"></textarea>
    </details>
  </div>

  <div class="card hide" id="pair-card">
    <h2>3. Pair the phone</h2>
    <p id="pair-for"></p>
    <label for="dtype">Device type</label>
    <select id="dtype"><option value="android">Android phone (wireless debugging)</option><option value="other">Other</option></select>
    <div id="android-form">
      <ol class="steps" style="margin-top:12px">
        <li>On the phone: Settings &rarr; Developer options &rarr; Wireless debugging &rarr; on.</li>
        <li>Note the <b>IP address &amp; Port</b> on that screen (the connect port).</li>
        <li>Tap <b>Pair device with pairing code</b> and note its port and code.</li>
      </ol>
      <label for="cport">Connect port</label>
      <input id="cport" inputmode="numeric" placeholder="e.g. 45437">
      <label for="pport">Pairing port</label>
      <input id="pport" inputmode="numeric" placeholder="e.g. 34083">
      <label for="pcode">Pairing code</label>
      <input id="pcode" inputmode="numeric" autocomplete="off" placeholder="6 digits">
      <details style="margin-top:14px">
        <summary>Keep it working without Wi-Fi (optional)</summary>
        <p class="hint" style="margin:8px 0">Wireless debugging only runs while the phone is on Wi-Fi. Turn <b>USB debugging</b> on too, then tick this: after pairing, tailshell switches the phone to adb over TCP on port 5555, which keeps working over mobile data through Tailscale. The setting resets when the phone restarts, and port 5555 is also open to other devices on the phone's networks (they still need your approval to connect).</p>
        <label style="display:flex;align-items:center;gap:8px;margin:0"><input type="checkbox" id="tcpip" style="width:auto;margin:0"> Switch to adb over TCP after pairing</label>
      </details>
      <div class="row"><button class="primary" id="pair">Pair and create MCP URL</button><span class="hint" id="pair-hint"></span></div>
    </div>
    <p class="hide" id="other-msg" style="margin-top:12px">Only Android phones are supported right now.</p>
    <div class="err hide" id="pair-err"></div>
  </div>

  <div class="card hide" id="out-card">
    <h2>MCP URL</h2>
    <p>Paste this into ChatGPT as an MCP server URL, with authentication set to none.</p>
    <div class="url" id="mcpurl"></div>
    <div class="row"><button id="copy">Copy</button><span class="hint" id="copy-hint"></span></div>
    <h2 style="margin-top:24px">HTTP API</h2>
    <p>The same tools over plain HTTP. <code>GET</code> the base URL to list endpoints; <code>/openapi.json</code> is an OpenAPI spec.</p>
    <div class="url" id="httpurl"></div>
    <div class="row"><button id="copyhttp">Copy</button><span class="hint" id="copyhttp-hint"></span></div>
    <p class="warn">Treat these URLs like passwords. They contain the phone's adb key and your tailnet node identity, so anyone who has one can control the phone while it is reachable.</p>
  </div>

  <div class="card" id="port-card">
    <details style="margin:0">
      <summary>Wireless debugging port changed?</summary>
      <p style="margin-top:10px">Turning wireless debugging off and on changes its port. The pairing stays valid; paste your MCP URL and the new connect port to get an updated URL.</p>
      <label for="oldurl">Current MCP URL</label>
      <input id="oldurl" spellcheck="false" autocomplete="off">
      <label for="newport">New connect port</label>
      <input id="newport" inputmode="numeric">
      <div class="row"><button id="upd">Update URL</button><span class="hint" id="upd-hint"></span></div>
      <div class="err hide" id="upd-err"></div>
      <div class="url hide" id="upd-url" style="margin-top:12px"></div>
    </details>
  </div>
  <footer class="foot">Open source &middot; <a href="https://github.com/theabbie/tailshell" target="_blank" rel="noopener noreferrer">View the code on GitHub <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg></a><br><a href="/privacy">Privacy</a> &middot; <a href="/terms">Terms</a> &middot; <a href="/support">Support</a></footer>
</main>
<script>
(function () {
  var $ = function (id) { return document.getElementById(id); };
  var state = "", chosen = null;
  var OAUTH = /*TS_OAUTH*/null;
  if (OAUTH) {
    $("oauth-msg").textContent = (OAUTH.name || "An app") + " (" + OAUTH.host + ") wants to use one of your Android phones through tailshell. It will be able to see your screen, tap, swipe, type, press keys, open apps and links, and run shell commands. Join your tailnet and pair the phone below, and you will be sent back to " + OAUTH.host + ". Only continue if you started this from that app.";
    show("oauth-card", true); show("existing-card", true); show("port-card", false);
    $("pair").textContent = "Pair and authorize";
    $("oauth-cancel").onclick = function () { window.location.assign(OAUTH.cancel); };
  }

  function show(id, on) { $(id).classList.toggle("hide", !on); }
  function err(id, msg) { var e = $(id); e.textContent = msg || ""; show(id, !!msg); }
  function api(path, body) {
    return fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) })
      .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || ("HTTP " + r.status)); return j; }); });
  }
  function busy(btn, hint, on, msg) { btn.disabled = on; $(hint).textContent = on ? msg : ""; }

  fetch("/api/status").then(function (r) { return r.json(); }).then(function (s) {
    if (s.sealSecretConfigured) { show("setup-card", false); return; }
    $("setup-msg").innerHTML = "Not configured. Set the <code>SEAL_SECRET</code> environment variable (a random string, e.g. <code>openssl rand -hex 32</code>) and redeploy.";
    $("join").disabled = true;
  }).catch(function () { show("setup-card", false); });

  function renderDevices(list) {
    var box = $("devs"); box.innerHTML = "";
    if (!list.length) { box.textContent = "No other devices found on this tailnet."; return; }
    list.forEach(function (d) {
      var b = document.createElement("button"); b.className = "dev"; b.type = "button";
      var dot = document.createElement("span"); dot.className = "dot" + (d.online ? " up" : ""); dot.title = d.online ? "online" : "offline";
      var n = document.createElement("span"); n.className = "n"; n.textContent = d.name;
      var m = document.createElement("span"); m.className = "m"; m.textContent = d.os + " · " + (d.ips[0] || "");
      b.appendChild(dot); b.appendChild(n); b.appendChild(m);
      b.onclick = function () {
        chosen = d;
        Array.prototype.forEach.call(box.children, function (c) { c.classList.remove("on"); });
        b.classList.add("on");
        $("pair-for").textContent = d.name + " (" + (d.dnsName || d.ips[0]) + ")";
        $("dtype").value = d.os === "android" ? "android" : "other";
        syncType(); show("pair-card", true); show("out-card", false);
      };
      box.appendChild(b);
    });
  }
  function syncType() {
    var android = $("dtype").value === "android";
    show("android-form", android); show("other-msg", !android);
  }
  $("dtype").onchange = syncType;

  $("join").onclick = function () {
    err("join-err", "");
    var st = $("statein").value.trim(), key = $("authkey").value.trim();
    if (!st && !key) { err("join-err", "Enter an auth key or paste node state."); return; }
    busy($("join"), "join-hint", true, "Joining the tailnet (can take ~10s)…");
    api("/api/connect", st ? { state: st } : { authKey: key }).then(function (r) {
      state = r.state; $("authkey").value = ""; $("stateout").value = state;
      renderDevices(r.devices); show("dev-card", true); show("pair-card", false); show("out-card", false);
    }).catch(function (e) { err("join-err", e.message); })
      .then(function () { busy($("join"), "join-hint", false); });
  };

  $("pair").onclick = function () {
    err("pair-err", "");
    if (!chosen) { err("pair-err", "Choose a device first."); return; }
    var body = {
      state: state, host: chosen.dnsName || chosen.ips[0],
      connectPort: parseInt($("cport").value, 10), pairPort: parseInt($("pport").value, 10), code: $("pcode").value.trim()
    };
    if (!body.connectPort || !body.pairPort || !body.code) { err("pair-err", "Fill in all three fields."); return; }
    if (OAUTH) body.oauth = OAUTH.ctx;
    if ($("tcpip").checked) body.tcpip = true;
    busy($("pair"), "pair-hint", true, body.tcpip ? "Pairing, then switching to TCP…" : "Pairing…");
    api("/api/pair", body).then(function (r) {
      if (r.redirect) { window.location.assign(r.redirect); return; }
      $("mcpurl").textContent = r.url; $("httpurl").textContent = r.httpUrl; show("out-card", true); $("pcode").value = "";
      if (r.warning) err("pair-err", r.warning);
      $("out-card").scrollIntoView({ behavior: "smooth", block: "nearest" });
    }).catch(function (e) { err("pair-err", e.message); })
      .then(function () { busy($("pair"), "pair-hint", false); });
  };

  function copier(btn, src, hint) {
    $(btn).onclick = function () {
      navigator.clipboard.writeText($(src).textContent).then(function () {
        $(hint).textContent = "Copied"; setTimeout(function () { $(hint).textContent = ""; }, 1500);
      });
    };
  }
  $("existing-go").onclick = function () {
    err("existing-err", "");
    var v = $("existing").value.trim();
    if (!v) { err("existing-err", "Paste a URL or token."); return; }
    var body = { oauth: OAUTH.ctx, token: v };
    var port = parseInt($("existing-port").value, 10);
    if (port) body.port = port;
    busy($("existing-go"), "existing-hint", true, "Checking\u2026");
    api("/api/authorize", body).then(function (r) { window.location.assign(r.redirect); })
      .catch(function (e) { err("existing-err", e.message); })
      .then(function () { busy($("existing-go"), "existing-hint", false); });
  };
  copier("copy", "mcpurl", "copy-hint");
  copier("copyhttp", "httpurl", "copyhttp-hint");

  $("upd").onclick = function () {
    err("upd-err", ""); show("upd-url", false);
    var port = parseInt($("newport").value, 10), url = $("oldurl").value.trim();
    if (!port || !url) { err("upd-err", "Enter the MCP URL and the new port."); return; }
    busy($("upd"), "upd-hint", true, "Reconnecting…");
    api("/api/port", { url: url, port: port }).then(function (r) {
      $("upd-url").textContent = "MCP:  " + r.url + "\nHTTP: " + r.httpUrl; show("upd-url", true);
    }).catch(function (e) { err("upd-err", e.message); })
      .then(function () { busy($("upd"), "upd-hint", false); });
  };
})();
</script>
</body>
</html>
`
