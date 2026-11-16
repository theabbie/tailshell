# Listing tailshell in OpenAI's plugin directory

Status: **not submitted.** This tracks OpenAI's [submission requirements](https://developers.openai.com/plugins/deploy/submission) and [plugin guidelines](https://developers.openai.com/plugins/plugin-guidelines) against what exists in this repository.

## Done

| Requirement | Where |
|---|---|
| Plugin package: `plugin.json`, `mcp.json`, `assets/`, onboarding skill | `plugin/` |
| Names, short and long descriptions, default prompts within the stated limits | `plugin/plugin.json` |
| Square logo and composer icon, PNG at 512 and 192 px plus SVG | `plugin/assets/` |
| Public HTTPS pages: website, support, privacy policy, terms | `/`, `/support`, `/privacy`, `/terms` |
| Remote streamable-HTTP MCP endpoint with OAuth (dynamic client registration, client ID metadata documents, PKCE) | `/mcp`, `oauthx/` |
| Domain verification file | `/.well-known/openai-apps-challenge`, served from the `OPENAI_APPS_CHALLENGE` setting |
| Explicit `readOnlyHint`, `destructiveHint` and `openWorldHint` on every tool | `phone/phone.go` |
| Tool descriptions that state side effects and privacy cautions | `phone/phone.go` |
| Inputs limited to what each tool needs; no conversation history requested | `phone/phone.go` |
| Permissions listed on the consent screen | `/oauth/authorize` |
| Errors that say what went wrong and what to do | `phone/phone.go`, `tailx/` |
| No credentials or secrets in the package | `plugin/` |

Annotation choices: tap, swipe, type, key and shell are marked destructive because they can trigger irreversible actions; opening an app or a link is not. Only `phone_open_url` is open-world.

## Needs the publisher

- A verified developer identity in the OpenAI platform dashboard, and a category from its list.
- Set `OPENAI_APPS_CHALLENGE` to the token the dashboard shows, then redeploy.
- A dedicated review setup (below), release notes, a demo recording, and the commerce declaration (none).
- Upload the plugin archive from the latest [release](https://github.com/theabbie/tailshell/releases) (`tailshell-plugin-<version>.zip`, built with `cd plugin && zip -r ../tailshell-plugin-1.0.0.zip . -x '.*' -x '*/.*'`, manifest at the archive root, no secrets, no `.app.json`) and submit. The portal reads the package details from it and shows them read-only; change them in `plugin/plugin.json` and upload a new version. Uploading it as a private plugin in ChatGPT does not connect (no registered connection), so it is only for the portal.
- `interface.category` must be a category title from the dashboard. It is set to `Productivity`; change it if the dashboard lists a better fit.

## Where the guidelines and this project pull apart

Left as they are on purpose. A reviewer may object to any of them.

- **`phone_shell` is a generic executor.** The guidelines say operations should be individually exposed.
- **The pairing page asks for a Tailscale auth key and a pairing code.** The guidelines list authentication secrets as data a plugin must not collect. They are used once, never stored, and only entered on tailshell's own page, never in chat.
- **Screens can contain anything**, including messages, one-time codes and health data. Instructions and descriptions tell agents to treat the screen as private.
- **Setup is heavy**: a Tailscale account, Android developer options and wireless debugging.
- **Whoever runs the listed deployment holds the encryption secret** and can decrypt the sessions it issued.

## Review setup

Reviewers need something that works without an account sign-up, MFA, or private-network access. Suggested: a dedicated Android phone or emulator on its own tailnet, kept awake and unlocked, with a prepared session. Give the reviewer that session token and tell them to paste it under **Already set up?** on the authorization page. Enter it in the portal's secure review-details form, never in this repository.

## Test cases

The five positive and three negative cases are part of the package, in `plugin/plugin.json` under `extensions.com.openai.review`; the portal reads them from the uploaded ZIP. Edit them there and upload a new version.

Entered in the dashboard (not in the ZIP): the demo video link, the release notes and the reviewer credentials. For tailshell the reviewer credential is a session token for a dedicated test phone, used under "Already set up?" on the authorization page. Never commit that token.
