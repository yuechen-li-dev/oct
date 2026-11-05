"""Read-only reference navigation, source provenance, and coverage checks.

Run from any directory with Python 3.10+; no third-party dependencies.
This checks documentation ownership, not Oct semantics. Execute the corpus
commands in Language/reference/maintaining.md to validate behavior.
"""

from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[1]
REF = ROOT / "Language/reference"
LINK = re.compile(r"(?<!!)\[[^\]\n]+\]\(([^)\n]+)\)")
SAMPLE = re.compile(
    r"<!-- verified-example: (.*?) -->\s*```oct\n(.*?)\n```", re.S
)


def contents(path):
    return path.read_text(encoding="utf-8-sig")


def anchors(path):
    found = set()
    counts = {}
    fenced = False
    for line in contents(path).splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
        if fenced:
            continue
        match = re.match(r"^#{1,6} (.+?)\s*#*\s*$", line)
        if not match:
            continue
        heading = match.group(1)
        # GitHub-style IDs retain Unicode letters/numbers, '_' and '-'.
        base = "".join(
            c for c in heading.lower() if c.isalnum() or c in "_ -"
        ).replace(" ", "-")
        count = counts.get(base, 0)
        counts[base] = count + 1
        found.add(base + (f"-{count}" if count else ""))
    found.update(re.findall(r'<a\s+(?:id|name)="([^"]+)"', contents(path)))
    return found


def main():
    errors = []
    pages = set(REF.rglob("*.md"))
    edges = {page: set() for page in pages}
    anchor_cache = {}
    link_count = 0
    sample_count = 0
    sample_sources = set()
    library_names = set()
    coverage_areas = set()
    covered_ast = set()

    def issue(path, text):
        errors.append(f"{path.relative_to(ROOT).as_posix()}: {text}")

    def check_link(page, href, internal=False):
        nonlocal link_count
        if re.match(r"^[a-zA-Z][a-zA-Z0-9+.-]*:", href) or href.startswith("//"):
            return
        path_text, _, fragment = href.partition("#")
        path_text = unquote(path_text.split("?", 1)[0].strip("<>"))
        target = (page.parent / path_text).resolve() if path_text else page
        # Outside-reference incoming links are checked only when aimed here.
        if not internal and not target.is_relative_to(REF):
            return
        link_count += 1
        if not target.exists():
            issue(page, f"missing local link {href}")
            return
        if target.suffix.lower() == ".md" and fragment:
            if target not in anchor_cache:
                anchor_cache[target] = anchors(target)
            if unquote(fragment) not in anchor_cache[target]:
                issue(page, f"missing heading anchor {href}")
        if internal and target in pages:
            edges[page].add(target)
        if internal and target.is_relative_to(ROOT / "Libraries"):
            relative = target.relative_to(ROOT / "Libraries")
            if relative.parts:
                library_names.add(relative.parts[0])

    for page in sorted(pages):
        text = contents(page)
        for href in LINK.findall(text):
            check_link(page, href, internal=True)
        for source, snippet in SAMPLE.findall(text):
            sample_count += 1
            target = (page.parent / source).resolve()
            if not target.is_relative_to(ROOT / "Language/ReferenceExamples"):
                issue(page, f"lesson provenance must name a ReferenceExamples source: {source}")
            if not target.is_file():
                issue(page, f"missing lesson source {source}")
            elif snippet.strip() != contents(target).strip():
                issue(page, f"lesson differs from {source}")
            sample_sources.add(target)
        if page.name == "coverage.md":
            coverage_areas.update(re.findall(r"\[Language/([^\]]+)\]", text))
            for group in re.findall(r"<!-- ast-coverage: (.*?) -->", text):
                covered_ast.update(group.split())

    # Validate links into the renamed reference from the rest of the checkout.
    tracked = subprocess.check_output(
        ["git", "ls-files", "-z", "--", "*.md"], cwd=ROOT
    ).decode().split("\0")
    for relative in tracked:
        page = ROOT / relative
        if not relative or page in pages or not page.is_file():
            continue
        for href in LINK.findall(contents(page)):
            check_link(page, href)

    visited = set()
    pending = [REF / "README.md"]
    while pending:
        page = pending.pop()
        if page in visited:
            continue
        visited.add(page)
        pending.extend(edges.get(page, set()) - visited)
    for page in sorted(pages - visited):
        issue(page, "chapter is unreachable from the reference home")

    actual_areas = set()
    for pattern in ("*.octest", "*.octfail"):
        for path in (ROOT / "Language").rglob(pattern):
            parts = path.relative_to(ROOT / "Language").parts
            if parts[0] in {"reference", "SDSL-V"}:
                continue
            area = "ReferenceExamples" if parts[0] == "ReferenceExamples" else "/".join(parts[:2])
            actual_areas.add(area)
    for area in sorted(actual_areas - coverage_areas):
        errors.append(f"Unowned contract area: Language/{area}")
    for area in sorted(coverage_areas - actual_areas):
        errors.append(f"Stale contract area: Language/{area}")

    ast_source = contents(ROOT / "internal/ast/program.go")
    actual_ast = set(re.findall(r"^func \((\w+)\) (?:stmtNode|exprNode)\(\)", ast_source, re.M))
    for kind in sorted(actual_ast - covered_ast):
        errors.append(f"Unowned statement/expression form: {kind}")
    for kind in sorted(covered_ast - actual_ast):
        errors.append(f"Stale statement/expression form: {kind}")

    actual_libraries = {p.name for p in (ROOT / "Libraries").iterdir() if p.is_dir()}
    for name in sorted(actual_libraries - library_names):
        errors.append(f"Unlisted library: {name}")
    for path in (ROOT / "Language/ReferenceExamples").rglob("*.octest"):
        if path not in sample_sources:
            issue(path, "lesson source has no matching verified-example block")

    if errors:
        print("Reference check failed:", file=sys.stderr)
        for error in errors:
            print("- " + error, file=sys.stderr)
        return 1
    print(
        f"Reference OK: {len(pages)} reachable pages, {link_count} local links, "
        f"{sample_count} exact lesson blocks, {len(actual_ast)} AST forms, "
        f"{len(actual_areas)} contract areas, {len(actual_libraries)} library directories."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
