import { createHash } from "node:crypto"
import { parse, parseFragment, serialize, type DefaultTreeAdapterTypes as Tree } from "parse5"
import { displayHeaders, type DisplayConfiguration } from "./policy.ts"

const credit = "https://github.com/haydenschmitty/floorplan"
// Keep Floorplan's palettes and theme switch. Fonts use local fallbacks.
const style = `:root{--sans:'Inter',system-ui,sans-serif;--mono:'JetBrains Mono',ui-monospace,monospace}
:root:not([data-theme="light"]){--recent:#a9d4ae;--recent-text:#15231b}
:root[data-theme="light"]{--recent:#1f7446;--recent-text:#fff}
.lgtm-floorplan-credit{position:fixed!important;bottom:12px!important;right:12px!important;z-index:2147483647!important;display:block!important;visibility:visible!important;opacity:1!important;padding:8px 12px;background:var(--surface,#fff);color:var(--text,#15231b);font:13px system-ui,sans-serif;border:1px solid var(--hair,#dbe3d8);border-radius:5px}`

export const mapHeaders = (configuration: DisplayConfiguration) => {
  const { "access-control-allow-origin": _cors, "cross-origin-resource-policy": _corp, ...headers } = displayHeaders(configuration, "mock")
  return {
    ...headers,
    "referrer-policy": "no-referrer",
    // Keep popups for the credit link. Navigation can carry map content away;
    // the sandbox prevents access to the owner's origin, not all navigation.
    "content-security-policy": ["default-src 'none'", "sandbox allow-scripts allow-popups", `frame-ancestors ${configuration.ownerOrigin}`,
      "base-uri 'none'", "form-action 'none'", "object-src 'none'", "frame-src 'none'", "connect-src 'none'",
      "script-src 'unsafe-inline'", "style-src 'unsafe-inline'", "img-src data: blob:", "font-src 'none'", "worker-src 'none'", "webrtc 'block'"].join("; ")
  }
}

export const reskinMap = (text: string) => {
  const document = parse(text)
  const elements: Tree.Element[] = []
  const visit = (node: Tree.Node) => {
    if ("tagName" in node) elements.push(node)
    if ("childNodes" in node) node.childNodes.forEach(visit)
  }
  visit(document)
  const head = elements.find((node) => node.tagName === "head")!, body = elements.find((node) => node.tagName === "body")!
  const append = (parent: Tree.Element, html: string) => {
    for (const node of parseFragment(html).childNodes) { node.parentNode = parent; parent.childNodes.push(node) }
  }
  // Remove automatic refresh and speculative fetch hints. Scripts can still
  // navigate or create new links; this is not a navigation security boundary.
  const speculative = new Set(["dns-prefetch", "preconnect", "prefetch", "preload", "modulepreload", "prerender"])
  for (const node of elements) {
    const attr = (name: string) => node.attrs.find((item) => item.name === name)?.value.toLowerCase()
    if (node.attrs.some((item) => item.name === "id" && ["lgtm-map-style", "lgtm-map-credit-script"].includes(item.value))
      || node.tagName === "meta" && attr("http-equiv") === "refresh"
      || node.tagName === "link" && attr("rel")?.split(/\s+/).some((rel) => speculative.has(rel))) {
      node.parentNode!.childNodes.splice(node.parentNode!.childNodes.indexOf(node), 1)
    }
  }
  append(head, `<style id="lgtm-map-style">${style}</style>`)
  const textOf = (node: Tree.Node): string => "value" in node ? node.value : "childNodes" in node ? node.childNodes.map(textOf).join(" ") : ""
  const existing = elements.find((node) => node.tagName === "a" && node.attrs.some((attr) => attr.name === "href" && attr.value === credit)
    && (node.attrs.some((attr) => attr.name === "id" && attr.value === "madewith") || /Made with/i.test(textOf(node))))
  if (existing !== undefined) {
    existing.attrs = existing.attrs.filter((attr) => !["target", "rel"].includes(attr.name))
    const classes = existing.attrs.find((attr) => attr.name === "class")
    if (classes === undefined) existing.attrs.push({ name: "class", value: "lgtm-floorplan-credit" })
    else classes.value += " lgtm-floorplan-credit"
    existing.attrs.push({ name: "target", value: "_blank" }, { name: "rel", value: "noopener noreferrer" })
  }
  if (existing === undefined) append(body, `<a id="lgtm-map-credit" class="lgtm-floorplan-credit" href="${credit}" target="_blank" rel="noopener noreferrer">Made with Floorplan</a>`)
  // Floorplan's exported page creates #madewith at boot. Keep that original
  // credit and remove our fallback after boot, including deferred boot.
  const script = `addEventListener('load',()=>{const links=[...document.querySelectorAll('a[href="${credit}"]')].filter(a=>a.id==='madewith'||/Made with/i.test(a.textContent));const original=links.find(a=>a.id!=='lgtm-map-credit');const keep=original||links[0];if(!keep)return;for(const a of links)if(a!==keep)a.remove();keep.target='_blank';keep.rel='noopener noreferrer';keep.classList.add('lgtm-floorplan-credit');if(!keep.textContent.includes('Made with'))keep.textContent='Made with Floorplan';});`
  append(body, `<script id="lgtm-map-credit-script">${script}</script>`)
  // Floorplan ships a hashed script policy in its own meta element. Admit
  // only our added script there; keep every existing restriction intact.
  const hash = `'sha256-${createHash("sha256").update(script).digest("base64")}'`
  for (const node of elements) if (node.tagName === "meta" && node.attrs.some((attr) => attr.name === "data-floorplan-policy")) {
    const policy = node.attrs.find((attr) => attr.name === "content")
    if (policy !== undefined) policy.value = policy.value.replace(/(^|;)(\s*)(script-src(?:-elem)?)\s+([^;]*)/gi, (_all, separator, whitespace, directive, sources) => `${separator}${whitespace}${directive} ${sources} ${hash}`)
  }
  return { text: serialize(document) }
}
