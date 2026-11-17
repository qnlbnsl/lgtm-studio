// Private, bounded replay memory. Nothing in this class logs or persists data.
const limit = 64 * 1024, capacity = 1024;
type Chunk = { text: string; bytes: number; safe: number };
type Escape = 'ground' | 'escape' | 'csi' | 'string' | 'stringEscape';
export class TerminalHistory {
  private chunks: Array<Chunk | undefined> = new Array(capacity);
  private head = 0;
  private count = 0;
  private bytes = 0;
  private escape: Escape = 'ground';
  append(text: string) {
    if (!text) return false;
    let bytes = Buffer.byteLength(text), start = 0;
    if (bytes > limit) {
      // Only oversized native chunks need a suffix calculation. Never encode
      // the accumulated history, and never split a UTF-16 surrogate pair.
      let kept = 0;
      start = text.length;
      while (start > 0) {
        let previous = start - 1, code = text.charCodeAt(previous);
        if (code >= 0xdc00 && code <= 0xdfff && previous > 0) { previous--; code = text.codePointAt(previous)!; }
        const size = code < 0x80 ? 1 : code < 0x800 ? 2 : code < 0x10000 ? 3 : 4;
        if (kept + size > limit) break;
        kept += size; start = previous;
      }
      bytes = kept;
    }
    let safe = -1, turnedOver = start > 0;
    for (let i = 0; i < text.length; i++) {
      if (i >= start && safe < 0 && this.escape === 'ground') safe = i - start;
      const code = text.charCodeAt(i);
      if (this.escape === 'ground') {
        if (code === 0x1b) this.escape = 'escape';
        else if (code === 0x9b) this.escape = 'csi';
        else if (code === 0x90 || code === 0x98 || code === 0x9d || code === 0x9e || code === 0x9f) this.escape = 'string';
      } else if (code === 0x18 || code === 0x1a || code === 0x9c) this.escape = 'ground';
      else if (this.escape === 'escape') {
        if (code === 0x5b) this.escape = 'csi';
        else if (code === 0x50 || code === 0x58 || code === 0x5d || code === 0x5e || code === 0x5f) this.escape = 'string';
        else if (code >= 0x30 && code <= 0x7e) this.escape = 'ground';
      } else if (this.escape === 'csi') {
        if (code >= 0x40 && code <= 0x7e) this.escape = 'ground';
        else if (code === 0x1b) this.escape = 'escape';
      } else if (this.escape === 'string') {
        if (code === 0x07) this.escape = 'ground';
        else if (code === 0x1b) this.escape = 'stringEscape';
      } else if (this.escape === 'stringEscape') this.escape = code === 0x5c ? 'ground' : code === 0x1b ? 'stringEscape' : 'string';
    }
    const last = this.count ? this.chunks[(this.head + this.count - 1) % capacity]! : undefined;
    // Bound metadata without making tiny PTY writes shorten the replay window.
    // This joins at most 4 KiB; its byte count was already measured above.
    if (!start && last && last.bytes + bytes <= 4096) {
      if (last.safe < 0 && safe >= 0) last.safe = last.text.length + safe;
      last.text += text; last.bytes += bytes; this.bytes += bytes;
      while (this.bytes > limit && this.count > 1) {
        turnedOver = true;
        this.bytes -= this.chunks[this.head]!.bytes; this.chunks[this.head] = undefined;
        this.head = (this.head + 1) % capacity; this.count--;
      }
      return turnedOver;
    }
    while (this.count && (this.bytes + bytes > limit || this.count === capacity)) {
      turnedOver = true;
      this.bytes -= this.chunks[this.head]!.bytes; this.chunks[this.head] = undefined;
      this.head = (this.head + 1) % capacity; this.count--;
    }
    this.chunks[(this.head + this.count) % capacity] = { text: start ? text.slice(start) : text, bytes, safe };
    this.count++; this.bytes += bytes;
    return turnedOver;
  }
  replay() {
    const parts: string[] = [];
    for (let i = 0; i < this.count; i++) {
      const chunk = this.chunks[(this.head + i) % capacity]!;
      if (parts.length) parts.push(chunk.text);
      else if (chunk.safe >= 0) parts.push(chunk.text.slice(chunk.safe));
    }
    return parts.join('');
  }
  clear() { this.chunks.fill(undefined); this.head = 0; this.count = 0; this.bytes = 0; this.escape = 'ground'; }
}
