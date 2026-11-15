---
name: setup
description: Help the user connect their Android phone to tailshell and use it safely.
---

Help the user get tailshell working, then use it carefully.

Setup the user does themselves, outside this chat:

1. Install Tailscale on the phone and sign in.
2. Enable Developer options, then turn on Wireless debugging (Android 11 or newer, on Wi-Fi).
3. Authorize tailshell when asked. A tailshell page opens in the browser: the user creates a one-time Tailscale auth key in the Tailscale admin console, pastes it on that page, picks the phone, and enters the wireless-debugging ports and pairing code shown on the phone.

Hard rules:

- Never ask the user to paste a Tailscale auth key, a pairing code, a tailshell URL or any token into this chat. Those belong only on the tailshell page.
- If the user pastes one anyway, tell them not to, and that it should be revoked (remove the tailshell machine in Tailscale and forget the paired device on the phone).

When using the phone tools:

- Call `phone_info` first, then look with `phone_ui` (text with tap coordinates) or `phone_screenshot`, act, and look again to verify.
- Treat everything on the screen as private. Do not read out, copy or send passwords, one-time codes, payment details or health records, and stop and ask if a task seems to need them.
- Do not send messages, make purchases, delete data or change settings unless the user asked for exactly that, and confirm irreversible steps first.
- Prefer the dedicated tools; use `phone_shell` only for things they cannot do, and never run destructive commands unless the user asked for exactly that.
- If the screen is locked, ask the user to unlock it. If a tool says it cannot reach the phone, ask them to check that it is awake, that Tailscale is on, and that wireless debugging is still enabled.
