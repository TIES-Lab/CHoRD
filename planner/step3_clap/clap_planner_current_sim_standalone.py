#!/usr/bin/env python3
"""Standalone CLAP-style planner for the current mgpusim-chiplet simulator.

Design goals
------------
- Completely independent from the user's ILP / step1-step3 planner pipeline.
- First profiling run is the plain simulator baseline:
    * kernel/workgroup scheduling: round-robin (CLI controlled)
    * data placement: simulator default round-robin baseline
- This planner consumes only the generated profile.json (+ optional size metadata)
  and emits a *placement-only* plan for the current simulator.
- Output semantics are restricted to what the current simulator already supports:
    buffer_placement[tag] = {
        "policy": "explicit",
        "pages": [...]
    }
  i.e. no CLAP-specific simulator mechanism is required.
- kernel_scheduling is always left empty so the final CLAP run still uses
  simulator round-robin scheduling from the CLI.

Relationship to the paper / previous artifact
---------------------------------------------
- Uses CLAP's chiplet-local contiguous page grouping idea.
- Uses runtime profiling to infer dominant-chiplet grouped placement.
- By default selects one application-wide group size (matching the previous
  independent clap_planner artifact), but can optionally switch to per-buffer
  selection for more paper-like experimentation.
- Because the current simulator branch does not include the old CLAP-specific
  merged-page / coalesced-translation mechanism, this planner reproduces the
  *placement* side of CLAP only.
"""

from __future__ import annotations

import argparse
import json
import math
from collections import defaultdict
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Dict, List, Optional, Tuple


@dataclass
class BufferInfo:
    tag: str
    size: int
    num_pages: int
    source: str
    max_observed_page: int = -1


@dataclass
class RegionDecision:
    start_page: int
    num_pages: int
    assigned_chiplet: int
    dominant_chiplet: int
    purity: float
    total_bytes: int
    gain: int
    chiplet_bytes: List[int]


