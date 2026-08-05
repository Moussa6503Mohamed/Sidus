import type { CurriculumMapNode } from "./types";

/** Bounds the ancestor walk below so a corrupted/very deep chain in the loaded set cannot loop
 * forever — mirrors the bound Core itself applies server-side (maxParentHops). */
const MAX_DEPTH = 1000;

/** Depth of each node within the currently-loaded set, for display indentation only. A parent
 * outside the loaded set (should not happen — nodes are always loaded per-syllabus and a parent
 * must share its syllabus) is treated as depth 0. */
export function nodeDepths(nodes: CurriculumMapNode[]): Map<string, number> {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const depths = new Map<string, number>();

  function depthOf(node: CurriculumMapNode, seen: Set<string>): number {
    const cached = depths.get(node.id);
    if (cached !== undefined) return cached;
    if (!node.parentNodeId || seen.has(node.id) || seen.size >= MAX_DEPTH) return 0;
    const parent = byId.get(node.parentNodeId);
    if (!parent) return 0;
    const depth = 1 + depthOf(parent, new Set(seen).add(node.id));
    depths.set(node.id, depth);
    return depth;
  }

  for (const node of nodes) {
    depths.set(node.id, depthOf(node, new Set()));
  }
  return depths;
}

/** The parent node's editorial nodeCode, for display — null for a root node or one whose parent
 * fell outside the currently-loaded set. */
export function parentCodeOf(node: CurriculumMapNode, nodes: CurriculumMapNode[]): string | null {
  if (!node.parentNodeId) return null;
  return nodes.find((n) => n.id === node.parentNodeId)?.nodeCode ?? null;
}
