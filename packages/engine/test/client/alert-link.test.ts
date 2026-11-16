import * as Effect from "effect/Effect";
import { expect, it } from "vitest";
import { append } from "../support/alertsRig.ts";
import { configure, delivered, ntfyServer, posts, raiseQuestion, withCarrier } from "../support/carrierRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { seedAgent } from '../support/handlerRig.ts';

it.each([
  { proxyMode: false, publicAddress: "https://lgtm.example", linked: false },
  { proxyMode: true, linked: false },
  { proxyMode: true, publicAddress: "https://lgtm.example", linked: true },
  { proxyMode: true, publicAddress: "https://lgtm.example", linked: false, unassigned: true },
])("6.10 link requires proxy mode and a public address: $proxyMode / $publicAddress", async (scenario) => {
  const server = await ntfyServer();
  try {
    await withCarrier(() => Effect.gen(function* () {
      yield* registerRebuildCheck();
      yield* configure(server.origin);
      yield* seedAgent({ id: 'worker', state: 'working' });
      if (!('unassigned' in scenario && scenario.unassigned)) yield* append({ _tag: 'project.Created', projectId: 'link-project', repoId: 'link-repo', worktreeId: 'link-worktree', name: 'Links', goal: 'Read the alert', machineId: 'local', leadAgentId: 'worker' });
      yield* append({ _tag: "client.WebSettingsChanged", settings: {
        enabled: true, port: 8787, heartbeatSeconds: 25, proxyMode: scenario.proxyMode,
        ...(scenario.publicAddress === undefined ? {} : { publicAddress: scenario.publicAddress }),
      } });
      const id = yield* raiseQuestion(); expect(yield* delivered(id)).toHaveLength(1);
      const request = posts(server)[0]!;
      const json = request.headers["content-type"]?.includes("application/json") ? JSON.parse(request.body) : {};
      const link = json.click ?? request.headers.click ?? request.headers["x-click"];
      if (!scenario.linked) { expect(link).toBeUndefined(); return; }
      expect(link).toBeTypeOf("string");
      const url = new URL(link);
      expect(url.origin).toBe("https://lgtm.example");
      expect(url.hash).toBe('#/p/link-project/chat/worker?req=question-6b');
      expect(decodeURIComponent(url.pathname + url.hash)).toContain("worker");
      expect(url.username).toBe(""); expect(url.password).toBe("");
      expect(url.search).toBe("");
      expect(link).not.toMatch(/token|credential|password|approve|answer|dismiss|command=/i);
    }));
  } finally { await server.close(); }
});