class ClapExplicitPlanner:
    def __init__(
        self,
        *,
        profile: dict,
        hardware: dict,
        global_json: Optional[dict] = None,
        use_global_sizes: bool = False,
        candidate_groups: Optional[List[int]] = None,
        purity_threshold: Optional[float] = None,
        diffuse_threshold: Optional[float] = None,
        overflow_pages: Optional[int] = None,
        group_selection_mode: str = "per-buffer",
        application_group_pages: Optional[int] = None,
    ):
        self.profile = profile or {}
        self.hardware = hardware or {}
        self.global_json = global_json or {}
        self.use_global_sizes = bool(use_global_sizes)
        self.group_selection_mode = str(group_selection_mode or "app")

        self.chiplets = int(self.profile.get("chiplets") or self.hardware.get("chiplets") or 1)
        self.page_bytes = int(self.profile.get("page_bytes") or self.hardware.get("page_bytes") or 4096)
        self.cap_pages = int(self.hardware.get("chiplet_cap_pages", 0) or 0)
        self.overflow_pages = int(
            overflow_pages if overflow_pages is not None else self.hardware.get("overflow_pages", 0) or 0
        )

        raw_candidates = candidate_groups or self.hardware.get("candidate_group_pages") or [1, 2, 4, 8, 16, 32]
        self.candidate_groups = sorted({int(x) for x in raw_candidates if int(x) > 0}) or [1]
        self.purity_threshold = float(
            purity_threshold if purity_threshold is not None else self.hardware.get("purity_threshold", 1.00)
        )
        self.diffuse_threshold = float(
            diffuse_threshold if diffuse_threshold is not None else self.hardware.get("diffuse_threshold", 0.45)
        )
        self.require_contiguous_regions = bool(self.hardware.get("require_contiguous_regions", True))
        self.max_contiguous_group_pages = int(
            self.hardware.get("max_contiguous_group_pages")
            or self.hardware.get("banks_per_chiplet")
            or 0
        )
        if self.require_contiguous_regions and self.max_contiguous_group_pages > 0:
            self.candidate_groups = [g for g in self.candidate_groups if g <= self.max_contiguous_group_pages] or [1]

        self.application_group_pages_override = int(application_group_pages) if application_group_pages else 0

        self.page_chiplet_bytes = self._aggregate_page_chiplet_bytes()
        self.buffers = self._collect_buffers()

    # ---------- profile ingestion ----------

    def _aggregate_page_chiplet_bytes(self) -> Dict[str, Dict[int, List[int]]]:
        agg: Dict[str, Dict[int, List[int]]] = defaultdict(lambda: defaultdict(lambda: [0] * self.chiplets))
        for kernel in self.profile.get("kernels", []) or []:
            for tag, tag_info in (kernel.get("tags", {}) or {}).items():
                for page_info in (tag_info.get("pages", []) or []):
                    try:
                        page = int(page_info.get("page", 0) or 0)
                    except Exception:
                        continue
                    arr = agg[tag][page]
                    r = page_info.get("r", []) or []
                    w = page_info.get("w", []) or []
                    for c in range(self.chiplets):
                        rv = int(r[c]) if c < len(r) else 0
                        wv = int(w[c]) if c < len(w) else 0
                        arr[c] += rv + wv
        return agg

    def _collect_buffers(self) -> Dict[str, BufferInfo]:
        out: Dict[str, BufferInfo] = {}
        observed_max_page: Dict[str, int] = {}
        for tag, pages in self.page_chiplet_bytes.items():
            if pages:
                observed_max_page[tag] = max(int(p) for p in pages.keys())

        for b in self.profile.get("buffers", []) or []:
            tag = b.get("tag")
            if not tag:
                continue
            size = int(b.get("size", 0) or 0)
            num_pages = max(1, math.ceil(size / self.page_bytes)) if size > 0 else 0
            max_page = observed_max_page.get(tag, -1)
            if max_page >= 0:
                num_pages = max(num_pages, max_page + 1)
                if size <= 0:
                    size = num_pages * self.page_bytes
                    source = "profile-observed-pages"
                else:
                    source = "profile+observed-pages"
            else:
                source = "profile"
            out[tag] = BufferInfo(tag=tag, size=size, num_pages=num_pages, source=source, max_observed_page=max_page)

        if self.use_global_sizes:
            for b in self.global_json.get("buffers", []) or []:
                tag = b.get("tag")
                if not tag:
                    continue
                size = int(b.get("bytes", 0) or 0)
                num_pages = int(b.get("pages", 0) or 0)
                if num_pages <= 0 and size > 0:
                    num_pages = math.ceil(size / self.page_bytes)
                max_page = observed_max_page.get(tag, -1)
                if max_page >= 0:
                    num_pages = max(num_pages, max_page + 1)
                    if size <= 0:
                        size = num_pages * self.page_bytes
                    source = "global-json+observed-pages"
                else:
                    source = "global-json"
                prev = out.get(tag)
                if prev is None or num_pages > prev.num_pages or size > prev.size:
                    out[tag] = BufferInfo(tag=tag, size=size, num_pages=num_pages, source=source, max_observed_page=max_page)

        for tag, max_page in observed_max_page.items():
            if tag in out:
                continue
            num_pages = max_page + 1
            out[tag] = BufferInfo(
                tag=tag,
                size=num_pages * self.page_bytes,
                num_pages=num_pages,
                source="observed-pages-only",
                max_observed_page=max_page,
            )
        return out

    # ---------- CLAP-ish scoring ----------

    def _candidate_metrics_for_tag(self, tag: str, num_pages: int, group_pages: int) -> dict:
        total_bytes = 0
        matched_bytes = 0
        weighted_gain = 0
        regions = 0
        nonzero_regions = 0
        per_page = self.page_chiplet_bytes.get(tag, {})
        effective_group = max(1, min(group_pages, num_pages))

        # RT-like sharedness estimate: if a tag is globally shared, relax the threshold.
        base_total = 0
        base_dom = 0
        for p in range(num_pages):
            pb = per_page.get(p)
            if pb is None:
                continue
            s = sum(pb)
            if s <= 0:
                continue
            base_total += s
            base_dom += max(pb)
        rt_like_shared_ratio = 1.0 - (float(base_dom) / float(base_total)) if base_total > 0 else 0.0
        effective_threshold = max(0.0, self.purity_threshold - rt_like_shared_ratio)

        for start in range(0, num_pages, effective_group):
            end = min(num_pages, start + effective_group)
            chip_bytes = [0] * self.chiplets
            for p in range(start, end):
                pb = per_page.get(p)
                if pb is not None:
                    for c in range(self.chiplets):
                        chip_bytes[c] += pb[c]
            region_total = sum(chip_bytes)
            regions += 1
            if region_total == 0:
                continue
            nonzero_regions += 1
            total_bytes += region_total
            ordered = sorted(chip_bytes, reverse=True)
            matched_bytes += ordered[0]
            weighted_gain += ordered[0] - ordered[1] if len(ordered) > 1 else ordered[0]

        weighted_purity = (matched_bytes / total_bytes) if total_bytes else 0.0
        gain_norm = (weighted_gain / total_bytes) if total_bytes else 0.0
        passes = weighted_purity >= effective_threshold
        bonus = 0.08 * math.log2(group_pages + 1)
        score = weighted_purity + (bonus if passes else 0.0) + 0.15 * gain_norm
        return {
            "group_pages": int(group_pages),
            "effective_group_pages": int(effective_group),
            "regions": int(regions),
            "nonzero_regions": int(nonzero_regions),
            "weighted_purity": float(weighted_purity),
            "gain_norm": float(gain_norm),
            "rt_like_shared_ratio": float(rt_like_shared_ratio),
            "effective_threshold": float(effective_threshold),
            "passes": bool(passes),
            "score": float(score),
            "total_profiled_bytes": int(total_bytes),
        }

    def _candidate_metrics_global(self, group_pages: int) -> dict:
        total_bytes = 0
        matched_bytes = 0.0
        weighted_gain = 0.0
        weighted_shared = 0.0
        regions = 0
        nonzero_regions = 0
        active_buffers = 0
        for tag, info in self.buffers.items():
            if info.num_pages <= 0:
                continue
            m = self._candidate_metrics_for_tag(tag, info.num_pages, group_pages)
            regions += m["regions"]
            nonzero_regions += m["nonzero_regions"]
            total_bytes += m["total_profiled_bytes"]
            if m["total_profiled_bytes"] > 0:
                active_buffers += 1
                matched_bytes += m["weighted_purity"] * m["total_profiled_bytes"]
                weighted_gain += m["gain_norm"] * m["total_profiled_bytes"]
                weighted_shared += m["rt_like_shared_ratio"] * m["total_profiled_bytes"]
        weighted_purity = (matched_bytes / total_bytes) if total_bytes else 0.0
        gain_norm = (weighted_gain / total_bytes) if total_bytes else 0.0
        shared_ratio = (weighted_shared / total_bytes) if total_bytes else 0.0
        effective_threshold = max(0.0, self.purity_threshold - shared_ratio)
        passes = weighted_purity >= effective_threshold
        bonus = 0.08 * math.log2(group_pages + 1)
        score = weighted_purity + (bonus if passes else 0.0) + 0.15 * gain_norm
        return {
            "group_pages": int(group_pages),
            "regions": int(regions),
            "nonzero_regions": int(nonzero_regions),
            "active_buffers": int(active_buffers),
            "weighted_purity": float(weighted_purity),
            "gain_norm": float(gain_norm),
            "rt_like_shared_ratio": float(shared_ratio),
            "effective_threshold": float(effective_threshold),
            "passes": bool(passes),
            "score": float(score),
            "total_profiled_bytes": int(total_bytes),
        }

    def _choose_application_group_pages(self) -> Tuple[int, List[dict]]:
        if self.application_group_pages_override > 0:
            one = self._candidate_metrics_global(self.application_group_pages_override)
            return int(self.application_group_pages_override), [one]
        cand = [self._candidate_metrics_global(g) for g in self.candidate_groups]
        passing = [m for m in cand if m["passes"] and m["active_buffers"] > 0]
        if passing:
            chosen = max(passing, key=lambda m: (m["group_pages"], m["gain_norm"], m["weighted_purity"]))
            return int(chosen["group_pages"]), cand
        # Paper-faithful fallback: if no larger grouped mapping satisfies the
        # threshold rule, keep the smallest base-page grouping.
        return 1, cand

    def _choose_group_for_tag(self, tag: str, num_pages: int) -> Tuple[int, List[dict]]:
        cand = [self._candidate_metrics_for_tag(tag, num_pages, g) for g in self.candidate_groups]
        if self.application_group_pages_override > 0:
            forced = [m for m in cand if m["group_pages"] == self.application_group_pages_override]
            return int(self.application_group_pages_override), forced or cand
        passing = [m for m in cand if m["passes"]]
        if passing:
            chosen = max(passing, key=lambda m: (m["group_pages"], m["gain_norm"], m["weighted_purity"]))
            return int(chosen["group_pages"]), cand
        # Paper-faithful fallback: if no larger grouped mapping satisfies the
        # threshold rule, remain at the smallest base-page group.
        return 1, cand

    # ---------- placement ----------

    def _make_regions(self, tag: str, num_pages: int, group_pages: int) -> List[RegionDecision]:
        per_page = self.page_chiplet_bytes.get(tag, {})
        effective_group = max(1, min(group_pages, num_pages))
        regions: List[RegionDecision] = []
        for start in range(0, num_pages, effective_group):
            end = min(num_pages, start + effective_group)
            chip_bytes = [0] * self.chiplets
            for p in range(start, end):
                pb = per_page.get(p)
                if pb is not None:
                    for c in range(self.chiplets):
                        chip_bytes[c] += pb[c]
            total = sum(chip_bytes)
            if total == 0:
                dominant = start % self.chiplets
                purity = 0.0
                gain = 0
            else:
                order = sorted(range(self.chiplets), key=lambda c: chip_bytes[c], reverse=True)
                dominant = order[0]
                purity = chip_bytes[dominant] / total
                gain = chip_bytes[order[0]] - chip_bytes[order[1]] if len(order) > 1 else chip_bytes[order[0]]
            regions.append(RegionDecision(
                start_page=start,
                num_pages=end - start,
                assigned_chiplet=dominant,
                dominant_chiplet=dominant,
                purity=purity,
                total_bytes=total,
                gain=gain,
                chiplet_bytes=chip_bytes,
            ))
        return regions

    def _assign_regions(self, regions: List[RegionDecision]) -> None:
        if self.cap_pages <= 0:
            return
        chip_cap = self.cap_pages + self.overflow_pages
        used = [0] * self.chiplets
        ordered = sorted(regions, key=lambda r: (r.gain, r.total_bytes, r.num_pages), reverse=True)
        for r in ordered:
            prefs = sorted(range(self.chiplets), key=lambda c: (r.chiplet_bytes[c], -used[c]), reverse=True)
            assigned = None
            for c in prefs:
                if used[c] + r.num_pages <= chip_cap:
                    assigned = c
                    break
            if assigned is None:
                assigned = min(range(self.chiplets), key=lambda c: used[c])
            r.assigned_chiplet = assigned
            used[assigned] += r.num_pages

    def _expand_pages_explicit(self, regions: List[RegionDecision]) -> Tuple[str, List[int]]:
        pages: List[int] = []
        rr_cursor = 0
        all_diffuse = True
        for r in regions:
            if r.total_bytes > 0 and r.purity >= self.diffuse_threshold:
                all_diffuse = False
            for _ in range(r.num_pages):
                if r.total_bytes == 0 or (r.purity < self.diffuse_threshold and r.num_pages == 1):
                    pages.append(rr_cursor % self.chiplets)
                    rr_cursor += 1
                else:
                    pages.append(r.assigned_chiplet)
        if all_diffuse:
            pages = [i % self.chiplets for i in range(len(pages))]
            return "round-robin", pages
        return "clap-explicit", pages

    def plan(self) -> Tuple[dict, dict]:
        app_group = None
        app_scores = []
        if self.group_selection_mode == "app":
            app_group, app_scores = self._choose_application_group_pages()

        debug = {
            "tool": "planner-clap-current-sim-standalone",
            "version": 1,
            "chiplets": self.chiplets,
            "page_bytes": self.page_bytes,
            "uses_global_json_for_sizes": self.use_global_sizes,
            "purity_threshold": self.purity_threshold,
            "diffuse_threshold": self.diffuse_threshold,
            "candidate_group_pages": self.candidate_groups,
            "require_contiguous_regions": self.require_contiguous_regions,
            "max_contiguous_group_pages": self.max_contiguous_group_pages,
            "group_selection_mode": self.group_selection_mode,
            "selection_rule": "largest-passing-group-else-base-page",
            "application_group_pages": app_group,
            "application_candidate_scores": app_scores,
            "buffers": [],
        }
        plan = {
            "tool": "planner-clap-current-sim-standalone",
            "version": 1,
            "objective": "clap-placement-only-explicit-pages",
            "hardware": {"chiplets": self.chiplets, "page_bytes": self.page_bytes},
            "kernel_scheduling": [],
            "launch_hints": [],
            "buffer_placement": {},
        }
        if app_group is not None:
            plan["application_group_pages"] = app_group

        for tag, info in sorted(self.buffers.items()):
            if info.num_pages <= 0:
                continue
            if self.group_selection_mode == "app":
                chosen_group = int(app_group or 1)
                cand = [self._candidate_metrics_for_tag(tag, info.num_pages, g) for g in self.candidate_groups]
            else:
                chosen_group, cand = self._choose_group_for_tag(tag, info.num_pages)
            regions = self._make_regions(tag, info.num_pages, chosen_group)
            self._assign_regions(regions)
            source_policy, pages = self._expand_pages_explicit(regions)
            total_profiled = sum(r.total_bytes for r in regions)
            weighted_purity = (
                sum(r.total_bytes * r.purity for r in regions) / total_profiled if total_profiled else 0.0
            )
            debug["buffers"].append({
                "tag": tag,
                "num_pages": info.num_pages,
                "size_bytes": info.size,
                "size_source": info.source,
                "max_observed_page": info.max_observed_page,
                "chosen_group_pages": int(chosen_group),
                "effective_group_pages": int(min(chosen_group, info.num_pages)),
                "weighted_purity": float(weighted_purity),
                "total_profiled_bytes": int(total_profiled),
                "source_policy": source_policy,
                "candidate_scores": cand,
                "regions": [asdict(r) for r in regions],
            })
            plan["buffer_placement"][tag] = {
                "policy": "explicit",
                "source_policy": source_policy,
                "group_pages": int(chosen_group if source_policy != "round-robin" else 1),
                "pages": pages,
            }
        return plan, debug


