"""Create metadata-only content inventories from temporary archive extraction."""

from __future__ import annotations

import csv
import hashlib
import os
import re
import sys
from collections import Counter
from pathlib import Path


FIELDS = [
    "archive_path", "filename", "file_type", "size_bytes", "sha256",
    "exam_board", "subject", "level", "syllabus_code", "year_edition", "likely_category",
]


def find(pattern: str, value: str) -> str:
    match = re.search(pattern, value, re.IGNORECASE)
    return match.group(1) if match else "Unknown"


def classify(path: Path, root: Path) -> dict[str, str]:
    text = str(path.relative_to(root)).replace("\\", "/")
    lower = text.lower()
    name = path.name.lower()
    board = "Cambridge International" if "cambridge" in lower else "Unknown"
    subject_map = {
        "biology": "Biology", "chemistry": "Chemistry", "physics": "Physics",
        "computer science": "Computer Science", "accounting": "Accounting",
        "business": "Business", "economics": "Economics", "mathematics": "Mathematics",
        "math": "Mathematics", "english": "English", "psychology": "Psychology",
        "french": "French", "german": "German", "ict": "ICT",
        "environmental": "Environmental Management",
    }
    subject = next((v for k, v in subject_map.items() if k in lower), "Unknown")
    if "as and a level" in lower or "as _ a level" in lower or "/as and a level/" in lower:
        level = "Cambridge International AS & A Level"
    elif "igcse" in lower and "o level" in lower:
        level = "Cambridge IGCSE / O Level"
    elif "igcse" in lower:
        level = "Cambridge IGCSE"
    elif "o level" in lower:
        level = "Cambridge O Level"
    else:
        level = "Unknown"
    if "mark scheme" in lower:
        category = "Mark scheme"
    elif "past paper" in lower or "question paper" in lower:
        category = "Past paper"
    elif "syllabus" in lower:
        category = "Syllabus"
    elif "examiner report" in lower:
        category = "Examiner report"
    elif "answers" in lower or "worked solutions" in lower:
        category = "Answer key / worked solutions"
    elif "revision guide" in lower or "study" in lower:
        category = "Revision guide"
    elif "workbook" in lower or "practice book" in lower or "practical skills" in lower:
        category = "Workbook / practice"
    elif "notes" in lower:
        category = "Notes"
    elif "coursebook" in lower or "student book" in lower or "textbook" in lower or "biology" in lower:
        category = "Textbook / coursebook"
    else:
        category = "Unknown"
    code = find(r"\b(0(?:610|5090))\b", text)
    edition = find(r"\b(\d+(?:st|nd|rd|th)\s+edition|\d{4})\b", text)
    return {"exam_board": board, "subject": subject, "level": level,
            "syllabus_code": code, "year_edition": edition, "likely_category": category}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: generate_inventory.py EXTRACTION_ROOT OUTPUT_DIRECTORY")
    root, output = map(Path, sys.argv[1:])
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        data = classify(path, root)
        rows.append({
            "archive_path": str(path.relative_to(root)).replace("\\", "/"),
            "filename": path.name, "file_type": path.suffix.lower().lstrip(".").upper() or "Unknown",
            "size_bytes": path.stat().st_size, "sha256": sha256(path), **data,
        })
    with (output / "resource_inventory.csv").open("w", newline="", encoding="utf-8") as stream:
        writer = csv.DictWriter(stream, fieldnames=FIELDS)
        writer.writeheader(); writer.writerows(rows)
    counts = Counter(row["subject"] for row in rows)
    lines = ["# Sidus resource inventory", "", "Metadata-only index. No source PDFs, extracted text, diagrams, or derivative content are in this repository.", "", "## Scope", "", f"- Files indexed: {len(rows)}", f"- Total bytes: {sum(int(r['size_bytes']) for r in rows):,}", "- File types: PDF (all files)", "", "## Subject counts", ""]
    lines += [f"- {subject}: {count}" for subject, count in sorted(counts.items())]
    lines += ["", "## Complete inventory", "", "`Unknown` means metadata was not identifiable from safe filename/path inspection.", "", "| Archive path | Filename | Type | Bytes | Board | Subject | Level | Syllabus | Year/edition | Category |", "| --- | --- | --- | ---: | --- | --- | --- | --- | --- | --- |"]
    for row in rows:
        values = [row[key].replace("|", "\\|") for key in ("archive_path", "filename", "file_type", "exam_board", "subject", "level", "syllabus_code", "year_edition", "likely_category")]
        lines.append(f"| {values[0]} | {values[1]} | {values[2]} | {row['size_bytes']} | {values[3]} | {values[4]} | {values[5]} | {values[6]} | {values[7]} | {values[8]} |")
    (output / "resource_inventory.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
