import type {
  CreateCurriculumMapNodeRequest,
  CurriculumMapNode,
  CurriculumMapNodeKind,
  UpdateCurriculumMapNodeRequest,
} from "./types";

export interface NodeFieldValues {
  nodeKind: CurriculumMapNodeKind;
  nodeCode: string;
  label: string;
  parentNodeId: string;
  contentSourceId: string;
  sourceLocator: string;
}

export const EMPTY_NODE_VALUES: NodeFieldValues = {
  nodeKind: "topic",
  nodeCode: "",
  label: "",
  parentNodeId: "",
  contentSourceId: "",
  sourceLocator: "",
};

export function valuesFromNode(node: CurriculumMapNode): NodeFieldValues {
  return {
    nodeKind: node.nodeKind,
    nodeCode: node.nodeCode,
    label: node.label,
    parentNodeId: node.parentNodeId ?? "",
    contentSourceId: node.contentSourceId,
    sourceLocator: node.sourceLocator ?? "",
  };
}

/** Validates and builds a create request, or returns a validation error message. */
export function buildCreateInput(
  syllabusId: string,
  values: NodeFieldValues,
): { input: CreateCurriculumMapNodeRequest } | { error: string } {
  if (values.nodeCode.trim() === "" || values.label.trim() === "" || values.contentSourceId === "") {
    return { error: "Node code, label, and content source are required." };
  }
  const input: CreateCurriculumMapNodeRequest = {
    syllabusId,
    nodeKind: values.nodeKind,
    nodeCode: values.nodeCode.trim(),
    label: values.label.trim(),
    contentSourceId: values.contentSourceId,
  };
  if (values.parentNodeId) input.parentNodeId = values.parentNodeId;
  if (values.sourceLocator.trim()) input.sourceLocator = values.sourceLocator.trim();
  return { input };
}

/**
 * Validates and builds an update patch, or returns a validation error message.
 * parentNodeId/sourceLocator support an explicit clear (empty value -> null); nodeCode/label/
 * contentSourceId cannot be cleared, only changed to another non-empty value.
 */
export function buildUpdatePatch(
  original: NodeFieldValues,
  values: NodeFieldValues,
): { patch: UpdateCurriculumMapNodeRequest } | { error: string } {
  const patch: UpdateCurriculumMapNodeRequest = {};

  if (values.nodeKind !== original.nodeKind) patch.nodeKind = values.nodeKind;

  for (const [field, label] of [
    ["nodeCode", "Node code"],
    ["label", "Label"],
    ["contentSourceId", "Content source"],
  ] as const) {
    const next = field === "contentSourceId" ? values[field] : values[field].trim();
    const prev = field === "contentSourceId" ? original[field] : original[field].trim();
    if (next === prev) continue;
    if (next === "") return { error: `${label} cannot be cleared — supply a value or leave it unchanged.` };
    patch[field] = next;
  }

  const nextParent = values.parentNodeId;
  if (nextParent !== original.parentNodeId) {
    patch.parentNodeId = nextParent === "" ? null : nextParent;
  }

  const nextLocator = values.sourceLocator.trim();
  const prevLocator = original.sourceLocator.trim();
  if (nextLocator !== prevLocator) {
    patch.sourceLocator = nextLocator === "" ? null : nextLocator;
  }

  if (Object.keys(patch).length === 0) {
    return { error: "Change at least one field to update this node." };
  }
  return { patch };
}
