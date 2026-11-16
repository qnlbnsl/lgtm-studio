// A phone bookmark stays at its existing subject route through sign-in.
// Only routes emitted by the alert sender are retained here.
export function afterSignIn(firstRun) {
  if (firstRun) return '/setup';
  const bookmark = location.hash.slice(1);
  return /^\/p\/[^/?#]+\/(?:chat|messages)\/[^/?#]+(?:\?[^#]*)?$/.test(bookmark) ? bookmark : '/home';
}