def load_json(path: Path) -> dict:
    with path.open() as f:
        return json.load(f)


def parse_group_list(s: Optional[str]) -> Optional[List[int]]:
    if not s:
        return None
    return [int(x.strip()) for x in s.split(",") if x.strip()]


def build_argparser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description="Standalone CLAP-style placement planner for the current simulator")
    p.add_argument("--profile-json", required=True, help="profile.json generated by a plain baseline RR run")
    p.add_argument("--hardware-config", required=True, help="independent CLAP hardware/config json")
    p.add_argument("--global-json", help="optional global.<bench>.json only for size/page fallback")
    p.add_argument("--use-global-sizes", action="store_true", help="allow optional size/page fallback from global.<bench>.json")
    p.add_argument("--out", required=True, help="output clap plan json")
    p.add_argument("--debug-out", help="optional debug json")
    p.add_argument("--group-selection-mode", choices=["app", "per-buffer"], default="per-buffer", help="per-buffer is more faithful to CLAP\'s per-data-structure page-size selection; app keeps one application-wide group size")
    p.add_argument("--group-candidates", help="comma-separated candidate group sizes in pages, e.g. 1,2,4,8,16,32")
    p.add_argument("--application-group-pages", type=int, help="force one application-wide group size in pages")
    p.add_argument("--purity-threshold", type=float, help="minimum weighted purity to accept a larger group")
    p.add_argument("--diffuse-threshold", type=float, help="below this purity, single pages fall back to explicit RR")
    p.add_argument("--overflow-pages", type=int, help="extra soft capacity beyond chiplet_cap_pages")
    return p


def main() -> None:
    args = build_argparser().parse_args()
    profile = load_json(Path(args.profile_json))
    hardware = load_json(Path(args.hardware_config))
    global_json = load_json(Path(args.global_json)) if args.global_json else None
    planner = ClapExplicitPlanner(
        profile=profile,
        hardware=hardware,
        global_json=global_json,
        use_global_sizes=bool(args.use_global_sizes),
        candidate_groups=parse_group_list(args.group_candidates),
        purity_threshold=args.purity_threshold,
        diffuse_threshold=args.diffuse_threshold,
        overflow_pages=args.overflow_pages,
        group_selection_mode=str(args.group_selection_mode),
        application_group_pages=args.application_group_pages,
    )
    plan, debug = planner.plan()
    out_path = Path(args.out)
    out_path.write_text(json.dumps(plan, indent=2, sort_keys=False))
    if args.debug_out:
        Path(args.debug_out).write_text(json.dumps(debug, indent=2, sort_keys=False))
    print(
        f"[planner-clap-current-sim-standalone] wrote {out_path} "
        f"({len(plan['buffer_placement'])} buffers, kernel_scheduling=empty, explicit-pages-only=yes)"
    )


if __name__ == "__main__":
    main()
