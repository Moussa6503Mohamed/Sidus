import { describe, expect, it } from "vitest";
import { buildCreateInput, buildUpdatePatch, valuesFromNode, type NodeFieldValues } from "./node-form-diff";
import type { CurriculumMapNode } from "./types";

function makeNode(overrides: Partial<CurriculumMapNode> = {}): CurriculumMapNode {
  return {
    id: "node-1",
    syllabusId: "syl-1",
    parentNodeId: "parent-1",
    nodeKind: "topic",
    nodeCode: "T1",
    label: "Cell biology",
    status: "draft",
    contentSourceId: "src-1",
    sourceLocator: "section-2",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("buildUpdatePatch", () => {
  it("omits parentNodeId and sourceLocator when left unchanged", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, label: "New label" });

    expect(result).toEqual({ patch: { label: "New label" } });
  });

  it("clears parentNodeId to null when the field is emptied", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, parentNodeId: "" });

    expect(result).toEqual({ patch: { parentNodeId: null } });
  });

  it("clears sourceLocator to null when the field is emptied", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, sourceLocator: "" });

    expect(result).toEqual({ patch: { sourceLocator: null } });
  });

  it("sends a new value (not null) when a nullable field changes to another value", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, parentNodeId: "parent-2", sourceLocator: "section-9" });

    expect(result).toEqual({ patch: { parentNodeId: "parent-2", sourceLocator: "section-9" } });
  });

  it("clears both nullable fields to null in the same patch when both are emptied", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, parentNodeId: "", sourceLocator: "" });

    expect(result).toEqual({ patch: { parentNodeId: null, sourceLocator: null } });
  });

  it("rejects clearing a required field", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values, nodeCode: "" });

    expect(result).toEqual({ error: expect.stringMatching(/cannot be cleared/i) });
  });

  it("rejects a no-op submission", () => {
    const node = makeNode();
    const values = valuesFromNode(node);
    const result = buildUpdatePatch(values, { ...values });

    expect(result).toEqual({ error: expect.stringMatching(/change at least one field/i) });
  });

  it("a root node (no parent) already has an empty parentNodeId value, matching an unset field", () => {
    const node = makeNode({ parentNodeId: null, sourceLocator: null });
    const values = valuesFromNode(node);

    expect(values.parentNodeId).toBe("");
    expect(values.sourceLocator).toBe("");

    const result = buildUpdatePatch(values, { ...values, label: "Renamed" });
    expect(result).toEqual({ patch: { label: "Renamed" } });
  });
});

describe("buildCreateInput", () => {
  const emptyValues: NodeFieldValues = {
    nodeKind: "topic",
    nodeCode: "",
    label: "",
    parentNodeId: "",
    contentSourceId: "",
    sourceLocator: "",
  };

  it("requires nodeCode, label, and contentSourceId", () => {
    const result = buildCreateInput("syl-1", emptyValues);
    expect(result).toEqual({ error: expect.stringMatching(/required/i) });
  });

  it("omits optional parentNodeId/sourceLocator when not supplied", () => {
    const result = buildCreateInput("syl-1", { ...emptyValues, nodeCode: "T1", label: "Topic", contentSourceId: "src-1" });
    expect(result).toEqual({
      input: { syllabusId: "syl-1", nodeKind: "topic", nodeCode: "T1", label: "Topic", contentSourceId: "src-1" },
    });
  });

  it("includes optional parentNodeId/sourceLocator when supplied", () => {
    const result = buildCreateInput("syl-1", {
      ...emptyValues,
      nodeCode: "T1",
      label: "Topic",
      contentSourceId: "src-1",
      parentNodeId: "parent-1",
      sourceLocator: "section-2",
    });
    expect(result).toEqual({
      input: {
        syllabusId: "syl-1",
        nodeKind: "topic",
        nodeCode: "T1",
        label: "Topic",
        contentSourceId: "src-1",
        parentNodeId: "parent-1",
        sourceLocator: "section-2",
      },
    });
  });
});
