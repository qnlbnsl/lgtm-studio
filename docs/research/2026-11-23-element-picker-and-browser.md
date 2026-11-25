# How a browser inside a coding tool lets the owner pick an element

The owner asked on 2026-11-23 for a tool on the Browser page that picks an element of the running product and puts it in a chat box.
The owner asked how Cursor does it, and whether the same can work on a website.
This file holds what was read, what follows for LGTM Studio, and what the owner decided.
Nothing here was tested in LGTM Studio. Each claim about another product comes from the page named beside it.

## What Cursor does

Cursor is a desktop program, and it carries its own browser.

- Its documentation says: "Browser runs as a secure web view and is controlled using an MCP server running as an extension." It also says: "The browser opens as a pane within Cursor". Source: [Cursor documentation, Browser](https://cursor.com/docs/agent/tools/browser).
- Because the program owns the browser, it can read any page that the browser shows. The page does not have to help.
- A second desktop tool is built the same way. Codex shows its browser as an Electron web view and drives it with the Chrome DevTools Protocol. Source: [How Codex's in-app browser works](https://csarko.sh/research/how-codex-embeds-its-browser).

## What Cursor sends to the agent for a picked element

Source: [Cursor documentation, Design Mode](https://cursor.com/docs/agent/design-mode).

- "For each picked element, the agent gets the tag, id, classes, attributes, a CSS-style selector path, the first 200 characters of visible text, and the element's size and position."
- "It also gets six computed styles: color, background color, font size, font family, display, and position."
- "In React, Vue, and Svelte apps, the details include the component name and component stack, so the agent can find the source."
- "Each note includes the details of what you picked and a screenshot, so the agent can find the right code." The screenshot is cropped to the element.

How the owner picks, in Cursor:

- A click selects an element and opens a note.
- Shift and a click adds more elements to the same note.
- The Up and Down arrow keys move the highlight to the parent or the child, before the click.
- Alt and a click adds an element to the chat input.
- A drag marks an area. Escape leaves.

This list is a good first design for what LGTM Studio sends and how its picker behaves.

## Why a website cannot do what Cursor does

- The website shows the product in a frame. A frame from another origin cannot be read by the page around it. This is the browser's same-origin rule.
- The owner concluded that whatever renders both the website and the product's page has to be owned by LGTM Studio. That is right. The rule that stands in the way is the same-origin rule more than a content security policy, but the conclusion is the same.
- [SECURITY.md](../../SECURITY.md) also wants the frame closed: a product's page is untrusted, so the frame has a sandbox without `allow-same-origin`.
- The website is served over HTTPS. A browser refuses to show a plain `http://` address in a frame of an HTTPS page, unless the address is the viewer's own computer. So a frame cannot point straight at `http://192.0.2.20:5173` from a laptop. Either the server passes the page on, or the thing that shows the page is not an HTTPS web page.

## Four ways to get a picker

| Way | Works on any page | What it needs | What is weak |
|---|---|---|---|
| A browser extension beside the website | Yes | The owner installs it in each browser | It does not exist on a phone's browser |
| A desktop client that owns its web view | Yes | A program to install, such as one built with Electron or Tauri | It is one more program to build and update |
| A small script inside the product, in development | No, only a product whose code the agents own | One script in the project, and a server that passes the page on | A page the owner does not control cannot be picked from |
| A browser on the server, sent as pictures | Yes | A browser on a machine, and a stream | It is heavy, and it lags |

- The script way is proven outside LGTM Studio. React Grab is one script in development, and it copies the element with its component and source file: "[<a class=...>Forgot your password?</a> in LoginForm (at components/login-form.tsx:46:19)]". It is released under the MIT License. Source: [React Grab](https://github.com/aidenybai/react-grab). No code of it is copied into LGTM Studio.
- A script inside a page cannot take a true picture of an element. An extension and a desktop client can.
- A server that passes a product on and adds a script to it is fragile. Hot reload sockets, absolute addresses and the product's own security headers can each break it. It also lets the server fetch any address the owner types, and the real server needs a rule for that.

## What the owner decided on 2026-11-23

- The first picker is a browser extension, because it makes building much easier.
- A desktop client comes later. It could load the website from where it is hosted, such as Vercel, and add only the web view that it owns.
- Nothing is built now. The ideas are written down, and the Browser page gets one small button.
- The Browser page has a "Pick an element" button since this date. It turns on and off, and it does nothing else.

## What the picker must settle when it is built

- The picked element reaches the chat box as one message in the contract, whichever part does the picking. The chat box is then built once.
- The description of an element comes from a page that agents wrote. The website and the agents treat it as untrusted text.
- An extension that runs in every frame can read every page the owner opens. It should run only in frames that the website opened, and only while the picker is on.
- The owner also wants the picker on the Design file, and wants the element to go to any lead's chat box, not only the project lead's. Both are in [docs/features.md](../features.md).

## Sending another program's window, for later

The owner's idea of the same day, recorded and not built. It extends the screen share idea in [docs/features.md](../features.md).

- The client captures the window of a program that is not a web page, such as a Unity game or a Windows program, and sends it to the owner's browser.
- The client could run an efficient VNC server. It relays a stream in AV1, VP9, H.264 or H.265, whichever the user's computer can accelerate. Without acceleration, the client tells the user that the picture will be poor.
- The person launches the program, and the client grabs that program's window as it starts and forwards it to the remote session.
- No codec, protocol or capture method was looked up. That research is still to do.
