package web

import "strings"

const repo = "https://github.com/theabbie/tailshell"

const docShell = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{TITLE}} - tailshell</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<style>
:root{--bg:#09090b;--text:#fafafa;--muted:#a1a1aa;--border:#27272a;--link:#e4e4e7}
@media (prefers-color-scheme: light){:root{--bg:#fafafa;--text:#09090b;--muted:#52525b;--border:#e4e4e7;--link:#18181b}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.65 -apple-system,BlinkMacSystemFont,"Geist","Inter","Segoe UI",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
main{max-width:680px;margin:0 auto;padding:56px 20px 80px}
h1{font-size:22px;letter-spacing:-.01em;margin:0 0 4px}
h2{font-size:16px;margin:32px 0 6px}
p,li{color:var(--muted)}
ul{padding-left:20px}
a{color:var(--link)}
code{font-family:ui-monospace,Menlo,monospace;font-size:13px;border:1px solid var(--border);border-radius:4px;padding:1px 5px}
.mark{vertical-align:-6px;margin-right:9px;border-radius:6px}
.top{display:flex;justify-content:space-between;align-items:baseline;margin-bottom:24px}
.top a{font-size:13px;text-decoration:none;color:var(--muted)}
.upd{font-size:13px}
</style>
</head>
<body>
<main>
<div class="top"><h1><img class="mark" src="https://raw.githubusercontent.com/theabbie/tailshell/master/plugin/assets/icon.png" height="25" alt="">{{TITLE}}</h1><a href="/">&larr; tailshell</a></div>
{{BODY}}
</main>
</body>
</html>
`

func doc(title, body string) string {
	return strings.NewReplacer("{{TITLE}}", title, "{{BODY}}", body).Replace(docShell)
}

var Privacy = doc("Privacy policy", `
<p class="upd">Last updated: 15 November 2026. This is a plain-language description of what tailshell does with data. It describes the software as written; if you run your own copy, you are the operator of that copy.</p>

<h2>What tailshell is</h2>
<p>tailshell lets an AI assistant that you choose operate an Android phone that you own, over your own Tailscale network. It is open source (<a href="`+repo+`">`+repo+`</a>) and published by the repository owner, <code>theabbie</code>.</p>

<h2>Data it handles</h2>
<ul>
<li><b>Tailscale auth key.</b> You type it into the tailshell page once. It is used to add a node named <code>tailshell</code> to your tailnet and is not stored or logged by the application.</li>
<li><b>Pairing details.</b> The phone's address, the wireless-debugging ports and the six-digit pairing code are used once to pair the phone and are not stored.</li>
<li><b>Connection token.</b> After pairing, the connection (the phone's adb key, its address and port, and the Tailscale node identity) is encrypted into a token that is given to you or to your AI client. The application keeps no copy. Whoever operates the deployment holds the encryption secret and can decrypt tokens that deployment issued.</li>
<li><b>Phone content while in use.</b> When your AI client uses a tool, screen contents (screenshots and on-screen text), the commands it sends and their results pass through the server's memory between your AI client and your phone. The application does not store them.</li>
<li><b>Hosting logs.</b> The hosting provider (Vercel by default) keeps standard request logs such as time, IP address, path and status. If you connect with a secret URL, the token is part of the path and therefore appears in those logs; the OAuth connection sends it in a header instead.</li>
</ul>

<h2>Retention</h2>
<p>The application stores nothing. Data in memory is discarded when a request ends or the instance is recycled. Hosting logs follow the hosting provider's retention settings.</p>

<h2>Who else receives data</h2>
<ul>
<li><b>Your AI client</b> (for example ChatGPT, Claude or Mistral) receives whatever the tools return, including what is on your screen, and stores the connection token. Its own privacy policy applies to that data.</li>
<li><b>Tailscale</b> carries the connection between the server and your phone.</li>
<li><b>The hosting provider</b> runs the server and keeps request logs.</li>
<li><b>Google</b> serves the Android platform-tools download that tailshell fetches at runtime.</li>
<li><b>GitHub</b> serves the logo image shown at the top of these pages (from <code>raw.githubusercontent.com</code>), so it sees your IP address when a page loads.</li>
</ul>

<h2>What it does not do</h2>
<p>No analytics, advertising, tracking, profiling or sale of data. It is not directed at children under 13.</p>

<h2>Your controls</h2>
<ul>
<li>Remove the <code>tailshell</code> machine in the Tailscale admin console to cut the route to your phone.</li>
<li>Forget the paired device under Wireless debugging on the phone to revoke the adb key.</li>
<li>Turn wireless debugging off when you do not need it.</li>
<li>The operator can invalidate every token by changing the encryption secret.</li>
</ul>

<h2>Contact</h2>
<p>Questions or requests: <a href="`+repo+`/issues">`+repo+`/issues</a>. Please do not post URLs, tokens, keys or screenshots from your phone there.</p>
`)

var Terms = doc("Terms of use", `
<p class="upd">Last updated: 15 November 2026.</p>

<h2>Acceptable use</h2>
<ul>
<li>Use tailshell only with devices, accounts and networks that you own or are explicitly authorized to control.</li>
<li>Do not use it to access, monitor or track another person's device without their knowledge and consent, to bypass security controls, or for anything unlawful.</li>
<li>Follow the terms of the services you connect: Tailscale, Google, your AI provider and the apps on your phone.</li>
</ul>

<h2>Your responsibilities</h2>
<ul>
<li>A tailshell URL or token gives full control of your phone to whoever holds it. Keep it secret and revoke it if it may have leaked (see the <a href="`+repo+`#safeguard-your-urls">safeguarding guidance</a>).</li>
<li>An AI agent acts on your phone on your behalf and can make mistakes. You are responsible for what it does, including messages sent, purchases made and data changed or deleted. Review what you allow it to do.</li>
<li>Do not use tailshell for anything where a mistaken action could endanger people or property.</li>
</ul>

<h2>No warranty</h2>
<p>tailshell is provided as is, without warranty of any kind. To the extent the law allows, the publisher is not liable for any loss or damage arising from its use. It may change or stop working at any time.</p>

<h2>Open source</h2>
<p>tailshell is released under the MIT license (<a href="`+repo+`/blob/master/LICENSE">LICENSE</a>). These terms cover use of a deployment of it, and do not change the license of the code.</p>

<h2>Not affiliated</h2>
<p>tailshell is an independent project and is not affiliated with or endorsed by Tailscale, Google, Android, Vercel, OpenAI, Anthropic, Mistral or any other company named in its documentation.</p>

<h2>Contact</h2>
<p><a href="`+repo+`/issues">`+repo+`/issues</a></p>
`)

var Support = doc("Support", `
<p>tailshell is an open-source project. Help is handled in the public issue tracker.</p>

<h2>Get help</h2>
<ul>
<li>Setup and usage: the <a href="`+repo+`#readme">README</a> walks through Tailscale, wireless debugging, deployment, pairing and connecting an AI client.</li>
<li>Something broke: see <a href="`+repo+`#when-something-breaks">When something breaks</a>. Most problems are a changed wireless-debugging port, a sleeping phone or a removed Tailscale machine.</li>
<li>Bugs and questions: <a href="`+repo+`/issues">open an issue</a>.</li>
</ul>

<h2>When you report a problem</h2>
<p>Describe what you did and what you saw. <b>Never include</b> a tailshell URL or token, a Tailscale auth key, a pairing code, or screenshots of your phone, since any of these can expose your device or accounts.</p>
`)
