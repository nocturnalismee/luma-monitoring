// Shared helpers live under src/{app,components,features,lib,hooks} per AGENT.md section 5.

/** formatBytes renders a byte count with an Indonesian-locale grouping. */
export function formatVersion(major: number, minor: number): string {
  return `v${major}.${minor}`;
}
