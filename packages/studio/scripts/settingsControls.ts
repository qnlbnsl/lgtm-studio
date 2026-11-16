// Remove whole unsupported rows from the copied website, keeping sample markup.
export function removeSettingsRow(source: string, label: string, integration: string): string {
  const anchor = '<${Row} label="' + label + '"'
  const start = source.indexOf(anchor), end = source.indexOf('<//>', start)
  if (start < 0 || end < start || source.indexOf(anchor, start + 1) >= 0) throw new Error(`${integration}: ${label} row moved`)
  return source.slice(0, start) + source.slice(end + 5)
}
