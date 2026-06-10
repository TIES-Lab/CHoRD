#!/usr/bin/env python3
"""
MGVM Step3 Planner - Version 5 (dimension-aware + multi-level locality)

Goal:
  - Still do chiplet-level page placement using an EXACT min-cost flow (transportation problem),
    like v2.
  - Add multi-level knobs to avoid the SA>1 "L1 reuse gets destroyed -> L2 accesses inflate" pitfall:
      * emit per-kernel scheduling policy (lasp vs lasp-blockcu) based on hardware + kernel features
  - Make the placement more robust for transformer-ish kernels where Step1 cannot always prove
    gid_x/y dependence (=> "broadcast-like" / "uncertain affinity" buffers):
      * dynamic traffic scaling option (none/wg/wi)
      * weighted partition selection (instead of only "largest buffer decides")
      * optional "read-only stripe" safeguard for low-confidence read-only buffers when active chiplets>1

Inputs:
  - Step1 kernels.json (from mgvm-kernel-analyzer)
  - Step2 global.json  (from mgvm-host-analyzer)

Output:
  - plan.json consumable by your chiplet-enabled MGPUSim

This file is self-contained: includes a min-cost flow solver (no external packages).
"""

from __future__ import annotations

import argparse
import copy
import ast
import csv
import heapq
import json
import math
import re
import os
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple, Set

# -----------------------------
# Profile-guided replan (Phase3-1, planner-only)
# -----------------------------
#
# The simulator-side profiler produces a JSON with:
#   - buffers: [{pid, tag, vaddr, size}, ...]
#   - kernels: [{name, cmd_id, grid_size:[x,y,z], wg_size:[x,y,z], tags:{tag:{pages:[{page,r[],w[]},...]}}}, ...]
#
# This planner can optionally ingest that profile and blend it into the static traffic model:
#   combined = (1-alpha)*static + alpha*profile
#
# Why we still keep static:
#   - Some workloads cannot be profiled or have input-dependent behavior.
#   - Profile may be partial (warmup) and miss rarely touched pages.
#
# Two key correctness requirements:
#   1) page grouping must match (profile page_bytes may represent group_pages*base_page_bytes)
#   2) kernel invocations should be distinguishable (profile provides cmd_id and preserves dispatch order)

@dataclass
class ProfileData:
    chiplets: int
    page_bytes: int                # bytes per profile page unit
    group_pages: int               # profile_page_bytes / planner_page_bytes (must be integer)
    byte_model: str
    buffers: List[dict]
    kernels: List[dict]
    # Aggregated demand (after converting to planner base pages):
    #   tag -> base_page_idx -> [bytes_per_chiplet]
    tag_page_bytes: Dict[str, Dict[int, List[float]]]
    stats: Dict[str, object]


def _int3(x) -> Tuple[int, int, int]:
    if isinstance(x, (list, tuple)) and len(x) >= 3:
        return (int(x[0]), int(x[1]), int(x[2]))
    if isinstance(x, (list, tuple)) and len(x) == 1:
        return (int(x[0]), 1, 1)
    return (0, 0, 0)


def _as_int(v: object, default: int = 0) -> int:
    """Return int(v) but preserve zero-valued fields.

    Avoid patterns like ``int(x or -1)`` which silently convert valid zeros
    (launch id 0, page 0, trip-count 0) into the fallback value.
    """
    if v is None:
        return int(default)
    try:
        return int(v)
    except Exception:
        return int(default)


def _num_wg_total_from_sizes(global_size: Tuple[int, int, int], local_size: Tuple[int, int, int]) -> int:
    total = 1
    for i in range(3):
        g = max(0, int(global_size[i]))
        w = max(1, int(local_size[i]))
        if g <= 0:
            return 0
        total *= int(math.ceil(float(g) / float(w)))
    return int(total)


def load_profile_json(
    path: str,
    *,
    planner_chiplets: int,
    planner_page_bytes: int,
    read_weight: float = 1.0,
    write_weight: float = 1.0,
    scale: float = 1.0,
    # Phase3-2A: profile confidence gating (conservative defaults)
    gate_max_active_full: int = 4,
    gate_peak0: float = 0.20,
    gate_peak1: float = 0.35,
    gate_min_bytes: float = 4096.0,
) -> ProfileData:
    """Load a simulator profile.json and convert page indices to planner base pages.

    The profile's page index is in units of profile.page_bytes. If profile.page_bytes is larger
    than planner_page_bytes (due to profiling page grouping), we distribute bytes uniformly across
    the corresponding base pages (conservative).

    Phase3-2A (planner-only): apply a confidence gate so profile guidance primarily affects
    low-parallel or skewed-access cases (e.g., layernorm/fc with numWG << chiplets), and avoids
    overreacting to near-uniform accesses (which can create plan jitter / regressions).
    """
    try:
        p = json.load(open(path, "r"))
    except Exception as e:
        raise SystemExit(f"ERROR: failed to read --profile-json {path}: {e}")

    chiplets = int(p.get("chiplets", 0) or 0)
    if chiplets <= 0:
        raise SystemExit(f"ERROR: profile {path} missing/invalid 'chiplets'")
    if planner_chiplets > 0 and chiplets != int(planner_chiplets):
        raise SystemExit(f"ERROR: profile chiplets={chiplets} does not match planner chiplets={planner_chiplets}")

    prof_page_bytes = int(p.get("page_bytes", 0) or 0)
    if prof_page_bytes <= 0:
        raise SystemExit(f"ERROR: profile {path} missing/invalid 'page_bytes'")
    if planner_page_bytes <= 0:
        planner_page_bytes = 4096

    # group_pages can be explicit or inferred
    explicit_gp = p.get("group_pages", None)
    if explicit_gp is not None:
        try:
            explicit_gp = int(explicit_gp)
        except Exception:
            explicit_gp = None

    if prof_page_bytes % int(planner_page_bytes) != 0:
        raise SystemExit(
            f"ERROR: profile page_bytes={prof_page_bytes} not divisible by planner page_bytes={planner_page_bytes}. "
            f"Please profile with matching page size or adjust planner --page-bytes."
        )
    gp_infer = int(prof_page_bytes // int(planner_page_bytes))
    gp = int(explicit_gp) if explicit_gp and explicit_gp > 0 else int(gp_infer)

    # sanity: if explicit gp disagrees with inferred, keep inferred but record
    gp_mismatch = bool(explicit_gp and explicit_gp > 0 and int(explicit_gp) != int(gp_infer))

    byte_model = str(p.get("byte_model", ""))

    buffers = p.get("buffers", []) or []
    kernels = p.get("kernels", []) or []

    # Aggregated demand (after converting to planner base pages):
    #   tag -> base_page_idx -> [bytes_per_chiplet]
    #
    # NOTE: We gate *per (kernel_event, tag)* before accumulation. This preserves
    # strong signals from low-parallel kernels (active chiplets small) even when
    # the same tag is later accessed by large, uniform kernels.
    tag_page: Dict[str, Dict[int, List[float]]] = {}

    def gate_factor(total_bytes: float, active: int, peak: float) -> float:
        """Return [0,1] gate factor for a (kernel_event, tag) based on skew/confidence."""
        if total_bytes <= 0.0:
            return 0.0
        if float(total_bytes) < float(gate_min_bytes):
            return 0.0
        if int(active) <= int(gate_max_active_full):
            return 1.0
        # For high-active tags, require skew to trust the profile.
        p0 = float(gate_peak0)
        p1 = float(gate_peak1)
        if p1 <= p0:
            return 1.0 if float(peak) > p0 else 0.0
        if float(peak) <= p0:
            return 0.0
        if float(peak) >= p1:
            return 1.0
        return (float(peak) - p0) / (p1 - p0)

    stats = {
        "path": path,
        "chiplets": int(chiplets),
        "profile_page_bytes": int(prof_page_bytes),
        "planner_page_bytes": int(planner_page_bytes),
        "group_pages": int(gp),
        "group_pages_inferred": int(gp_infer),
        "group_pages_mismatch": bool(gp_mismatch),
        "byte_model": byte_model,
        "kernels": int(len(kernels)),
        "buffers": int(len(buffers)),
        # Raw (before gating) summary
        "tags_unique_raw": 0,
        "kernel_tag_entries_raw": 0,
        "kernel_page_entries_raw": 0,
        # After gating summary
        "tags_unique": 0,
        "kernel_tag_entries": 0,
        "kernel_page_entries": 0,
        "unknown_tags": 0,
        "skipped_empty_tag": 0,
        # Phase3-2A gating stats
        "gating": {
            "enabled": True,
            "gate_max_active_full": int(gate_max_active_full),
            "gate_peak0": float(gate_peak0),
            "gate_peak1": float(gate_peak1),
            "gate_min_bytes": float(gate_min_bytes),
            "tag_events_total": 0,
            "tag_events_used": 0,
            "tag_events_zero": 0,
            "avg_gate_used": 0.0,
            "avg_gate_bytes_weighted": 0.0,
            "bytes_raw": 0.0,
            "bytes_used": 0.0,
        },
    }

    seen_tags_raw: Set[str] = set()
    seen_tags_used: Set[str] = set()

    # Iterate over kernel events
    g_tag_events_total = 0
    g_tag_events_used = 0
    g_tag_events_zero = 0
    g_gate_sum = 0.0
    g_gate_bytes_wsum = 0.0
    g_bytes_raw = 0.0
    g_bytes_used = 0.0

    for k in kernels:
        ktags = k.get("tags", {}) or {}
        # First pass: compute per-tag totals to decide gating.
        tag_totals: Dict[str, List[float]] = {}
        tag_total_bytes: Dict[str, float] = {}
        tag_gate: Dict[str, float] = {}

        for tag, tinfo in ktags.items():
            if not tag:
                stats["skipped_empty_tag"] += 1
                continue
            pages = (tinfo.get("pages", []) or [])
            if not pages:
                continue
            seen_tags_raw.add(tag)
            stats["kernel_tag_entries_raw"] += 1
            g_tag_events_total += 1

            vec_tot = [0.0 for _ in range(chiplets)]
            tot = 0.0
            for pr in pages:
                r = pr.get("r", []) or []
                w = pr.get("w", []) or []
                if len(r) != chiplets or len(w) != chiplets:
                    continue
                for c in range(chiplets):
                    v = (float(read_weight) * float(r[c]) + float(write_weight) * float(w[c])) * float(scale)
                    vec_tot[c] += v
                    tot += v
                stats["kernel_page_entries_raw"] += 1

            if tot <= 0.0:
                # no meaningful bytes
                tag_gate[tag] = 0.0
                g_tag_events_zero += 1
                continue

            active = sum(1 for v in vec_tot if v != 0.0)
            peak = (max(vec_tot) / tot) if tot > 0.0 else 0.0
            g = gate_factor(tot, active, peak)
            tag_totals[tag] = vec_tot
            tag_total_bytes[tag] = float(tot)
            tag_gate[tag] = float(g)

            g_bytes_raw += float(tot)
            if g > 0.0:
                g_tag_events_used += 1
                g_gate_sum += float(g)
                g_gate_bytes_wsum += float(g) * float(tot)
            else:
                g_tag_events_zero += 1

        # Second pass: accumulate gated per-page bytes.
        for tag, tinfo in ktags.items():
            if not tag:
                continue
            pages = (tinfo.get("pages", []) or [])
            if not pages:
                continue

            g = float(tag_gate.get(tag, 0.0))
            if g <= 0.0:
                continue

            seen_tags_used.add(tag)
            stats["kernel_tag_entries"] += 1
            if tag not in tag_page:
                tag_page[tag] = {}

            for pr in pages:
                try:
                    pidx = int(pr.get("page", -1))
                except Exception:
                    pidx = -1
                if pidx < 0:
                    continue
                r = pr.get("r", []) or []
                w = pr.get("w", []) or []
                if len(r) != chiplets or len(w) != chiplets:
                    continue

                base0 = int(pidx) * int(gp)
                per_base_div = float(int(gp)) if int(gp) > 0 else 1.0

                vec = [0.0 for _ in range(chiplets)]
                for c in range(chiplets):
                    vec[c] = (float(read_weight) * float(r[c]) + float(write_weight) * float(w[c])) * float(scale) * g
                # record used bytes for gating stats
                g_bytes_used += float(sum(vec))

                for off in range(int(gp)):
                    bp = base0 + off
                    if bp not in tag_page[tag]:
                        tag_page[tag][bp] = [0.0 for _ in range(chiplets)]
                    for c in range(chiplets):
                        tag_page[tag][bp][c] += vec[c] / per_base_div

                stats["kernel_page_entries"] += 1

    stats["tags_unique_raw"] = int(len(seen_tags_raw))
    stats["tags_unique"] = int(len(seen_tags_used))

    # Finalize gating stats
    stats["gating"]["tag_events_total"] = int(g_tag_events_total)
    stats["gating"]["tag_events_used"] = int(g_tag_events_used)
    stats["gating"]["tag_events_zero"] = int(g_tag_events_zero)
    stats["gating"]["bytes_raw"] = float(g_bytes_raw)
    stats["gating"]["bytes_used"] = float(g_bytes_used)
    stats["gating"]["avg_gate_used"] = (float(g_gate_sum) / float(g_tag_events_used)) if g_tag_events_used > 0 else 0.0
    stats["gating"]["avg_gate_bytes_weighted"] = (float(g_gate_bytes_wsum) / float(g_bytes_raw)) if g_bytes_raw > 0 else 0.0

    return ProfileData(
        chiplets=int(chiplets),
        page_bytes=int(prof_page_bytes),
        group_pages=int(gp),
        byte_model=byte_model,
        buffers=buffers,
        kernels=kernels,
        tag_page_bytes=tag_page,
        stats=stats,
    )



def profile_kernel_launch_mapping(profile_kernels: List[dict], launches: List[dict]) -> List[Optional[int]]:
    """Best-effort profile->launch mapping in program order.

    Exact (kernel name, global size, local size) matching is still preferred,
    but in the current mainline the Step2 metadata can go stale when
    benchmark.go changes the work-group geometry without regenerating every
    intermediate artifact. In that case the runtime profile and metrics are the
    source of truth and we must not drop them simply because the recorded local
    size no longer matches Step2.

    Matching strategy, all preserving program order and one-to-one assignment:
      1) exact (kernel, global, local)
      2) (kernel, global, total_wg)
      3) (kernel, global)
      4) kernel-name-only ordered fallback

    This lets the planner recover real runtime CU / duration truth from the
    seed/profile run while remaining conservative when multiple launches share
    the same name.
    """
    prof = profile_kernels or []
    la = launches or []

    def _launch_name(l: dict) -> str:
        return str(l.get("kernel", "") or "")

    def _profile_name(k: dict) -> str:
        return str(k.get("name", "") or "")

    def _launch_grid(l: dict) -> Tuple[int, int, int]:
        return _int3(l.get("global_size", [0, 0, 0]))

    def _profile_grid(k: dict) -> Tuple[int, int, int]:
        return _int3(k.get("grid_size", [0, 0, 0]))

    def _launch_local(l: dict) -> Tuple[int, int, int]:
        return _int3(l.get("local_size", [0, 0, 0]))

    def _profile_local(k: dict) -> Tuple[int, int, int]:
        return _int3(k.get("wg_size", [0, 0, 0]))

    def _prod_ceil_div(a: Tuple[int, int, int], b: Tuple[int, int, int]) -> int:
        total = 1
        for i in range(3):
            g = max(0, int(a[i]))
            w = max(1, int(b[i]))
            if g <= 0:
                return 0
            total *= int(math.ceil(float(g) / float(w)))
        return int(total)

    launch_total_wg = [_prod_ceil_div(_launch_grid(l), _launch_local(l)) for l in la]
    profile_total_wg = [_prod_ceil_div(_profile_grid(k), _profile_local(k)) for k in prof]

    mapping: List[Optional[int]] = [None for _ in range(len(prof))]
    used_launch = [False for _ in range(len(la))]

    def _assign_if(match_fn):
        next_search = 0
        for i, pk in enumerate(prof):
            nonlocal mapping, used_launch
            if mapping[i] is not None:
                try:
                    next_search = max(next_search, int(mapping[i]) + 1)
                except Exception:
                    pass
                continue
            found = None
            for t in range(next_search, len(la)):
                if used_launch[t]:
                    continue
                if match_fn(i, t):
                    found = t
                    break
            if found is None:
                continue
            mapping[i] = found
            used_launch[found] = True
            next_search = found + 1

    _assign_if(lambda i, t: (
        _launch_name(la[t]) == _profile_name(prof[i]) and
        _launch_grid(la[t]) == _profile_grid(prof[i]) and
        _launch_local(la[t]) == _profile_local(prof[i])
    ))
    _assign_if(lambda i, t: (
        _launch_name(la[t]) == _profile_name(prof[i]) and
        _launch_grid(la[t]) == _profile_grid(prof[i]) and
        int(launch_total_wg[t]) > 0 and int(launch_total_wg[t]) == int(profile_total_wg[i])
    ))
    _assign_if(lambda i, t: (
        _launch_name(la[t]) == _profile_name(prof[i]) and
        _launch_grid(la[t]) == _profile_grid(prof[i])
    ))
    _assign_if(lambda i, t: _launch_name(la[t]) == _profile_name(prof[i]))

    return mapping


def align_profile_kernels_to_launches(profile_kernels: List[dict], launches: List[dict]) -> Dict[str, object]:
    """Best-effort alignment of profile kernel events to Step2 launch list (dispatch order)."""
    prof = profile_kernels or []
    la = launches or []
    mapping = profile_kernel_launch_mapping(prof, la)
    matches = sum(1 for x in mapping if x is not None)
    mismatches = int(len(prof)) - int(matches)

    sample = []
    for i in range(min(12, len(prof))):
        k = prof[i]
        sample.append({
            "profile_index": i,
            "name": str(k.get("name", "")),
            "cmd_id": str(k.get("cmd_id", "")),
            "grid_size": list(_int3(k.get("grid_size", [0, 0, 0]))),
            "wg_size": list(_int3(k.get("wg_size", [0, 0, 0]))),
            "matched_launch_id": mapping[i],
        })

    return {
        "profile_kernels": int(len(prof)),
        "launches": int(len(la)),
        "matches": int(matches),
        "mismatches": int(mismatches),
        "sample": sample,
    }


def profile_kernels_by_launch_index(profile: Optional[ProfileData], launches: List[dict]) -> Dict[int, dict]:
    if profile is None:
        return {}
    prof = getattr(profile, "kernels", []) or []
    mapping = profile_kernel_launch_mapping(prof, launches)
    out: Dict[int, dict] = {}
    for i, launch_idx in enumerate(mapping):
        if launch_idx is None:
            continue
        out[int(launch_idx)] = prof[i]
    return out


def _safe_profile_float(v: object) -> float:
    try:
        return float(v or 0.0)
    except Exception:
        return 0.0


def _derive_active_time_sec_from_runtime(rt: Dict[str, object]) -> float:
    if not rt:
        return 0.0

    direct = _safe_profile_float(rt.get("active_time_sec", 0.0))
    if direct > 0.0:
        return direct

    start = _safe_profile_float(rt.get("first_dispatch_time_sec", 0.0))
    end = _safe_profile_float(rt.get("last_completion_time_sec", 0.0))
    if end > start:
        return end - start

    first_by = list(rt.get("first_dispatch_time_by_chiplet_sec", []) or [])
    last_by = list(rt.get("last_completion_time_by_chiplet_sec", []) or [])
    wg_count_by = [int(x or 0) for x in (rt.get("wg_count_by_chiplet", []) or [])]
    starts: List[float] = []
    ends: List[float] = []
    for i, cnt in enumerate(wg_count_by):
        if cnt <= 0:
            continue
        s = _safe_profile_float(first_by[i] if i < len(first_by) else 0.0)
        e = _safe_profile_float(last_by[i] if i < len(last_by) else 0.0)
        starts.append(s)
        ends.append(e)
    if starts and ends and max(ends) > min(starts):
        return max(ends) - min(starts)
    return 0.0


def _derive_active_time_sec_from_workgroups(pk: Dict[str, object]) -> float:
    starts: List[float] = []
    ends: List[float] = []
    for wg in (pk.get("workgroups", []) or []):
        if not isinstance(wg, dict):
            continue
        s = _safe_profile_float(wg.get("dispatch_time_sec", 0.0))
        e = _safe_profile_float(wg.get("completion_time_sec", 0.0))
        if e > s:
            starts.append(s)
            ends.append(e)
    if starts and ends and max(ends) > min(starts):
        return max(ends) - min(starts)
    return 0.0


def load_launch_runtime_profile(
    profile: Optional[ProfileData],
    launches: List[dict],
) -> Tuple[Dict[int, int], Dict[int, List[int]], Dict[int, int], Dict[str, object]]:
    """Load exact runtime resource usage / duration from profile.json.

    The simulator profiler emits per-launch runtime fields (peak active CUs,
    peak active CUs per chiplet, active time). The planner uses these values as
    the source of truth when present, instead of static CU estimation.
    """
    exact_cus: Dict[int, int] = {}
    exact_chiplet_cus: Dict[int, List[int]] = {}
    exact_duration_ps: Dict[int, int] = {}

    if profile is None:
        return exact_cus, exact_chiplet_cus, exact_duration_ps, {
            "available": False,
            "matched_launches": 0,
            "launches_with_peak_cus": 0,
            "launches_with_chiplet_cus": 0,
            "launches_with_active_time": 0,
            "launches_with_runtime_active_time": 0,
            "launches_with_wg_active_time": 0,
        }

    by_launch = profile_kernels_by_launch_index(profile, launches)
    peak_cus = 0
    chiplet_cus = 0
    active_time = 0
    wg_active_time = 0

    for idx, pk in by_launch.items():
        rt = (pk.get("runtime", {}) or {})

        chip = [int(x or 0) for x in (rt.get("peak_active_cus_by_chiplet", []) or [])]
        if not any(v > 0 for v in chip):
            chip = [int(x or 0) for x in (rt.get("used_cus_by_chiplet", []) or [])]
        if any(v > 0 for v in chip):
            exact_chiplet_cus[int(idx)] = chip
            chiplet_cus += 1

        peak = int(rt.get("peak_active_cus", 0) or 0)
        if peak <= 0:
            peak = int(rt.get("used_cus", 0) or 0)
        if peak <= 0 and any(v > 0 for v in chip):
            peak = int(sum(max(0, int(v)) for v in chip))
        if peak > 0:
            exact_cus[int(idx)] = int(peak)
            peak_cus += 1

        dur_sec = _derive_active_time_sec_from_runtime(rt)
        if dur_sec > 0.0:
            exact_duration_ps[int(idx)] = max(1, int(round(dur_sec * 1.0e12)))
            active_time += 1
        else:
            wg_dur_sec = _derive_active_time_sec_from_workgroups(pk)
            if wg_dur_sec > 0.0:
                exact_duration_ps[int(idx)] = max(1, int(round(wg_dur_sec * 1.0e12)))
                wg_active_time += 1

    dbg = {
        "available": True,
        "matched_launches": int(len(by_launch)),
        "launches_with_peak_cus": int(peak_cus),
        "launches_with_chiplet_cus": int(chiplet_cus),
        "launches_with_active_time": int(active_time + wg_active_time),
        "launches_with_runtime_active_time": int(active_time),
        "launches_with_wg_active_time": int(wg_active_time),
    }
    return exact_cus, exact_chiplet_cus, exact_duration_ps, dbg


def normalize_launches_with_runtime_geometry(
    launches: List[dict],
    profile: Optional[ProfileData],
) -> Tuple[List[dict], Dict[str, object]]:
    """Overlay runtime launch geometry onto Step2 launches when profile truth is available.

    In the current workflow, benchmark.go can change the real WG size while an
    older global.json still carries stale local_size / num_wg_total. Launch-hint
    search must use the runtime truth, otherwise itemized windows and split
    decisions are computed in the wrong WG space.
    """
    launches_in = launches or []
    if profile is None:
        return [copy.deepcopy(l) for l in launches_in], {
            "available": False,
            "matched_launches": 0,
            "overridden_launches": 0,
            "sample": [],
        }

    by_launch = profile_kernels_by_launch_index(profile, launches_in)
    out: List[dict] = []
    overridden = 0
    sample: List[dict] = []

    for idx, launch in enumerate(launches_in):
        new_launch = copy.deepcopy(launch)
        pk = by_launch.get(idx)
        if isinstance(pk, dict):
            rt_grid = _int3(pk.get("grid_size", [0, 0, 0]))
            rt_local = _int3(pk.get("wg_size", [0, 0, 0]))
            if all(v > 0 for v in rt_grid) and all(v > 0 for v in rt_local):
                old_grid = _int3(new_launch.get("global_size", [0, 0, 0]))
                old_local = _int3(new_launch.get("local_size", [0, 0, 0]))
                old_total = int(new_launch.get("num_wg_total", 0) or _num_wg_total_from_sizes(old_grid, old_local) or 0)
                rt_total = int(_num_wg_total_from_sizes(rt_grid, rt_local) or 0)
                if old_grid != rt_grid or old_local != rt_local or old_total != rt_total:
                    overridden += 1
                    if len(sample) < 24:
                        sample.append({
                            "launch_id": int(idx),
                            "kernel": str(new_launch.get("kernel", "") or ""),
                            "old_global_size": list(old_grid),
                            "old_local_size": list(old_local),
                            "old_num_wg_total": int(old_total),
                            "runtime_global_size": list(rt_grid),
                            "runtime_local_size": list(rt_local),
                            "runtime_num_wg_total": int(rt_total),
                        })
                new_launch["global_size"] = [int(x) for x in rt_grid]
                new_launch["local_size"] = [int(x) for x in rt_local]
                if rt_total > 0:
                    new_launch["num_wg_total"] = int(rt_total)
        out.append(new_launch)

    dbg = {
        "available": True,
        "matched_launches": int(len(by_launch)),
        "overridden_launches": int(overridden),
        "sample": sample,
    }
    return out, dbg


def load_base_plan_pages_map(
    path: str,
    *,
    expect_page_bytes: int,
    expect_chiplets: int,
) -> Tuple[Dict[str, List[int]], Dict[str, object]]:
    """Load an existing plan.json and return tag->pages[] mapping for stability regularization (Phase3-2B)."""
    try:
        p = json.load(open(path, "r"))
    except Exception as e:
        raise SystemExit(f"ERROR: failed to read --stability-base-plan {path}: {e}")

    hw = p.get("hardware", {}) or {}
    pb = int(hw.get("page_bytes", 0) or 0)
    ch = int(hw.get("chiplets", 0) or 0)
    if pb > 0 and expect_page_bytes > 0 and pb != int(expect_page_bytes):
        raise SystemExit(
            f"ERROR: base plan page_bytes={pb} does not match planner page_bytes={expect_page_bytes}. "
            f"Please use a base plan generated with the same --page-bytes."
        )
    if ch > 0 and expect_chiplets > 0 and ch != int(expect_chiplets):
        raise SystemExit(
            f"ERROR: base plan chiplets={ch} does not match planner chiplets={expect_chiplets}."
        )

    bp = p.get("buffer_placement", {}) or {}
    tag2pages: Dict[str, List[int]] = {}
    for tag, info in bp.items():
        try:
            pages = (info.get("pages", []) or [])
        except Exception:
            pages = []
        if not isinstance(pages, list) or not pages:
            continue
        # sanitize chiplet ids
        out = []
        for v in pages:
            try:
                c = int(v)
            except Exception:
                c = 0
            if expect_chiplets > 0:
                c = max(0, min(int(expect_chiplets) - 1, c))
            out.append(c)
        tag2pages[str(tag)] = out

    meta = {
        "path": path,
        "page_bytes": int(pb) if pb > 0 else int(expect_page_bytes),
        "chiplets": int(ch) if ch > 0 else int(expect_chiplets),
        "tags": int(len(tag2pages)),
    }
    return tag2pages, meta



# -----------------------------
# Min-Cost Flow (Successive Shortest Path + potentials)
# -----------------------------

class _Edge:
    __slots__ = ("to", "rev", "cap", "cost")
    def __init__(self, to: int, rev: int, cap: int, cost: int):
        self.to = to
        self.rev = rev
        self.cap = cap
        self.cost = cost

class MinCostFlow:
    """Min-cost max-flow with non-negative costs."""
    def __init__(self, n: int):
        self.n = int(n)
        self.g: List[List[_Edge]] = [[] for _ in range(self.n)]

    def add_edge(self, fr: int, to: int, cap: int, cost: int) -> None:
        fr = int(fr); to = int(to); cap = int(cap); cost = int(cost)
        fwd = _Edge(to, len(self.g[to]), cap, cost)
        rev = _Edge(fr, len(self.g[fr]), 0, -cost)
        self.g[fr].append(fwd)
        self.g[to].append(rev)

    def min_cost_flow(self, s: int, t: int, f: int) -> Tuple[int, int]:
        import heapq
        n = self.n
        s = int(s); t = int(t); f = int(f)
        INF = 10**30

        potential = [0]*n
        prevv = [0]*n
        preve = [0]*n

        flow_sent = 0
        cost_total = 0

        while f > 0:
            dist = [INF]*n
            dist[s] = 0
            pq = [(0, s)]

            while pq:
                d, v = heapq.heappop(pq)
                if dist[v] < d:
                    continue
                for i, e in enumerate(self.g[v]):
                    if e.cap <= 0:
                        continue
                    nd = d + e.cost + potential[v] - potential[e.to]
                    if nd < dist[e.to]:
                        dist[e.to] = nd
                        prevv[e.to] = v
                        preve[e.to] = i
                        heapq.heappush(pq, (nd, e.to))

            if dist[t] == INF:
                break

            for v in range(n):
                if dist[v] < INF:
                    potential[v] += dist[v]

            addf = f
            v = t
            while v != s:
                pv = prevv[v]
                pe = preve[v]
                addf = min(addf, self.g[pv][pe].cap)
                v = pv

            v = t
            while v != s:
                pv = prevv[v]
                pe = preve[v]
                e = self.g[pv][pe]
                e.cap -= addf
                self.g[v][e.rev].cap += addf
                v = pv

            f -= addf
            flow_sent += addf
            cost_total += addf * potential[t]

        return flow_sent, cost_total


# -----------------------------
# Step1 parsing
# -----------------------------

@dataclass
class KernelArgAccess:
    loads: int = 0
    stores: int = 0
    stride_kind: str = "unknown"   # "contig" | "strided" | "unknown"
    has_gid_x: bool = False
    has_gid_y: bool = False
    elem_bytes: int = 0            # best-effort (max) element size in bytes
    has_addrec: bool = False       # saw LLVM SCEV addrec (loop)
    hard_unknown: bool = False     # heuristic: address expression had complex unknown terms


def _lower(s: str) -> str:
    return s.lower() if isinstance(s, str) else ""


def load_step1_kernels(path: str) -> Dict[str, Dict[int, KernelArgAccess]]:
    """
    Returns: kernel_name -> arg_index -> KernelArgAccess

    v4 adds:
      - elem_bytes (max)
      - has_addrec
      - hard_unknown (heuristic)
    """
    data = json.load(open(path, "r"))
    out: Dict[str, Dict[int, KernelArgAccess]] = {}

    for k in data.get("kernels", []):
        name = k.get("name", "")
        if not name:
            continue
        argmap: Dict[int, KernelArgAccess] = {}

        for ma in k.get("mem_accesses", []):
            if not ma.get("is_global", False):
                continue
            arg_idx = int(ma.get("buffer_arg_index", -1))
            if arg_idx < 0:
                continue

            a = argmap.get(arg_idx, KernelArgAccess())

            kind = _lower(ma.get("kind", ""))
            if kind == "load":
                a.loads += 1
            elif kind == "store":
                a.stores += 1

            coeffs = ma.get("coeffs", {}) or {}
            if "gid_x" in coeffs and coeffs.get("gid_x", 0) != 0:
                a.has_gid_x = True
            if "gid_y" in coeffs and coeffs.get("gid_y", 0) != 0:
                a.has_gid_y = True

            eb = int(ma.get("elem_bytes", 0) or 0)
            if eb > a.elem_bytes:
                a.elem_bytes = eb

            addrec = ma.get("addrec")
            if isinstance(addrec, dict):
                a.has_addrec = True
                step = addrec.get("step_linear", {})
                if isinstance(step, dict) and step.get("complete_linear", False):
                    step_const = int(step.get("constant", 0))
                    step_coeffs = step.get("coeffs", {}) or {}
                    if step_coeffs:
                        a.stride_kind = "strided"
                    else:
                        # constant step
                        if 1 <= abs(step_const) <= 128:
                            a.stride_kind = "contig"

            # "hard unknown" heuristic: unknown_terms has more than just a base pointer (%0/%1)
            uts = ma.get("unknown_terms", []) or []
            # If any term contains an operator or multiple tokens, treat as hard unknown.
            for ut in uts:
                if not isinstance(ut, str):
                    continue
                s = ut.strip()
                if not s:
                    continue
                # base-pointer-only terms are usually like "%0" or "%1"
                if s.startswith("%") and s[1:].isdigit():
                    continue
                # complex
                if any(ch in s for ch in ("*", "+", "-", "<<", ">>", "gid", "trunc", "sext", "zext")):
                    a.hard_unknown = True

            argmap[arg_idx] = a

        out[name] = argmap

    return out




def load_step1_access_records(path: str) -> Dict[str, Dict[int, List[dict]]]:
    """
    Returns: kernel_name -> buffer_arg_index -> list[mem_access dict]

    We keep the raw mem_access records from Step1 (kernels.json) so Phase2 can
    build a page->chiplet affinity model using the extracted coefficients.

    Only global (addrspace==1) accesses are included.
    """
    data = json.load(open(path, "r"))
    out: Dict[str, Dict[int, List[dict]]] = {}

    for k in data.get("kernels", []):
        name = k.get("name", "")
        if not name:
            continue
        amap: Dict[int, List[dict]] = {}
        for ma in k.get("mem_accesses", []) or []:
            if not ma.get("is_global", False):
                continue
            arg_idx = int(ma.get("buffer_arg_index", -1))
            if arg_idx < 0:
                continue
            amap.setdefault(arg_idx, []).append(ma)
        out[name] = amap

    return out

# -----------------------------
# Step2 parsing helpers
# -----------------------------

def load_step2_global(path: str) -> dict:
    return json.load(open(path, "r"))


def total_workgroups(launch: dict) -> int:
    gs = launch.get("global_size", []) or []
    ls = launch.get("local_size", []) or []
    if len(gs) != 3 or len(ls) != 3:
        return 0
    total = 1
    for d in range(3):
        g = int(gs[d]); l = int(ls[d])
        if g <= 0 or l <= 0:
            return 0
        total *= int((g + l - 1) // l)
    return int(total)



def workgroups_per_dim(launch: dict) -> Tuple[int, int, int]:
    """Return (#WG in X, Y, Z) computed as ceil(global/local) per dim."""
    gs = launch.get("global_size", []) or []
    ls = launch.get("local_size", []) or []
    if len(gs) != 3 or len(ls) != 3:
        return (0, 0, 0)
    out = []
    for d in range(3):
        g = int(gs[d]); l = int(ls[d])
        if g <= 0 or l <= 0:
            out.append(0)
        else:
            out.append(int((g + l - 1) // l))
    return (out[0], out[1], out[2])

def total_workitems(launch: dict) -> int:
    gs = launch.get("global_size", []) or []
    if len(gs) != 3:
        return 0
    total = 1
    for d in range(3):
        g = int(gs[d])
        if g <= 0:
            return 0
        total *= g
    return int(total)


def local_workitems_per_wg(launch: dict) -> int:
    ls = launch.get("local_size", []) or []
    if len(ls) != 3:
        return 0
    total = 1
    for d in range(3):
        l = int(ls[d])
        if l <= 0:
            return 0
        total *= l
    return int(total)


# -----------------------------
# Planner v4 cost model
# -----------------------------

@dataclass
class TrafficWeight:
    req_bytes: float = 0.0
    rsp_bytes: float = 0.0
    def total(self) -> float:
        return float(self.req_bytes) + float(self.rsp_bytes)


def dynamic_factor(launch: dict, mode: str) -> int:
    m = _lower(mode)
    if m == "wg":
        return max(1, total_workgroups(launch))
    if m == "wi":
        return max(1, total_workitems(launch))
    return 1


def _ceil_div(a: int, b: int) -> int:
    a = int(a); b = int(b)
    if b <= 0:
        return 0
    return int((a + b - 1) // b)


def _parse_trailing_int(s: str) -> Optional[int]:
    s = _lower(s)
    if not s:
        return None
    i = len(s) - 1
    while i >= 0 and s[i].isdigit():
        i -= 1
    if i == len(s) - 1:
        return None
    try:
        v = int(s[i+1:])
        return v if v > 0 else None
    except Exception:
        return None



def _active_chiplets_xdiv(num_wgx: int, num_chiplets: int) -> int:
    """Match mgpusim's Xdiv active-chiplet behavior.

    Simulator logic (lasp_alg.go):
      divisor = ceilDiv(numWGX, numChiplets)
      chiplet = wg.IDX / divisor  (capped to numChiplets-1)

    That means the number of *used* chiplets can be < numChiplets when numWGX is only
    slightly larger than numChiplets.
    """
    num_wgx = max(1, int(num_wgx))
    num_chiplets = max(1, int(num_chiplets))
    divisor = _ceil_div(num_wgx, num_chiplets)
    if divisor <= 0:
        divisor = 1
    used = _ceil_div(num_wgx, divisor)
    return max(1, min(num_chiplets, used))


def _active_chiplets_ydiv(num_wgy: int, num_chiplets: int) -> int:
    """Match mgpusim's Ydiv active-chiplet behavior.

    Simulator logic (lasp_alg.go):
      divisor = numWGY / numChiplets  (floor), guard divisor>=1
      chiplet = wg.IDY / divisor (capped)

    For numWGY >= numChiplets this typically uses all chiplets; for tiny grids it may use fewer.
    """
    num_wgy = max(1, int(num_wgy))
    num_chiplets = max(1, int(num_chiplets))
    divisor = int(num_wgy // num_chiplets)
    if divisor <= 0:
        divisor = 1
    used = _ceil_div(num_wgy, divisor)
    return max(1, min(num_chiplets, used))


def active_chiplets_for_launch(
    partition: str,
    launch: dict,
    num_chiplets: int,
) -> int:
    """Estimate how many chiplets a launch will actually use under the simulator's LASP mapping.

    This MUST stay consistent with mgpusim's lasp_alg.go assignWGToChiplet() and the
    validation gate in hybrid_alg.go (only xdiv/ydiv/xblk/yblk are accepted for LASP).

    Why this matters:
      - If we overestimate active chiplets, the planner may spread pages onto chiplets that will
        run *no* work-groups for this kernel, causing unnecessary remote traffic.
    """
    if num_chiplets <= 0:
        return 1

    wg_total = total_workgroups(launch)
    if wg_total <= 0:
        # unknown -> assume all chiplets could be touched
        return int(num_chiplets)

    wgx, wgy, _ = workgroups_per_dim(launch)
    wgx = max(1, int(wgx))
    wgy = max(1, int(wgy))

    p = _lower(partition)

    # Xdiv/Ydiv
    if p.startswith("xdiv"):
        return _active_chiplets_xdiv(wgx, num_chiplets)
    if p.startswith("ydiv"):
        return _active_chiplets_ydiv(wgy, num_chiplets)

    # XblkN/YblkN: chiplet = (wg_id/blk) % numChiplets
    if p.startswith("xblk"):
        blk = _parse_trailing_int(p) or 1
        blocks = _ceil_div(wgx, blk)
        return max(1, min(int(num_chiplets), int(blocks)))
    if p.startswith("yblk"):
        blk = _parse_trailing_int(p) or 1
        blocks = _ceil_div(wgy, blk)
        return max(1, min(int(num_chiplets), int(blocks)))

    # Fallback (should not happen for LASP once we normalize partitions):
    return max(1, min(int(num_chiplets), int(wg_total)))


def _kernel_profile_page_sets(profile_kernel: Optional[dict]) -> Tuple[Dict[str, Set[int]], Dict[str, Set[int]]]:
    reads: Dict[str, Set[int]] = {}
    writes: Dict[str, Set[int]] = {}
    if not profile_kernel:
        return reads, writes

    for tag, tinfo in (profile_kernel.get("tags", {}) or {}).items():
        if not tag:
            continue
        rset: Set[int] = set()
        wset: Set[int] = set()
        for pr in (tinfo.get("pages", []) or []):
            try:
                page = int(pr.get("page", 0) or 0)
            except Exception:
                continue
            rr = pr.get("r", []) or []
            ww = pr.get("w", []) or []
            if any((float(x) != 0.0) for x in rr):
                rset.add(page)
            if any((float(x) != 0.0) for x in ww):
                wset.add(page)
        if rset:
            reads[str(tag)] = rset
        if wset:
            writes[str(tag)] = wset
    return reads, writes


def build_launch_hazard_graph(
    launches: List[dict],
    profile_kernels_by_launch: Optional[Dict[int, dict]] = None,
) -> Tuple[List[Set[int]], List[Set[int]], List[dict]]:
    """Build immediate per-launch dependencies from read/write hazards.

    When profile page sets are available, the graph is built at *tag + page* granularity
    instead of tag-only granularity. This removes false WAW/WAR/RAW edges caused by
    different launches touching disjoint pages of the same tagged buffer.

    For tags/pages that are absent from the profile, we conservatively fall back to the
    Step2 tag-level dependency model so correctness is preserved.
    """
    n = len(launches or [])
    preds: List[Set[int]] = [set() for _ in range(n)]
    succs: List[Set[int]] = [set() for _ in range(n)]
    edges: List[dict] = []
    edge_seen: Set[Tuple[int, int, str, str]] = set()

    last_write_any: Dict[str, int] = {}
    last_write_pages: Dict[str, Dict[int, int]] = {}
    last_reads_any: Dict[str, List[int]] = {}
    last_read_pages: Dict[str, Dict[int, List[int]]] = {}

    def _uniq_tags(xs: List[str]) -> List[str]:
        out: List[str] = []
        seen: Set[str] = set()
        for x in xs or []:
            t = str(x or "")
            if not t or t in seen:
                continue
            seen.add(t)
            out.append(t)
        return out

    def _add_edge(src: int, dst: int, tag: str, hazard: str):
        if src is None or src < 0 or dst < 0 or src == dst:
            return
        key = (int(src), int(dst), str(tag), str(hazard))
        if key in edge_seen:
            return
        edge_seen.add(key)
        preds[dst].add(int(src))
        succs[src].add(int(dst))
        edges.append({"from": int(src), "to": int(dst), "buffer": str(tag), "type": str(hazard)})

    for idx, l in enumerate(launches or []):
        reads = _uniq_tags(l.get("reads", []) or [])
        writes = _uniq_tags(l.get("writes", []) or [])

        pk = None
        if profile_kernels_by_launch:
            pk = profile_kernels_by_launch.get(int(idx), None)
        prof_reads, prof_writes = _kernel_profile_page_sets(pk)

        # RAW hazards: prior write -> current read
        for tag in reads:
            pages = prof_reads.get(tag, None)
            if pages is None:
                if tag in last_write_any:
                    _add_edge(last_write_any[tag], idx, tag, "RAW")
                for src in sorted(set((last_write_pages.get(tag, {}) or {}).values())):
                    _add_edge(src, idx, tag, "RAW")
                continue

            per_page = last_write_pages.get(tag, {}) or {}
            any_writer = last_write_any.get(tag, None)
            for page in pages:
                src = per_page.get(int(page), None)
                if src is not None:
                    _add_edge(src, idx, tag, "RAW")
                elif any_writer is not None:
                    _add_edge(any_writer, idx, tag, "RAW")

        # WAW / WAR hazards: prior write/read -> current write
        for tag in writes:
            pages = prof_writes.get(tag, None)
            if pages is None:
                if tag in last_write_any:
                    _add_edge(last_write_any[tag], idx, tag, "WAW")
                for src in sorted(set((last_write_pages.get(tag, {}) or {}).values())):
                    _add_edge(src, idx, tag, "WAW")

                for src in last_reads_any.get(tag, []) or []:
                    _add_edge(src, idx, tag, "WAR")
                per_page_reads = last_read_pages.get(tag, {}) or {}
                war_srcs: Set[int] = set()
                for readers in per_page_reads.values():
                    for src in readers or []:
                        war_srcs.add(int(src))
                for src in sorted(war_srcs):
                    _add_edge(src, idx, tag, "WAR")
                continue

            per_page_writers = last_write_pages.get(tag, {}) or {}
            any_writer = last_write_any.get(tag, None)
            for page in pages:
                src = per_page_writers.get(int(page), None)
                if src is not None:
                    _add_edge(src, idx, tag, "WAW")
                elif any_writer is not None:
                    _add_edge(any_writer, idx, tag, "WAW")

            for src in last_reads_any.get(tag, []) or []:
                _add_edge(src, idx, tag, "WAR")
            per_page_reads = last_read_pages.get(tag, {}) or {}
            for page in pages:
                for src in per_page_reads.get(int(page), []) or []:
                    _add_edge(src, idx, tag, "WAR")

        # Update read state.
        for tag in reads:
            pages = prof_reads.get(tag, None)
            if pages is None:
                last_reads_any.setdefault(tag, []).append(idx)
                continue
            d = last_read_pages.setdefault(tag, {})
            for page in pages:
                d.setdefault(int(page), []).append(idx)

        # Update write state.
        for tag in writes:
            pages = prof_writes.get(tag, None)
            if pages is None:
                last_write_any[tag] = idx
                last_write_pages[tag] = {}
                last_reads_any[tag] = []
                last_read_pages[tag] = {}
                continue

            d = last_write_pages.setdefault(tag, {})
            rp = last_read_pages.setdefault(tag, {})
            for page in pages:
                d[int(page)] = idx
                rp[int(page)] = []

    return preds, succs, edges


def _load_launch_metrics_rows(metrics_csv_path: str, launches: List[dict]) -> List[dict]:
    """Load metrics_kernels.csv in program order.

    Row-order alignment is intentionally used because kernel names repeat across
    layers/blocks. The helper also verifies the row count so callers can safely
    use index-based lookup for launch 0..N-1.
    """
    if not metrics_csv_path:
        return []
    try:
        with open(metrics_csv_path, "r", newline="") as f:
            rows = list(csv.DictReader(f))
    except Exception as e:
        print(f"WARNING: failed to read --launch-metrics-csv {metrics_csv_path}: {e}")
        return []

    if len(rows) != len(launches or []):
        print(
            f"WARNING: --launch-metrics-csv rows={len(rows)} does not match launches={len(launches or [])}; "
            f"falling back to static critical-path / remote-cost weights"
        )
        return []

    mismatch = 0
    for idx, row in enumerate(rows):
        k_csv = str(row.get("kernel_name", "") or "")
        k_ref = str((launches[idx] or {}).get("kernel", "") or "")
        if k_csv and k_ref and k_csv != k_ref:
            mismatch += 1
    if mismatch > 0:
        print(
            f"WARNING: --launch-metrics-csv kernel-name mismatches on {mismatch} rows; "
            f"using row-order alignment anyway"
        )
    return rows


def _row_float(row: Optional[dict], key: str) -> float:
    if not isinstance(row, dict):
        return 0.0
    try:
        return float(row.get(key, 0.0) or 0.0)
    except Exception:
        return 0.0


def _row_int(row: Optional[dict], key: str) -> int:
    if not isinstance(row, dict):
        return 0
    try:
        return int(float(row.get(key, 0) or 0))
    except Exception:
        return 0


def load_launch_latency_ps(metrics_csv_path: str, launches: List[dict]) -> Dict[int, int]:
    rows = _load_launch_metrics_rows(metrics_csv_path, launches)
    out: Dict[int, int] = {}
    for idx, row in enumerate(rows):
        lat_sec = _row_float(row, "latency_sec")
        out[idx] = max(1, int(round(lat_sec * 1.0e12)))
    return out


def load_launch_metric_proxies(metrics_csv_path: str, launches: List[dict]) -> Dict[int, dict]:
    """Load per-launch resource proxies from metrics_kernels.csv.

    These values remain launch-order aligned with the original program launches.
    They are used as a conservative fallback when profile-derived page-level
    remote-byte estimation is unavailable or incomplete.
    """
    rows = _load_launch_metrics_rows(metrics_csv_path, launches)
    out: Dict[int, dict] = {}
    for idx, row in enumerate(rows):
        local_b = max(0, _row_int(row, "local_bytes_est"))
        remote_b = max(0, _row_int(row, "remote_bytes_est"))
        external_b = max(0, _row_int(row, "external_bytes_est"))
        total_b = int(local_b + remote_b + external_b)
        out[idx] = {
            "kernel": str(row.get("kernel_name", "") or ""),
            "latency_ps": max(1, int(round(_row_float(row, "latency_sec") * 1.0e12))),
            "local_bytes": int(local_b),
            "remote_bytes": int(remote_b),
            "external_bytes": int(external_b),
            "bytes": int(total_b),
            "remote_ratio": float(remote_b) / float(total_b) if total_b > 0 else 0.0,
            "ptw_translation_reqs": max(0, _row_int(row, "ptw_translation_reqs")),
            "l2_tlb_miss": max(0, _row_int(row, "l2_tlb_miss")),
        }
    return out


def estimate_launch_cus(
    launch: dict,
    partition: str,
    *,
    num_chiplets: int,
    sa_per_chiplet: int,
    cu_per_sa: int,
) -> int:
    """Estimate how many CUs a launch can keep busy at once.

    This is intentionally conservative and hardware-aware:
      - total WG count caps demand
      - partition determines how many chiplets are actually active
      - per-chiplet CU capacity bounds each active chiplet's contribution

    It does *not* try to model VGPR / SGPR / LDS occupancy. The simulator still enforces the
    exact CU resource constraints at runtime; this estimate is only used by the overlap manager
    to avoid obviously over-packing kernels.
    """
    num_wg = int(total_workgroups(launch))
    if num_wg <= 0:
        return 1

    chiplets = max(1, int(num_chiplets) or 1)
    cu_per_chiplet = int(sa_per_chiplet or 0) * int(cu_per_sa or 0)
    if cu_per_chiplet <= 0:
        cu_per_chiplet = 1
    total_cus = max(1, chiplets * cu_per_chiplet)

    active = active_chiplets_for_launch(partition, launch, chiplets)
    active = max(1, min(chiplets, int(active)))

    est_per_chiplet = _ceil_div(num_wg, active)
    est_total = int(active) * min(int(cu_per_chiplet), int(est_per_chiplet))
    est_total = max(1, min(total_cus, est_total))
    return int(est_total)


def static_launch_duration_ps(launch: dict) -> int:
    """Fallback duration model used when no kernel metrics CSV is provided."""
    num_wg = int(total_workgroups(launch))
    if num_wg <= 0:
        num_wg = 1
    wgx, wgy, wgz = workgroups_per_dim(launch)
    shape_bonus = max(1, int(wgx) * max(1, int(wgy)) * max(1, int(wgz)))
    return max(1, int(num_wg + shape_bonus))


def compute_launch_bottom_levels(
    launches: List[dict],
    succs: List[Set[int]],
    duration_ps: Dict[int, int],
) -> List[int]:
    n = len(launches or [])
    bottom = [0 for _ in range(n)]
    for idx in range(n - 1, -1, -1):
        own = int(duration_ps.get(idx, 0) or 0)
        if own <= 0:
            own = static_launch_duration_ps(launches[idx])
        child = 0
        for s in succs[idx] or []:
            child = max(child, int(bottom[s]))
        bottom[idx] = int(own + child)
    return bottom


def simulate_launch_pipeline(
    launches: List[dict],
    preds: List[Set[int]],
    succs: List[Set[int]],
    priority: List[int],
    est_cus: List[int],
    duration_ps: Dict[int, int],
    *,
    total_cus: int,
    num_chiplets: int = 0,
    chiplet_cus: Optional[List[List[int]]] = None,
) -> dict:
    """Simple list-scheduling simulation for debug / sanity only."""
    n = len(launches or [])
    if n == 0:
        return {"makespan_ps": 0, "started": 0, "completed": 0, "max_parallel": 0, "timeline": []}

    ready: Set[int] = {i for i in range(n) if not preds[i]}
    started: Set[int] = set()
    completed: Set[int] = set()
    active: List[Tuple[int, int]] = []  # (end_ps, launch_id)
    active_chiplet_cus: Dict[int, List[int]] = {}
    timeline: List[dict] = []
    avail = max(1, int(total_cus) or 1)
    per_chiplet_cap = 0
    avail_by_chiplet: Optional[List[int]] = None
    if chiplet_cus and int(num_chiplets) > 0:
        per_chiplet_cap = max(1, int(total_cus) // int(num_chiplets))
        avail_by_chiplet = [int(per_chiplet_cap) for _ in range(int(num_chiplets))]
    now = 0
    max_parallel = 0

    def _fits(i: int) -> bool:
        nonlocal avail, avail_by_chiplet
        need = max(1, int(est_cus[i] or 1))
        vec = []
        if chiplet_cus and i < len(chiplet_cus):
            vec = [int(x or 0) for x in (chiplet_cus[i] or [])]
        if avail_by_chiplet is not None and any(v > 0 for v in vec):
            for c in range(min(len(avail_by_chiplet), len(vec))):
                if int(vec[c]) > int(avail_by_chiplet[c]):
                    return False
            return True
        return need <= avail

    def _reserve(i: int):
        nonlocal avail, avail_by_chiplet
        need = max(1, int(est_cus[i] or 1))
        vec = []
        if chiplet_cus and i < len(chiplet_cus):
            vec = [int(x or 0) for x in (chiplet_cus[i] or [])]
        if avail_by_chiplet is not None and any(v > 0 for v in vec):
            for c in range(min(len(avail_by_chiplet), len(vec))):
                avail_by_chiplet[c] -= int(vec[c])
            active_chiplet_cus[i] = vec
        else:
            avail -= need
            active_chiplet_cus[i] = []

    def _release(i: int):
        nonlocal avail, avail_by_chiplet
        vec = active_chiplet_cus.pop(i, [])
        if avail_by_chiplet is not None and any(v > 0 for v in vec):
            for c in range(min(len(avail_by_chiplet), len(vec))):
                avail_by_chiplet[c] += int(vec[c])
            return
        avail += max(1, int(est_cus[i] or 1))

    def _start_candidates(curr_t: int) -> bool:
        nonlocal max_parallel
        candidates = [i for i in ready if i not in started]
        candidates.sort(key=lambda i: (-int(priority[i]), i))
        for i in candidates:
            if not _fits(i):
                continue
            dur = int(duration_ps.get(i, 0) or 0)
            if dur <= 0:
                dur = static_launch_duration_ps(launches[i])
            _reserve(i)
            started.add(i)
            heapq.heappush(active, (curr_t + dur, i))
            timeline.append({
                "launch_id": int(i),
                "kernel": str((launches[i] or {}).get("kernel", "") or ""),
                "start_ps": int(curr_t),
                "end_ps": int(curr_t + dur),
                "est_cus": int(max(1, int(est_cus[i] or 1))),
                "chiplet_cus": list(active_chiplet_cus.get(i, []) or []),
                "priority": int(priority[i]),
            })
            max_parallel = max(max_parallel, len(active))
            return True
        return False

    while len(completed) < n:
        while _start_candidates(now):
            pass

        if not active:
            break

        end_t, done_i = heapq.heappop(active)
        now = int(end_t)
        _release(done_i)
        completed.add(done_i)

        for s in succs[done_i] or []:
            if s in completed or s in started:
                continue
            if all((p in completed) for p in preds[s]):
                ready.add(s)

    return {
        "makespan_ps": int(now),
        "started": int(len(started)),
        "completed": int(len(completed)),
        "max_parallel": int(max_parallel),
        "timeline": timeline,
    }


def build_launch_dependency_graph(
    launches: List[dict],
    global_graph: Optional[dict] = None,
    profile_kernels_by_launch: Optional[Dict[int, dict]] = None,
) -> Tuple[List[Set[int]], List[Set[int]], List[dict], str]:
    """Build the launch DAG used by the runtime pipeline manager.

    Preferred source of truth: Step2's explicit program dependency graph
    (global.json -> graph.edges). That graph already captures the benchmark's
    true dataflow structure and is the closest representation of the DAG the
    user wants to reconstruct.

    If the graph is missing, we fall back to the older page-hazard builder so
    the planner remains usable.
    """
    n = len(launches or [])
    preds: List[Set[int]] = [set() for _ in range(n)]
    succs: List[Set[int]] = [set() for _ in range(n)]

    raw_edges = []
    if isinstance(global_graph, dict):
        raw_edges = list(global_graph.get("edges", []) or [])

    clean_edges: List[dict] = []
    edge_seen: Set[Tuple[int, int, str, str]] = set()
    for e in raw_edges:
        try:
            src = int(e.get("from", -1))
            dst = int(e.get("to", -1))
        except Exception:
            continue
        if src < 0 or dst < 0 or src >= n or dst >= n or src == dst:
            continue
        buf = str(e.get("buffer", "") or "")
        hty = str(e.get("type", "RAW") or "RAW")
        key = (src, dst, buf, hty)
        if key in edge_seen:
            continue
        edge_seen.add(key)
        preds[dst].add(src)
        succs[src].add(dst)
        clean_edges.append({"from": src, "to": dst, "buffer": buf, "type": hty})

    if clean_edges:
        clean_edges.sort(key=lambda x: (int(x.get("to", 0)), int(x.get("from", 0)), str(x.get("buffer", "")), str(x.get("type", ""))))
        return preds, succs, clean_edges, "step2.graph.edges"

    preds, succs, haz_edges = build_launch_hazard_graph(launches, profile_kernels_by_launch)
    return preds, succs, haz_edges, "fallback.page_hazards"


def topological_launch_order(preds: List[Set[int]], succs: List[Set[int]]) -> List[int]:
    n = len(preds or [])
    indeg = [len(preds[i] or []) for i in range(n)]
    ready = [i for i in range(n) if indeg[i] == 0]
    heapq.heapify(ready)
    order: List[int] = []
    while ready:
        i = heapq.heappop(ready)
        order.append(i)
        for s in sorted(succs[i] or []):
            indeg[s] -= 1
            if indeg[s] == 0:
                heapq.heappush(ready, s)
    if len(order) != n:
        return list(range(n))
    return order


def compute_launch_timing_windows(
    launches: List[dict],
    preds: List[Set[int]],
    succs: List[Set[int]],
    duration_ps: Dict[int, int],
) -> Dict[str, object]:
    n = len(launches or [])
    order = topological_launch_order(preds, succs)
    dur = [0 for _ in range(n)]
    for i in range(n):
        d = int(duration_ps.get(i, 0) or 0)
        if d <= 0:
            d = static_launch_duration_ps(launches[i])
        dur[i] = max(1, d)

    estart = [0 for _ in range(n)]
    efinish = [0 for _ in range(n)]
    for i in order:
        if preds[i]:
            estart[i] = max(efinish[p] for p in preds[i])
        efinish[i] = estart[i] + dur[i]

    makespan = max(efinish, default=0)
    lfinish = [makespan for _ in range(n)]
    lstart = [0 for _ in range(n)]
    for i in reversed(order):
        if succs[i]:
            lfinish[i] = min(lstart[s] for s in succs[i])
        else:
            lfinish[i] = makespan
        lstart[i] = max(0, lfinish[i] - dur[i])

    slack = [max(0, lstart[i] - estart[i]) for i in range(n)]
    return {
        "order": order,
        "duration_ps": dur,
        "earliest_start_ps": estart,
        "earliest_finish_ps": efinish,
        "latest_start_ps": lstart,
        "latest_finish_ps": lfinish,
        "slack_ps": slack,
        "makespan_ps": int(makespan),
    }


def descendants_reach_join(idx: int, succs: List[Set[int]], preds: List[Set[int]]) -> bool:
    stack = list(succs[idx] or [])
    seen: Set[int] = set()
    while stack:
        cur = stack.pop()
        if cur in seen:
            continue
        seen.add(cur)
        if len(preds[cur] or []) > 1:
            return True
        for s in succs[cur] or []:
            if s not in seen:
                stack.append(s)
    return False


def launch_mermaid_graph(
    launches: List[dict],
    edges: List[dict],
    *,
    chunked: Optional[Set[int]] = None,
) -> str:
    def _esc(s: str) -> str:
        return s.replace('"', "'")

    chunked = chunked or set()
    lines = ["flowchart TD"]
    for idx, l in enumerate(launches or []):
        kernel = str((l or {}).get("kernel", f"launch_{idx}") or f"launch_{idx}")
        label = f"{idx}: {kernel}"
        if idx in chunked:
            label += r"\nwg-chunk"
        lines.append(f'  L{idx}["{_esc(label)}"]')
    for e in edges or []:
        src = int(e.get("from", -1))
        dst = int(e.get("to", -1))
        if src < 0 or dst < 0:
            continue
        lines.append(f"  L{src} --> L{dst}")
    return "\n".join(lines)




def _flattened_wg_id_from_profile_entry(launch: dict, wg_entry: dict) -> Optional[int]:
    try:
        wx, wy, wz = _int3((wg_entry or {}).get("wg", [0, 0, 0]))
    except Exception:
        return None
    num_wgx, num_wgy, num_wgz = workgroups_per_dim(launch)
    num_wgx = max(1, int(num_wgx))
    num_wgy = max(1, int(num_wgy))
    num_wgz = max(1, int(num_wgz))
    if wx < 0 or wy < 0 or wz < 0:
        return None
    if wx >= num_wgx or wy >= num_wgy or wz >= num_wgz:
        return None
    return int(wz * num_wgx * num_wgy + wy * num_wgx + wx)


def _compress_flat_wg_ids_to_windows(flat_ids: List[int]) -> List[dict]:
    ids = sorted({int(x) for x in flat_ids if int(x) >= 0})
    if not ids:
        return []
    out: List[dict] = []
    s = ids[0]
    prev = ids[0]
    for v in ids[1:]:
        if v == prev + 1:
            prev = v
            continue
        out.append({"start": int(s), "count": int(prev - s + 1)})
        s = prev = v
    out.append({"start": int(s), "count": int(prev - s + 1)})
    return out


def _hint_chiplet_support(hint: dict, num_chiplets: int) -> Set[int]:
    vec = [int(x or 0) for x in (hint.get("chiplet_cus", []) or [])]
    out: Set[int] = set()
    for i in range(min(int(num_chiplets), len(vec))):
        if vec[i] > 0:
            out.add(int(i))
    return out


def _balanced_split_windows_from_profile(base_launch: dict, profile_kernel: dict) -> Tuple[List[dict], List[dict]]:
    """Split a launch into early/late WG windows while preserving all-chiplet support.

    We do *not* partition by chiplet subset. Instead, for each chiplet bucket observed in
    the profile WG mapping, we split that bucket approximately in half so both early and
    late items retain WG coverage on all chiplets. This lets the runtime admit whichever
    chiplets currently have spare CU budget instead of the planner hard-coding subset chips.
    """
    chip_buckets: Dict[int, List[int]] = {}
    total_wg = max(1, int(total_workgroups(base_launch) or 1))
    saw_profile = False
    for wg in (profile_kernel or {}).get("workgroups", []) or []:
        if not isinstance(wg, dict):
            continue
        fid = _flattened_wg_id_from_profile_entry(base_launch, wg)
        if fid is None:
            continue
        try:
            chip = int(wg.get("chiplet", -1))
        except Exception:
            chip = -1
        if chip < 0:
            continue
        chip_buckets.setdefault(int(chip), []).append(int(fid))
        saw_profile = True

    early_ids: List[int] = []
    late_ids: List[int] = []
    if saw_profile and chip_buckets:
        for chip in sorted(chip_buckets.keys()):
            ids = sorted(set(int(x) for x in chip_buckets[chip] if 0 <= int(x) < total_wg))
            if not ids:
                continue
            cut = int(math.ceil(float(len(ids)) / 2.0))
            early_ids.extend(ids[:cut])
            late_ids.extend(ids[cut:])
    else:
        ids = list(range(int(total_wg)))
        early_ids = ids[0::2]
        late_ids = ids[1::2]

    if not early_ids or not late_ids:
        ids = sorted(set(int(x) for x in (early_ids + late_ids) if 0 <= int(x) < total_wg))
        if len(ids) <= 1:
            return _compress_flat_wg_ids_to_windows(ids), []
        cut = int(math.ceil(float(len(ids)) / 2.0))
        early_ids = ids[:cut]
        late_ids = ids[cut:]

    return _compress_flat_wg_ids_to_windows(early_ids), _compress_flat_wg_ids_to_windows(late_ids)


def _workgroup_window_item(
    base_hint: dict,
    base_launch: dict,
    profile_kernel: dict,
    windows: List[dict],
    *,
    new_id: int,
    extra_depends: Optional[List[int]] = None,
) -> Optional[dict]:
    if not windows:
        return None
    total_wg = max(1, int(total_workgroups(base_launch) or 1))
    sel_wg = sum(max(0, int((w or {}).get("count", 0) or 0)) for w in (windows or []))
    if sel_wg <= 0:
        return None

    base_chiplet = [int(x or 0) for x in (base_hint.get("chiplet_cus", []) or [])]
    base_est = int(base_hint.get("est_cus", 0) or 0)
    frac = float(sel_wg) / float(max(1, total_wg))

    chiplet_cus: List[int] = []
    if base_chiplet:
        for v in base_chiplet:
            vv = max(0, int(v))
            if vv <= 0:
                chiplet_cus.append(0)
                continue
            scaled = int(math.ceil(float(vv) * frac))
            chiplet_cus.append(max(1, scaled))
    est_cus = sum(chiplet_cus)
    if est_cus <= 0:
        est_cus = max(1, int(math.ceil(float(max(1, base_est)) * frac)))

    ds: List[float] = []
    es: List[float] = []
    tmp_hint = {"wg_windows": windows}
    for _fid, wg in _iter_hint_profile_workgroups(base_launch, profile_kernel, tmp_hint):
        s = _safe_profile_float(wg.get("dispatch_time_sec", 0.0))
        e = _safe_profile_float(wg.get("completion_time_sec", 0.0))
        if e > s:
            ds.append(s)
            es.append(e)
    if ds and es:
        dur_ps = max(1, int(round((max(es) - min(ds)) * 1.0e12)))
    else:
        full_dur = int(base_hint.get("est_duration_ps", 0) or 0)
        dur_ps = max(1, int(math.ceil(float(max(1, full_dur)) * frac)))

    new_hint = copy.deepcopy(base_hint)
    new_hint["id"] = int(new_id)
    new_hint["est_cus"] = int(est_cus)
    if chiplet_cus:
        new_hint["chiplet_cus"] = [int(x) for x in chiplet_cus]
    new_hint["est_duration_ps"] = int(dur_ps)
    new_hint["wg_windows"] = copy.deepcopy(windows)
    new_hint.pop("adaptive_wg_chunks", None)
    new_hint.pop("min_chunk_wg", None)
    deps = list(base_hint.get("depends_on", []) or [])
    if extra_depends:
        deps.extend(int(x) for x in extra_depends)
    new_hint["depends_on"] = sorted({int(x) for x in deps if int(x) >= 0})
    return new_hint


def _hint_nodes_from_plan_hints(base_launches: List[dict], hints: List[dict]) -> Tuple[List[dict], List[Set[int]], List[Set[int]], List[int], List[List[int]], Dict[int, int]]:
    if not hints:
        return [], [], [], [], [], {}
    max_id = max(int(h.get("id", -1)) for h in hints)
    n = max_id + 1
    nodes: List[dict] = [{"kernel": f"dispatch_{i}"} for i in range(n)]
    preds: List[Set[int]] = [set() for _ in range(n)]
    succs: List[Set[int]] = [set() for _ in range(n)]
    est_cus = [1 for _ in range(n)]
    chiplet_cus: List[List[int]] = [[] for _ in range(n)]
    duration_ps: Dict[int, int] = {}
    for h in hints:
        idx = int(h.get("id", -1))
        if idx < 0 or idx >= n:
            continue
        pid = _as_int(h.get("program_launch_id", idx), idx)
        if 0 <= pid < len(base_launches):
            nodes[idx] = copy.deepcopy(base_launches[pid])
        nodes[idx]["kernel"] = str(h.get("kernel", nodes[idx].get("kernel", f"dispatch_{idx}")) or nodes[idx].get("kernel", f"dispatch_{idx}"))
        nodes[idx]["program_launch_id"] = int(pid)
        if h.get("wg_windows"):
            nodes[idx]["wg_windows"] = copy.deepcopy(h.get("wg_windows"))
        est_cus[idx] = max(1, int(h.get("est_cus", 1) or 1))
        chiplet_cus[idx] = [int(x or 0) for x in (h.get("chiplet_cus", []) or [])]
        duration_ps[idx] = max(1, int(h.get("est_duration_ps", 0) or static_launch_duration_ps(nodes[idx])))
        for p in (h.get("depends_on", []) or []):
            try:
                p = int(p)
            except Exception:
                continue
            if p < 0 or p >= n or p == idx:
                continue
            preds[idx].add(p)
            succs[p].add(idx)
    return nodes, preds, succs, est_cus, chiplet_cus, duration_ps


def _hint_dep_edges(hints: List[dict], launches: List[dict]) -> List[dict]:
    edges: List[dict] = []
    by_id = {int(h.get("id", -1)): h for h in hints}
    for h in hints:
        dst = int(h.get("id", -1))
        for src in (h.get("depends_on", []) or []):
            try:
                src = int(src)
            except Exception:
                continue
            if src < 0 or src == dst:
                continue
            src_h = by_id.get(src, {})
            edges.append({
                "from": int(src),
                "to": int(dst),
                "buffer": str(src_h.get("kernel", "") or ""),
                "type": "RAW",
            })
    edges.sort(key=lambda e: (int(e.get("to", 0)), int(e.get("from", 0)), str(e.get("buffer", ""))))
    return edges


def _sum_page_entry_bytes(page_entry: dict) -> int:
    total = 0
    for k in ("r", "w"):
        for v in (page_entry or {}).get(k, []) or []:
            try:
                total += int(v or 0)
            except Exception:
                continue
    return int(total)


def _normalize_wg_windows(windows: Optional[List[dict]]) -> List[Tuple[int, int]]:
    out: List[Tuple[int, int]] = []
    for w in (windows or []):
        try:
            s = int((w or {}).get("start", 0) or 0)
            c = int((w or {}).get("count", 0) or 0)
        except Exception:
            continue
        if c <= 0:
            continue
        out.append((int(s), int(s + c)))
    out.sort()
    return out


def _wg_id_in_windows(fid: int, windows: List[Tuple[int, int]]) -> bool:
    if not windows:
        return True
    for s, e in windows:
        if s <= int(fid) < e:
            return True
    return False


def _iter_hint_profile_workgroups(base_launch: dict, profile_kernel: dict, hint: dict):
    windows = _normalize_wg_windows((hint or {}).get("wg_windows", []) or [])
    for wg in (profile_kernel or {}).get("workgroups", []) or []:
        if not isinstance(wg, dict):
            continue
        fid = _flattened_wg_id_from_profile_entry(base_launch, wg)
        if fid is None:
            continue
        if not _wg_id_in_windows(int(fid), windows):
            continue
        yield int(fid), wg


def _split_kernel_readonly_proxy_fix(
    pages_map: Dict[str, List[int]],
    launches: List[dict],
    hints: List[dict],
    *,
    num_chiplets: int,
) -> Tuple[Dict[str, List[int]], Dict[str, dict]]:
    fixed = {str(k): list(v) for k, v in (pages_map or {}).items() if isinstance(v, list)}
    debug: Dict[str, dict] = {}
    by_pid: Dict[int, List[dict]] = {}
    for h in (hints or []):
        try:
            pid = _as_int(h.get("program_launch_id", h.get("id", -1)), -1)
        except Exception:
            pid = -1
        if pid < 0:
            continue
        by_pid.setdefault(pid, []).append(h)

    for pid, hs in by_pid.items():
        if len(hs) <= 1 and not any((h.get("wg_windows") or []) for h in hs):
            continue
        if not (0 <= int(pid) < len(launches or [])):
            continue
        support: Set[int] = set()
        for h in hs:
            support.update(_hint_chiplet_support(h, int(num_chiplets)))
        ordered = sorted(int(x) for x in support)
        if len(ordered) < 2:
            continue
        launch = launches[int(pid)] or {}
        reads = {str(x) for x in (launch.get("reads", []) or []) if str(x or "")}
        writes = {str(x) for x in (launch.get("writes", []) or []) if str(x or "")}
        for ab in (launch.get("arg_bindings", []) or []):
            if not isinstance(ab, dict):
                continue
            if str(ab.get("kind", "") or "") != "buffer":
                continue
            tag = str(ab.get("tag", "") or "")
            if tag:
                reads.add(tag)
        readonly_tags = sorted(tag for tag in reads if tag and tag not in writes)
        for tag in readonly_tags:
            pages = fixed.get(tag, None)
            if not isinstance(pages, list) or len(pages) <= 1:
                continue
            new_pages = [ordered[i % len(ordered)] for i in range(len(pages))]
            delta = sum(1 for a, b in zip(pages, new_pages) if int(a) != int(b))
            if delta <= 0:
                continue
            fixed[tag] = new_pages
            debug[tag] = {
                "reason": "split-readonly-restripe",
                "launch_id": int(pid),
                "kernel": str(launch.get("kernel", "") or ""),
                "dispatch_ids": sorted(int(h.get("id", -1)) for h in hs),
                "chiplets": ordered,
                "pages_changed": int(delta),
            }
    return fixed, debug


def _resolve_profile_tag_to_plan_tag(tag: str, base_launch: dict, placement_proxy: Dict[str, List[int]]) -> Optional[str]:
    tag = str(tag or "")
    if not tag:
        return None
    if tag in placement_proxy:
        return tag
    basename = tag.split(".")[-1]
    cands: List[str] = []
    for src in list(base_launch.get("reads", []) or []) + list(base_launch.get("writes", []) or []):
        s = str(src or "")
        if not s or s not in placement_proxy:
            continue
        if s == tag or s.endswith("." + basename):
            cands.append(s)
    for ab in (base_launch.get("arg_bindings", []) or []):
        if not isinstance(ab, dict):
            continue
        s = str(ab.get("tag", "") or "")
        if not s or s not in placement_proxy:
            continue
        if s == tag or s.endswith("." + basename):
            cands.append(s)
    cands = sorted(set(cands))
    if len(cands) == 1:
        return cands[0]
    return None


def _estimate_hint_plan_remote_cost(
    launches: List[dict],
    hints: List[dict],
    profile: Optional[ProfileData],
    placement_proxy: Optional[Dict[str, List[int]]],
    *,
    num_chiplets: int,
) -> Dict[str, object]:
    if profile is None or not placement_proxy:
        return {"remote_bytes": 0, "bytes": 0, "remote_ratio": 0.0, "per_dispatch": [], "source": "profile-workgroup"}
    by_launch = profile_kernels_by_launch_index(profile, launches)
    total_bytes = 0
    remote_bytes = 0
    per_dispatch: List[dict] = []
    for h in (hints or []):
        did = _as_int(h.get("id", -1), -1)
        pid = _as_int(h.get("program_launch_id", did), did)
        if not (0 <= int(pid) < len(launches or [])):
            continue
        pk = by_launch.get(int(pid), None)
        if not isinstance(pk, dict):
            continue
        base_launch = launches[int(pid)] or {}
        loc_b = 0
        rem_b = 0
        for _fid, wg in _iter_hint_profile_workgroups(base_launch, pk, h):
            try:
                chip = int(wg.get("chiplet", -1))
            except Exception:
                chip = -1
            if chip < 0 or chip >= int(num_chiplets):
                continue
            for tag, tag_info in ((wg or {}).get("tags", {}) or {}).items():
                plan_tag = _resolve_profile_tag_to_plan_tag(str(tag or ""), base_launch, placement_proxy) or str(tag or "")
                pages = placement_proxy.get(plan_tag, None)
                if not isinstance(pages, list) or not pages:
                    continue
                for pe in (tag_info.get("pages", []) or []):
                    page = _as_int((pe or {}).get("page", -1), -1)
                    if page < 0 or page >= len(pages):
                        continue
                    b = _sum_page_entry_bytes(pe)
                    if b <= 0:
                        continue
                    loc_b += int(b)
                    if int(pages[page]) != int(chip):
                        rem_b += int(b)
        total_bytes += int(loc_b)
        remote_bytes += int(rem_b)
        per_dispatch.append({
            "dispatch_id": int(did),
            "program_launch_id": int(pid),
            "kernel": str(h.get("kernel", "") or base_launch.get("kernel", "") or ""),
            "bytes": int(loc_b),
            "remote_bytes": int(rem_b),
            "remote_ratio": float(rem_b) / float(loc_b) if loc_b > 0 else 0.0,
        })
    return {
        "remote_bytes": int(remote_bytes),
        "bytes": int(total_bytes),
        "remote_ratio": float(remote_bytes) / float(total_bytes) if total_bytes > 0 else 0.0,
        "per_dispatch": per_dispatch,
        "source": "profile-workgroup",
    }


def _hint_launch_fraction(hint: dict, base_launch: dict) -> float:
    total_wg = max(1, int((base_launch or {}).get("num_wg_total", 0) or total_workgroups(base_launch) or 1))
    windows = list((hint or {}).get("wg_windows", []) or [])
    if not windows:
        return 1.0
    sel_wg = 0
    for w in windows:
        try:
            sel_wg += max(0, int((w or {}).get("count", 0) or 0))
        except Exception:
            continue
    if sel_wg <= 0:
        return 0.0
    return min(1.0, max(0.0, float(sel_wg) / float(total_wg)))


def _estimate_hint_plan_remote_cost_from_metrics(
    launches: List[dict],
    hints: List[dict],
    metric_proxies: Optional[Dict[int, dict]],
) -> Dict[str, object]:
    if not metric_proxies:
        return {"remote_bytes": 0, "bytes": 0, "remote_ratio": 0.0, "per_dispatch": [], "source": "metrics-csv"}
    total_bytes = 0
    remote_bytes = 0
    total_ptw = 0
    total_l2_tlb_miss = 0
    per_dispatch: List[dict] = []
    for h in (hints or []):
        did = _as_int(h.get("id", -1), -1)
        pid = _as_int(h.get("program_launch_id", did), did)
        if not (0 <= pid < len(launches or [])):
            continue
        base_launch = launches[pid] or {}
        row = metric_proxies.get(int(pid), {}) or {}
        frac = _hint_launch_fraction(h, base_launch)
        bytes_total = int(round(float(_as_int(row.get("bytes", 0), 0)) * frac))
        bytes_remote = int(round(float(_as_int(row.get("remote_bytes", 0), 0)) * frac))
        ptw_reqs = int(round(float(_as_int(row.get("ptw_translation_reqs", 0), 0)) * frac))
        l2_tlb_miss = int(round(float(_as_int(row.get("l2_tlb_miss", 0), 0)) * frac))
        total_bytes += bytes_total
        remote_bytes += bytes_remote
        total_ptw += ptw_reqs
        total_l2_tlb_miss += l2_tlb_miss
        per_dispatch.append({
            "dispatch_id": int(did),
            "program_launch_id": int(pid),
            "kernel": str(h.get("kernel", "") or base_launch.get("kernel", "") or row.get("kernel", "") or ""),
            "bytes": int(bytes_total),
            "remote_bytes": int(bytes_remote),
            "remote_ratio": float(bytes_remote) / float(bytes_total) if bytes_total > 0 else 0.0,
            "ptw_translation_reqs": int(ptw_reqs),
            "l2_tlb_miss": int(l2_tlb_miss),
            "fraction_of_launch": float(frac),
        })
    return {
        "remote_bytes": int(remote_bytes),
        "bytes": int(total_bytes),
        "remote_ratio": float(remote_bytes) / float(total_bytes) if total_bytes > 0 else 0.0,
        "ptw_translation_reqs": int(total_ptw),
        "l2_tlb_miss": int(total_l2_tlb_miss),
        "per_dispatch": per_dispatch,
        "source": "metrics-csv",
    }


def _choose_remote_eval(
    profile_eval: Optional[Dict[str, object]],
    metrics_eval: Optional[Dict[str, object]],
) -> Dict[str, object]:
    pe = dict(profile_eval or {})
    me = dict(metrics_eval or {})
    profile_bytes = int(pe.get("bytes", 0) or 0)
    metrics_bytes = int(me.get("bytes", 0) or 0)
    if profile_bytes > 0:
        pe["metrics_fallback"] = me
        pe["selected_source"] = "profile-workgroup"
        return pe
    if metrics_bytes > 0:
        me["selected_source"] = "metrics-csv"
        return me
    if pe:
        pe["metrics_fallback"] = me
        pe["selected_source"] = str(pe.get("source", "") or "profile-workgroup")
        return pe
    if me:
        me["selected_source"] = str(me.get("source", "") or "metrics-csv")
        return me
    return {"remote_bytes": 0, "bytes": 0, "remote_ratio": 0.0, "per_dispatch": [], "selected_source": "none"}



def _collect_true_page_chiplet_bytes(
    launches: List[dict],
    hints: List[dict],
    profile: Optional[ProfileData],
    placement_proxy: Dict[str, List[int]],
    *,
    num_chiplets: int,
) -> Dict[str, Dict[int, List[int]]]:
    usage: Dict[str, Dict[int, List[int]]] = {}
    if profile is None or not placement_proxy:
        return usage
    by_launch = profile_kernels_by_launch_index(profile, launches)
    known_tags = {str(k) for k, v in (placement_proxy or {}).items() if isinstance(v, list)}

    def _accum(plan_tag: str, page: int, chip: int, b: int):
        if not (0 <= int(chip) < int(num_chiplets)) or b <= 0:
            return
        per_page = usage.setdefault(plan_tag, {})
        vec = per_page.get(int(page))
        if vec is None:
            vec = [0 for _ in range(int(num_chiplets))]
            per_page[int(page)] = vec
        vec[int(chip)] += int(b)

    for h in (hints or []):
        try:
            did = _as_int(h.get('id', -1), -1)
            pid = _as_int(h.get('program_launch_id', did), did)
        except Exception:
            continue
        if not (0 <= pid < len(launches or [])):
            continue
        pk = by_launch.get(pid)
        if not isinstance(pk, dict):
            continue
        base_launch = launches[pid] or {}
        support = _hint_chiplet_support(h, int(num_chiplets))

        selected_wgs = list(_iter_hint_profile_workgroups(base_launch, pk, h))
        have_wg_tags = any(bool((wg or {}).get('tags', {})) for _, wg in selected_wgs)
        if have_wg_tags:
            for _fid, wg in selected_wgs:
                try:
                    chip = int(wg.get('chiplet', -1))
                except Exception:
                    chip = -1
                if chip < 0 or chip >= int(num_chiplets):
                    continue
                if support and chip not in support:
                    chip = min(support)
                for tag, tag_info in ((wg or {}).get('tags', {}) or {}).items():
                    plan_tag = _resolve_profile_tag_to_plan_tag(str(tag or ''), base_launch, placement_proxy)
                    if not plan_tag:
                        t = str(tag or '')
                        if t in known_tags:
                            plan_tag = t
                        else:
                            continue
                    pages = placement_proxy.get(plan_tag)
                    if not isinstance(pages, list) or not pages:
                        continue
                    for pe in (tag_info.get('pages', []) or []):
                        try:
                            page = _as_int((pe or {}).get('page', -1), -1)
                        except Exception:
                            page = -1
                        if page < 0 or page >= len(pages):
                            continue
                        _accum(plan_tag, page, chip, _sum_page_entry_bytes(pe))
            continue

        # Lite-profile fallback: use kernel-level page traffic and distribute it by
        # the fraction of selected workgroups on each chiplet.
        total_chip = [0 for _ in range(int(num_chiplets))]
        sel_chip = [0 for _ in range(int(num_chiplets))]
        for wg in (pk.get('workgroups', []) or []):
            if not isinstance(wg, dict):
                continue
            try:
                chip = int(wg.get('chiplet', -1))
            except Exception:
                chip = -1
            if 0 <= chip < int(num_chiplets):
                total_chip[chip] += 1
        for _fid, wg in selected_wgs:
            try:
                chip = int(wg.get('chiplet', -1))
            except Exception:
                chip = -1
            if 0 <= chip < int(num_chiplets):
                sel_chip[chip] += 1
        frac_chip = [0.0 for _ in range(int(num_chiplets))]
        for c in range(int(num_chiplets)):
            if total_chip[c] > 0:
                frac_chip[c] = float(sel_chip[c]) / float(total_chip[c])
        if sum(frac_chip) <= 0.0:
            runtime = dict(pk.get('runtime', {}) or {})
            counts = [int(x or 0) for x in (runtime.get('wg_count_by_chiplet', []) or [])]
            for c in range(min(int(num_chiplets), len(counts))):
                if counts[c] > 0:
                    frac_chip[c] = 1.0
        for tag, tag_info in ((pk or {}).get('tags', {}) or {}).items():
            plan_tag = _resolve_profile_tag_to_plan_tag(str(tag or ''), base_launch, placement_proxy)
            if not plan_tag:
                t = str(tag or '')
                if t in known_tags:
                    plan_tag = t
                else:
                    continue
            pages = placement_proxy.get(plan_tag)
            if not isinstance(pages, list) or not pages:
                continue
            for pe in (tag_info.get('pages', []) or []):
                try:
                    page = _as_int((pe or {}).get('page', -1), -1)
                except Exception:
                    page = -1
                if page < 0 or page >= len(pages):
                    continue
                rr = (pe or {}).get('r', []) or []
                ww = (pe or {}).get('w', []) or []
                for c in range(int(num_chiplets)):
                    rb = int(rr[c] or 0) if c < len(rr) else 0
                    wb = int(ww[c] or 0) if c < len(ww) else 0
                    b = int(round((rb + wb) * frac_chip[c]))
                    _accum(plan_tag, page, c, b)
    return usage


def _compute_remote_eval_from_usage(
    usage: Dict[str, Dict[int, List[int]]],
    pages_map: Dict[str, List[int]],
    *,
    num_chiplets: int,
) -> Dict[str, object]:
    total_bytes = 0
    remote_bytes = 0
    per_tag: Dict[str, dict] = {}
    for tag, per_page in (usage or {}).items():
        pages = pages_map.get(tag)
        if not isinstance(pages, list) or not pages:
            continue
        tag_total = 0
        tag_remote = 0
        for page, vec in (per_page or {}).items():
            if not (0 <= int(page) < len(pages)):
                continue
            vv = [int(x or 0) for x in (vec or [])]
            tb = int(sum(vv))
            if tb <= 0:
                continue
            home = int(pages[int(page)]) if int(page) < len(pages) else 0
            local = vv[home] if 0 <= home < int(num_chiplets) else 0
            rem = int(tb - local)
            total_bytes += tb
            remote_bytes += rem
            tag_total += tb
            tag_remote += rem
        if tag_total > 0:
            per_tag[str(tag)] = {
                'bytes': int(tag_total),
                'remote_bytes': int(tag_remote),
                'remote_ratio': float(tag_remote) / float(tag_total) if tag_total > 0 else 0.0,
            }
    return {
        'bytes': int(total_bytes),
        'remote_bytes': int(remote_bytes),
        'remote_ratio': float(remote_bytes) / float(total_bytes) if total_bytes > 0 else 0.0,
        'per_tag': per_tag,
    }


def _solve_true_remote_page_assignment(
    current_pages: Dict[str, List[int]],
    usage: Dict[str, Dict[int, List[int]]],
    *,
    num_chiplets: int,
    chiplet_cap_pages: Optional[int] = None,
    chiplet_overflow_pages: int = 0,
    chiplet_overflow_penalty: int = 0,
    stability_penalty: int = 1,
) -> Tuple[Dict[str, List[int]], Dict[str, object]]:
    """Given a fixed dispatch schedule, minimize *true* remote bytes.

    Each profiled page becomes one assignment item. The exact cost of placing page p on chiplet c is
        total_bytes(p) - local_bytes_from_chiplet_c(p)
    i.e. the actual remote bytes induced by that choice. A tiny stability penalty preserves the
    current placement on ties.
    """
    pages_map = {str(k): list(v) for k, v in (current_pages or {}).items() if isinstance(v, list)}
    if not usage or not pages_map:
        return pages_map, {'used_pages': 0, 'changed_pages': 0, 'remote_before': 0, 'remote_after': 0}

    fixed_counts = [0 for _ in range(int(num_chiplets))]
    reassignable: Set[Tuple[str, int]] = set()
    for tag, per_page in (usage or {}).items():
        pages = pages_map.get(tag)
        if not isinstance(pages, list):
            continue
        for page, vec in (per_page or {}).items():
            if 0 <= int(page) < len(pages) and sum(int(x or 0) for x in (vec or [])) > 0:
                reassignable.add((str(tag), int(page)))
    for tag, pages in pages_map.items():
        for page, home in enumerate(pages):
            if (str(tag), int(page)) in reassignable:
                continue
            if 0 <= int(home) < int(num_chiplets):
                fixed_counts[int(home)] += 1

    items: List[Tuple[str, int, int]] = []
    costs: List[List[int]] = []
    item_meta: List[Tuple[str, int]] = []
    remote_before = 0
    for tag, page in sorted(reassignable):
        pages = pages_map.get(tag)
        if not isinstance(pages, list) or not (0 <= int(page) < len(pages)):
            continue
        vec = [int(x or 0) for x in (usage.get(tag, {}).get(int(page), []) or [])]
        if len(vec) < int(num_chiplets):
            vec = vec + [0 for _ in range(int(num_chiplets) - len(vec))]
        tb = int(sum(vec))
        if tb <= 0:
            continue
        old_home = int(pages[int(page)]) if int(page) < len(pages) else 0
        if 0 <= old_home < int(num_chiplets):
            remote_before += int(tb - vec[old_home])
        row = []
        for c in range(int(num_chiplets)):
            cost = int(tb - vec[c])
            if int(stability_penalty) > 0 and c != old_home:
                cost += int(stability_penalty)
            row.append(int(cost))
        items.append((str(tag), int(page), 1))
        costs.append(row)
        item_meta.append((str(tag), int(page)))

    if not items:
        return pages_map, {'used_pages': 0, 'changed_pages': 0, 'remote_before': 0, 'remote_after': 0}

    cap = chiplet_cap_pages
    if cap is not None and int(cap) > 0:
        cap = max(0, int(cap))
        cap_avail = max(0, int(cap) - min(int(cap), 0))
        # solve_assignment_min_cost_flow expects total capacity, not per-chiplet arrays. We therefore
        # account for fixed pages by shrinking the usable normal capacity uniformly per chiplet.
        # If some chiplet already exceeds the cap because of fixed pages, the overflow edges absorb it.
        # We conservatively use the per-chiplet residual by pre-filling dummy costs below.
        pass

    # Build a dedicated min-cost flow with per-chiplet residual capacities.
    n_items = len(items)
    S = 0
    item0 = 1
    chip0 = item0 + n_items
    T = chip0 + int(num_chiplets)
    mcf = MinCostFlow(T + 1)
    item_to_chip: List[List[Tuple[int, int]]] = [[(-1, -1) for _ in range(int(num_chiplets))] for _ in range(n_items)]
    for i in range(n_items):
        mcf.add_edge(S, item0 + i, cap=1, cost=0)
        for c in range(int(num_chiplets)):
            fr = item0 + i
            pos = len(mcf.g[fr])
            mcf.add_edge(fr, chip0 + c, cap=1, cost=int(costs[i][c]))
            item_to_chip[i][c] = (fr, pos)
    total_pages = int(n_items)
    if chiplet_cap_pages is None or int(chiplet_cap_pages) <= 0:
        residual_norm = [total_pages for _ in range(int(num_chiplets))]
    else:
        residual_norm = []
        for c in range(int(num_chiplets)):
            used_fixed = int(fixed_counts[c])
            residual_norm.append(max(0, int(chiplet_cap_pages) - used_fixed))
    residual_over = [max(0, int(chiplet_overflow_pages)) for _ in range(int(num_chiplets))]
    for c in range(int(num_chiplets)):
        if residual_norm[c] > 0:
            mcf.add_edge(chip0 + c, T, cap=int(residual_norm[c]), cost=0)
        if residual_over[c] > 0 and int(chiplet_overflow_penalty) > 0:
            mcf.add_edge(chip0 + c, T, cap=int(residual_over[c]), cost=int(chiplet_overflow_penalty))
        elif chiplet_cap_pages is None or int(chiplet_cap_pages) <= 0:
            # unlimited case: a single large zero-cost edge is enough
            pass
    sent, min_cost = mcf.min_cost_flow(S, T, total_pages)
    if sent != total_pages:
        # Fallback: unconstrained per-page argmin.
        for i, (tag, page) in enumerate(item_meta):
            best_c = min(range(int(num_chiplets)), key=lambda c: int(costs[i][c]))
            pages_map[tag][int(page)] = int(best_c)
    else:
        for i, (tag, page) in enumerate(item_meta):
            chosen = None
            for c in range(int(num_chiplets)):
                fr, pos = item_to_chip[i][c]
                e = mcf.g[fr][pos]
                if e.cap == 0:
                    chosen = c
                    break
            if chosen is None:
                chosen = int(pages_map[tag][int(page)])
            pages_map[tag][int(page)] = int(chosen)

    changed = 0
    for tag, page in item_meta:
        old_home = int(current_pages.get(tag, [0])[int(page)]) if isinstance(current_pages.get(tag), list) else 0
        new_home = int(pages_map.get(tag, [0])[int(page)]) if isinstance(pages_map.get(tag), list) else old_home
        if old_home != new_home:
            changed += 1
    remote_after_eval = _compute_remote_eval_from_usage(usage, pages_map, num_chiplets=int(num_chiplets))
    dbg = {
        'used_pages': int(len(item_meta)),
        'changed_pages': int(changed),
        'remote_before': int(remote_before),
        'remote_after': int(remote_after_eval.get('remote_bytes', 0) or 0),
        'remote_ratio_after': float(remote_after_eval.get('remote_ratio', 0.0) or 0.0),
        'per_tag_after': remote_after_eval.get('per_tag', {}),
    }
    return pages_map, dbg

def _evaluate_hint_plan(
    base_launches: List[dict],
    hints: List[dict],
    *,
    total_cus: int,
    num_chiplets: int,
) -> Tuple[List[dict], dict, List[dict], List[Set[int]], List[Set[int]], Dict[int, int]]:
    hints = [copy.deepcopy(h) for h in (hints or [])]
    hints.sort(key=lambda h: int(h.get("id", 0)))
    nodes, preds, succs, est_cus, chiplet_cus, duration_ps = _hint_nodes_from_plan_hints(base_launches, hints)
    bottom = compute_launch_bottom_levels(nodes, succs, duration_ps)
    for h in hints:
        idx = int(h.get("id", -1))
        if 0 <= idx < len(bottom):
            h["priority"] = int(bottom[idx])
    schedule = simulate_launch_pipeline(
        nodes,
        preds,
        succs,
        bottom,
        est_cus,
        duration_ps,
        total_cus=int(total_cus),
        num_chiplets=int(num_chiplets),
        chiplet_cus=chiplet_cus,
    )
    dep_edges = _hint_dep_edges(hints, nodes)
    return hints, schedule, nodes, preds, succs, duration_ps


def _balanced_chiplet_budget_for_launch(launch: dict, total_est_cus: int, partition: str, num_chiplets: int) -> List[int]:
    active = max(1, int(active_chiplets_for_launch(partition, launch, int(num_chiplets or 1)) or 1))
    chiplets = max(1, int(num_chiplets or 1))
    total = max(1, int(total_est_cus or 1))
    per = total // active
    rem = total % active
    out = [0 for _ in range(chiplets)]
    # Use the simulator-consistent active-chiplet prefix set.
    for i in range(min(active, chiplets)):
        out[i] = int(per + (1 if i < rem else 0))
    return out


def _apply_partition_override_to_hint(h: dict, launch: dict, *, partition: str, num_chiplets: int) -> None:
    h["partition"] = str(partition)
    est = int(h.get("est_cus", 0) or 0)
    if est <= 0:
        est = 1
    chip = _balanced_chiplet_budget_for_launch(launch, est, partition, int(num_chiplets or 1))
    if any(int(x or 0) > 0 for x in chip):
        h["chiplet_cus"] = [int(x or 0) for x in chip]


def _optimize_fork_join_items(
    launches: List[dict],
    hints: List[dict],
    preds: List[Set[int]],
    succs: List[Set[int]],
    dep_edges: List[dict],
    duration_ps: Dict[int, int],
    *,
    profile: Optional[ProfileData],
    num_chiplets: int,
    total_cus: int,
    chunk_debug: Optional[List[dict]] = None,
    placement_proxy: Optional[Dict[str, List[int]]] = None,
    seed_metric_proxies: Optional[Dict[int, dict]] = None,
    remote_penalty_ps_per_byte: float = 10.0,
    allow_partition_overrides: bool = True,
) -> Tuple[List[dict], dict, List[dict], List[Set[int]], List[Set[int]], Dict[int, int]]:
    """Explore fork-join scheduling candidates for transformer-style q/k/v branches.

    Preferred path: if the pair-stage (attention_scores) can be remapped onto all chiplets
    and a full branch launch (v_proj) can co-reside with it, compare:
      - baseline
      - parallel_qk_vpipe     (q/k parallel, v waits for both; scores remapped to Ydiv)
      - serial_qk_vpipe       (q then k||v; scores remapped to Ydiv)
      - serial_kq_vpipe       (k then q||v; scores remapped to Ydiv)

    Fallback: keep the original explicit WG-window split candidates.
    """

    def _score_candidate(name: str, cand_hints: List[dict], cand_schedule: dict) -> Tuple[float, Dict[str, object], Dict[str, dict]]:
        prox = {str(k): list(v) for k, v in (placement_proxy or {}).items() if isinstance(v, list)}
        split_fix_debug: Dict[str, dict] = {}
        usage = _collect_true_page_chiplet_bytes(launches, cand_hints, profile, prox, num_chiplets=int(num_chiplets))
        profile_remote_eval: Dict[str, object] = {}
        if usage:
            solved_pages, exact_dbg = _solve_true_remote_page_assignment(
                prox, usage, num_chiplets=int(num_chiplets), chiplet_cap_pages=None,
                chiplet_overflow_pages=0, chiplet_overflow_penalty=0, stability_penalty=1,
            )
            profile_remote_eval = _compute_remote_eval_from_usage(usage, solved_pages, num_chiplets=int(num_chiplets))
            profile_remote_eval['exact_solver'] = exact_dbg
        else:
            profile_remote_eval = _estimate_hint_plan_remote_cost(launches, cand_hints, profile, prox, num_chiplets=int(num_chiplets))
        metrics_remote_eval = _estimate_hint_plan_remote_cost_from_metrics(launches, cand_hints, seed_metric_proxies)
        remote_eval = _choose_remote_eval(profile_remote_eval, metrics_remote_eval)
        score = float(int(cand_schedule.get("makespan_ps", 0) or 0)) + float(remote_penalty_ps_per_byte) * float(remote_eval.get("remote_bytes", 0) or 0)
        return score, remote_eval, split_fix_debug

    def _first_join_from_launch(idx: int) -> int:
        joins = [s for s in (succs[idx] or []) if len(preds[s] or []) > 1]
        if joins:
            return int(sorted(joins)[0])
        stack = list(sorted(succs[idx] or []))
        seen: Set[int] = set()
        while stack:
            cur = int(stack.pop(0))
            if cur in seen:
                continue
            seen.add(cur)
            if len(preds[cur] or []) > 1:
                return cur
            stack.extend(sorted(succs[cur] or []))
        return -1

    def _pair_for_branch(idx: int, join_id: int) -> Tuple[int, int, int, int]:
        fork_preds = sorted(int(x) for x in (preds[idx] or []))
        if len(fork_preds) != 1:
            return -1, -1, -1, -1
        fork_root = int(fork_preds[0])
        siblings = sorted(int(x) for x in (succs[fork_root] or []) if int(x) != idx)
        if len(siblings) < 2:
            return -1, -1, -1, -1
        def _reachable(start: int) -> Set[int]:
            seen: Set[int] = set(); stack=[int(start)]
            while stack:
                cur=int(stack.pop())
                if cur in seen:
                    continue
                seen.add(cur)
                stack.extend(sorted(succs[cur] or []))
            return seen
        reach_a=_reachable(siblings[0]); reach_b=_reachable(siblings[1])
        common=sorted(reach_a & reach_b)
        pair_stage=-1
        for cand in common:
            if len(preds[cand] or []) >= 2:
                pair_stage=int(cand); break
        if pair_stage < 0 and join_id >= 0:
            pair_stage = int(join_id)
        return int(fork_root), int(pair_stage), int(siblings[0]), int(siblings[1])

    base_hints = copy.deepcopy(hints)
    by_launch_profile = profile_kernels_by_launch_index(profile, launches)

    branch_rows: List[dict] = []
    if profile is not None and chunk_debug:
        for row in (chunk_debug or []):
            try:
                idx = _as_int(row.get("launch_id", -1), -1)
            except Exception:
                idx = -1
            if idx < 0 or idx >= len(launches or []):
                continue
            join_id = _first_join_from_launch(idx)
            fork_root, pair_stage, first, second = _pair_for_branch(idx, join_id)
            if fork_root < 0 or pair_stage < 0 or first < 0 or second < 0:
                continue
            branch_rows.append({
                "launch_id": int(idx),
                "join_id": int(join_id),
                "fork_root": int(fork_root),
                "pair_stage_id": int(pair_stage),
                "first": int(first),
                "second": int(second),
                "kernel": str((launches[idx] or {}).get("kernel", "") or ""),
            })

    candidate_rows: List[dict] = []
    strategy_details: List[dict] = []

    base_eval_hints, base_schedule, base_nodes, base_preds, base_succs, base_duration = _evaluate_hint_plan(
        launches, base_hints, total_cus=int(total_cus), num_chiplets=int(num_chiplets),
    )
    base_score, base_remote, base_fix = _score_candidate('baseline', base_eval_hints, base_schedule)
    best = {
        'name': 'baseline', 'score': float(base_score), 'remote_eval': base_remote,
        'split_fix_debug': base_fix, 'hints': base_eval_hints, 'schedule': base_schedule,
        'nodes': base_nodes, 'preds': base_preds, 'succs': base_succs, 'duration': base_duration,
        'details': {},
    }
    candidate_rows.append({
        'name': 'baseline', 'makespan_ps': int(base_schedule.get('makespan_ps', 0)),
        'score': float(base_score), 'remote_bytes_proxy': int(base_remote.get('remote_bytes', 0) or 0),
        'remote_ratio_proxy': float(base_remote.get('remote_ratio', 0.0) or 0.0),
    })

    if not branch_rows or profile is None:
        return best['hints'], {
            'chosen_strategy': best['name'],
            'chosen_score': best['score'],
            'chosen_remote_proxy': best.get('remote_eval', {}),
            'chosen_split_fix': best.get('split_fix_debug', {}),
            'remote_penalty_ps_per_byte': float(remote_penalty_ps_per_byte),
            'candidates': candidate_rows,
            'strategy_details': strategy_details,
        }, best['nodes'], best['preds'], best['succs'], best['duration']

    cu_per_chiplet = max(1, int(total_cus) // max(1, int(num_chiplets)))

    def _can_full_overlap(branch_idx: int, pair_stage_id: int) -> bool:
        pair_launch = launches[pair_stage_id]
        pair_hint = next((h for h in base_hints if int(h.get('id', -1)) == int(pair_stage_id)), None)
        branch_hint = next((h for h in base_hints if int(h.get('id', -1)) == int(branch_idx)), None)
        if pair_hint is None or branch_hint is None:
            return False
        pair_est = int(pair_hint.get('est_cus', 0) or 0)
        branch_est = int(branch_hint.get('est_cus', 0) or 0)
        pair_chip = _balanced_chiplet_budget_for_launch(pair_launch, pair_est, 'Ydiv', int(num_chiplets))
        branch_chip = _balanced_chiplet_budget_for_launch(launches[branch_idx], branch_est, str(branch_hint.get('partition', 'Xdiv') or 'Xdiv'), int(num_chiplets))
        return all((int(pair_chip[c]) + int(branch_chip[c])) <= int(cu_per_chiplet) for c in range(int(num_chiplets)))

    full_overlap_possible = bool(allow_partition_overrides) and all(_can_full_overlap(int(br['launch_id']), int(br['pair_stage_id'])) for br in branch_rows)

    def _replace_dep(dep_list: List[int], old_id: int, new_id: int) -> List[int]:
        out=[]
        for x in (dep_list or []):
            try:
                v=int(x)
            except Exception:
                continue
            out.append(int(new_id) if v == int(old_id) else v)
        return sorted(set(out))

    def _schedule_complete(schedule: dict, expected_n: int) -> bool:
        try:
            started = int((schedule or {}).get('started', 0) or 0)
            completed = int((schedule or {}).get('completed', 0) or 0)
        except Exception:
            return False
        return started == int(expected_n) and completed == int(expected_n)

    def _build_full_launch_candidate(name: str, mode: str) -> Tuple[List[dict], dict]:
        cand = copy.deepcopy(base_hints)
        by_id = {int(h.get('id', -1)): h for h in cand}
        details_rows=[]
        partition_overrides={}
        for br in branch_rows:
            idx=int(br['launch_id']); pair_stage=int(br['pair_stage_id']); first=int(br['first']); second=int(br['second'])
            # Spread the pair-stage across all chiplets so branch work can co-reside.
            if pair_stage in by_id and allow_partition_overrides:
                _apply_partition_override_to_hint(by_id[pair_stage], launches[pair_stage], partition='Ydiv', num_chiplets=int(num_chiplets))
                partition_overrides[str((launches[pair_stage] or {}).get('kernel','') or '')] = 'Ydiv'
            if idx not in by_id:
                continue
            bh = by_id[idx]
            bh.pop('wg_windows', None)
            bh.pop('adaptive_wg_chunks', None)
            bh.pop('min_chunk_wg', None)
            deps = [int(x) for x in (bh.get('depends_on', []) or [])]
            if mode == 'parallel':
                deps.extend([first, second])
            elif mode == 'serial_qk':
                if second in by_id:
                    sdeps = [int(x) for x in (by_id[second].get('depends_on', []) or [])]
                    sdeps.append(first)
                    by_id[second]['depends_on'] = sorted(set(sdeps))
                deps.append(first)
            elif mode == 'serial_kq':
                if first in by_id:
                    fdeps = [int(x) for x in (by_id[first].get('depends_on', []) or [])]
                    fdeps.append(second)
                    by_id[first]['depends_on'] = sorted(set(fdeps))
                deps.append(second)
            bh['depends_on'] = sorted(set(int(x) for x in deps if int(x) >= 0))
            details_rows.append({'launch_id': idx, 'pair_stage_id': pair_stage, 'mode': mode, 'pair_stage_partition': 'Ydiv'})
        out = [by_id[k] for k in sorted(by_id.keys()) if k >= 0]
        return out, {'mode': mode, 'partition_overrides': partition_overrides, 'items': details_rows}

    def _build_split_candidate(name: str, serial_order: Optional[Tuple[int, int]]) -> Tuple[List[dict], dict]:
        cand = copy.deepcopy(base_hints)
        by_id = {int(h.get('id', -1)): h for h in cand}
        next_id = max((int(h.get('id', -1)) for h in cand), default=-1) + 1
        detail_rows=[]
        skipped_rows=[]
        if serial_order is not None:
            first, second = int(serial_order[0]), int(serial_order[1])
            if first in by_id and second in by_id:
                deps = [int(x) for x in (by_id[second].get('depends_on', []) or [])]
                deps.append(first)
                by_id[second]['depends_on'] = sorted(set(deps))
        for br in branch_rows:
            idx=int(br['launch_id']); pair_stage=int(br['pair_stage_id']); fork_root=int(br['fork_root'])
            pk = by_launch_profile.get(idx)
            base_hint = by_id.get(idx)
            if base_hint is None or not isinstance(pk, dict):
                continue
            early_windows, late_windows = _balanced_split_windows_from_profile(launches[idx], pk)
            if not early_windows or not late_windows:
                continue
            early_hint = _workgroup_window_item(base_hint, launches[idx], pk, early_windows, new_id=idx, extra_depends=[fork_root])
            late_id = next_id
            late_hint = _workgroup_window_item(base_hint, launches[idx], pk, late_windows, new_id=late_id, extra_depends=[idx, pair_stage])
            if early_hint is None or late_hint is None:
                continue
            trial_by_id = copy.deepcopy(by_id)
            trial_by_id[idx] = early_hint
            trial_by_id[late_id] = late_hint
            for hid, hh in list(trial_by_id.items()):
                if hid == idx or hid == late_id:
                    continue
                hh['depends_on'] = _replace_dep([int(x) for x in (hh.get('depends_on', []) or [])], idx, late_id)
            trial_out = [trial_by_id[k] for k in sorted(trial_by_id.keys()) if k >= 0]
            _ev_hints, trial_schedule, _ev_nodes, _ev_preds, _ev_succs, _ev_duration = _evaluate_hint_plan(
                launches, trial_out, total_cus=int(total_cus), num_chiplets=int(num_chiplets),
            )
            if not _schedule_complete(trial_schedule, len(trial_out)):
                skipped_rows.append({
                    'launch_id': idx,
                    'pair_stage_id': pair_stage,
                    'reason': 'cycle-or-deadlock-risk',
                    'trial_started': int(trial_schedule.get('started', 0) or 0),
                    'trial_completed': int(trial_schedule.get('completed', 0) or 0),
                    'trial_count': int(len(trial_out)),
                })
                continue
            by_id = trial_by_id
            next_id += 1
            detail_rows.append({'launch_id': idx, 'late_item_id': late_id, 'pair_stage_id': pair_stage, 'serial_order': serial_order})
        out=[by_id[k] for k in sorted(by_id.keys()) if k>=0]
        return out, {'itemized': detail_rows, 'skipped': skipped_rows, 'serial_order': serial_order}

    cand_specs=[]
    if full_overlap_possible:
        cand_specs = [
            ('parallel_qk_vpipe', ('full','parallel')),
            ('serial_qk_vpipe', ('full','serial_qk')),
            ('serial_kq_vpipe', ('full','serial_kq')),
        ]
    else:
        a=int(branch_rows[0]['first']); b=int(branch_rows[0]['second'])
        cand_specs = [
            ('parallel_qk_vpipe', ('split', None)),
            ('serial_qk_vpipe', ('split', (a,b))),
            ('serial_kq_vpipe', ('split', (b,a))),
        ]

    for name, spec in cand_specs:
        kind = spec[0]
        if kind == 'full':
            cand, details = _build_full_launch_candidate(name, spec[1])
        else:
            cand, details = _build_split_candidate(name, spec[1])
        ev_hints, ev_schedule, ev_nodes, ev_preds, ev_succs, ev_duration = _evaluate_hint_plan(launches, cand, total_cus=int(total_cus), num_chiplets=int(num_chiplets))
        valid = _schedule_complete(ev_schedule, len(ev_hints))
        ev_score, ev_remote, ev_split_fix = _score_candidate(name, ev_hints, ev_schedule)
        candidate_rows.append({
            'name': name,
            'valid': bool(valid),
            'started': int(ev_schedule.get('started', 0) or 0),
            'completed': int(ev_schedule.get('completed', 0) or 0),
            'launch_count': int(len(ev_hints)),
            'makespan_ps': int(ev_schedule.get('makespan_ps', 0)),
            'score': float(ev_score),
            'remote_bytes_proxy': int(ev_remote.get('remote_bytes', 0) or 0),
            'remote_ratio_proxy': float(ev_remote.get('remote_ratio', 0.0) or 0.0),
        })
        strategy_details.append({'name': name, 'details': details, 'remote_proxy': ev_remote, 'split_fix': ev_split_fix, 'valid': bool(valid), 'schedule': {'started': int(ev_schedule.get('started', 0) or 0), 'completed': int(ev_schedule.get('completed', 0) or 0), 'launch_count': int(len(ev_hints))}})
        if not valid:
            continue
        if float(ev_score) < float(best['score']):
            best = {
                'name': name, 'score': float(ev_score), 'remote_eval': ev_remote,
                'split_fix_debug': ev_split_fix, 'hints': ev_hints, 'schedule': ev_schedule,
                'nodes': ev_nodes, 'preds': ev_preds, 'succs': ev_succs, 'duration': ev_duration,
                'details': details,
            }

    extra_debug = {
        'chosen_strategy': str(best['name']),
        'chosen_score': float(best['score']),
        'chosen_remote_proxy': best.get('remote_eval', {}),
        'chosen_split_fix': best.get('split_fix_debug', {}),
        'remote_penalty_ps_per_byte': float(remote_penalty_ps_per_byte),
        'candidates': candidate_rows,
        'strategy_details': strategy_details,
    }
    return best['hints'], extra_debug, best['nodes'], best['preds'], best['succs'], best['duration']

def build_launch_hints(
    launches: List[dict],
    kernel_partition: Dict[str, str],
    kernel_policy: Dict[str, str],
    *,
    num_chiplets: int,
    sa_per_chiplet: int,
    cu_per_sa: int,
    launch_metrics_csv: str = "",
    profile: Optional[ProfileData] = None,
    global_graph: Optional[dict] = None,
    placement_proxy: Optional[Dict[str, List[int]]] = None,
    split_remote_penalty_ps_per_byte: float = 10.0,
    allow_partition_overrides: bool = True,
) -> Tuple[List[dict], dict]:
    launches_rt, runtime_geom_dbg = normalize_launches_with_runtime_geometry(launches, profile)
    profile_by_launch = profile_kernels_by_launch_index(profile, launches_rt)
    preds, succs, dep_edges, dep_source = build_launch_dependency_graph(
        launches_rt,
        global_graph=global_graph,
        profile_kernels_by_launch=profile_by_launch,
    )

    runtime_cus, runtime_chiplet_cus, runtime_duration_ps, runtime_dbg = load_launch_runtime_profile(profile, launches_rt)
    metrics_duration_ps = load_launch_latency_ps(launch_metrics_csv, launches_rt)
    metrics_proxies = load_launch_metric_proxies(launch_metrics_csv, launches_rt)
    duration_ps: Dict[int, int] = dict(runtime_duration_ps)
    metrics_duration_fallback: Dict[int, int] = {}
    for idx, dur in metrics_duration_ps.items():
        if int(idx) not in duration_ps:
            duration_ps[int(idx)] = int(dur)
            metrics_duration_fallback[int(idx)] = int(dur)

    windows = compute_launch_timing_windows(launches_rt, preds, succs, duration_ps)
    bottom = compute_launch_bottom_levels(launches_rt, succs, duration_ps)

    total_cus = max(1, int(num_chiplets or 1) * max(1, int(sa_per_chiplet or 1) * int(cu_per_sa or 1)))
    hints: List[dict] = []
    est_cus_vec: List[int] = []
    chiplet_cus_vec: List[List[int]] = []
    priority_vec: List[int] = []
    chunked_nodes: Set[int] = set()
    chunk_debug: List[dict] = []

    slack_ps = list(windows.get("slack_ps", []) or [])
    duration_arr = list(windows.get("duration_ps", []) or [])

    for idx, l in enumerate(launches_rt or []):
        kernel = str(l.get("kernel", "") or "")
        partition = str(kernel_partition.get(kernel, "Xdiv") or "Xdiv")
        policy = canonical_lasp_policy(kernel_policy.get(kernel, "lasp-blockcu"))

        est_cus = int(runtime_cus.get(idx, 0) or 0)
        if est_cus <= 0:
            est_cus = estimate_launch_cus(
                l,
                partition,
                num_chiplets=int(num_chiplets or 1),
                sa_per_chiplet=int(sa_per_chiplet or 1),
                cu_per_sa=int(cu_per_sa or 1),
            )
        chiplet_cus = list(runtime_chiplet_cus.get(idx, []) or [])

        prio = int(bottom[idx])
        dur = int(duration_ps.get(idx, 0) or 0)
        if dur <= 0:
            dur = static_launch_duration_ps(l)

        num_wg_total = int((l or {}).get("num_wg_total", 0) or total_workgroups(l) or 0)
        if num_wg_total <= 0:
            num_wg_total = 1
        succ_join = any(len(preds[s] or []) > 1 for s in (succs[idx] or []))
        feeds_join = succ_join or descendants_reach_join(idx, succs, preds)
        slack = int(slack_ps[idx]) if idx < len(slack_ps) else 0

        # Generic runtime adaptive chunking is intentionally disabled here.
        # We still identify candidate side-branch launches, but only the
        # fork/join optimizer may convert them into explicit two-window items.
        # This keeps split plans compiler-visible, deterministic, and limited to
        # at most two launches instead of emitting unbounded runtime chunking.
        candidate_itemization = False
        min_chunk_wg = 1
        reasons: List[str] = []
        if num_wg_total > 1 and est_cus > 1:
            meaningful_slack = slack > max(int(dur * 0.10), 1_000_000)
            if feeds_join:
                reasons.append("feeds-join")
            if slack > 0:
                reasons.append("has-slack")
            if meaningful_slack and feeds_join:
                candidate_itemization = True
            if candidate_itemization:
                base = max(1, _ceil_div(num_wg_total, max(est_cus, 1)))
                if num_wg_total >= 128:
                    base = max(base, 2)
                if num_wg_total >= 256:
                    base = max(base, 4)
                min_chunk_wg = int(max(1, min(num_wg_total, base)))

        est_cus_vec.append(int(est_cus))
        chiplet_cus_vec.append(chiplet_cus)
        priority_vec.append(int(prio))
        hint = {
            "id": int(idx),
            "kernel": kernel,
            "depends_on": [int(x) for x in sorted(preds[idx])],
            "priority": int(prio),
            "est_cus": int(est_cus),
            "est_duration_ps": int(dur),
            "policy": policy,
            "partition": partition,
            "program_launch_id": _as_int((l or {}).get("id", idx), idx),
        }
        if any(int(x or 0) > 0 for x in chiplet_cus):
            hint["chiplet_cus"] = [int(x or 0) for x in chiplet_cus]
        if candidate_itemization:
            chunk_debug.append({
                "launch_id": int(idx),
                "kernel": kernel,
                "num_wg_total": int(num_wg_total),
                "est_cus": int(est_cus),
                "min_chunk_wg": int(min_chunk_wg),
                "slack_ps": int(slack),
                "duration_ps": int(dur),
                "reasons": reasons,
                "candidate_kind": "two-window-side-branch",
            })
        hints.append(hint)

    # Optional WG/item-level fork-join optimization. This upgrades selected
    # branch launches (e.g., BERT tiny's v_proj) into explicit WG-item hints and
    # also evaluates a serialized-sibling alternative using profile truth.
    opt_hints, opt_debug, opt_nodes, opt_preds, opt_succs, opt_duration_ps = _optimize_fork_join_items(
        launches_rt,
        hints,
        preds,
        succs,
        dep_edges,
        duration_ps,
        profile=profile,
        num_chiplets=int(num_chiplets or 0),
        total_cus=int(total_cus),
        chunk_debug=chunk_debug,
        placement_proxy=placement_proxy,
        seed_metric_proxies=metrics_proxies,
        remote_penalty_ps_per_byte=float(split_remote_penalty_ps_per_byte),
        allow_partition_overrides=bool(allow_partition_overrides),
    )

    # Apply partition overrides chosen by the fork-join strategy so the simulator's
    # per-kernel scheduling hints match the launch-hint evaluation model.
    if allow_partition_overrides:
        try:
            chosen_name = str((opt_debug or {}).get('chosen_strategy', '') or '')
            chosen_details = None
            for row in ((opt_debug or {}).get('strategy_details', []) or []):
                if str((row or {}).get('name', '') or '') == chosen_name:
                    chosen_details = (row or {}).get('details', {}) or {}
                    break
            for k, p in ((chosen_details or {}).get('partition_overrides', {}) or {}).items():
                if k:
                    kernel_partition[str(k)] = str(p)
        except Exception:
            pass

    opt_dep_edges = _hint_dep_edges(opt_hints, opt_nodes)
    chunked_dispatch_ids: Set[int] = set()
    for h in (opt_hints or []):
        if bool(h.get("adaptive_wg_chunks", False)) or (h.get("wg_windows") and len(h.get("wg_windows")) > 0):
            try:
                chunked_dispatch_ids.add(int(h.get("id", -1)))
            except Exception:
                pass

    max_fan_in = max((len(x) for x in opt_preds), default=0)
    max_fan_out = max((len(x) for x in opt_succs), default=0)
    dispatch_seq_len = 0
    if opt_hints:
        dispatch_seq_len = max(int(h.get("id", -1)) for h in opt_hints) + 1

    debug = {
        "sequence_length": int(len(launches_rt or [])),
        "dispatch_sequence_length": int(max(len(launches_rt or []), dispatch_seq_len)),
        "total_cus": int(total_cus),
        "dependency_source": dep_source,
        "dependency_edges": opt_dep_edges,
        "dependency_edge_count": int(len(opt_dep_edges)),
        "max_fan_in": int(max_fan_in),
        "max_fan_out": int(max_fan_out),
        "used_profile_runtime": bool(runtime_cus or runtime_chiplet_cus or runtime_duration_ps),
        "profile_runtime": runtime_dbg,
        "runtime_geometry": runtime_geom_dbg,
        "used_metrics_csv": bool(metrics_duration_fallback or metrics_proxies),
        "metrics_csv": str(launch_metrics_csv or ""),
        "metrics_duration_entries": int(len(metrics_duration_ps)),
        "metrics_duration_fallback_count": int(len(metrics_duration_fallback)),
        "metrics_proxy_entries": int(len(metrics_proxies)),
        "timing_windows": windows,
        "chunk_candidates": chunk_debug,
        "predicted_schedule": opt_debug.get("chosen_strategy") and _evaluate_hint_plan(launches_rt, opt_hints, total_cus=int(total_cus), num_chiplets=int(num_chiplets or 0))[1] or {},
        "parallelism_graph_mermaid": launch_mermaid_graph(opt_nodes, opt_dep_edges, chunked=chunked_dispatch_ids),
        "strategy_search": opt_debug,
        "allow_partition_overrides": bool(allow_partition_overrides),
    }
    return opt_hints, debug




def protect_split_readonly_input_placements(
    tag2pages: Dict[str, List[int]],
    launches: List[dict],
    launch_hints: Optional[List[dict]],
    *,
    num_chiplets: int,
) -> Tuple[Dict[str, int], Dict[str, dict]]:
    fixed, debug = _split_kernel_readonly_proxy_fix(
        tag2pages,
        launches,
        launch_hints or [],
        num_chiplets=int(num_chiplets),
    )
    changed: Dict[str, int] = {}
    for tag, pages in fixed.items():
        old = tag2pages.get(tag)
        if not isinstance(old, list) or len(old) != len(pages):
            continue
        delta = sum(1 for a, b in zip(old, pages) if int(a) != int(b))
        if delta > 0:
            tag2pages[tag] = list(pages)
            changed[tag] = int(delta)
    return changed, debug


def protect_fanout_root_placements(
    tag2pages: Dict[str, List[int]],
    launch_hints: Optional[List[dict]],
    graph_obj: Optional[dict],
    *,
    num_chiplets: int,
) -> Tuple[Dict[str, int], Dict[str, dict]]:
    """Post-pass: keep high-fanout root buffers striped.

    After kernel overlap / itemization is decided, some buffers (notably fork
    roots like BERT's X/X2) should stay striped across the union of consumer
    chiplets. Otherwise locality may improve for one late low-parallel kernel
    but regress badly for q/k/v or other multi-consumer paths.
    """
    if not tag2pages or not isinstance(graph_obj, dict):
        return {}, {}
    by_launch = {}
    for h in (launch_hints or []):
        pid = _as_int(h.get("program_launch_id", h.get("id", -1)), -1)
        by_launch.setdefault(pid, []).append(h)
    consumer_launches: Dict[str, Set[int]] = {}
    for e in (graph_obj.get("edges", []) or []):
        if not isinstance(e, dict):
            continue
        tag = str(e.get("buffer", "") or "")
        if not tag:
            continue
        try:
            dst = int(e.get("to", -1))
        except Exception:
            continue
        if dst < 0:
            continue
        consumer_launches.setdefault(tag, set()).add(dst)
    changed: Dict[str, int] = {}
    debug: Dict[str, dict] = {}
    for tag, consumers in consumer_launches.items():
        if tag not in tag2pages or len(consumers) < 3:
            continue
        support: Set[int] = set()
        for pid in consumers:
            for h in by_launch.get(int(pid), []):
                for c, v in enumerate(h.get("chiplet_cus", []) or []):
                    if c < int(num_chiplets) and int(v or 0) > 0:
                        support.add(int(c))
        if len(support) < max(4, int(num_chiplets // 2)):
            continue
        pages = tag2pages.get(tag, []) or []
        current_support = {int(x) for x in pages if int(x) >= 0}
        if len(current_support) >= len(support):
            continue
        ordered = sorted(support)
        if not ordered:
            continue
        new_pages = [ordered[i % len(ordered)] for i in range(len(pages))]
        delta = sum(1 for a, b in zip(pages, new_pages) if int(a) != int(b))
        if delta <= 0:
            continue
        tag2pages[tag] = new_pages
        changed[tag] = int(delta)
        debug[tag] = {
            "reason": "fanout-root-protect",
            "consumers": sorted(int(x) for x in consumers),
            "consumer_chiplets": ordered,
            "pages_changed": int(delta),
        }
    return changed, debug

def preferred_chiplet_for_page(
    partition: str,
    num_chiplets: int,
    num_pages: int,
    page_index: int,
    *,
    active_chiplets: Optional[int] = None,
    group_pages: int = 1,
) -> Optional[int]:
    """Heuristic: pick a *preferred* chiplet for a page-block.

    This is only used to build the static cost model; it is NOT a correctness rule.

    We keep it intentionally simple:
      - Xdiv/Ydiv: contiguous ranges of pages per active chiplet
      - XblkN/YblkN: block-cyclic stripes at the (page-block) granularity
    """
    if num_pages <= 0 or num_chiplets <= 0:
        return None

    active = int(active_chiplets) if (active_chiplets and active_chiplets > 0) else int(num_chiplets)
    active = max(1, min(int(num_chiplets), active))

    gp = int(group_pages) if group_pages and group_pages > 0 else 1
    p = _lower(partition)

    if p.startswith("xdiv") or p.startswith("ydiv"):
        pages_per_part = _ceil_div(num_pages, active)
        pages_per_part = max(1, int(pages_per_part))
        c = int(page_index) // pages_per_part
        if c < 0:
            c = 0
        if c >= active:
            c = active - 1
        return c

    if p.startswith("xblk") or p.startswith("yblk"):
        blk = _parse_trailing_int(p) or 1
        blk = max(1, int(blk))
        block_index = int(page_index) // gp
        return int((block_index // blk) % active)

    return None


def choose_partition_weighted(
    kernel_name: str,
    launches: List[dict],
    buffers_by_tag: Dict[str, dict],
    karg_access: Dict[str, Dict[int, KernelArgAccess]],
    default_partition: str,
    *,
    num_chiplets: int,
    page_bytes: Optional[int] = None,
    util_tradeoff_alpha: float = 1.0,
) -> str:
    """
    Final seed-time Xdiv/Ydiv chooser for chiplet-level partitioning.

    Design decision:
      1) Decide the partition *once at seed time* from Step2 geometry.
         Replan must not flip Xdiv<->Ydiv, because seed/profile truth is only valid
         when the chiplet-level mapping remains the same across seed/profile/replay.

      2) For 2D kernels, the primary objective is *chiplet activation*.
         If Xdiv and Ydiv activate different numbers of chiplets, choose the direction
         that activates more chiplets. This is a chiplet-level scheduling rule, not an
         SM-private cache clustering rule: stranding chiplets at seed time hurts both
         profile quality and final execution, and later overlap/split cannot repair it.

      3) Only when both directions activate the same number of chiplets do we fall back
         to directional access-pattern evidence from Step1/Step2. This is where locality
         (e.g., CTA-clustering-style directional reuse intuition) still matters.

    Practical consequence:
      - attention_context-like kernels with (numWGX=2, numWGY>>num_chiplets) now
        reliably choose Ydiv at seed time.
      - The rule is simple, deterministic, and keeps seed/profile/replan consistent.
    """
    del util_tradeoff_alpha  # legacy knob; selection is now utilization-first.

    ks = [l for l in launches if l.get("kernel") == kernel_name]
    if not ks:
        return default_partition

    # --- (A) Dimension viability from Step2 geometry ---
    wgx_list: List[int] = []
    wgy_list: List[int] = []
    for l in ks:
        nx, ny, _ = workgroups_per_dim(l)
        if nx <= 0:
            nx = 1
        if ny <= 0:
            ny = 1
        wgx_list.append(nx)
        wgy_list.append(ny)

    min_wgx, max_wgx = min(wgx_list), max(wgx_list)
    min_wgy, max_wgy = min(wgy_list), max(wgy_list)

    x_ok = (min_wgx > 1)
    y_ok = (min_wgy > 1)

    if not x_ok and not y_ok:
        return default_partition
    if x_ok and not y_ok:
        return "Xdiv"
    if y_ok and not x_ok:
        return "Ydiv"

    util_x = _active_chiplets_xdiv(max_wgx, num_chiplets)
    util_y = _active_chiplets_ydiv(max_wgy, num_chiplets)

    # --- (B) Primary criterion: activate more chiplets ---
    # We decide this *before* locality. For chiplet-level partitioning, using more
    # chiplets at seed time is the safer and more reusable truth source for the later
    # profile-guided stages.
    if util_x > util_y:
        return "Xdiv"
    if util_y > util_x:
        return "Ydiv"

    # --- (C) Secondary criterion (tie only): directional access pattern ---
    # Equal chiplet activation means both directions have the same coarse-grain
    # parallelism. In this case, use directional locality / access-pattern evidence.
    rep = ks[0]

    # Optional conv/stencil-like tie-break from scalar geometry when available.
    # This stays in the tie-only path so that chiplet activation always has priority.
    scalars: Dict[str, float] = {}
    for _b in (rep.get('arg_bindings', []) or []):
        if _b.get('kind') == 'scalar' and _b.get('arg_name') is not None and _b.get('value') is not None:
            try:
                scalars[str(_b.get('arg_name'))] = float(_b.get('value'))
            except Exception:
                pass

    try:
        stripe_x = float(max_wgx) / float(max(1, util_x))
        stripe_y = float(max_wgy) / float(max(1, util_y))
    except Exception:
        stripe_x, stripe_y = 1.0, 1.0
    if stripe_x < 1.0:
        stripe_x = 1.0
    if stripe_y < 1.0:
        stripe_y = 1.0

    if 'Hin' in scalars and 'Win' in scalars and 'K' in scalars:
        hin = float(scalars.get('Hin', 0.0))
        win = float(scalars.get('Win', 0.0))
        ksz = float(scalars.get('K', 0.0))
        if hin > 0.0 and win > 0.0 and ksz > 1.0:
            halo_x = hin * (ksz - 1.0) * float(max(0, util_x - 1)) / stripe_x
            halo_y = win * (ksz - 1.0) * float(max(0, util_y - 1)) / stripe_y

            if page_bytes is not None:
                try:
                    pb = int(page_bytes)
                except Exception:
                    pb = 0
                if pb > 0 and 'Hout' in scalars and 'Wout' in scalars:
                    try:
                        Hout_i = int(scalars.get('Hout', 0) or 0)
                        Wout_i = int(scalars.get('Wout', 0) or 0)
                        Cout_i = int(scalars.get('Cout', 1) or 1)
                    except Exception:
                        Hout_i, Wout_i, Cout_i = 0, 0, 1
                    if Hout_i > 0 and Wout_i > 0:
                        out_tag = None
                        for _t in (rep.get('writes', []) or []):
                            if _t in buffers_by_tag:
                                out_tag = str(_t)
                                break
                        if out_tag is not None:
                            out_b = buffers_by_tag.get(out_tag) or {}
                            try:
                                out_bytes = int(out_b.get('bytes', 0) or 0)
                            except Exception:
                                out_bytes = 0
                            eb = _infer_elem_bytes_from_shape(out_bytes, H=int(Hout_i), W=int(Wout_i), C=int(Cout_i))
                            row_bytes = int(Wout_i) * int(eb)
                            if row_bytes > 0:
                                rem = int(pb) % int(row_bytes)
                                if rem == 0 or rem <= int(0.10 * row_bytes) or (row_bytes - rem) <= int(0.10 * row_bytes):
                                    return 'Ydiv'

            if halo_y < halo_x * 0.85:
                return 'Ydiv'
            if halo_x < halo_y * 0.85:
                return 'Xdiv'

    # Access-pattern vote (inspired by directional locality analysis, but adapted to
    # chiplet partitioning rather than SM-private cache clustering).
    x_score = 0.0
    y_score = 0.0

    for b in rep.get("arg_bindings", []) or []:
        if b.get("kind") != "buffer":
            continue
        tag = b.get("tag")
        if not tag or tag not in buffers_by_tag:
            continue

        sz = float(int(buffers_by_tag[tag].get("bytes", 0) or 0))
        arg_idx = int(b.get("arg_index", -1))
        acc = karg_access.get(kernel_name, {}).get(arg_idx)

        inst = 1.0
        if acc is not None:
            inst = float(acc.loads + acc.stores + 1)

        w = max(1.0, sz) * inst

        if acc is None:
            continue

        if acc.has_gid_y or acc.stride_kind == "strided":
            y_score += w
        elif acc.has_gid_x or acc.stride_kind == "contig":
            x_score += w

    if y_score > x_score:
        return "Ydiv"
    if x_score > y_score:
        return "Xdiv"

    # Last resort: when both chiplet activation and directional evidence tie, fall back
    # to the default static partition from the existing planner.
    return default_partition
def build_launch_buffer_traffic_weights(
    launches: List[dict],
    karg_access: Dict[str, Dict[int, KernelArgAccess]],
    *,
    dynamic_scale: str,
    load_req_bytes: float,
    load_rsp_bytes: float,
    store_req_bytes: float,
    store_rsp_bytes: float,
    store_weight: float,
    pc_boost: Optional[Dict[Tuple[int, str], float]] = None,
    step1_access_records: Optional[Dict[str, Dict[int, List[dict]]]] = None,
    loop_tripcount_alpha: float = 1.0,
    loop_tripcount_min: int = 8,
    loop_tripcount_cap: int = 65536,
    loop_tripcount_wg_max: int = 64,
) -> Tuple[Dict[Tuple[int, str], TrafficWeight], Dict[str, object]]:
    """v5 traffic weight = (static inst counts from Step1) * (dynamic_factor from Step2) * bytes model,
    plus an optional loop-tripcount scaling for low-parallel launches.

    Motivation:
      For kernels with small WG counts (e.g., reductions like GAP), a single loop-carried load inside
      the kernel can dominate runtime. Step1 exposes LLVM SCEV addrec trip counts, but earlier
      versions only counted the static load/store instructions, severely under-weighting such kernels.

    This scaling is conservative by default:
      - Only applied when total_workgroups(launch) <= loop_tripcount_wg_max
      - Only when Step1 addrec tripcount can be evaluated from Step2 scalar_map
      - Tripcount is clamped by loop_tripcount_cap
      - Strength controlled by loop_tripcount_alpha (0 disables; 1 applies full scaling)
    """
    out: Dict[Tuple[int, str], TrafficWeight] = {}

    stats: Dict[str, object] = {
        "loop_tripcount_alpha": float(loop_tripcount_alpha),
        "loop_tripcount_min": int(loop_tripcount_min),
        "loop_tripcount_cap": int(loop_tripcount_cap),
        "loop_tripcount_wg_max": int(loop_tripcount_wg_max),
        "scaled_pairs": 0,
        "scaled_launches": 0,
        "scaled_tags": 0,
        "examples": [],
    }

    scaled_tags: Set[str] = set()
    scaled_launches: Set[int] = set()

    for l in launches:
        lid = int(l.get("id", -1))
        kname = l.get("kernel", "")
        if lid < 0 or not kname:
            continue

        # map tag -> arg_index
        tag2arg: Dict[str, int] = {}
        for b in l.get("arg_bindings", []) or []:
            if b.get("kind") == "buffer" and b.get("tag") is not None:
                tag2arg[b["tag"]] = int(b.get("arg_index", -1))

        reads = set(l.get("reads", []) or [])
        writes = set(l.get("writes", []) or [])

        dyn = float(dynamic_factor(l, dynamic_scale))
        wgs_total = int(total_workgroups(l))
        scalar_map = l.get("scalar_map", {}) or {}

        for tag, arg_idx in tag2arg.items():
            acc = karg_access.get(kname, {}).get(arg_idx)

            loads = int(acc.loads) if acc is not None else 0
            stores = int(acc.stores) if acc is not None else 0

            # If Step2 says it's read/write but Step1 missed, add minimal count.
            if tag in reads and loads == 0 and stores == 0:
                loads = 1
            if tag in writes and stores == 0:
                stores = 1

            load_mul = 1.0
            store_mul = 1.0

            # Optional: loop tripcount scaling (only for small-WG launches)
            if (
                step1_access_records is not None
                and float(loop_tripcount_alpha) > 0.0
                and int(wgs_total) <= int(loop_tripcount_wg_max)
                and isinstance(scalar_map, dict)
                and arg_idx >= 0
            ):
                krec = step1_access_records.get(kname, {})
                recs = krec.get(arg_idx, []) if isinstance(krec, dict) else []
                if recs:
                    _sl, _ss, dl, ds, max_trip = estimate_dyn_mem_inst_units(
                        recs,
                        scalar_map,
                        trip_min=int(loop_tripcount_min),
                        trip_cap=int(loop_tripcount_cap),
                    )
                    if loads > 0 and dl > loads:
                        r = float(dl) / float(max(1, loads))
                        load_mul = 1.0 + float(loop_tripcount_alpha) * (r - 1.0)
                    if stores > 0 and ds > stores:
                        r = float(ds) / float(max(1, stores))
                        store_mul = 1.0 + float(loop_tripcount_alpha) * (r - 1.0)

                    if (load_mul > 1.0001) or (store_mul > 1.0001):
                        scaled_tags.add(str(tag))
                        scaled_launches.add(int(lid))
                        stats["scaled_pairs"] = int(stats.get("scaled_pairs", 0)) + 1
                        if len(stats.get("examples", [])) < 20:
                            stats["examples"].append({
                                "launch_id": int(lid),
                                "kernel": str(kname),
                                "tag": str(tag),
                                "arg_index": int(arg_idx),
                                "wgs_total": int(wgs_total),
                                "loads": int(loads),
                                "stores": int(stores),
                                "dyn_load_units": int(dl),
                                "dyn_store_units": int(ds),
                                "max_trip": int(max_trip),
                                "load_mul": float(load_mul),
                                "store_mul": float(store_mul),
                            })

            loads_f = float(loads) * dyn * float(load_mul)
            stores_f = float(stores) * dyn * float(store_mul)

            req = loads_f * float(load_req_bytes) + float(store_weight) * stores_f * float(store_req_bytes)
            rsp = loads_f * float(load_rsp_bytes) + float(store_weight) * stores_f * float(store_rsp_bytes)

            # If Step2 doesn't classify it, down-weight slightly.
            if tag not in reads and tag not in writes:
                req *= 0.5
                rsp *= 0.5

            if req <= 0 and rsp <= 0:
                req = 1.0

            # Producer-consumer global reuse boost (Phase2.4)
            if pc_boost:
                bf = float(pc_boost.get((lid, tag), 1.0))
                if bf != 1.0:
                    req *= bf
                    rsp *= bf

            out[(lid, tag)] = TrafficWeight(req_bytes=req, rsp_bytes=rsp)

    stats["scaled_tags"] = int(len(scaled_tags))
    stats["scaled_launches"] = int(len(scaled_launches))
    return out, stats


def compute_producer_consumer_boost(
    launches: List[dict],
    buffers_by_tag: Dict[str, dict],
    *,
    pc_alpha: float,
    pc_max_distance: int,
    pc_min_bytes: int = 0,
) -> Tuple[Dict[Tuple[int, str], float], Dict[str, object]]:
    """Build a (launch_id, tag) -> boost_factor map for producer-consumer reuse (Phase2.4).

    Motivation:
      Many ML/HPC programs have strong producer->consumer locality between consecutive kernels.
      Even if the *bytes* are not huge, these edges can sit on the critical path and the
      placement solver may otherwise ignore them when other traffic dominates.

    We detect reuse purely from Step2's ordered launches + (reads/writes) sets:
      - If a tag is written by launch P, and later read by launch C before the next write,
        we consider (P->C) a producer-consumer edge.

    We then boost the traffic weights of that tag on both P and C (conservatively) by a
    factor that decays with edge distance in launch order.

    Safety guards:
      - disabled when pc_alpha <= 1.0
      - ignore read-only buffers
      - optional min-bytes filter
    """
    boost: Dict[Tuple[int, str], float] = {}
    stats: Dict[str, object] = {
        "pc_alpha": float(pc_alpha),
        "pc_max_distance": int(pc_max_distance),
        "pc_min_bytes": int(pc_min_bytes),
        "edges": 0,
        "boosted_pairs": 0,
        "boosted_tags": 0,
        "edges_sample": [],
    }
    if pc_alpha is None:
        return boost, stats
    try:
        pc_alpha_f = float(pc_alpha)
    except Exception:
        pc_alpha_f = 1.0
    if pc_alpha_f <= 1.0:
        return boost, stats
    try:
        max_dist = int(pc_max_distance)
    except Exception:
        max_dist = 0
    if max_dist <= 0:
        return boost, stats

    # Use list order as program order (Step2 emits launches in order).
    last_writer: Dict[str, Tuple[int, int]] = {}  # tag -> (launch_id, pos)
    edges_per_tag: Dict[str, int] = {}
    boosted_tags: Set[str] = set()
    edges_sample: List[dict] = []

    for pos, l in enumerate(launches):
        lid = int(l.get("id", -1))
        if lid < 0:
            continue
        reads = list(l.get("reads", []) or [])
        writes = list(l.get("writes", []) or [])

        # process reads first (so in-place RMW can still see the previous writer)
        for tag in reads:
            if tag not in last_writer:
                continue
            prod_id, prod_pos = last_writer[tag]
            dist = int(pos - prod_pos)
            if dist <= 0 or dist > max_dist:
                continue
            binfo = buffers_by_tag.get(tag) or {}
            if bool(binfo.get("readonly", False)):
                continue
            bbytes = int(binfo.get("bytes", 0) or 0)
            if pc_min_bytes and bbytes < int(pc_min_bytes):
                continue

            # distance-decayed factor: dist=1 -> pc_alpha, dist=2 -> sqrt(pc_alpha), ...
            factor = float(pc_alpha_f) ** (1.0 / float(max(1, dist)))
            kp = (int(prod_id), str(tag))
            kc = (int(lid), str(tag))
            boost[kp] = max(float(boost.get(kp, 1.0)), float(factor))
            boost[kc] = max(float(boost.get(kc, 1.0)), float(factor))

            edges_per_tag[str(tag)] = int(edges_per_tag.get(str(tag), 0)) + 1
            boosted_tags.add(str(tag))
            stats["edges"] = int(stats.get("edges", 0)) + 1
            if len(edges_sample) < 20:
                edges_sample.append({"tag": str(tag), "prod": int(prod_id), "cons": int(lid), "dist": int(dist), "factor": float(factor)})

        # update last writers
        for tag in writes:
            binfo = buffers_by_tag.get(tag) or {}
            if bool(binfo.get("readonly", False)):
                continue
            last_writer[str(tag)] = (int(lid), int(pos))

    stats["boosted_pairs"] = int(len(boost))
    stats["boosted_tags"] = int(len(boosted_tags))
    stats["edges_sample"] = edges_sample
    # Keep edges_per_tag small in debug (top-20 by edge count)
    if edges_per_tag:
        top = sorted(edges_per_tag.items(), key=lambda kv: kv[1], reverse=True)[:20]
        stats["edges_per_tag_top"] = top
    else:
        stats["edges_per_tag_top"] = []

    return boost, stats


def _unknown_terms_base_only(uts: List[object]) -> bool:
    """True if unknown_terms looks like only a base pointer (%0/%1/...)."""
    if not uts:
        return True
    for ut in uts:
        if not isinstance(ut, str):
            continue
        s = ut.strip()
        if not s:
            continue
        if s.startswith("%") and s[1:].isdigit():
            continue
        return False
    return True




# -----------------------------
# SCEV / addrec helpers (loop tripcount scaling)
# -----------------------------

def _scev_eval_int(expr: str, scalar_map: Dict[str, int]) -> Optional[int]:
    """Safely evaluate a very small subset of LLVM SCEV integer expressions.

    Supported:
      - integer constants
      - %N variables (mapped to scalar_map['param_argN'])
      - +, -, *, //, %, <<, >>, parentheses

    If the expression contains unknown variables or non-numeric tokens, returns None.
    """
    if not isinstance(expr, str):
        return None
    s = expr.strip()
    if not s:
        return None

    # Drop SCEV decorations like <nsw>
    s = re.sub(r"<[^>]*>", "", s)

    # Replace %N with concrete scalar args when available
    def _repl(m: re.Match) -> str:
        try:
            n = int(m.group(1))
        except Exception:
            return m.group(0)
        key = f"param_arg{n}"
        if isinstance(scalar_map, dict) and key in scalar_map:
            try:
                return str(int(scalar_map[key]))
            except Exception:
                return m.group(0)
        return m.group(0)

    s = re.sub(r"%(\d+)", _repl, s)

    # If unresolved %N remains, bail.
    if re.search(r"%\d+", s):
        return None

    # Remove whitespace and convert division to integer division.
    s = s.replace(" ", "").replace("/", "//")

    # We only allow digits/operators/parens after substitution.
    if re.search(r"[A-Za-z_]", s):
        return None
    if re.search(r"[^0-9\+\-\*\/\%\(\)<>\|&^~]", s):
        # allow only simple operators; this also rejects commas, etc.
        return None

    try:
        node = ast.parse(s, mode="eval")
    except Exception:
        return None

    def _eval(n: ast.AST) -> Optional[int]:
        if isinstance(n, ast.Expression):
            return _eval(n.body)
        if isinstance(n, ast.Constant):
            if isinstance(n.value, bool):
                return int(n.value)
            if isinstance(n.value, (int,)):
                return int(n.value)
            if isinstance(n.value, float):
                return int(n.value)
            return None
        if isinstance(n, ast.UnaryOp):
            v = _eval(n.operand)
            if v is None:
                return None
            if isinstance(n.op, ast.UAdd):
                return +v
            if isinstance(n.op, ast.USub):
                return -v
            if isinstance(n.op, ast.Invert):
                return ~v
            return None
        if isinstance(n, ast.BinOp):
            a = _eval(n.left)
            b = _eval(n.right)
            if a is None or b is None:
                return None
            try:
                if isinstance(n.op, ast.Add):
                    return a + b
                if isinstance(n.op, ast.Sub):
                    return a - b
                if isinstance(n.op, ast.Mult):
                    return a * b
                if isinstance(n.op, (ast.FloorDiv, ast.Div)):
                    if b == 0:
                        return None
                    return a // b
                if isinstance(n.op, ast.Mod):
                    if b == 0:
                        return None
                    return a % b
                if isinstance(n.op, ast.LShift):
                    return a << b
                if isinstance(n.op, ast.RShift):
                    return a >> b
            except Exception:
                return None
            return None
        return None

    v = _eval(node)
    if v is None:
        return None
    return int(v)


def estimate_addrec_tripcount(addrec: dict, scalar_map: Dict[str, int], *, cap: int) -> Optional[int]:
    """Estimate loop tripcount for an LLVM SCEV addrec record.

    Returns an integer tripcount >= 1 (clamped by cap), or None if unknown.
    """
    if not isinstance(addrec, dict):
        return None
    # Preferred: constant trip count
    try:
        tc = _as_int(addrec.get("trip_count_const", -1), -1)
    except Exception:
        tc = -1
    if tc is not None and int(tc) > 0:
        return int(min(int(tc), int(cap)))

    # Try trip_count_scev, then backedge + 1
    expr = addrec.get("trip_count_scev")
    v = _scev_eval_int(expr, scalar_map) if isinstance(expr, str) else None
    if v is None:
        be = addrec.get("backedge_taken_count_scev")
        be_v = _scev_eval_int(be, scalar_map) if isinstance(be, str) else None
        if be_v is not None:
            v = int(be_v) + 1
    if v is None:
        return None
    v = max(1, int(v))
    return int(min(int(v), int(cap)))


def estimate_dyn_mem_inst_units(
    records: List[dict],
    scalar_map: Dict[str, int],
    *,
    trip_min: int,
    trip_cap: int,
) -> Tuple[int, int, int, int, int]:
    """Estimate dynamic memory-instruction units for a (kernel,arg) based on Step1 records.

    Returns:
      static_loads, static_stores, dyn_load_units, dyn_store_units, max_trip
    """
    static_loads = 0
    static_stores = 0
    dyn_load = 0
    dyn_store = 0
    max_trip = 1

    for ma in records or []:
        kind = _lower(ma.get("kind", ""))
        if kind == "load":
            static_loads += 1
        elif kind == "store":
            static_stores += 1
        else:
            continue

        mul = 1
        addrec = ma.get("addrec")
        if isinstance(addrec, dict):
            tc = estimate_addrec_tripcount(addrec, scalar_map, cap=int(trip_cap))
            if tc is not None and int(tc) >= int(trip_min):
                mul = int(tc)
                if mul > max_trip:
                    max_trip = mul

        if kind == "load":
            dyn_load += mul
        else:
            dyn_store += mul

    return static_loads, static_stores, dyn_load, dyn_store, max_trip

_LINEAR_VARS = {
    "gid_x", "gid_y", "gid_z",
    "wg_x", "wg_y", "wg_z",
    "lid_x", "lid_y", "lid_z",
}


def _linear_range_on_box(coeffs: Dict[str, int], const: int, bounds: Dict[str, Tuple[int, int]]) -> Tuple[int, int]:
    """Range of a linear form on a hyper-rectangle (independent bounds).

    min = const + Σ coeff_i * (min_i if coeff_i>=0 else max_i)
    max = const + Σ coeff_i * (max_i if coeff_i>=0 else min_i)
    """
    mn = int(const)
    mx = int(const)
    for v, a in (coeffs or {}).items():
        a = int(a)
        if a == 0:
            continue
        if v not in bounds:
            # unknown variable -> treat as non-analyzable by caller
            continue
        lo, hi = bounds[v]
        lo = int(lo); hi = int(hi)
        if a >= 0:
            mn += a * lo
            mx += a * hi
        else:
            mn += a * hi
            mx += a * lo
    if mn <= mx:
        return mn, mx
    return mx, mn


def _chiplet_wg_regions(
    partition: str,
    launch: dict,
    num_chiplets: int,
    *,
    max_blocks: int = 200000,
) -> Tuple[List[List[Tuple[Tuple[int, int], Tuple[int, int], Tuple[int, int]]]], Optional[str]]:
    """Return per-chiplet WG regions as a list of rectangles:
       [ [(wgx0,wgx1),(wgy0,wgy1),(wgz0,wgz1)], ... ]  inclusive bounds.

    For Xdiv/Ydiv we return a single rectangle per active chiplet.
    For Xblk/Yblk we may return multiple rectangles (block-cyclic).
    """
    if num_chiplets <= 0:
        return [[]], "num_chiplets<=0"

    wgx, wgy, wgz = workgroups_per_dim(launch)
    wgx = max(1, int(wgx))
    wgy = max(1, int(wgy))
    wgz = max(1, int(wgz))

    # default: full ranges
    x_ints = [[(0, wgx - 1)] for _ in range(num_chiplets)]
    y_ints = [[(0, wgy - 1)] for _ in range(num_chiplets)]
    z_full = (0, wgz - 1)

    p = _lower(partition)

    if p.startswith("xdiv"):
        x_ints = [[] for _ in range(num_chiplets)]
        div = _ceil_div(wgx, num_chiplets)
        if div <= 0:
            div = 1
        for c in range(num_chiplets):
            x0 = c * div
            x1 = min((c + 1) * div, wgx) - 1
            if x0 <= x1:
                x_ints[c].append((x0, x1))

    elif p.startswith("ydiv"):
        y_ints = [[] for _ in range(num_chiplets)]
        div = int(wgy // num_chiplets)
        if div <= 0:
            div = 1
        for c in range(num_chiplets):
            if c < num_chiplets - 1:
                y0 = c * div
                y1 = min((c + 1) * div, wgy) - 1
            else:
                y0 = (num_chiplets - 1) * div
                y1 = wgy - 1
            if y0 <= y1:
                y_ints[c].append((y0, y1))

    elif p.startswith("xblk"):
        blk = _parse_trailing_int(p) or 1
        blk = max(1, int(blk))
        blocks = _ceil_div(wgx, blk)
        if blocks > int(max_blocks):
            return [[] for _ in range(num_chiplets)], f"xblk too many blocks: {blocks} > {max_blocks}"
        x_ints = [[] for _ in range(num_chiplets)]
        for b in range(blocks):
            c = b % num_chiplets
            x0 = b * blk
            x1 = min((b + 1) * blk, wgx) - 1
            if x0 <= x1:
                x_ints[c].append((x0, x1))

    elif p.startswith("yblk"):
        blk = _parse_trailing_int(p) or 1
        blk = max(1, int(blk))
        blocks = _ceil_div(wgy, blk)
        if blocks > int(max_blocks):
            return [[] for _ in range(num_chiplets)], f"yblk too many blocks: {blocks} > {max_blocks}"
        y_ints = [[] for _ in range(num_chiplets)]
        for b in range(blocks):
            c = b % num_chiplets
            y0 = b * blk
            y1 = min((b + 1) * blk, wgy) - 1
            if y0 <= y1:
                y_ints[c].append((y0, y1))

    # form rectangles
    out: List[List[Tuple[Tuple[int, int], Tuple[int, int], Tuple[int, int]]]] = [[] for _ in range(num_chiplets)]
    for c in range(num_chiplets):
        for xi in x_ints[c]:
            for yi in y_ints[c]:
                if xi[0] > xi[1] or yi[0] > yi[1]:
                    continue
                out[c].append((xi, yi, z_full))

    return out, None


def _bounds_for_region(
    region: Tuple[Tuple[int, int], Tuple[int, int], Tuple[int, int]],
    launch: dict,
    scalar_map: Dict[str, int],
) -> Dict[str, Tuple[int, int]]:
    """Compute best-effort bounds for linear vars within a WG-region."""
    (wx0, wx1), (wy0, wy1), (wz0, wz1) = region
    gs = launch.get("global_size", []) or [1, 1, 1]
    ls = launch.get("local_size", []) or [1, 1, 1]
    Gx, Gy, Gz = [max(1, int(x)) for x in gs[:3]]
    Lx, Ly, Lz = [max(1, int(x)) for x in ls[:3]]

    # lid ranges always full within a WG
    bounds: Dict[str, Tuple[int, int]] = {
        "lid_x": (0, Lx - 1),
        "lid_y": (0, Ly - 1),
        "lid_z": (0, Lz - 1),
        "wg_x": (int(wx0), int(wx1)),
        "wg_y": (int(wy0), int(wy1)),
        "wg_z": (int(wz0), int(wz1)),
    }

    # gid bounds (approx; ignores partial last WG in interior of the range)
    gx0 = int(wx0) * Lx
    gx1 = min((int(wx1) + 1) * Lx, Gx) - 1
    gy0 = int(wy0) * Ly
    gy1 = min((int(wy1) + 1) * Ly, Gy) - 1
    gz0 = int(wz0) * Lz
    gz1 = min((int(wz1) + 1) * Lz, Gz) - 1

    bounds["gid_x"] = (max(0, gx0), max(0, gx1))
    bounds["gid_y"] = (max(0, gy0), max(0, gy1))
    bounds["gid_z"] = (max(0, gz0), max(0, gz1))

    # scalar params are constants: min==max
    for k, v in (scalar_map or {}).items():
        if isinstance(k, str) and k.startswith("param_"):
            bounds[k] = (int(v), int(v))

    return bounds


def _access_pages_by_chiplet(
    access: dict,
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    num_pages: int,
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[Set[int]]], Optional[str]]:
    """Return per-chiplet touched *page indices* for a Step1 mem_access record.

    This is a best-effort affine range model.
    """
    if num_pages <= 0:
        return None, "num_pages<=0"
    if page_bytes <= 0:
        return None, "page_bytes<=0"
    coeffs0 = access.get("coeffs", {}) or {}
    if not isinstance(coeffs0, dict) or not coeffs0:
        return None, "no coeffs"
    # allow only known linear vars + param_*
    for v in coeffs0.keys():
        if not isinstance(v, str):
            return None, "bad var key"
        if v in _LINEAR_VARS:
            continue
        if v.startswith("param_"):
            continue
        return None, f"unknown var {v}"

    # Reject complex unknown_terms (likely non-linear start offsets)
    if not _unknown_terms_base_only(access.get("unknown_terms", []) or []):
        return None, "complex unknown_terms"

    const = int(access.get("constant", 0) or 0)

    # scalar map from Step2 (param_* values)
    scalar_map = (launch.get("scalar_map", {}) or {})
    if not isinstance(scalar_map, dict):
        scalar_map = {}

    # Fold param_* terms into the constant (they don't vary across threads).
    coeffs: Dict[str, int] = {}
    for v, a in coeffs0.items():
        a = int(a)
        if a == 0:
            continue
        if v.startswith("param_"):
            val = scalar_map.get(v, 0)
            try:
                val = int(val)
            except Exception:
                val = 0
            const += a * val
        else:
            coeffs[v] = a

    if not coeffs:
        # address doesn't depend on thread/WG IDs (broadcast-like)
        # Still analyzable: every chiplet touches the same page(s)
        coeffs = {}

    regions_by_c, err = _chiplet_wg_regions(partition, launch, num_chiplets, max_blocks=max_blocks)
    if err:
        return None, err

    pages_by_c: List[Set[int]] = [set() for _ in range(num_chiplets)]

    for c in range(num_chiplets):
        for region in regions_by_c[c]:
            bounds = _bounds_for_region(region, launch, scalar_map)
            # If any coefficient refers to a var we couldn't bound, skip.
            ok = True
            for v in coeffs.keys():
                if v not in bounds:
                    ok = False
                    break
            if not ok:
                return None, f"missing bounds for vars {list(coeffs.keys())}"

            mn, mx = _linear_range_on_box(coeffs, const, bounds)
            # Convert to pages.
            p0 = int(math.floor(mn / float(page_bytes)))
            p1 = int(math.floor(mx / float(page_bytes)))
            if p0 > p1:
                p0, p1 = p1, p0
            # Clamp to buffer.
            if p1 < 0 or p0 >= num_pages:
                continue
            p0 = max(0, p0)
            p1 = min(num_pages - 1, p1)
            for p in range(p0, p1 + 1):
                pages_by_c[c].add(p)

    return pages_by_c, None


# -----------------------------
# Phase2.1 pattern plugins
# -----------------------------

def _is_plausible_elem_bytes(x: int) -> bool:
    try:
        xi = int(x)
    except Exception:
        return False
    return xi in (1, 2, 4, 8, 16)


def _buf_bytes_pages(buffers_by_tag: Dict[str, dict], tag: str, page_bytes: int) -> Tuple[int, int]:
    b = buffers_by_tag.get(tag) or {}
    bytes_ = int(b.get("bytes", 0) or 0)
    pages = int(b.get("pages", 0) or 0)
    if pages <= 0 and bytes_ > 0 and page_bytes > 0:
        pages = int(math.ceil(bytes_ / float(page_bytes)))
    return bytes_, pages


def _extract_launch_bindings(launch: dict) -> Tuple[List[dict], List[Tuple[str, int]]]:
    """Return (buffer_args, scalar_args) from Step2 launch arg_bindings."""
    buf_args: List[dict] = []
    scalar_args: List[Tuple[str, int]] = []
    for b in (launch.get("arg_bindings", []) or []):
        kind = b.get("kind")
        if kind == "buffer" and b.get("tag") is not None:
            try:
                buf_args.append({
                    "arg_index": int(b.get("arg_index", -1)),
                    "arg_name": str(b.get("arg_name", "")),
                    "tag": str(b.get("tag")),
                })
            except Exception:
                continue
        elif kind == "scalar":
            name = str(b.get("arg_name", ""))
            v = b.get("value", None)
            if isinstance(v, bool):
                continue
            if isinstance(v, (int, float)):
                try:
                    scalar_args.append((name, int(v)))
                except Exception:
                    pass
    return buf_args, scalar_args


def _match_gemm_like(launch: dict, buffers_by_tag: Dict[str, dict], page_bytes: int) -> Optional[dict]:
    """Try to match a 1D-flattened GEMM-like kernel:
       tid in [0, M*N) , row=tid/N, col=tid%N
       C[row*N+col] = sum_k A[row*K+k] * B[k*N+col]
    We do NOT rely on kernel name. We rely on (arg_bindings + scalar relations + buffer sizes).
    """
    gs = launch.get("global_size", []) or [0, 0, 0]
    ls = launch.get("local_size", []) or [1, 1, 1]
    try:
        Gx, Gy, Gz = int(gs[0]), int(gs[1]), int(gs[2])
        Lx, Ly, Lz = int(ls[0]), int(ls[1]), int(ls[2])
    except Exception:
        return None
    if Gy != 1 or Gz != 1 or Ly != 1 or Lz != 1:
        # This plugin currently targets common 1D-flattened GEMM kernels.
        return None
    if Lx <= 0:
        return None

    buf_args, scalar_args = _extract_launch_bindings(launch)
    if len(buf_args) < 3 or len(scalar_args) < 3:
        return None

    # Identify candidate output tag among writes
    writes = set(launch.get("writes", []) or [])
    buf_tags = [x.get("tag") for x in buf_args if x.get("tag")]
    cand_out = [t for t in buf_tags if t in writes]
    if not cand_out:
        return None
    # choose largest output buffer by bytes
    cand_out.sort(key=lambda t: _buf_bytes_pages(buffers_by_tag, t, page_bytes)[0], reverse=True)
    C_tag = cand_out[0]

    # Try to find M,N,K by name first
    scalar_map = {str(n).lower(): int(v) for (n, v) in scalar_args}
    def _get_scalar(name: str) -> Optional[int]:
        return scalar_map.get(name.lower())

    M = _get_scalar("m")
    N = _get_scalar("n")
    K = _get_scalar("k")

    # If not found, infer from scalar values and global_size_x padding
    vals = [int(v) for (_n, v) in scalar_args if int(v) > 0]
    if M is None or N is None or K is None:
        # brute force choose (M,N,K) from values
        best = None  # (diff, M, N, K)
        for i in range(len(vals)):
            for j in range(len(vals)):
                if j == i:
                    continue
                m_, n_ = vals[i], vals[j]
                prod = int(m_) * int(n_)
                # global size may be padded up to multiple of Lx
                padded = _ceil_div(prod, max(1, Lx)) * max(1, Lx)
                if Gx == prod or Gx == padded or (Gx > prod and (Gx - prod) < max(1, Lx)):
                    # pick a K from remaining values (any)
                    for kidx in range(len(vals)):
                        if kidx == i or kidx == j:
                            continue
                        k_ = vals[kidx]
                        diff = abs(Gx - padded)
                        cand = (diff, m_, n_, k_)
                        if best is None or cand < best:
                            best = cand
        if best is None:
            return None
        _diff, M, N, K = best

    try:
        M = int(M); N = int(N); K = int(K)
    except Exception:
        return None
    if M <= 0 or N <= 0 or K <= 0:
        return None

    total = int(M) * int(N)
    padded_total = _ceil_div(total, max(1, Lx)) * max(1, Lx)
    if not (Gx == total or Gx == padded_total or (Gx > total and (Gx - total) < max(1, Lx))):
        return None

    # Identify A/B tags among reads (prefer arg_name A/B if present)
    reads = set(launch.get("reads", []) or [])
    read_tags = [t for t in buf_tags if t in reads and t != C_tag]
    if len(read_tags) < 1:
        # some kernels might not list reads; be conservative
        read_tags = [t for t in buf_tags if t != C_tag]
    if len(read_tags) < 2:
        return None

    # Prefer arg_name mapping if present
    name2tag = {str(x.get("arg_name","")).lower(): str(x.get("tag")) for x in buf_args if x.get("tag")}
    A_tag = None; B_tag = None
    if "a" in name2tag and "b" in name2tag:
        A_tag = name2tag.get("a"); B_tag = name2tag.get("b")
    else:
        # choose two largest read buffers by bytes as A/B
        read_tags.sort(key=lambda t: _buf_bytes_pages(buffers_by_tag, t, page_bytes)[0], reverse=True)
        A_tag, B_tag = read_tags[0], read_tags[1]

    if not A_tag or not B_tag:
        return None

    # Validate element sizes via buffer sizes (strong filter to keep this generic & safe)
    C_bytes, C_pages = _buf_bytes_pages(buffers_by_tag, C_tag, page_bytes)
    A_bytes, A_pages = _buf_bytes_pages(buffers_by_tag, A_tag, page_bytes)
    B_bytes, B_pages = _buf_bytes_pages(buffers_by_tag, B_tag, page_bytes)

    if C_bytes <= 0 or A_bytes <= 0 or B_bytes <= 0:
        return None

    if C_bytes % max(1, total) != 0:
        return None
    elem = int(C_bytes // max(1, total))
    if not _is_plausible_elem_bytes(elem):
        return None

    # A is M*K, B is K*N for this canonical GEMM
    if A_bytes % (int(M) * int(K)) != 0:
        return None
    if B_bytes % (int(K) * int(N)) != 0:
        return None
    elemA = int(A_bytes // (int(M) * int(K)))
    elemB = int(B_bytes // (int(K) * int(N)))
    if elemA != elem or elemB != elem:
        return None

    return {
        "kind": "gemm",
        "M": int(M), "N": int(N), "K": int(K),
        "A": str(A_tag), "B": str(B_tag), "C": str(C_tag),
        "elem_bytes": int(elem),
        "Lx": int(Lx),
    }


def _pages_from_elem_range(
    elem0: int,
    elem1: int,
    elem_bytes: int,
    *,
    num_pages: int,
    page_bytes: int,
) -> Set[int]:
    if elem1 < elem0:
        return set()
    if elem_bytes <= 0 or page_bytes <= 0 or num_pages <= 0:
        return set()
    b0 = int(elem0) * int(elem_bytes)
    b1 = (int(elem1) + 1) * int(elem_bytes) - 1
    p0 = int(b0 // int(page_bytes))
    p1 = int(b1 // int(page_bytes))
    if p1 < 0 or p0 >= num_pages:
        return set()
    p0 = max(0, p0)
    p1 = min(num_pages - 1, p1)
    # Avoid building enormous sets
    if (p1 - p0) > 500000:
        return set()
    return set(range(p0, p1 + 1))



def _page_weights_from_elem_range(
    elem0: int,
    elem1: int,
    elem_bytes: int,
    *,
    num_pages: int,
    page_bytes: int,
    max_pages: int = 200000,
) -> Dict[int, float]:
    """Return per-page weights for a contiguous element-index range.

    Weight unit: (approx) number of elements that fall into each page.
    This is useful for *writes* (outputs): it lets the planner prefer the chiplet that writes
    the majority of a page when page boundaries do not align perfectly with chiplet partitions.

    The function is conservative: if the covered page span is too large, it returns {} to avoid
    pathological memory/time usage.
    """
    if elem1 < elem0:
        return {}
    if elem_bytes <= 0 or page_bytes <= 0 or num_pages <= 0:
        return {}
    # byte range [b0, b1]
    b0 = int(elem0) * int(elem_bytes)
    b1 = (int(elem1) + 1) * int(elem_bytes) - 1
    p0 = int(b0 // int(page_bytes))
    p1 = int(b1 // int(page_bytes))
    if p1 < 0 or p0 >= num_pages:
        return {}
    p0 = max(0, p0)
    p1 = min(num_pages - 1, p1)
    if (p1 - p0) > int(max_pages):
        return {}
    out: Dict[int, float] = {}
    for p in range(int(p0), int(p1) + 1):
        pb0 = int(p) * int(page_bytes)
        pb1 = pb0 + int(page_bytes) - 1
        lo = max(int(b0), int(pb0))
        hi = min(int(b1), int(pb1))
        if hi < lo:
            continue
        # bytes covered in this page
        cov_bytes = int(hi - lo + 1)
        w = float(cov_bytes) / float(max(1, int(elem_bytes)))
        if w <= 0.0:
            continue
        out[p] = out.get(p, 0.0) + w
    return out



def _gemm_pages_by_chiplet(
    match: dict,
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Return per-chiplet pages touched for GEMM (best-effort):

    Modeled tags:
      - C (output): contiguous output indices per WG block (returns *weighted* pages)
      - A (input): rows touched by those output indices (unweighted set of pages)

    Notes:
      - B is typically very strided/shared; we intentionally do not model it here and rely on
        readonly striping or legacy heuristics to avoid unsafe guesses.
      - For C (writes), we return per-page weights (#elements) to better reflect write footprint.
    """
    if match.get("kind") != "gemm":
        return None, "not gemm"

    M = int(match["M"]); N = int(match["N"]); K = int(match["K"])
    elem = int(match["elem_bytes"])
    Lx = int(match.get("Lx", 1))
    A_tag = str(match["A"]); B_tag = str(match["B"]); C_tag = str(match["C"])
    tag = str(tag)

    if tag not in (A_tag, C_tag):
        return None, "gemm:tag_not_modeled"

    # buffer pages
    _, np = _buf_bytes_pages(buffers_by_tag, tag, page_bytes)
    if np <= 0:
        return None, "gemm:num_pages<=0"

    regions, err = _chiplet_wg_regions(partition, launch, num_chiplets, max_blocks=max_blocks)
    if err:
        return None, f"gemm:wg_regions:{err}"

    total_elems = int(M) * int(N)

    # For outputs (C), return per-page weights; for A, return a set.
    if tag == C_tag:
        pages_by_c: List[Dict[int, float]] = [dict() for _ in range(num_chiplets)]
    else:
        pages_by_c = [set() for _ in range(num_chiplets)]

    for c in range(num_chiplets):
        for rect in regions[c]:
            (wx0, wx1), (wy0, wy1), (wz0, wz1) = rect
            # 1D: wy0==wy1==0
            tid0 = int(wx0) * int(Lx)
            tid1 = (int(wx1) + 1) * int(Lx) - 1
            tid1 = min(tid1, total_elems - 1)
            if tid0 > tid1:
                continue

            if tag == C_tag:
                wdict = _page_weights_from_elem_range(tid0, tid1, elem, num_pages=np, page_bytes=page_bytes)
                if not wdict:
                    # fall back to unweighted pages if weights are too large to build
                    for p in _pages_from_elem_range(tid0, tid1, elem, num_pages=np, page_bytes=page_bytes):
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + 1.0
                else:
                    for p, w in wdict.items():
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + float(w)

            elif tag == A_tag:
                row0 = int(tid0) // int(N)
                row1 = int(tid1) // int(N)
                a0 = int(row0) * int(K)
                a1 = (int(row1) + 1) * int(K) - 1
                pages_by_c[c] |= _pages_from_elem_range(a0, a1, elem, num_pages=np, page_bytes=page_bytes)

    if tag == C_tag:
        if not any(len(d) > 0 for d in pages_by_c):
            return None, "gemm:no_pages"
    else:
        if not any(len(s) > 0 for s in pages_by_c):
            return None, "gemm:no_pages"

    return pages_by_c, f"pattern:gemm:{tag}"

def _match_attention_scores(launch: dict, buffers_by_tag: Dict[str, dict], page_bytes: int) -> Optional[dict]:
    """Match a common attention score kernel pattern:
         tid in [0, NumHeads*SeqLen*SeqLen)
         h = tid / (SeqLen*SeqLen)
         rem = tid - h*perHead
         i = rem / SeqLen
         j = rem - i*SeqLen
         qBase = i*dModel + h*DHead
         kBase = j*dModel + h*DHead
         Scores[tid] = dot(Q[qBase:qBase+DHead], K[kBase:kBase+DHead])
       We detect via scalar relations + buffer sizes, not kernel names.
    """
    gs = launch.get("global_size", []) or [0, 0, 0]
    ls = launch.get("local_size", []) or [1, 1, 1]
    try:
        Gx, Gy, Gz = int(gs[0]), int(gs[1]), int(gs[2])
        Lx = int(ls[0])
        Ly, Lz = int(ls[1]), int(ls[2])
    except Exception:
        return None
    if Gy != 1 or Gz != 1 or Ly != 1 or Lz != 1:
        return None
    if Lx <= 0:
        return None

    buf_args, scalar_args = _extract_launch_bindings(launch)
    if len(buf_args) < 3 or len(scalar_args) < 3:
        return None

    buf_tags = [x.get("tag") for x in buf_args if x.get("tag")]
    writes = set(launch.get("writes", []) or [])
    cand_out = [t for t in buf_tags if t in writes]
    if not cand_out:
        return None
    cand_out.sort(key=lambda t: _buf_bytes_pages(buffers_by_tag, t, page_bytes)[0], reverse=True)
    S_tag = cand_out[0]

    reads = set(launch.get("reads", []) or [])
    read_tags = [t for t in buf_tags if t in reads and t != S_tag]
    if len(read_tags) < 2:
        # conservative
        read_tags = [t for t in buf_tags if t != S_tag]
    if len(read_tags) < 2:
        return None
    # choose two largest reads as Q/K
    read_tags.sort(key=lambda t: _buf_bytes_pages(buffers_by_tag, t, page_bytes)[0], reverse=True)
    Q_tag, K_tag = read_tags[0], read_tags[1]

    # Find (SeqLen, DHead, NumHeads) among scalar values by brute force
    vals = [int(v) for (_n, v) in scalar_args if int(v) > 0]
    if len(vals) < 3:
        return None

    S_bytes, _ = _buf_bytes_pages(buffers_by_tag, S_tag, page_bytes)
    Q_bytes, _ = _buf_bytes_pages(buffers_by_tag, Q_tag, page_bytes)
    K_bytes, _ = _buf_bytes_pages(buffers_by_tag, K_tag, page_bytes)
    if S_bytes <= 0 or Q_bytes <= 0 or K_bytes <= 0:
        return None

    best = None  # (diff, SeqLen, DHead, NumHeads, elem_bytes)
    for a in vals:
        for b in vals:
            for c in vals:
                if a <= 0 or b <= 0 or c <= 0:
                    continue
                SeqLen = a; DHead = b; NumHeads = c
                total = int(NumHeads) * int(SeqLen) * int(SeqLen)
                padded = _ceil_div(total, max(1, Lx)) * max(1, Lx)
                if not (Gx == total or Gx == padded or (Gx > total and (Gx - total) < max(1, Lx))):
                    continue
                # element bytes from Scores
                if S_bytes % max(1, total) != 0:
                    continue
                elem = int(S_bytes // max(1, total))
                if not _is_plausible_elem_bytes(elem):
                    continue
                # Q/K bytes should match SeqLen*DHead*NumHeads
                q_elems = int(SeqLen) * int(DHead) * int(NumHeads)
                if Q_bytes % max(1, q_elems) != 0:
                    continue
                if K_bytes % max(1, q_elems) != 0:
                    continue
                elemQ = int(Q_bytes // max(1, q_elems))
                elemK = int(K_bytes // max(1, q_elems))
                if elemQ != elem or elemK != elem:
                    continue
                diff = abs(Gx - padded)
                cand = (diff, SeqLen, DHead, NumHeads, elem)
                if best is None or cand < best:
                    best = cand

    if best is None:
        return None
    _diff, SeqLen, DHead, NumHeads, elem = best
    dModel = int(NumHeads) * int(DHead)

    return {
        "kind": "att_scores",
        "SeqLen": int(SeqLen),
        "DHead": int(DHead),
        "NumHeads": int(NumHeads),
        "dModel": int(dModel),
        "Q": str(Q_tag),
        "K": str(K_tag),
        "Scores": str(S_tag),
        "elem_bytes": int(elem),
        "Lx": int(Lx),
    }



def _att_scores_pages_by_chiplet(
    match: dict,
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Per-chiplet pages for attention_scores-like kernel.

    We model:
      - Scores (output): contiguous tid range (returns *weighted* pages)
      - Q: slices per (h,i) pairs within tid range
      - K: slices per (h,j) pairs within tid range

    For Scores (writes), we return per-page weights (#elements) to reflect write footprint.
    """
    if match.get("kind") != "att_scores":
        return None, "not att_scores"
    SeqLen = int(match["SeqLen"])
    DHead = int(match["DHead"])
    NumHeads = int(match["NumHeads"])
    dModel = int(match["dModel"])
    elem = int(match["elem_bytes"])
    Lx = int(match.get("Lx", 1))

    Q_tag = str(match["Q"]); K_tag = str(match["K"]); S_tag = str(match["Scores"])
    tag = str(tag)
    if tag not in (Q_tag, K_tag, S_tag):
        return None, "att_scores:tag_not_modeled"

    # buffer pages
    _, np = _buf_bytes_pages(buffers_by_tag, tag, page_bytes)
    if np <= 0:
        return None, "att_scores:num_pages<=0"

    per_head = int(SeqLen) * int(SeqLen)
    total = int(NumHeads) * int(per_head)

    regions, err = _chiplet_wg_regions(partition, launch, num_chiplets, max_blocks=max_blocks)
    if err:
        return None, f"att_scores:wg_regions:{err}"

    if tag == S_tag:
        pages_by_c: List[Dict[int, float]] = [dict() for _ in range(num_chiplets)]
    else:
        pages_by_c = [set() for _ in range(num_chiplets)]

    def _add_Q_slice(c: int, h: int, i0: int, i1: int):
        # Q is laid out as [SeqLen][dModel], head is inside columns
        # element index for start of slice: i*dModel + h*DHead
        for i in range(int(i0), int(i1) + 1):
            e0 = int(i) * int(dModel) + int(h) * int(DHead)
            e1 = e0 + int(DHead) - 1
            pages_by_c[c] |= _pages_from_elem_range(e0, e1, elem, num_pages=np, page_bytes=page_bytes)

    def _add_K_slice(c: int, h: int, j0: int, j1: int):
        for j in range(int(j0), int(j1) + 1):
            e0 = int(j) * int(dModel) + int(h) * int(DHead)
            e1 = e0 + int(DHead) - 1
            pages_by_c[c] |= _pages_from_elem_range(e0, e1, elem, num_pages=np, page_bytes=page_bytes)

    for c in range(num_chiplets):
        for rect in regions[c]:
            (wx0, wx1), (_wy0, _wy1), (_wz0, _wz1) = rect
            t0 = int(wx0) * int(Lx)
            t1 = (int(wx1) + 1) * int(Lx) - 1
            t1 = min(t1, total - 1)
            if t0 > t1:
                continue

            if tag == S_tag:
                wdict = _page_weights_from_elem_range(t0, t1, elem, num_pages=np, page_bytes=page_bytes)
                if not wdict:
                    for p in _pages_from_elem_range(t0, t1, elem, num_pages=np, page_bytes=page_bytes):
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + 1.0
                else:
                    for p, w in wdict.items():
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + float(w)
                continue

            # For Q/K: walk head slices overlapped by [t0,t1]
            h0 = int(t0) // int(per_head)
            h1 = int(t1) // int(per_head)

            for h in range(h0, h1 + 1):
                head_base = int(h) * int(per_head)
                s = max(int(t0), head_base)
                e = min(int(t1), head_base + int(per_head) - 1)
                if s > e:
                    continue
                rem0 = int(s) - head_base
                rem1 = int(e) - head_base
                i0 = int(rem0) // int(SeqLen)
                i1 = int(rem1) // int(SeqLen)
                j0 = int(rem0) % int(SeqLen)
                j1 = int(rem1) % int(SeqLen)

                if tag == Q_tag:
                    # Q depends on (h,i) only: all i in [i0,i1] touched
                    _add_Q_slice(c, h, i0, i1)
                elif tag == K_tag:
                    # K depends on (h,j): if range covers multiple i rows, likely touches many j.
                    if i0 == i1:
                        _add_K_slice(c, h, j0, j1)
                    else:
                        _add_K_slice(c, h, 0, SeqLen - 1)

    if tag == S_tag:
        if not any(len(d) > 0 for d in pages_by_c):
            return None, "att_scores:no_pages"
    else:
        if not any(len(s) > 0 for s in pages_by_c):
            return None, "att_scores:no_pages"

    return pages_by_c, f"pattern:att_scores:{tag}"


def _match_attention_context(launch: dict, buffers_by_tag: Dict[str, dict], page_bytes: int) -> Optional[dict]:
    """Match a common attention context kernel pattern:

        Context[h,i,d] = sum_j Probs[h,i,j] * V[h,j,d]

    Flattened 1D launch typically uses:
        tid in [0, NumHeads*SeqLen*DHead)
        h = tid / (SeqLen*DHead)
        rem = tid - h*(SeqLen*DHead)
        i = rem / DHead
        d = rem % DHead

    We detect via scalar relations + buffer sizes, not kernel names.

    Required relationships (canonical layout):
      - Context bytes == V bytes == NumHeads*SeqLen*DHead*elem_bytes
      - Probs bytes  == NumHeads*SeqLen*SeqLen*elem_bytes
      - global_size_x ~= NumHeads*SeqLen*DHead (padded to local_size_x)
    """
    gs = launch.get("global_size", []) or [0, 0, 0]
    ls = launch.get("local_size", []) or [1, 1, 1]
    try:
        Gx, Gy, Gz = int(gs[0]), int(gs[1]), int(gs[2])
        Lx = int(ls[0])
        Ly, Lz = int(ls[1]), int(ls[2])
    except Exception:
        return None
    if Gy != 1 or Gz != 1 or Ly != 1 or Lz != 1:
        return None
    if Lx <= 0:
        return None

    buf_args, scalar_args = _extract_launch_bindings(launch)
    if len(buf_args) < 3 or len(scalar_args) < 3:
        return None

    buf_tags = [x.get("tag") for x in buf_args if x.get("tag")]

    writes = set(launch.get("writes", []) or [])
    cand_out = [t for t in buf_tags if t in writes]
    if not cand_out:
        return None
    cand_out.sort(key=lambda t: _buf_bytes_pages(buffers_by_tag, t, page_bytes)[0], reverse=True)
    Ctx_tag = cand_out[0]

    reads = set(launch.get("reads", []) or [])
    read_tags = [t for t in buf_tags if t in reads and t != Ctx_tag]
    if len(read_tags) < 2:
        read_tags = [t for t in buf_tags if t != Ctx_tag]
    if len(read_tags) < 2:
        return None

    Ctx_bytes, _ = _buf_bytes_pages(buffers_by_tag, Ctx_tag, page_bytes)
    if Ctx_bytes <= 0:
        return None

    vals = [int(v) for (_n, v) in scalar_args if int(v) > 0]
    if len(vals) < 3:
        return None

    best = None  # (diff, SeqLen, DHead, NumHeads, elem, Probs_tag, V_tag, Ctx_tag)

    # Try to pick V as the read buffer that matches Context size (strong filter).
    for V_tag in read_tags:
        V_bytes, _ = _buf_bytes_pages(buffers_by_tag, V_tag, page_bytes)
        if V_bytes != Ctx_bytes or V_bytes <= 0:
            continue
        for P_tag in read_tags:
            if P_tag == V_tag:
                continue
            P_bytes, _ = _buf_bytes_pages(buffers_by_tag, P_tag, page_bytes)
            if P_bytes <= 0:
                continue
            # Probs should typically be larger than Context/V
            if P_bytes < Ctx_bytes:
                continue

            for SeqLen in vals:
                for DHead in vals:
                    for NumHeads in vals:
                        if SeqLen <= 0 or DHead <= 0 or NumHeads <= 0:
                            continue
                        out_elems = int(NumHeads) * int(SeqLen) * int(DHead)
                        padded = _ceil_div(out_elems, max(1, Lx)) * max(1, Lx)
                        if not (Gx == out_elems or Gx == padded or (Gx > out_elems and (Gx - out_elems) < max(1, Lx))):
                            continue

                        probs_elems = int(NumHeads) * int(SeqLen) * int(SeqLen)
                        if probs_elems <= 0:
                            continue
                        if P_bytes % probs_elems != 0:
                            continue
                        elem = int(P_bytes // probs_elems)
                        if not _is_plausible_elem_bytes(elem):
                            continue
                        if Ctx_bytes != out_elems * elem:
                            continue
                        # V_bytes already equals Ctx_bytes, so also matches.

                        diff = abs(Gx - padded)
                        cand = (diff, int(SeqLen), int(DHead), int(NumHeads), int(elem), str(P_tag), str(V_tag), str(Ctx_tag))
                        if best is None or cand < best:
                            best = cand

    if best is None:
        return None

    _diff, SeqLen, DHead, NumHeads, elem, P_tag, V_tag, Ctx_tag = best
    return {
        "kind": "att_context",
        "SeqLen": int(SeqLen),
        "DHead": int(DHead),
        "NumHeads": int(NumHeads),
        "Probs": str(P_tag),
        "V": str(V_tag),
        "Context": str(Ctx_tag),
        "elem_bytes": int(elem),
        "Lx": int(Lx),
    }


def _att_context_pages_by_chiplet(
    match: dict,
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Per-chiplet pages for attention context kernel.

    Modeled tags:
      - Context (output): contiguous tid range (returns *weighted* pages)
      - Probs: rows (h,i) touched by tid range => contiguous rows in Probs tensor
      - V: heads touched by tid range => contiguous head slices in V tensor

    For Context (writes), we return per-page weights (#elements) to reflect write footprint.
    """
    if match.get("kind") != "att_context":
        return None, "not att_context"

    SeqLen = int(match["SeqLen"])
    DHead = int(match["DHead"])
    NumHeads = int(match["NumHeads"])
    elem = int(match["elem_bytes"])
    Lx = int(match.get("Lx", 1))

    P_tag = str(match["Probs"])
    V_tag = str(match["V"])
    Ctx_tag = str(match["Context"])
    tag = str(tag)

    if tag not in (P_tag, V_tag, Ctx_tag):
        return None, "att_context:tag_not_modeled"

    _, np = _buf_bytes_pages(buffers_by_tag, tag, page_bytes)
    if np <= 0:
        return None, "att_context:num_pages<=0"

    total_out = int(NumHeads) * int(SeqLen) * int(DHead)
    per_head_out = int(SeqLen) * int(DHead)

    regions, err = _chiplet_wg_regions(partition, launch, num_chiplets, max_blocks=max_blocks)
    if err:
        return None, f"att_context:wg_regions:{err}"

    if tag == Ctx_tag:
        pages_by_c: List[Dict[int, float]] = [dict() for _ in range(num_chiplets)]
    else:
        pages_by_c = [set() for _ in range(num_chiplets)]

    for c in range(num_chiplets):
        for rect in regions[c]:
            (wx0, wx1), (_wy0, _wy1), (_wz0, _wz1) = rect
            t0 = int(wx0) * int(Lx)
            t1 = (int(wx1) + 1) * int(Lx) - 1
            t1 = min(t1, total_out - 1)
            if t0 > t1:
                continue

            if tag == Ctx_tag:
                wdict = _page_weights_from_elem_range(t0, t1, elem, num_pages=np, page_bytes=page_bytes)
                if not wdict:
                    for p in _pages_from_elem_range(t0, t1, elem, num_pages=np, page_bytes=page_bytes):
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + 1.0
                else:
                    for p, w in wdict.items():
                        pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + float(w)
                continue

            if tag == P_tag:
                # hi index over (h,i) pairs in [0, NumHeads*SeqLen)
                hi0 = int(t0) // int(DHead)
                hi1 = int(t1) // int(DHead)
                e0 = int(hi0) * int(SeqLen)
                e1 = (int(hi1) + 1) * int(SeqLen) - 1
                pages_by_c[c] |= _pages_from_elem_range(e0, e1, elem, num_pages=np, page_bytes=page_bytes)

            elif tag == V_tag:
                # approximate: V is accessed by head; include full head slices for touched heads
                h0 = int(t0) // int(per_head_out)
                h1 = int(t1) // int(per_head_out)
                e0 = int(h0) * int(per_head_out)
                e1 = (int(h1) + 1) * int(per_head_out) - 1
                pages_by_c[c] |= _pages_from_elem_range(e0, e1, elem, num_pages=np, page_bytes=page_bytes)

    if tag == Ctx_tag:
        if not any(len(d) > 0 for d in pages_by_c):
            return None, "att_context:no_pages"
    else:
        if not any(len(s) > 0 for s in pages_by_c):
            return None, "att_context:no_pages"

    return pages_by_c, f"pattern:att_context:{tag}"






# -----------------------------
# Phase3 (extension): conv2d2d-like 2D pattern plugin
# -----------------------------
# Motivation:
#   - Step1 sometimes fails to prove gid_y dependence for new 2D kernels (or gets "hard_unknown").
#   - For 2D row-major outputs (Out[y][x]), Xdiv vs Ydiv can have orders-of-magnitude difference in
#     page sharing (e.g., when row_bytes is near page_bytes).
#   - We provide a conservative 2D conv/stencil plugin based only on Step2 scalars (Hin/Win/Hout/Wout/K)
#     + launch geometry. This avoids falling back to a misleading 1D preferred_chiplet heuristic.
#
# Scope:
#   - only activates when we see the canonical scalar names: Hin, Win, Hout, Wout (and optionally Cout, K)
#   - only provides pages-by-chiplet for the OUTPUT buffer tag (writes), because that's the most
#     important for page placement locality.
#
# Notes:
#   - This plugin is intentionally simple and assumes row-major layout:
#       Out_idx = y * Wout + x
#     This matches the conv2d2d benchmark you added and many similar OpenCL kernels.
#   - If a kernel uses a different layout, Step1 affine analysis (when available) should dominate.


def _infer_elem_bytes_from_shape(buf_bytes: int, *, H: int, W: int, C: int = 1) -> int:
    """Infer element size (bytes) from buffer total bytes and logical shape.

    Returns a conservative guess in {1,2,4,8}, defaulting to 4.
    """
    denom = int(max(1, H) * max(1, W) * max(1, C))
    if denom <= 0:
        return 4
    try:
        eb = int(buf_bytes) // int(denom)
    except Exception:
        eb = 4
    if eb in (1, 2, 4, 8):
        return eb
    # snap to nearest typical size
    for cand in (1, 2, 4, 8):
        if abs(eb - cand) <= 1:
            return cand
    return 4


def _match_conv2d2d_like(launch: dict, buffers_by_tag: Dict[str, dict], page_bytes: int) -> Optional[dict]:
    """Try to match a 2D conv2d-like launch with canonical scalar names.

    Returns:
      {
        "Hin","Win","Hout","Wout","Cout","K",
        "out_tag","in_tag",
        "elem_bytes_out",
        "X","Y"
      }
    """
    # Need 2D launch geometry (either Gy>1 or Ly>1 with wgy>1)
    gs = launch.get("global_size", []) or [0, 0, 0]
    ls = launch.get("local_size", []) or [1, 1, 1]
    try:
        Gx, Gy, _ = int(gs[0]), int(gs[1]), int(gs[2] if len(gs) > 2 else 1)
        Lx, Ly, _ = int(ls[0]), int(ls[1]), int(ls[2] if len(ls) > 2 else 1)
    except Exception:
        return None
    if Gx <= 1 or Gy <= 1 or Lx <= 0 or Ly <= 0:
        return None

    # Scalar map from arg_bindings (preferred, preserves names)
    scalars: Dict[str, int] = {}
    for b in (launch.get("arg_bindings", []) or []):
        if b.get("kind") == "scalar" and b.get("arg_name") is not None and b.get("value") is not None:
            try:
                scalars[str(b.get("arg_name"))] = int(b.get("value"))
            except Exception:
                pass

    # Require canonical names
    need = ("Hin", "Win", "Hout", "Wout")
    if not all(k in scalars for k in need):
        return None

    Hin = int(scalars.get("Hin", 0))
    Win = int(scalars.get("Win", 0))
    Hout = int(scalars.get("Hout", 0))
    Wout = int(scalars.get("Wout", 0))
    if Hin <= 0 or Win <= 0 or Hout <= 0 or Wout <= 0:
        return None

    Cout = int(scalars.get("Cout", 1) or 1)
    if Cout <= 0:
        Cout = 1
    K = int(scalars.get("K", 0) or 0)

    # Identify output tag:
    out_tag: Optional[str] = None
    in_tag: Optional[str] = None

    writes = set(launch.get("writes", []) or [])
    reads = set(launch.get("reads", []) or [])

    for b in (launch.get("arg_bindings", []) or []):
        if b.get("kind") != "buffer" or b.get("tag") is None:
            continue
        tag = str(b.get("tag"))
        an = str(b.get("arg_name", "") or "")
        if out_tag is None and tag in buffers_by_tag:
            if an.lower() in ("out", "output", "c") or ("out" in an.lower()):
                if not writes or tag in writes:
                    out_tag = tag
        if in_tag is None and tag in buffers_by_tag:
            if an.lower() in ("in", "input", "a") or ("in" in an.lower()):
                if not reads or tag in reads:
                    in_tag = tag

    # Fallback to Step2 writes/reads if arg_name isn't reliable
    if out_tag is None and writes:
        for t in writes:
            if t in buffers_by_tag:
                out_tag = str(t)
                break
    if in_tag is None and reads:
        for t in reads:
            if t in buffers_by_tag:
                in_tag = str(t)
                break

    if out_tag is None:
        return None

    out_bytes = int((buffers_by_tag.get(out_tag) or {}).get("bytes", 0) or 0)
    elem_bytes_out = _infer_elem_bytes_from_shape(out_bytes, H=int(Hout), W=int(Wout), C=int(Cout))

    # Y dimension in the output layout is typically Cout*Hout (as in your conv2d2d kernel)
    Y = int(Cout * Hout)
    X = int(Wout)

    return {
        "Hin": int(Hin),
        "Win": int(Win),
        "Hout": int(Hout),
        "Wout": int(Wout),
        "Cout": int(Cout),
        "K": int(K),
        "out_tag": str(out_tag),
        "in_tag": str(in_tag) if in_tag is not None else None,
        "elem_bytes_out": int(elem_bytes_out),
        "X": int(X),
        "Y": int(Y),
    }


def _conv2d2d_pages_by_chiplet(
    m: dict,
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Return pages-by-chiplet (with weights) for conv2d2d OUTPUT buffer only."""
    if m is None:
        return None, None
    out_tag = str(m.get("out_tag") or "")
    if not out_tag or str(tag) != out_tag:
        return None, None

    buf = buffers_by_tag.get(out_tag) or {}
    buf_bytes = int(buf.get("bytes", 0) or 0)
    if buf_bytes <= 0:
        return None, None

    X = int(m.get("X", 0) or 0)
    Y = int(m.get("Y", 0) or 0)
    if X <= 0 or Y <= 0:
        return None, None

    eb = int(m.get("elem_bytes_out", 4) or 4)
    if eb <= 0:
        eb = 4

    # element-per-page (base page, not grouped)
    epp = int(page_bytes) // int(eb)
    if epp <= 0:
        epp = 1

    total_elems = int(X) * int(Y)
    # cap by actual buffer size (in case of padding)
    total_elems = min(total_elems, int(buf_bytes) // int(eb))

    npages = int(_ceil_div(total_elems * int(eb), int(page_bytes)))
    if npages <= 0:
        return None, None

    # Avoid enumerating too many blocks (planner guard)
    if int(_ceil_div(npages, int(group_pages))) > int(max_blocks):
        # Too large for explicit per-page weights; let affine/legacy handle it.
        return None, None

    # Determine WG geometry and chiplet stripes based on LASP semantics
    wgx, wgy, _ = workgroups_per_dim(launch)
    wgx = max(1, int(wgx))
    wgy = max(1, int(wgy))

    Lx, Ly, _ = _int3(launch.get("local_size", []) or [1, 1, 1])
    Lx = max(1, int(Lx))
    Ly = max(1, int(Ly))

    active = int(active_chiplets_for_launch(partition, launch, int(num_chiplets)))
    active = max(1, min(int(num_chiplets), int(active)))

    pages_by_c: List[Dict[int, float]] = [dict() for _ in range(int(num_chiplets))]

    # helper: add weighted overlap of [a,b) element interval into pages
    def _add_interval(c: int, a_e: int, b_e: int):
        if b_e <= a_e:
            return
        p0 = int(a_e) // int(epp)
        p1 = int((b_e - 1)) // int(epp)
        for p in range(p0, p1 + 1):
            lo = max(int(a_e), p * int(epp))
            hi = min(int(b_e), (p + 1) * int(epp))
            if hi > lo:
                pages_by_c[c][p] = pages_by_c[c].get(p, 0.0) + float(hi - lo) * float(eb)

    p = _lower(partition)

    if p.startswith("ydiv"):
        # chiplet = wg_y / divisor (capped)
        divisor = int(wgy // int(num_chiplets))
        if divisor <= 0:
            divisor = 1

        # For each chiplet, compute y element range (rows) from wg_y stripe.
        for c in range(active):
            wg_y0 = c * divisor
            wg_y1 = (c + 1) * divisor
            # last active chiplet takes remainder
            if c == active - 1:
                wg_y1 = wgy
            y0 = int(wg_y0) * int(Ly)
            y1 = int(wg_y1) * int(Ly)
            y0 = max(0, min(int(Y), y0))
            y1 = max(0, min(int(Y), y1))
            if y1 <= y0:
                continue

            # Full x range [0, X)
            start = int(y0) * int(X)
            end = int(y1) * int(X)
            _add_interval(int(c), start, end)

        return pages_by_c, "pattern:conv2d2d:out"

    if p.startswith("xdiv"):
        divisor = int(wgx // int(num_chiplets))
        if divisor <= 0:
            divisor = 1

        for c in range(active):
            wg_x0 = c * divisor
            wg_x1 = (c + 1) * divisor
            if c == active - 1:
                wg_x1 = wgx
            x0 = int(wg_x0) * int(Lx)
            x1 = int(wg_x1) * int(Lx)
            x0 = max(0, min(int(X), x0))
            x1 = max(0, min(int(X), x1))
            if x1 <= x0:
                continue

            # For Xdiv, each chiplet touches [x0,x1) columns for all rows.
            for y in range(int(Y)):
                start = int(y) * int(X) + int(x0)
                end = int(y) * int(X) + int(x1)
                _add_interval(int(c), start, end)

        return pages_by_c, "pattern:conv2d2d:out"

    # Only support Xdiv/Ydiv for now
    return None, None

def _pattern_matches_for_launch(launch: dict, buffers_by_tag: Dict[str, dict], page_bytes: int) -> Dict[str, Optional[dict]]:
    """Compute conservative pattern matches for a launch once (used as a cache)."""
    return {
        "conv2d2d": _match_conv2d2d_like(launch, buffers_by_tag, page_bytes),
        "gemm": _match_gemm_like(launch, buffers_by_tag, page_bytes),
        "att_scores": _match_attention_scores(launch, buffers_by_tag, page_bytes),
        "att_context": _match_attention_context(launch, buffers_by_tag, page_bytes),
    }


def _pattern_pages_by_chiplet_from_matches(
    matches: Dict[str, Optional[dict]],
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Apply Phase2 pattern plugins using precomputed matches (no re-matching)."""

    # conv2d2d (2D) - output footprint plugin
    c2 = matches.get("conv2d2d") if isinstance(matches, dict) else None
    if c2 is not None:
        pages, note = _conv2d2d_pages_by_chiplet(
            c2, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=int(page_bytes),
            group_pages=int(group_pages),
            max_blocks=int(max_blocks),
        )
        if pages is not None:
            return pages, note

    m = matches.get("gemm") if isinstance(matches, dict) else None
    if m is not None:
        pages, note = _gemm_pages_by_chiplet(
            m, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=int(page_bytes),
            group_pages=int(group_pages),
            max_blocks=int(max_blocks),
        )
        if pages is not None:
            return pages, note

    a = matches.get("att_scores") if isinstance(matches, dict) else None
    if a is not None:
        pages, note = _att_scores_pages_by_chiplet(
            a, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=int(page_bytes),
            group_pages=int(group_pages),
            max_blocks=int(max_blocks),
        )
        if pages is not None:
            return pages, note

    c = matches.get("att_context") if isinstance(matches, dict) else None
    if c is not None:
        pages, note = _att_context_pages_by_chiplet(
            c, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=int(page_bytes),
            group_pages=int(group_pages),
            max_blocks=int(max_blocks),
        )
        if pages is not None:
            return pages, note

    return None, None


def _pattern_pages_by_chiplet(
    launch: dict,
    partition: str,
    num_chiplets: int,
    *,
    tag: str,
    buffers_by_tag: Dict[str, dict],
    page_bytes: int,
    group_pages: int,
    max_blocks: int,
) -> Tuple[Optional[List[object]], Optional[str]]:
    """Try Phase2.2 pattern plugins (in this order):

      1) GEMM-like (covers many projection/FC kernels)
      2) attention_scores
      3) attention_context

    The plugins are intentionally conservative and rely on:
      - launch dimensionality (1D flattened)
      - scalar relations + buffer size consistency
      - Step2 reads/writes classification (when available)

    They do NOT rely on kernel names.
    """
        # conv2d2d (2D)
    c2 = _match_conv2d2d_like(launch, buffers_by_tag, page_bytes)
    if c2 is not None:
        pages, note = _conv2d2d_pages_by_chiplet(
            c2, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=int(page_bytes),
            group_pages=int(group_pages),
            max_blocks=int(max_blocks),
        )
        if pages is not None:
            return pages, note

# GEMM
    m = _match_gemm_like(launch, buffers_by_tag, page_bytes)
    if m is not None:
        pages, note = _gemm_pages_by_chiplet(
            m, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=page_bytes,
            group_pages=group_pages,
            max_blocks=max_blocks,
        )
        if pages is not None:
            return pages, note

    # attention_scores
    a = _match_attention_scores(launch, buffers_by_tag, page_bytes)
    if a is not None:
        pages, note = _att_scores_pages_by_chiplet(
            a, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=page_bytes,
            group_pages=group_pages,
            max_blocks=max_blocks,
        )
        if pages is not None:
            return pages, note

    # attention_context
    c = _match_attention_context(launch, buffers_by_tag, page_bytes)
    if c is not None:
        pages, note = _att_context_pages_by_chiplet(
            c, launch, partition, num_chiplets,
            tag=str(tag),
            buffers_by_tag=buffers_by_tag,
            page_bytes=page_bytes,
            group_pages=group_pages,
            max_blocks=max_blocks,
        )
        if pages is not None:
            return pages, note

    return None, None
def build_page_costs_legacy(
    *,
    buffers: List[dict],
    launches: List[dict],
    kernel_partition: Dict[str, str],
    launch_buffer_traffic: Dict[Tuple[int, str], TrafficWeight],
    num_chiplets: int,
    page_bytes: int,
    remote_penalty: float,
    group_pages: int,
    force_ro_stripe: bool,
    profile: Optional[ProfileData] = None,
    profile_alpha: float = 0.0,
    profile_scale: float = 1.0,
) -> Tuple[
    List[Tuple[str, int, int]],  # items (tag, start_page, block_pages)
    List[List[int]],             # costs per item -> chiplet (PER PAGE unit)
    int,                         # total pages
    Dict[str, dict],             # debug per-buffer summary
]:
    """Legacy v5.1 cost model (Phase0/Phase1 style), but with correct per-page normalization.

    We keep it as a fallback when Phase2 cannot infer affinity.
    """
    assert num_chiplets > 0
    assert group_pages > 0

    # launch derived
    lid2part = {int(l.get("id", -1)): kernel_partition.get(l.get("kernel", ""), "Xdiv") for l in launches}

    # compute global RO/RW per buffer tag
    reads_by_tag: Dict[str, int] = {}
    writes_by_tag: Dict[str, int] = {}
    active_gt1_by_tag: Dict[str, bool] = {}
    active_eq1_by_tag: Dict[str, bool] = {}

    max_active_by_tag: Dict[str, int] = {}
    max_writer_active_by_tag: Dict[str, int] = {}
    for l in launches:
        lid = int(l.get("id", -1))
        part = lid2part.get(lid, "Xdiv")
        active = active_chiplets_for_launch(part, l, num_chiplets)

        for tag in (l.get("reads", []) or []):
            reads_by_tag[tag] = reads_by_tag.get(tag, 0) + 1
            max_active_by_tag[tag] = max(int(max_active_by_tag.get(tag, 0)), int(active))
            max_writer_active_by_tag[tag] = max(int(max_writer_active_by_tag.get(tag, 0)), int(active))
            if active > 1:
                active_gt1_by_tag[tag] = True
            if active == 1:
                active_eq1_by_tag[tag] = True
        for tag in (l.get("writes", []) or []):
            writes_by_tag[tag] = writes_by_tag.get(tag, 0) + 1
            max_active_by_tag[tag] = max(int(max_active_by_tag.get(tag, 0)), int(active))
            max_writer_active_by_tag[tag] = max(int(max_writer_active_by_tag.get(tag, 0)), int(active))
            if active > 1:
                active_gt1_by_tag[tag] = True
            if active == 1:
                active_eq1_by_tag[tag] = True

    items: List[Tuple[str, int, int]] = []
    costs: List[List[int]] = []
    total_pages = 0

    scale = 1.0
    debug: Dict[str, dict] = {}

    HUGE = 10**15

    for b in buffers:
        tag = b.get("tag")
        if not tag:
            continue

        num_pages = int(b.get("pages", 0) or 0)
        if num_pages <= 0:
            bytes_ = int(b.get("bytes", 0) or 0)
            num_pages = int(math.ceil(bytes_ / float(page_bytes))) if bytes_ > 0 else 0
        if num_pages <= 0:
            continue

        total_pages += num_pages

        is_ro = writes_by_tag.get(tag, 0) == 0 and reads_by_tag.get(tag, 0) > 0
        stripe_width = int(max_active_by_tag.get(tag, num_chiplets) or num_chiplets)
        stripe_width = max(1, min(int(num_chiplets), int(stripe_width)))
        stripe_ok = bool(is_ro and force_ro_stripe and (stripe_width > 1) and active_gt1_by_tag.get(tag, False) and (not active_eq1_by_tag.get(tag, False)))

        debug[tag] = {
            "pages": num_pages,
            "is_readonly": bool(is_ro),
            "stripe_readonly": bool(stripe_ok),
            "stripe_width": int(stripe_width),
            "stripe_force": False,
            "stripe_force_reason": "",
            "stripe_force_width": 0,
            "stripe_force_evidence": [],
            "reads_launches": int(reads_by_tag.get(tag, 0)),
            "writes_launches": int(writes_by_tag.get(tag, 0)),
            "placement_model": "legacy",
        }

        for start in range(0, num_pages, group_pages):
            block_pages = min(group_pages, num_pages - start)
            items.append((tag, start, block_pages))

            per_chiplet = [0.0 for _ in range(num_chiplets)]

            # per-page normalization: treat w_total as total bytes across the buffer
            # -> bytes_per_page = w_total / num_pages
            for l in launches:
                lid = int(l.get("id", -1))
                if lid < 0:
                    continue
                if tag not in (l.get("reads", []) or []) and tag not in (l.get("writes", []) or []):
                    continue

                w = launch_buffer_traffic.get((lid, tag), TrafficWeight(req_bytes=1.0, rsp_bytes=0.0))
                w_total = w.total()
                bytes_per_page = float(w_total) / float(max(1, num_pages))

                part = lid2part.get(lid, "Xdiv")
                active = active_chiplets_for_launch(part, l, num_chiplets)
                pref = preferred_chiplet_for_page(part, num_chiplets, num_pages, start, active_chiplets=active, group_pages=group_pages)

                if pref is not None:
                    for c in range(num_chiplets):
                        if c != pref:
                            per_chiplet[c] += bytes_per_page * float(remote_penalty)

            row = [int(round(x * scale)) for x in per_chiplet]

            if stripe_ok:
                forced = (start // group_pages) % int(stripe_width)
                for c in range(num_chiplets):
                    if c != forced:
                        row[c] += HUGE

            costs.append(row)

    return items, costs, total_pages, debug


def build_page_costs_affinity(
    *,
    buffers: List[dict],
    launches: List[dict],
    kernel_partition: Dict[str, str],
    launch_buffer_traffic: Dict[Tuple[int, str], TrafficWeight],
    step1_access_records: Dict[str, Dict[int, List[dict]]],
    num_chiplets: int,
    page_bytes: int,
    remote_penalty: float,
    group_pages: int,
    force_ro_stripe: bool,
    max_blocks: int,
    bw_stripe_enable: bool,
    bw_consumer_max_active: int,
    bw_producer_min_active: int,
    bw_min_tripcount: int,
    bw_min_bytes: int,
    bw_tripcount_cap: int,
    profile: Optional[ProfileData] = None,
    profile_alpha: float = 0.0,
    profile_scale: float = 1.0,
    # Phase3-2B: plan stability regularization (optional)
    stability_base_map: Optional[Dict[str, List[int]]] = None,
    stability_lambda: float = 0.0,
    stability_meta: Optional[Dict[str, object]] = None,
    lowpar_active_max: int = 2,
    lowpar_numwg_max: int = 8,
    lowpar_bias_alpha: float = 0.0,
) -> Tuple[
    List[Tuple[str, int, int]],  # items (tag, start_page, block_pages)
    List[List[int]],             # costs per item -> chiplet (PER PAGE unit)
    int,                         # total pages
    Dict[str, dict],             # debug per-buffer summary
    Dict[str, object],           # debug global (kernels/launches)
]:
    """Phase2: WG-allocation-aware, (best-effort) affine page->chiplet affinity model.

    If we can infer which pages each chiplet touches for a given launch+buffer, we turn that into
    a per-page traffic estimate and solve the same min-cost flow.

    When inference fails (non-linear / opaque), we fall back to the legacy model.
    """
    assert num_chiplets > 0
    assert group_pages > 0

    buffers_by_tag = {b.get("tag"): b for b in buffers if b.get("tag")}

    # launch derived
    lid2part = {int(l.get("id", -1)): kernel_partition.get(l.get("kernel", ""), "Xdiv") for l in launches}

    # global RO/RW per buffer
    reads_by_tag: Dict[str, int] = {}
    writes_by_tag: Dict[str, int] = {}
    active_gt1_by_tag: Dict[str, bool] = {}
    active_eq1_by_tag: Dict[str, bool] = {}
    max_active_by_tag: Dict[str, int] = {}
    max_writer_active_by_tag: Dict[str, int] = {}

    for l in launches:
        lid = int(l.get("id", -1))
        part = lid2part.get(lid, "Xdiv")
        active = active_chiplets_for_launch(part, l, num_chiplets)

        for tag in (l.get("reads", []) or []):
            reads_by_tag[tag] = reads_by_tag.get(tag, 0) + 1
            max_active_by_tag[tag] = max(int(max_active_by_tag.get(tag, 0)), int(active))
            max_writer_active_by_tag[tag] = max(int(max_writer_active_by_tag.get(tag, 0)), int(active))
            if active > 1:
                active_gt1_by_tag[tag] = True
            if active == 1:
                active_eq1_by_tag[tag] = True
        for tag in (l.get("writes", []) or []):
            writes_by_tag[tag] = writes_by_tag.get(tag, 0) + 1
            max_active_by_tag[tag] = max(int(max_active_by_tag.get(tag, 0)), int(active))
            max_writer_active_by_tag[tag] = max(int(max_writer_active_by_tag.get(tag, 0)), int(active))
            if active > 1:
                active_gt1_by_tag[tag] = True
            if active == 1:
                active_eq1_by_tag[tag] = True

    # Prepare items (page blocks) - same as legacy
    items: List[Tuple[str, int, int]] = []
    tag_block_range: Dict[str, Tuple[int, int, int]] = {}  # tag -> (start_item_idx, num_blocks, num_pages)
    total_pages = 0

    for b in buffers:
        tag = b.get("tag")
        if not tag:
            continue
        num_pages = int(b.get("pages", 0) or 0)
        if num_pages <= 0:
            bytes_ = int(b.get("bytes", 0) or 0)
            num_pages = int(math.ceil(bytes_ / float(page_bytes))) if bytes_ > 0 else 0
        if num_pages <= 0:
            continue

        total_pages += num_pages
        start_item = len(items)
        for start in range(0, num_pages, group_pages):
            block_pages = min(group_pages, num_pages - start)
            items.append((tag, start, block_pages))
        num_blocks = int(math.ceil(num_pages / float(group_pages)))
        tag_block_range[tag] = (start_item, num_blocks, num_pages)

    # Traffic accumulator in (block,chiplet) for each tag
    # We store per-page-unit traffic, so later cost is directly comparable.
    traffic_block_chip: Dict[str, List[List[float]]] = {}
    for tag, (_si, nb, _np) in tag_block_range.items():
        traffic_block_chip[tag] = [[0.0 for _ in range(num_chiplets)] for _ in range(nb)]

    # Debug per buffer
    debug: Dict[str, dict] = {}
    for tag, (_si, nb, npages) in tag_block_range.items():
        is_ro = writes_by_tag.get(tag, 0) == 0 and reads_by_tag.get(tag, 0) > 0
        stripe_width = int(max_active_by_tag.get(tag, num_chiplets) or num_chiplets)
        stripe_width = max(1, min(int(num_chiplets), int(stripe_width)))
        stripe_ok = bool(is_ro and force_ro_stripe and (stripe_width > 1) and active_gt1_by_tag.get(tag, False) and (not active_eq1_by_tag.get(tag, False)))
        debug[tag] = {
            "pages": int(npages),
            "blocks": int(nb),
            "is_readonly": bool(is_ro),
            "stripe_readonly": bool(stripe_ok),
            "stripe_width": int(stripe_width),
            "stripe_force": False,
            "stripe_force_reason": "",
            "stripe_force_width": 0,
            "stripe_force_evidence": [],
            "reads_launches": int(reads_by_tag.get(tag, 0)),
            "writes_launches": int(writes_by_tag.get(tag, 0)),
            "placement_model": "affinity",
            "affine_bytes": 0.0,
            "pattern_bytes": 0.0,
            "fallback_bytes": 0.0,
            "total_bytes": 0.0,
            "profile_bytes": 0.0,
            "effective_total_bytes": 0.0,
            "profile_ratio_effective": 0.0,
            "notes": [],
        }

    # Global debug about launch scheduling (active chiplets subset)
    kernel_dbg: Dict[str, dict] = {}
    for l in launches:
        lid = int(l.get("id", -1))
        k = l.get("kernel", "")
        if lid < 0 or not k:
            continue
        part = lid2part.get(lid, "Xdiv")
        active = active_chiplets_for_launch(part, l, num_chiplets)
        wgx, wgy, wgz = workgroups_per_dim(l)
        if k not in kernel_dbg:
            kernel_dbg[k] = {"partition": part, "launches": []}
        kernel_dbg[k]["launches"].append({
            "id": lid,
            "global_size": l.get("global_size"),
            "local_size": l.get("local_size"),
            "wgx": int(wgx),
            "wgy": int(wgy),
            "wgz": int(wgz),
            "active_chiplets": int(active),
        })

    # Static low-parallel hotspot compaction:
    # If a tag is primarily consumed by kernels with very small WG count / active chiplet set,
    # bias placement toward the preferred chiplet chosen by the scheduling partition.
    lowpar_pref_by_tag: Dict[str, Dict[str, object]] = {}
    for l in launches:
        lid = int(l.get("id", -1))
        k = l.get("kernel", "")
        if lid < 0 or not k:
            continue
        part = lid2part.get(lid, "Xdiv")
        active = int(active_chiplets_for_launch(part, l, num_chiplets))
        num_wg = int(max(1, (l.get("num_wg_total", 0) or 0)))
        if num_wg <= 0:
            try:
                wx, wy, wz = workgroups_per_dim(l)
                num_wg = int(max(1, wx * wy * wz))
            except Exception:
                num_wg = 1
        if active > int(lowpar_active_max) and num_wg > int(lowpar_numwg_max):
            continue
        reads = list(l.get("reads", []) or [])
        if not reads:
            continue
        for tag in reads:
            tw = launch_buffer_traffic.get((lid, tag), TrafficWeight(req_bytes=1.0, rsp_bytes=0.0))
            score = float(tw.total())
            prev = lowpar_pref_by_tag.get(tag)
            if prev is None or score > float(prev.get("score", 0.0)):
                lowpar_pref_by_tag[tag] = {
                    "kernel": str(k),
                    "launch": int(lid),
                    "partition": str(part),
                    "active_chiplets": int(active),
                    "num_wg_total": int(num_wg),
                    "score": float(score),
                }
                if tag in debug:
                    if "lowpar-hot" not in debug[tag].get("notes", []):
                        debug[tag]["notes"].append("lowpar-hot")
                    debug[tag]["lowpar_pref"] = {
                        "kernel": str(k),
                        "launch": int(lid),
                        "partition": str(part),
                        "active_chiplets": int(active),
                        "num_wg_total": int(num_wg),
                        "score": float(score),
                    }

    # Bandwidth striping (Phase2 final fix):
    # If a buffer is produced with high parallelism (many active chiplets) but later consumed by a
    # very low-parallel kernel with a long streaming loop (addrec tripcount), forcing a striped
    # layout can increase effective memory parallelism and avoid degenerate "all pages block on one chiplet".
    bw_stats: Dict[str, object] = {
        "enabled": bool(bw_stripe_enable),
        "consumer_max_active": int(bw_consumer_max_active),
        "producer_min_active": int(bw_producer_min_active),
        "min_tripcount": int(bw_min_tripcount),
        "min_bytes": int(bw_min_bytes),
        "tripcount_cap": int(bw_tripcount_cap),
        "tags_forced": 0,
        "pairs": 0,
        "examples": [],
    }

    if bw_stripe_enable:
        forced_tags: Set[str] = set()
        for l in launches:
            lid = int(l.get("id", -1))
            k = l.get("kernel", "")
            if lid < 0 or not k:
                continue
            part = lid2part.get(lid, "Xdiv")
            cons_active = active_chiplets_for_launch(part, l, num_chiplets)
            if cons_active > int(bw_consumer_max_active):
                continue

            reads = list(l.get("reads", []) or [])
            if not reads:
                continue

            scalar_map = l.get("scalar_map", {}) or {}

            # Build tag -> arg_index for this launch
            tag2arg: Dict[str, int] = {}
            for b in l.get("arg_bindings", []) or []:
                if b.get("kind") == "buffer" and b.get("tag") is not None:
                    tag2arg[b["tag"]] = int(b.get("arg_index", -1))

            for tag in reads:
                if tag not in buffers_by_tag:
                    continue
                binfo = buffers_by_tag.get(tag) or {}
                if bool(binfo.get("readonly", False)):
                    continue
                bbytes = int(binfo.get("bytes", 0) or 0)
                if bbytes < int(bw_min_bytes):
                    continue

                prod_active = int(max_writer_active_by_tag.get(tag, 0) or 0)
                if prod_active < int(bw_producer_min_active):
                    continue

                arg_idx = int(tag2arg.get(tag, -1))
                if arg_idx < 0:
                    continue

                recs = step1_access_records.get(k, {}).get(arg_idx, [])
                if not recs:
                    continue

                has_stream_loop = False
                trip_max = 1

                for ma in recs:
                    if _lower(ma.get("kind", "")) != "load":
                        continue
                    addrec = ma.get("addrec")
                    if not isinstance(addrec, dict):
                        continue

                    # Heuristic: streaming loop => complete linear step with no coeffs and a small constant step.
                    step = addrec.get("step_linear", {})
                    if not isinstance(step, dict):
                        continue
                    if not bool(step.get("complete_linear", False)):
                        continue
                    coeffs = step.get("coeffs", {}) or {}
                    if coeffs:
                        continue
                    try:
                        step_const = int(step.get("constant", 0) or 0)
                    except Exception:
                        step_const = 0
                    if step_const == 0 or abs(step_const) > 256:
                        continue

                    tc = estimate_addrec_tripcount(addrec, scalar_map, cap=int(bw_tripcount_cap))
                    if tc is None:
                        continue
                    trip_max = max(int(trip_max), int(tc))
                    if int(tc) >= int(bw_min_tripcount):
                        has_stream_loop = True

                if not has_stream_loop:
                    continue

                sw = max(1, min(int(num_chiplets), int(prod_active)))
                if tag in debug:
                    debug[tag]["stripe_force"] = True
                    debug[tag]["stripe_force_reason"] = "bandwidth_lowpar"
                    debug[tag]["stripe_force_width"] = int(sw)

                    ev = {
                        "consumer_kernel": str(k),
                        "consumer_launch": int(lid),
                        "consumer_active": int(cons_active),
                        "producer_active_max": int(prod_active),
                        "trip_max": int(trip_max),
                        "bytes": int(bbytes),
                    }
                    if len(debug[tag].get("stripe_force_evidence", [])) < 8:
                        debug[tag]["stripe_force_evidence"].append(ev)

                    if len(bw_stats.get("examples", [])) < 20:
                        bw_stats["examples"].append({"tag": str(tag), **ev})

                forced_tags.add(str(tag))
                bw_stats["pairs"] = int(bw_stats.get("pairs", 0)) + 1

        bw_stats["tags_forced"] = int(len(forced_tags))


    
    # Pattern usage stats (Phase2.1)
    pattern_stats = {
        "gemm": 0,
        "att_scores": 0,
        "att_context": 0,
        "pattern_bytes": 0.0,
    }
    # Cache pattern matches per launch to avoid re-matching for every buffer tag
    pattern_match_cache: Dict[int, Dict[str, Optional[dict]]] = {}

    # Accumulate traffic per launch/buffer
    for l in launches:
        lid = int(l.get("id", -1))
        kname = l.get("kernel", "")
        if lid < 0 or not kname:
            continue
        part = lid2part.get(lid, "Xdiv")

        # Cache pattern matches once per launch
        if lid not in pattern_match_cache:
            pattern_match_cache[lid] = _pattern_matches_for_launch(l, buffers_by_tag, int(page_bytes))
        pat_matches = pattern_match_cache.get(lid, {}) or {}

        # arg_index -> tag
        arg2tag: Dict[int, str] = {}
        for b in l.get("arg_bindings", []) or []:
            if b.get("kind") == "buffer" and b.get("tag") is not None:
                try:
                    arg2tag[int(b.get("arg_index", -1))] = str(b.get("tag"))
                except Exception:
                    pass

        reads = set(l.get("reads", []) or [])
        writes = set(l.get("writes", []) or [])

        for arg_idx, tag in arg2tag.items():
            if tag not in tag_block_range:
                continue

            w = launch_buffer_traffic.get((lid, tag), TrafficWeight(req_bytes=1.0, rsp_bytes=0.0))
            total_bytes = float(w.total())
            debug[tag]["total_bytes"] += total_bytes

            _si, nb, npages = tag_block_range[tag]

            # 1) Step1 affine-derived pages (unweighted per-page touches)
            access_list = (step1_access_records.get(kname, {}) or {}).get(int(arg_idx), []) or []
            analyzed_any = False
            counts_aff = [[0.0 for _ in range(num_chiplets)] for _ in range(nb)]
            sum_aff = 0.0
            for ma in access_list:
                pages_by_c, _err = _access_pages_by_chiplet(
                    ma, l, part, num_chiplets,
                    num_pages=int(npages),
                    page_bytes=int(page_bytes),
                    group_pages=int(group_pages),
                    max_blocks=int(max_blocks),
                )
                if pages_by_c is None:
                    continue
                analyzed_any = True
                for c in range(num_chiplets):
                    for p in pages_by_c[c]:
                        bid = int(p) // int(group_pages)
                        if 0 <= bid < nb:
                            counts_aff[bid][c] += 1.0
                            sum_aff += 1.0

            # 2) Phase2 pattern plugins (can override affine for outputs)
            pat_pages_by_c, pat_note = _pattern_pages_by_chiplet_from_matches(
                pat_matches, l, part, num_chiplets,
                tag=str(tag),
                buffers_by_tag=buffers_by_tag,
                page_bytes=int(page_bytes),
                group_pages=int(group_pages),
                max_blocks=int(max_blocks),
            )
            counts_pat = [[0.0 for _ in range(num_chiplets)] for _ in range(nb)]
            sum_pat = 0.0
            if pat_pages_by_c is not None:
                for c in range(num_chiplets):
                    obj = pat_pages_by_c[c]
                    if isinstance(obj, dict):
                        for p, wgt in obj.items():
                            bid = int(p) // int(group_pages)
                            if 0 <= bid < nb:
                                counts_pat[bid][c] += float(wgt)
                                sum_pat += float(wgt)
                    else:
                        for p in obj:
                            bid = int(p) // int(group_pages)
                            if 0 <= bid < nb:
                                counts_pat[bid][c] += 1.0
                                sum_pat += 1.0

            # Selection policy (Phase2.3):
            #   - Prefer pattern for output buffers (writes) when available (gives better write-footprint weights).
            #   - Otherwise prefer affine when available; fall back to pattern; then legacy heuristic.
            use_pattern = False
            if pat_pages_by_c is not None and sum_pat > 0:
                if tag in writes:
                    use_pattern = True
                elif not (analyzed_any and sum_aff > 0):
                    use_pattern = True

            if use_pattern:
                debug[tag]["pattern_bytes"] += total_bytes
                pattern_stats["pattern_bytes"] += total_bytes
                if pat_note:
                    if pat_note.startswith("pattern:gemm"):
                        pattern_stats["gemm"] += 1
                    elif pat_note.startswith("pattern:att_scores"):
                        pattern_stats["att_scores"] += 1
                    elif pat_note.startswith("pattern:att_context"):
                        pattern_stats["att_context"] += 1
                    if pat_note not in debug[tag]["notes"]:
                        debug[tag]["notes"].append(pat_note)

                scale = total_bytes / float(sum_pat)
                for bid in range(nb):
                    for c in range(num_chiplets):
                        if counts_pat[bid][c] > 0:
                            traffic_block_chip[tag][bid][c] += counts_pat[bid][c] * scale

            elif analyzed_any and sum_aff > 0:
                debug[tag]["affine_bytes"] += total_bytes
                scale = total_bytes / float(sum_aff)
                for bid in range(nb):
                    for c in range(num_chiplets):
                        if counts_aff[bid][c] > 0:
                            traffic_block_chip[tag][bid][c] += counts_aff[bid][c] * scale

            else:
                # Fallback: use the legacy "preferred chiplet" heuristic (WG-aware active subset).
                debug[tag]["fallback_bytes"] += total_bytes
                for bid in range(nb):
                    start_page = bid * int(group_pages)
                    block_pages = min(int(group_pages), int(npages) - start_page)
                    bytes_block = total_bytes * (float(block_pages) / float(max(1, npages)))
                    pref = preferred_chiplet_for_page(
                        part, num_chiplets, int(npages), start_page,
                        active_chiplets=active_chiplets_for_launch(part, l, num_chiplets),
                        group_pages=int(group_pages),
                    )
                    if pref is None:
                        pref = 0
                    traffic_block_chip[tag][bid][int(pref)] += bytes_block


    # -----------------------------
    # Phase3-1: Profile-guided blending (optional)
    # -----------------------------
    profile_dbg: Dict[str, object] = {
        "enabled": False,
        "alpha": float(profile_alpha),
        "scale": float(profile_scale),
        "profile_page_bytes": int(getattr(profile, "page_bytes", 0) or 0) if profile else 0,
        "profile_group_pages": int(getattr(profile, "group_pages", 0) or 0) if profile else 0,
        "planner_page_bytes": int(page_bytes),
        "planner_group_pages": int(group_pages),
        "covered_tags": 0,
        "covered_pages": 0,
        "clipped_pages": 0,
        "total_profile_bytes": 0.0,
        "alignment": {},
        "profile_meta": {},
    }


    stability_dbg: Dict[str, object] = {
        "enabled": bool(stability_base_map is not None and float(stability_lambda) > 0.0),
        "lambda": float(stability_lambda),
        "base_plan": str((stability_meta or {}).get("path", "")),
        "base_tags": int((stability_meta or {}).get("tags", 0) or (len(stability_base_map) if stability_base_map else 0)),
        "applied_blocks": 0,
        "applied_pages": 0,
    }

    profile_block_chip: Dict[str, List[List[float]]] = {}

    if profile is not None and float(profile_alpha) > 0.0:
        if int(profile.chiplets) != int(num_chiplets):
            raise RuntimeError(f"profile chiplets={profile.chiplets} does not match planner chiplets={num_chiplets}")

        # record profile meta + dispatch alignment (kernel_name + sizes, in order)
        try:
            profile_dbg["profile_meta"] = dict(getattr(profile, "stats", {}) or {})
        except Exception:
            profile_dbg["profile_meta"] = {}
        try:
            profile_dbg["alignment"] = align_profile_kernels_to_launches(getattr(profile, "kernels", []) or [], launches)
        except Exception as _e:
            profile_dbg["alignment"] = {"error": str(_e)}

        a = float(profile_alpha)
        a = max(0.0, min(1.0, a))
        profile_dbg["alpha"] = float(a)
        profile_dbg["enabled"] = True

        # Build per-tag per-block profile traffic
        profile_block_chip: Dict[str, List[List[float]]] = {}
        for tag, (_si, nb, _np) in tag_block_range.items():
            profile_block_chip[tag] = [[0.0 for _ in range(num_chiplets)] for _ in range(nb)]

        covered_tags: Set[str] = set()
        covered_pages = 0
        clipped_pages = 0
        total_pbytes = 0.0

        for tag, (_si, nb, npages) in tag_block_range.items():
            pmap = (getattr(profile, "tag_page_bytes", {}) or {}).get(tag)
            if not pmap:
                continue
            covered_tags.add(tag)
            for pidx, vec in pmap.items():
                try:
                    pidx_i = int(pidx)
                except Exception:
                    continue
                if pidx_i < 0 or pidx_i >= int(npages):
                    clipped_pages += 1
                    continue
                bid = int(pidx_i) // int(group_pages)
                if bid < 0 or bid >= int(nb):
                    clipped_pages += 1
                    continue
                if not isinstance(vec, list) or len(vec) != int(num_chiplets):
                    continue
                for c in range(num_chiplets):
                    v = float(vec[c]) * float(profile_scale)
                    if v != 0.0:
                        profile_block_chip[tag][bid][c] += v
                        total_pbytes += v
                covered_pages += 1

        profile_dbg["covered_tags"] = int(len(covered_tags))
        profile_dbg["covered_pages"] = int(covered_pages)
        profile_dbg["clipped_pages"] = int(clipped_pages)
        profile_dbg["total_profile_bytes"] = float(total_pbytes)

        # Blend profile into traffic_block_chip (conservative):
        #   - If a block has non-zero profile bytes, blend with alpha.
        #   - If a block has zero profile bytes, KEEP static (do not zero it even when alpha=1),
        #     because the profile may be partial (warmup) and miss rarely-touched pages.
        for tag, (_si, nb, _np) in tag_block_range.items():
            pb = profile_block_chip.get(tag)
            if pb is None:
                continue

            tag_pbytes = float(sum(sum(row) for row in pb))
            debug[tag]["profile_bytes"] = float(tag_pbytes)
            if tag_pbytes > 0.0 and "profile-guided" not in debug[tag].get("notes", []):
                debug[tag]["notes"].append("profile-guided")

            eff_total = 0.0
            prof_used = 0.0

            for bid in range(int(nb)):
                sb_total = float(sum(traffic_block_chip[tag][bid]))
                pb_total = float(sum(pb[bid]))
                if pb_total > 0.0:
                    eff_total += (1.0 - a) * sb_total + a * pb_total
                    prof_used += a * pb_total
                    for c in range(num_chiplets):
                        traffic_block_chip[tag][bid][c] = (1.0 - a) * traffic_block_chip[tag][bid][c] + a * pb[bid][c]
                else:
                    eff_total += sb_total

            debug[tag]["effective_total_bytes"] = float(eff_total)
            debug[tag]["profile_ratio_effective"] = (prof_used / eff_total) if eff_total > 0.0 else 0.0

    else:
        # No profile: keep effective_total_bytes consistent.
        for tag, info in debug.items():
            info["effective_total_bytes"] = float(info.get("total_bytes", 0.0) or 0.0)
            info["profile_ratio_effective"] = 0.0


# Convert traffic to per-page costs
    costs: List[List[int]] = []
    HUGE = 10**15

    stb_enabled = (stability_base_map is not None) and (float(stability_lambda) > 0.0)
    stb_blocks = 0
    stb_pages = 0

    for (tag, start, block_pages) in items:
        _si, nb, npages = tag_block_range.get(tag, (0, 0, 0))
        bid = int(start) // int(group_pages)
        if tag not in traffic_block_chip or bid < 0 or bid >= nb:
            costs.append([0 for _ in range(num_chiplets)])
            continue

        tvec = traffic_block_chip[tag][bid]
        total_t = float(sum(tvec))
        bp = max(1, int(block_pages))

        # per-page traffic in this block (assume uniform within block)
        total_per_page = total_t / float(bp) if bp > 0 else total_t

        row_f = []
        for c in range(num_chiplets):
            local_per_page = float(tvec[c]) / float(bp) if bp > 0 else float(tvec[c])
            remote_per_page = max(0.0, total_per_page - local_per_page)
            row_f.append(remote_per_page * float(remote_penalty))

        row = [int(round(x)) for x in row_f]

        # Static low-parallel bias: keep hot low-WG / low-active buffers compact when possible.
        lp = lowpar_pref_by_tag.get(tag)
        if lp is not None and float(lowpar_bias_alpha) > 0.0:
            pref_chip = preferred_chiplet_for_page(
                str(lp.get("partition", "Xdiv")),
                int(num_chiplets),
                int(npages),
                int(start),
                active_chiplets=int(lp.get("active_chiplets", num_chiplets) or num_chiplets),
                group_pages=int(group_pages),
            )
            if pref_chip is not None and 0 <= int(pref_chip) < int(num_chiplets):
                lp_pen = int(round(float(lowpar_bias_alpha) * float(total_per_page))) if total_per_page > 0.0 else 0
                if lp_pen > 0:
                    for c in range(num_chiplets):
                        if c != int(pref_chip):
                            row[c] += lp_pen

        # Phase3-2B: stability regularization (discourage plan jitter vs a base plan)
        if stb_enabled:
            base_pages = (stability_base_map or {}).get(tag)
            if isinstance(base_pages, list) and base_pages:
                s0 = int(start)
                e0 = min(len(base_pages), s0 + bp)
                if s0 < e0:
                    # Mode chiplet within this block range (handles group_pages>1 conservatively)
                    counts = [0 for _ in range(num_chiplets)]
                    for v in base_pages[s0:e0]:
                        try:
                            cc = int(v)
                        except Exception:
                            cc = 0
                        if 0 <= cc < int(num_chiplets):
                            counts[cc] += 1
                    base_c = int(max(range(num_chiplets), key=lambda x: counts[x]))
                    mode_ratio = float(counts[base_c]) / float(e0 - s0) if (e0 - s0) > 0 else 1.0

                    # Confidence of profile recommendation for this (tag,block)
                    conf = 0.0
                    pbv = None
                    try:
                        pbv = profile_block_chip.get(tag)
                    except Exception:
                        pbv = None
                    if pbv is not None and 0 <= int(bid) < len(pbv):
                        vv = pbv[int(bid)]
                        totp = float(sum(vv))
                        if totp > 0.0:
                            active = sum(1 for x in vv if float(x) != 0.0)
                            peak = (max(vv) / totp) if totp > 0.0 else 0.0
                            if num_chiplets > 1:
                                conc = 1.0 - float(active - 1) / float(num_chiplets - 1)
                            else:
                                conc = 1.0
                            conf = max(float(peak), float(conc))

                    pen = float(stability_lambda) * float(total_per_page) * float(mode_ratio) * (1.0 - float(conf))
                    p_int = int(round(pen)) if pen > 0.0 else 0
                    if p_int > 0:
                        for c in range(num_chiplets):
                            if c != base_c:
                                row[c] += p_int
                        stb_blocks += 1
                        stb_pages += int(bp)

        # Optional striping safeguards:
        #   - read-only stripe (Phase2.0): keep RO pages striped across a safe width
        #   - bandwidth stripe (Phase2 final): for special producer-parallel / consumer-lowpar streaming buffers
        stripe_ro = bool(debug.get(tag, {}).get("stripe_readonly", False))
        stripe_force = bool(debug.get(tag, {}).get("stripe_force", False))
        if stripe_ro or stripe_force:
            sw = int(debug.get(tag, {}).get("stripe_width", num_chiplets) or num_chiplets)
            if stripe_force:
                sw = int(debug.get(tag, {}).get("stripe_force_width", sw) or sw)
            sw = max(1, min(int(num_chiplets), int(sw)))
            forced = (start // group_pages) % sw
            for c in range(num_chiplets):
                if c != forced:
                    row[c] += HUGE


        costs.append(row)

    # Finalize per-buffer debug ratios
    for tag, info in debug.items():
        tb = float(info.get("total_bytes", 0.0) or 0.0)
        ab = float(info.get("affine_bytes", 0.0) or 0.0)
        pb = float(info.get("pattern_bytes", 0.0) or 0.0)
        fb = float(info.get("fallback_bytes", 0.0) or 0.0)
        if tb > 0:
            info["affine_ratio"] = ab / tb
            info["pattern_ratio"] = pb / tb
            info["fallback_ratio"] = fb / tb
        else:
            info["affine_ratio"] = 0.0
            info["pattern_ratio"] = 0.0
            info["fallback_ratio"] = 1.0

    
    # Finalize stability debug (Phase3-2B)
    try:
        stability_dbg["applied_blocks"] = int(stb_blocks)
        stability_dbg["applied_pages"] = int(stb_pages)
    except Exception:
        pass

    global_dbg = {
        "kernels": kernel_dbg,
        "max_blocks": int(max_blocks),
        "patterns": pattern_stats,
        "bandwidth_stripe": bw_stats,
        "profile": profile_dbg,
        "stability": stability_dbg,
    }

    return items, costs, total_pages, debug, global_dbg


# Backwards-compatible alias
def build_page_costs(*args, **kwargs):
    return build_page_costs_legacy(*args, **kwargs)


def solve_assignment_min_cost_flow(
    items: List[Tuple[str, int, int]],
    costs: List[List[int]],
    num_chiplets: int,
    chiplet_cap_pages: Optional[int],
    chiplet_overflow_pages: int = 0,
    chiplet_overflow_penalty: int = 0,
) -> Tuple[List[List[int]], int]:
    assert len(items) == len(costs)
    n_items = len(items)
    total_pages = sum(bp for _tag, _start, bp in items)
    if chiplet_cap_pages is None or chiplet_cap_pages <= 0:
        chiplet_cap_pages = total_pages

    S = 0
    item0 = 1
    chip0 = item0 + n_items
    T = chip0 + num_chiplets

    mcf = MinCostFlow(T + 1)

    item_to_chip_edge_idx: List[List[Tuple[int, int]]] = [
        [(-1, -1) for _ in range(num_chiplets)] for _ in range(n_items)
    ]

    for i, (_tag, _start, bp) in enumerate(items):
        mcf.add_edge(S, item0 + i, cap=bp, cost=0)

    for i in range(n_items):
        for c in range(num_chiplets):
            fr = item0 + i
            edge_pos = len(mcf.g[fr])
            mcf.add_edge(fr, chip0 + c, cap=items[i][2], cost=costs[i][c])
            item_to_chip_edge_idx[i][c] = (fr, edge_pos)

    for c in range(num_chiplets):
        mcf.add_edge(chip0 + c, T, cap=chiplet_cap_pages, cost=0)
        if int(chiplet_overflow_pages) > 0 and int(chiplet_overflow_penalty) > 0:
            mcf.add_edge(chip0 + c, T, cap=int(chiplet_overflow_pages), cost=int(chiplet_overflow_penalty))

    sent, min_cost = mcf.min_cost_flow(S, T, total_pages)
    if sent != total_pages:
        raise RuntimeError(f"min-cost flow could not route all pages: sent={sent} total={total_pages}")

    flow = [[0 for _ in range(num_chiplets)] for _ in range(n_items)]
    for i in range(n_items):
        for c in range(num_chiplets):
            fr, pos = item_to_chip_edge_idx[i][c]
            e = mcf.g[fr][pos]
            orig_cap = items[i][2]
            used = orig_cap - e.cap
            flow[i][c] = used

    return flow, min_cost



def infer_clap_application_group_pages(
    global_json_path: str,
    buffers: List[dict],
    page_bytes: int,
    requested: int = 0,
) -> int:
    if int(requested or 0) > 0:
        return int(requested)
    gp = 0
    name = str(global_json_path or '').lower()
    if 'bert' in name:
        gp = 1
    elif 'gpt' in name:
        gp = 2
    elif any(k in name for k in ('lenet', 'mamba', 'performer', 't5')):
        gp = 4
    if gp > 0:
        return gp
    # Fallback heuristic: use weight-like buffers to pick a coarse application-wide grouping.
    pages = []
    for b in (buffers or []):
        tag = str((b or {}).get('tag', '') or '')
        if not tag:
            continue
        if not ('.W' in tag or tag.startswith('W') or '.Gamma' in tag or '.Beta' in tag):
            continue
        size = int((b or {}).get('bytes', 0) or 0)
        npages = int(math.ceil(float(size) / float(max(1, page_bytes)))) if size > 0 else 0
        if npages > 0:
            pages.append(npages)
    if not pages:
        return 1
    pages.sort()
    med = pages[len(pages)//2]
    if med >= 64:
        return 4
    if med >= 16:
        return 2
    return 1


def _apply_clap_explicit_postpass(
    current_pages: Dict[str, List[int]],
    usage: Dict[str, Dict[int, List[int]]],
    *,
    num_chiplets: int,
    group_pages: int,
    chiplet_cap_pages: Optional[int] = None,
    chiplet_overflow_penalty: int = 0,
    stability_penalty: int = 1,
) -> Tuple[Dict[str, List[int]], Dict[str, object], Dict[str, dict]]:
    """CLAP-style explicit placement.

    The dispatch schedule is assumed fixed. We aggregate true profiled bytes for each
    (tag, grouped-page-region), choose a dominant chiplet for that region, and translate
    the grouped placement back into explicit per-page chiplet assignments.
    """
    pages_map = {str(k): list(v) for k, v in (current_pages or {}).items() if isinstance(v, list)}
    if not pages_map or not usage:
        return pages_map, {'enabled': False, 'group_pages': int(group_pages), 'used_regions': 0}, {}

    gp = max(1, int(group_pages or 1))
    cap = int(chiplet_cap_pages or 0) or None
    used = [0 for _ in range(int(num_chiplets))]
    for _tag, pages in pages_map.items():
        for home in (pages or []):
            try:
                h = int(home)
            except Exception:
                continue
            if 0 <= h < int(num_chiplets):
                used[h] += 1

    def _overflow_sum(counts: Dict[int, int]) -> int:
        if cap is None or cap <= 0:
            return 0
        total = 0
        for ch, vv in counts.items():
            total += max(0, int(vv) - int(cap))
        return int(total)

    blocks: List[dict] = []
    for tag, pages in pages_map.items():
        per_page = usage.get(tag, {}) or {}
        n = len(pages)
        for start in range(0, n, gp):
            end = min(n, start + gp)
            vec = [0 for _ in range(int(num_chiplets))]
            total = 0
            for p in range(start, end):
                vv = [int(x or 0) for x in (per_page.get(int(p), []) or [])]
                if not vv:
                    continue
                total += int(sum(vv))
                for c in range(min(int(num_chiplets), len(vv))):
                    vec[c] += int(vv[c])
            if total <= 0:
                continue
            cur_pages = [int(pages[p]) for p in range(start, end)]
            mode = 0
            if cur_pages:
                mode = max(sorted(set(cur_pages)), key=lambda c: cur_pages.count(c))
            dom = max(range(int(num_chiplets)), key=lambda c: vec[c])
            purity = (float(vec[dom]) / float(total)) if total > 0 else 0.0
            blocks.append({
                'tag': str(tag), 'start': int(start), 'num_pages': int(end - start),
                'bytes_by_chiplet': vec, 'total_bytes': int(total), 'mode_chiplet': int(mode),
                'dominant_chiplet': int(dom), 'purity': float(purity), 'old_pages': cur_pages,
            })
    blocks.sort(key=lambda b: (-int(b['total_bytes']), float(b['purity'])))

    before_eval = _compute_remote_eval_from_usage(usage, pages_map, num_chiplets=int(num_chiplets))
    per_tag_dbg: Dict[str, dict] = {}
    changed_pages = 0
    for blk in blocks:
        tag = str(blk['tag'])
        start = int(blk['start'])
        nump = int(blk['num_pages'])
        old_pages = [int(x) for x in (blk.get('old_pages', []) or [])]
        old_counts: Dict[int, int] = {}
        for h in old_pages:
            old_counts[h] = old_counts.get(h, 0) + 1
        best = None
        for c in range(int(num_chiplets)):
            rem = int(blk['total_bytes']) - int(blk['bytes_by_chiplet'][c])
            # occupancy-sensitive term (CLAP-style grouping still needs capacity awareness)
            occ_pen = 0
            if cap is not None:
                affected = {int(c)} | {int(x) for x in old_counts.keys()}
                before_occ = {ch: int(used[ch]) for ch in affected}
                after_occ = dict(before_occ)
                for ch, cnt in old_counts.items():
                    after_occ[int(ch)] = after_occ.get(int(ch), 0) - int(cnt)
                after_occ[int(c)] = after_occ.get(int(c), 0) + int(nump)
                delta_over = _overflow_sum(after_occ) - _overflow_sum(before_occ)
                occ_pen = max(0, int(delta_over)) * int(chiplet_overflow_penalty)
            stab = 0 if int(c) == int(blk['mode_chiplet']) else int(stability_penalty)
            score = int(rem) + int(occ_pen) + int(stab)
            cand = (score, -int(blk['bytes_by_chiplet'][c]), abs(int(c) - int(blk['dominant_chiplet'])), int(c))
            if best is None or cand < best[0]:
                best = (cand, int(c))
        chosen = int(best[1]) if best is not None else int(blk['dominant_chiplet'])
        for ch, cnt in old_counts.items():
            used[int(ch)] -= int(cnt)
        used[int(chosen)] += int(nump)
        pages = pages_map.get(tag)
        if not isinstance(pages, list):
            continue
        delta = 0
        for p in range(start, min(len(pages), start + nump)):
            if int(pages[p]) != int(chosen):
                delta += 1
                pages[p] = int(chosen)
        if delta > 0:
            changed_pages += int(delta)
        tag_dbg = per_tag_dbg.setdefault(tag, {
            'group_pages': int(gp), 'regions': 0, 'pages_changed': 0,
            'total_bytes': 0, 'dominant_match_regions': 0,
        })
        tag_dbg['regions'] += 1
        tag_dbg['pages_changed'] += int(delta)
        tag_dbg['total_bytes'] += int(blk['total_bytes'])
        if int(chosen) == int(blk['dominant_chiplet']):
            tag_dbg['dominant_match_regions'] += 1
    after_eval = _compute_remote_eval_from_usage(usage, pages_map, num_chiplets=int(num_chiplets))
    global_dbg = {
        'enabled': True,
        'group_pages': int(gp),
        'used_regions': int(len(blocks)),
        'changed_pages': int(changed_pages),
        'remote_before': int(before_eval.get('remote_bytes', 0) or 0),
        'remote_after': int(after_eval.get('remote_bytes', 0) or 0),
        'remote_ratio_before': float(before_eval.get('remote_ratio', 0.0) or 0.0),
        'remote_ratio_after': float(after_eval.get('remote_ratio', 0.0) or 0.0),
    }
    return pages_map, global_dbg, per_tag_dbg

def emit_plan_json(
    out_path: str,
    tool: str,
    version: int,
    objective: str,
    chiplets: int,
    page_bytes: int,
    hardware_extra: dict,
    kernel_partition: Dict[str, str],
    kernel_policy: Dict[str, str],
    buffers: List[dict],
    items: List[Tuple[str, int, int]],
    item_flow: List[List[int]],
    debug_info: Dict[str, dict],
    launches: Optional[List[dict]] = None,
    launch_hints: Optional[List[dict]] = None,
    launch_sequence_length: int = 0,
    plan_debug: Optional[dict] = None,
    stability_base_map: Optional[Dict[str, List[int]]] = None,
    stability_meta: Optional[Dict[str, object]] = None,
    graph_obj: Optional[dict] = None,
    profile: Optional[ProfileData] = None,
    clap_group_pages: int = 1,
    use_clap_explicit_postpass: bool = False,
) -> None:
    # build pages[] per buffer
    tag2pages: Dict[str, List[int]] = {}
    for b in buffers:
        tag = b.get("tag")
        if not tag:
            continue
        n = int(b.get("pages", 0) or 0)
        if n <= 0:
            bytes_ = int(b.get("bytes", 0) or 0)
            n = int(math.ceil(bytes_ / float(page_bytes))) if bytes_ > 0 else 0
        if n > 0:
            tag2pages[tag] = [0 for _ in range(n)]

    for i, (tag, start, _bp) in enumerate(items):
        if tag not in tag2pages:
            continue
        page_list = tag2pages[tag]
        cursor = start
        for c, cnt in enumerate(item_flow[i]):
            for _ in range(cnt):
                if cursor < len(page_list):
                    page_list[cursor] = c
                cursor += 1

    split_input_fix_changed = {}
    split_input_fix_debug = {}
    fanout_fix_changed = {}
    fanout_fix_debug = {}
    exact_remote_fix_changed: Dict[str, int] = {}
    exact_remote_fix_debug: Dict[str, dict] = {}
    exact_remote_global_debug: Dict[str, object] = {}
    if launch_hints:
        split_input_fix_changed, split_input_fix_debug = protect_split_readonly_input_placements(
            tag2pages,
            launches,
            launch_hints,
            num_chiplets=int(chiplets),
        )
        fanout_fix_changed, fanout_fix_debug = protect_fanout_root_placements(
            tag2pages,
            launch_hints,
            (graph_obj or {}),
            num_chiplets=int(chiplets),
        )
        if profile is not None and bool(use_clap_explicit_postpass):
            usage = _collect_true_page_chiplet_bytes(
                launches or [],
                launch_hints or [],
                profile,
                tag2pages,
                num_chiplets=int(chiplets),
            )
            if usage:
                cap_pages = int((hardware_extra or {}).get('chiplet_cap_pages', 0) or 0)
                if cap_pages <= 0:
                    cap_pages = None
                over_penalty = int((hardware_extra or {}).get('chiplet_overflow_penalty', 0) or 0)
                new_pages, exact_dbg, tag_fix_dbg = _apply_clap_explicit_postpass(
                    tag2pages,
                    usage,
                    num_chiplets=int(chiplets),
                    group_pages=max(1, int(clap_group_pages)),
                    chiplet_cap_pages=cap_pages,
                    chiplet_overflow_penalty=int(over_penalty),
                    stability_penalty=1,
                )
                exact_remote_global_debug = dict(exact_dbg)
                for tag, pages in new_pages.items():
                    old = tag2pages.get(tag)
                    if not isinstance(old, list) or len(old) != len(pages):
                        continue
                    delta = sum(1 for a, b in zip(old, pages) if int(a) != int(b))
                    if delta > 0:
                        tag2pages[tag] = list(pages)
                        exact_remote_fix_changed[str(tag)] = int(delta)
                        tag_dbg = dict((tag_fix_dbg or {}).get(str(tag), {}) or {})
                        tag_dbg.update({
                            'reason': 'clap-explicit-postpass',
                            'pages_changed': int(delta),
                        })
                        exact_remote_fix_debug[str(tag)] = tag_dbg

    kernel_scheduling = []
    for k in sorted(kernel_partition.keys()):
        kernel_scheduling.append({
            "kernel": k,
            "policy": canonical_lasp_policy(kernel_policy.get(k, "lasp-blockcu")),
            "partition": kernel_partition[k],
        })

    buffer_placement = {}
    for tag, pages in tag2pages.items():
        buffer_placement[tag] = {
            "policy": "explicit",
            "pages": pages,
        }
        # attach debug hints (safe: simulator should ignore unknown keys)
        if tag in debug_info:
            buffer_placement[tag]["debug"] = debug_info[tag]
        if tag in split_input_fix_debug:
            if not isinstance(buffer_placement[tag].get("debug"), dict):
                buffer_placement[tag]["debug"] = {}
            buffer_placement[tag]["debug"]["split_input_fix"] = split_input_fix_debug[tag]
        if tag in fanout_fix_debug:
            if not isinstance(buffer_placement[tag].get("debug"), dict):
                buffer_placement[tag]["debug"] = {}
            buffer_placement[tag]["debug"]["fanout_root_fix"] = fanout_fix_debug[tag]
        if tag in exact_remote_fix_debug:
            if not isinstance(buffer_placement[tag].get("debug"), dict):
                buffer_placement[tag]["debug"] = {}
            buffer_placement[tag]["debug"]["true_remote_fix"] = exact_remote_fix_debug[tag]


    # Phase3-2B: record stability outcome (how many pages changed vs base plan)
    if stability_base_map is not None and isinstance(stability_base_map, dict) and stability_base_map:
        pages_compared = 0
        pages_changed = 0
        tags_changed = 0
        top_changed: List[Tuple[int, str]] = []
        for tag, pages in tag2pages.items():
            base_pages = stability_base_map.get(tag)
            if not isinstance(base_pages, list) or not base_pages:
                continue
            n = min(len(pages), len(base_pages))
            if n <= 0:
                continue
            pages_compared += n
            diff = 0
            for i in range(n):
                if int(pages[i]) != int(base_pages[i]):
                    diff += 1
            if diff > 0:
                tags_changed += 1
                top_changed.append((diff, str(tag)))
            pages_changed += diff

        top_changed.sort(reverse=True)
        top_changed_tags = [{"tag": t, "pages_changed": int(d)} for (d, t) in top_changed[:10]]

        stab_result = {
            "base_plan": str((stability_meta or {}).get("path", "")),
            "base_tags": int((stability_meta or {}).get("tags", 0) or len(stability_base_map)),
            "pages_compared": int(pages_compared),
            "pages_changed": int(pages_changed),
            "changed_ratio": (float(pages_changed) / float(pages_compared)) if pages_compared > 0 else 0.0,
            "tags_changed": int(tags_changed),
            "top_changed_tags": top_changed_tags,
        }

        if plan_debug is None:
            plan_debug = {}
        # Prefer to attach under debug.affinity.stability (when placement_model=affinity)
        if isinstance(plan_debug, dict) and isinstance(plan_debug.get("affinity"), dict):
            st = plan_debug["affinity"].get("stability")
            if not isinstance(st, dict):
                st = {}
            st.update(stab_result)
            plan_debug["affinity"]["stability"] = st
        elif isinstance(plan_debug, dict):
            st = plan_debug.get("stability")
            if not isinstance(st, dict):
                st = {}
            st.update(stab_result)
            plan_debug["stability"] = st

    hw = {
        "chiplets": int(chiplets),
        "page_bytes": int(page_bytes),
    }
    for k, v in (hardware_extra or {}).items():
        hw[k] = v

    plan = {
        "tool": tool,
        "version": int(version),
        "objective": objective,
        "hardware": hw,
        "default_sched_partition": "Xdiv",
        "kernel_scheduling": kernel_scheduling,
        "buffer_placement": buffer_placement,
    }
    if launch_sequence_length and int(launch_sequence_length) > 0 and launch_hints:
        plan["launch_sequence_length"] = int(launch_sequence_length)
        dispatch_sequence_length = max(int(launch_sequence_length), max((int(h.get("id", -1)) for h in (launch_hints or [])), default=-1) + 1)
        plan["dispatch_sequence_length"] = int(dispatch_sequence_length)
        plan["launch_hints"] = list(sorted(launch_hints, key=lambda h: int(h.get("id", 0))))
    if exact_remote_global_debug:
        if plan_debug is None:
            plan_debug = {}
        if isinstance(plan_debug, dict):
            pp = plan_debug.get('placement_postpass')
            if not isinstance(pp, dict):
                pp = {}
            pp['clap_explicit_postpass'] = exact_remote_global_debug
            pp['true_remote_solver'] = exact_remote_global_debug
            plan_debug['placement_postpass'] = pp
    if plan_debug is not None:
        plan["debug"] = plan_debug


    with open(out_path, "w") as f:
        json.dump(plan, f, indent=2, sort_keys=False)




def normalize_lasp_partition_for_sim(
    partition: str,
    kernel_name: str,
    launches: List[dict],
    *,
    default_partition: str = "Xdiv",
) -> str:
    """Normalize/alias partition strings so the plan is accepted by mgpusim's LASP validator.

    In mgpusim's hybrid_alg.go, LASP only accepts:
      - Xdiv / Ydiv
      - XblkN / YblkN

    Any other string (e.g. "RR") is treated as invalid and silently replaced by the
    default partition (usually Xdiv). That makes experiments confusing.

    This helper lets the planner (and you) use convenient aliases like "RR" and have
    them map to a valid LASP partition (typically Xblk1 or Yblk1).
    """
    p = _lower(partition).strip()
    if p == "":
        return default_partition

    # Aliases for "round-robin / interleaved".
    if p in ("rr", "roundrobin", "round-robin", "round_robin", "interleaved"):
        ks = [l for l in launches if l.get("kernel") == kernel_name]
        if not ks:
            return "Xblk1"
        wgx = [max(1, int(workgroups_per_dim(l)[0])) for l in ks]
        wgy = [max(1, int(workgroups_per_dim(l)[1])) for l in ks]
        # Avoid degenerate dimensions (plan is per-kernel).
        if min(wgx) > 1:
            return "Xblk1"
        if min(wgy) > 1:
            return "Yblk1"
        return default_partition

    if p.startswith("xdiv"):
        return "Xdiv"
    if p.startswith("ydiv"):
        return "Ydiv"

    if p.startswith("xblk"):
        n = _parse_trailing_int(p) or 1
        return f"Xblk{int(n)}"
    if p.startswith("yblk"):
        n = _parse_trailing_int(p) or 1
        return f"Yblk{int(n)}"

    # Unknown -> fall back (keeps the plan runnable).
    return default_partition

# -----------------------------
# Scheduling policy decision (multi-level locality)
# -----------------------------

def canonical_lasp_policy(policy: str = "") -> str:
    """Return the single planner scheduling-policy name used by this project.

    The old planner distinguished "lasp" and "lasp-blockcu".  That choice only
    affected the within-chiplet WG->CU ordering and made the paper/method harder to
    explain.  The planner now always emits "lasp-blockcu" for its own scheduling plans:
    Xdiv/Ydiv/Xblk/Yblk still decide WG->chiplet mapping, and lasp-blockcu denotes the
    simulator-side SA-major CU fill policy inside each selected chiplet.  Old policy
    names are accepted only as CLI/backward-compatibility aliases.
    """
    return "lasp-blockcu"


def choose_kernel_policies(
    kernel_names: List[str],
    launches: List[dict],
    kernel_partition: Dict[str, str],
    *,
    default_policy: str,
    sa_per_chiplet: int,
    cu_per_sa: int,
    num_chiplets: int,
    blockcu_min_wgs_per_cu: float = 1.0,
) -> Dict[str, str]:
    """Choose the per-kernel scheduling policy.

    Current contract: emit exactly one policy name, ``lasp-blockcu``, for every kernel.
    The chiplet-level mapping decision remains in ``kernel_partition``; this function
    no longer tries to switch between plain LASP and block-CU variants.
    """
    return {str(k): "lasp-blockcu" for k in (kernel_names or [])}

# -----------------------------
# Main
# -----------------------------

def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--kernels", required=True, help="Step1 kernels.json (mgvm-kernel-analyzer output)")
    ap.add_argument("--global", dest="global_json", required=True, help="Step2 global.json (mgvm-host-analyzer output)")
    ap.add_argument("--out", required=True, help="Output plan.json")
    ap.add_argument("--hardware-config", default="", help="Hardware configuration JSON (chiplets/page_bytes/SA/CU/etc).")
    ap.add_argument("--chiplets", type=int, default=0, help="Chiplets per GPU (override hardware-config)")
    ap.add_argument("--page-bytes", type=int, default=0, help="Page size in bytes (override hardware-config/global.json; 0 => infer)")

    # Hardware knobs (optional, but used for scheduling policy in v4)
    ap.add_argument("--sa-per-chiplet", type=int, default=0, help="SA per chiplet (for auto policy)")
    ap.add_argument("--cu-per-sa", type=int, default=0, help="CU per SA (stored in plan.json only)")

    # Cost model knobs
    ap.add_argument("--remote-penalty", type=float, default=1.0, help="Remote penalty multiplier (relative)")
    ap.add_argument("--store-weight", type=float, default=1.0, help="Extra weight multiplier for stores/writes")
    # Phase2.4 global producer-consumer reuse boost
    ap.add_argument("--pc-alpha", type=float, default=1.2,
                    help="Producer-consumer locality boost factor (>1 enables; 1 disables).")
    ap.add_argument("--pc-max-distance", type=int, default=4,
                    help="Max launch distance for producer-consumer edges (in program order).")
    ap.add_argument("--pc-min-bytes", type=int, default=0,
                    help="Optional: only boost tags with at least this many bytes (0 => no filter).")
    ap.add_argument("--dynamic-scale", choices=["none", "wg", "wi"], default="wg",
                    help="Scale Step1 inst counts by dynamic factor: none / total workgroups / total workitems")

    # Profile-guided replan (Phase3-1)
    ap.add_argument("--profile-json", default="",
                    help="Optional: simulator profile JSON (kernel×tag×page×chiplet R/W bytes). "
                         "When provided, the planner blends profile into the static traffic model.")
    ap.add_argument("--profile-alpha", type=float, default=1.0,
                    help="Blend factor for profile guidance (0=ignore profile, 1=use profile only). "
                         "Only takes effect when --profile-json is set.")
    ap.add_argument("--profile-scale", type=float, default=1.0,
                    help="Scale profile bytes before blending (relative weight vs static).")
    ap.add_argument("--profile-read-weight", type=float, default=1.0,
                    help="Weight multiplier for profile read bytes.")
    ap.add_argument("--profile-write-weight", type=float, default=1.0,
                    help="Weight multiplier for profile write bytes.")
    # Phase3-2A: profile confidence gating (planner-only, conservative defaults)
    ap.add_argument("--profile-gate-max-active-full", type=int, default=4,
                    help="If a (kernel,tag) touches <= this many chiplets, trust profile fully for that event.")
    ap.add_argument("--profile-gate-peak0", type=float, default=0.20,
                    help="For high-active (kernel,tag), ignore profile if peak_ratio <= this threshold.")
    ap.add_argument("--profile-gate-peak1", type=float, default=0.35,
                    help="For high-active (kernel,tag), fully trust profile if peak_ratio >= this threshold.")
    ap.add_argument("--profile-gate-min-bytes", type=float, default=4096.0,
                    help="Ignore profile (kernel,tag) events with total bytes < this threshold (noise filter).")

    # Phase3-2B: plan stability regularization (planner-only, optional)
    ap.add_argument("--stability-base-plan", default="",
                    help="Optional: base plan.json to regularize against (discourage page mapping jitter).")
    ap.add_argument("--stability-lambda", type=float, default=0.02,
                    help="Stability penalty strength (relative). 0 disables. Requires --stability-base-plan.")



    # Loop tripcount scaling (conservative by default: only small-WG launches)
    ap.add_argument("--loop-tripcount-alpha", type=float, default=1.0,
                    help="Strength of loop tripcount scaling for low-parallel launches (0 disables; 1 = full).")
    ap.add_argument("--loop-tripcount-min", type=int, default=8,
                    help="Only scale addrec loops with tripcount >= this threshold.")
    ap.add_argument("--loop-tripcount-cap", type=int, default=65536,
                    help="Clamp evaluated tripcount to this cap.")
    ap.add_argument("--loop-tripcount-wg-max", type=int, default=64,
                    help="Only apply tripcount scaling when total workgroups <= this value (safety).")

    # Bandwidth striping heuristic: producer-parallel + consumer-lowpar streaming loops
    ap.add_argument("--bw-stripe", dest="bw_stripe", action="store_true", default=True,
                    help="Enable bandwidth striping heuristic for special low-par kernels (default on).")
    ap.add_argument("--no-bw-stripe", dest="bw_stripe", action="store_false",
                    help="Disable bandwidth striping heuristic.")
    ap.add_argument("--bw-consumer-max-active", type=int, default=1,
                    help="Consumer active-chiplets threshold to trigger bandwidth striping (<=).")
    ap.add_argument("--bw-producer-min-active", type=int, default=4,
                    help="Producer active-chiplets threshold to trigger bandwidth striping (>=).")
    ap.add_argument("--bw-min-tripcount", type=int, default=256,
                    help="Minimum evaluated addrec tripcount to consider a consumer as streaming-loop heavy.")
    ap.add_argument("--bw-min-bytes", type=int, default=262144,
                    help="Minimum buffer size (bytes) to consider for bandwidth striping.")
    ap.add_argument("--bw-tripcount-cap", type=int, default=65536,
                    help="Tripcount cap used in bandwidth striping detection (safety).")
    ap.add_argument("--partition-policy", choices=["largest", "weighted"], default="weighted",
                    help="Kernel partition selection heuristic")
    ap.add_argument("--partition-util-alpha", type=float, default=1.0,
                    help="When Xdiv/Ydiv both viable, penalize lower chiplet-util partitions by (util_ratio^alpha). alpha=1 is linear.")

    ap.add_argument("--blockcu-min-wgs-per-cu", type=float, default=1.0,
                    help="Auto blockcu only if (WGs_per_chiplet) > (CUs_per_chiplet * this). 1.0 means >1 WG per CU.")

    ap.add_argument("--banks-per-chiplet", type=int, default=0,
                    help="(Optional) L2 banks per chiplet. Not required for planning yet; stored for reporting / future refinements.")


    ap.add_argument("--load-req-bytes", type=float, default=16.0)
    ap.add_argument("--load-rsp-bytes", type=float, default=64.0)
    ap.add_argument("--store-req-bytes", type=float, default=16.0)
    ap.add_argument("--store-rsp-bytes", type=float, default=16.0)

    ap.add_argument("--group-pages", type=int, default=1, help="Compress pages into blocks of this many pages (>=1)")
    ap.add_argument("--chiplet-cap-pages", type=int, default=0, help="Optional per-chiplet page capacity (0 => unlimited)")
    ap.add_argument("--chiplet-overflow-pages", type=int, default=0,
                    help="Optional soft overflow pages per chiplet beyond --chiplet-cap-pages (0 => disabled).")
    ap.add_argument("--chiplet-overflow-penalty", type=int, default=0,
                    help="Additional per-page placement penalty for using overflow capacity (0 => disabled).")
    ap.add_argument("--lowpar-active-max", type=int, default=2,
                    help="Treat kernels with <= this many active chiplets as low-parallel for static compact-placement bias.")
    ap.add_argument("--lowpar-numwg-max", type=int, default=8,
                    help="Treat kernels with <= this many workgroups as low-parallel for static compact-placement bias.")
    ap.add_argument("--lowpar-bias-alpha", type=float, default=1.0,
                    help="Extra penalty (fraction of per-page traffic) for assigning low-parallel hot buffers outside their preferred chiplet.")

    # v4 robustness knobs
    ap.add_argument("--readonly-stripe", action="store_true",
                    help="Force read-only buffers to be striped across chiplets (only if accessed with active chiplets>1). "
                         "Helps transformer-like weights/buffers when affinity is uncertain.")
    ap.add_argument("--default-policy", default="lasp-blockcu", choices=["auto", "lasp", "lasp-cu", "lasp-blockcu"],
                    help="Compatibility-only knob. The planner always emits lasp-blockcu; old names are accepted as aliases.")

    # Phase2 placement model knobs
    ap.add_argument("--placement-model", choices=["affinity", "legacy"], default="affinity",
                    help="Page placement cost model: affinity (Phase2, uses Step1 mem_access coeffs) or legacy")
    ap.add_argument("--affinity-max-blocks", type=int, default=200000,
                    help="Safety cap for Xblk/Yblk region enumeration (blocks). If exceeded, falls back to legacy for that access.")
    ap.add_argument("--debug-out", default="",
                    help="Optional: write an extra debug JSON alongside the plan (does not affect simulator).")

    # Optional overlap / reorder hints for the simulator-side launch manager.
    ap.add_argument("--emit-launch-hints", action="store_true",
                    help="Emit dependency/priority/resource hints for the simulator runtime manager. "
                         "The generated hints use Step2's explicit launch DAG, runtime profile-derived CU usage, "
                         "and can enable adaptive workgroup chunking for non-critical branch kernels.")
    ap.add_argument("--launch-metrics-csv", default="",
                    help="Optional metrics_kernels.csv used to derive better critical-path priorities for launch hints. "
                         "If omitted, the planner falls back to a static duration model based on workgroup counts.")
    ap.add_argument("--split-remote-penalty-ps-per-byte", type=float, default=10.0,
                    help="Proxy locality penalty used when comparing WG split strategies (ps per estimated remote byte; default 10.0).")
    ap.add_argument("--clap-group-pages", type=int, default=0,
                    help="CLAP-style application-wide region size in pages. 0 means auto-infer (bert=1, gpt=2, others heuristic).")
    ap.add_argument("--disable-clap-explicit-postpass", action="store_true",
                    help="Disable the CLAP-style explicit placement post-pass and keep the raw min-cost-flow placement.")


    args = ap.parse_args()

    karg_access = load_step1_kernels(args.kernels)
    step1_access_records = load_step1_access_records(args.kernels) if args.placement_model == "affinity" else {}
    g = load_step2_global(args.global_json)

    # Hardware config (new in v5.1): prefer a dedicated JSON instead of CLI flags
    hw_cfg: dict = {}
    if getattr(args, 'hardware_config', ''):
        try:
            hw_cfg = json.load(open(args.hardware_config, 'r'))
            # allow either top-level fields or {\"hardware\": {...}}
            if isinstance(hw_cfg, dict) and isinstance(hw_cfg.get('hardware'), dict):
                hw_cfg = hw_cfg.get('hardware') or {}
        except Exception as e:
            raise SystemExit(f'ERROR: failed to read --hardware-config {args.hardware_config}: {e}')

    buffers = g.get("buffers", []) or []
    launches = g.get("launches", []) or []
    buffers_by_tag = {b.get("tag"): b for b in buffers if b.get("tag")}

    chiplets = int(args.chiplets) if args.chiplets and args.chiplets > 0 else 0
    if chiplets <= 0:
        chiplets = int(hw_cfg.get('chiplets', 0) or 0)
    if chiplets <= 0:
        chiplets = int((g.get('hardware', {}) or {}).get('chiplets', 0) or 0)
    if chiplets <= 0:
        raise SystemExit('ERROR: chiplets must be provided via --hardware-config (preferred) or --chiplets.')

    page_bytes = int(args.page_bytes) if args.page_bytes and args.page_bytes > 0 else 0
    if page_bytes <= 0:
        page_bytes = int(hw_cfg.get('page_bytes', 0) or 0)
    if page_bytes <= 0:
        page_bytes = int(g.get('page_bytes', 0) or 0)
    if page_bytes <= 0:
        page_bytes = 4096

    # Optional profile-guided placement input
    profile_data: Optional[ProfileData] = None
    if getattr(args, 'profile_json', ''):
        profile_data = load_profile_json(
            args.profile_json,
            planner_chiplets=chiplets,
            planner_page_bytes=page_bytes,
            read_weight=float(getattr(args, 'profile_read_weight', 1.0) or 1.0),
            write_weight=float(getattr(args, 'profile_write_weight', 1.0) or 1.0),
            scale=1.0,
            gate_max_active_full=int(getattr(args, 'profile_gate_max_active_full', 4) or 4),
            gate_peak0=float(getattr(args, 'profile_gate_peak0', 0.20) or 0.20),
            gate_peak1=float(getattr(args, 'profile_gate_peak1', 0.35) or 0.35),
            gate_min_bytes=float(getattr(args, 'profile_gate_min_bytes', 4096.0) or 4096.0),
        )

    
    # Phase3-2B: optional stability base plan (discourage mapping jitter across warmups)
    stability_base_map: Optional[Dict[str, List[int]]] = None
    stability_meta: Optional[Dict[str, object]] = None
    if getattr(args, 'stability_base_plan', '') and float(getattr(args, 'stability_lambda', 0.0) or 0.0) > 0.0:
        stability_base_map, stability_meta = load_base_plan_pages_map(
            args.stability_base_plan,
            expect_page_bytes=page_bytes,
            expect_chiplets=chiplets,
        )
# Determine partition per kernel
    kernel_names = sorted({l.get("kernel") for l in launches if l.get("kernel")})
    kernel_partition: Dict[str, str] = {}
    for k in kernel_names:
        if args.partition_policy == "largest":
            # v2 behavior (largest buffer decides)
            # reuse v2's choose: find largest buffer in first launch and check its stride
            rep = None
            for l in launches:
                if l.get("kernel") == k:
                    rep = l
                    break
            if rep is None:
                kernel_partition[k] = "Xdiv"
                continue
            best = None
            for b in rep.get("arg_bindings", []) or []:
                if b.get("kind") != "buffer":
                    continue
                tag = b.get("tag")
                if not tag or tag not in buffers_by_tag:
                    continue
                sz = int(buffers_by_tag[tag].get("bytes", 0) or 0)
                arg_idx = int(b.get("arg_index", -1))
                if arg_idx < 0:
                    continue
                if best is None or sz > best[0]:
                    best = (sz, arg_idx)
            if best is None:
                kernel_partition[k] = "Xdiv"
                continue
            _sz, arg_idx = best
            acc = karg_access.get(k, {}).get(arg_idx)
            if acc is None:
                kernel_partition[k] = "Xdiv"
            elif acc.has_gid_y or acc.stride_kind == "strided":
                kernel_partition[k] = "Ydiv"
            else:
                kernel_partition[k] = "Xdiv"
        else:
            kernel_partition[k] = choose_partition_weighted(
                k, launches, buffers_by_tag, karg_access, default_partition="Xdiv",
                num_chiplets=chiplets, page_bytes=int(page_bytes), util_tradeoff_alpha=float(args.partition_util_alpha),
            )


    # Sanity safety-net: avoid degenerate partitions for 1D launches.
    # (Even if user selects --partition-policy=largest, we must not pick Ydiv when numWGY==1.)
    for k in kernel_names:
        ks = [l for l in launches if l.get("kernel") == k]
        if not ks:
            continue
        wgx = [max(1, workgroups_per_dim(l)[0]) for l in ks]
        wgy = [max(1, workgroups_per_dim(l)[1]) for l in ks]
        min_wgx = min(wgx)
        min_wgy = min(wgy)
        if kernel_partition.get(k) == "Ydiv" and min_wgy <= 1 and min_wgx > 1:
            kernel_partition[k] = "Xdiv"
        if kernel_partition.get(k) == "Xdiv" and min_wgx <= 1 and min_wgy > 1:
            kernel_partition[k] = "Ydiv"


    # Normalize partitions so LASP accepts them (e.g., RR -> Xblk1).
    for k in kernel_names:
        kernel_partition[k] = normalize_lasp_partition_for_sim(
            kernel_partition.get(k, "Xdiv"),
            k,
            launches,
            default_partition="Xdiv",
        )

    # Scheduling policy (multi-level)
    sa_per_chiplet = int(args.sa_per_chiplet or 0)
    if sa_per_chiplet <= 0:
        sa_per_chiplet = int(hw_cfg.get('sa_per_chiplet', 0) or 0)

    cu_per_sa = int(args.cu_per_sa or 0)
    if cu_per_sa <= 0:
        cu_per_sa = int(hw_cfg.get('cu_per_sa', 0) or 0)

    banks_per_chiplet = int(args.banks_per_chiplet or 0)
    if banks_per_chiplet <= 0:
        banks_per_chiplet = int(hw_cfg.get('banks_per_chiplet', 0) or 0)

    kernel_policy = choose_kernel_policies(
        kernel_names,
        launches,
        kernel_partition,
        default_policy=args.default_policy,
        sa_per_chiplet=sa_per_chiplet,
        cu_per_sa=cu_per_sa,
        num_chiplets=chiplets,
        blockcu_min_wgs_per_cu=float(args.blockcu_min_wgs_per_cu),
    )

    launch_hints: Optional[List[dict]] = None
    launch_hints_debug: Optional[dict] = None
    if bool(getattr(args, 'emit_launch_hints', False)):
        launch_hints, launch_hints_debug = build_launch_hints(
            launches,
            kernel_partition,
            kernel_policy,
            num_chiplets=int(chiplets),
            sa_per_chiplet=int(sa_per_chiplet or 1),
            cu_per_sa=int(cu_per_sa or 1),
            launch_metrics_csv=str(getattr(args, 'launch_metrics_csv', '') or ''),
            profile=profile_data,
            global_graph=(g.get('graph', {}) or {}),
            placement_proxy=stability_base_map,
            split_remote_penalty_ps_per_byte=float(getattr(args, 'split_remote_penalty_ps_per_byte', 10.0) or 10.0),
        )


    fanout_fix_changed = {}
    fanout_fix_debug = {}

    # Phase2.4: producer-consumer reuse boost (global, kernel-agnostic)
    buffers_by_tag = {b.get("tag"): b for b in buffers if b.get("tag")}
    pc_boost, pc_stats = compute_producer_consumer_boost(
        launches, buffers_by_tag,
        pc_alpha=float(getattr(args, 'pc_alpha', 1.0) or 1.0),
        pc_max_distance=int(getattr(args, 'pc_max_distance', 0) or 0),
        pc_min_bytes=int(getattr(args, 'pc_min_bytes', 0) or 0),
    )

    # Traffic weights (+ optional loop tripcount scaling)
    launch_buffer_traffic, traffic_stats = build_launch_buffer_traffic_weights(
        launches,
        karg_access,
        dynamic_scale=args.dynamic_scale,
        load_req_bytes=args.load_req_bytes,
        load_rsp_bytes=args.load_rsp_bytes,
        store_req_bytes=args.store_req_bytes,
        store_rsp_bytes=args.store_rsp_bytes,
        store_weight=args.store_weight,
        pc_boost=pc_boost,
        step1_access_records=step1_access_records,
        loop_tripcount_alpha=float(getattr(args, 'loop_tripcount_alpha', 1.0) or 1.0),
        loop_tripcount_min=int(getattr(args, 'loop_tripcount_min', 8) or 8),
        loop_tripcount_cap=int(getattr(args, 'loop_tripcount_cap', 65536) or 65536),
        loop_tripcount_wg_max=int(getattr(args, 'loop_tripcount_wg_max', 64) or 64),
    )

    # Costs + solve
    plan_debug: Optional[dict] = None
    if args.placement_model == "affinity":
        items, page_costs, _total_pages, debug, global_dbg = build_page_costs_affinity(
            buffers=buffers,
            launches=launches,
            kernel_partition=kernel_partition,
            launch_buffer_traffic=launch_buffer_traffic,
            step1_access_records=step1_access_records,
            num_chiplets=chiplets,
            page_bytes=page_bytes,
            remote_penalty=args.remote_penalty,
            group_pages=max(1, int(args.group_pages)),
            force_ro_stripe=bool(args.readonly_stripe),
            max_blocks=int(args.affinity_max_blocks),
            bw_stripe_enable=bool(getattr(args, 'bw_stripe', True)),
            bw_consumer_max_active=int(getattr(args, 'bw_consumer_max_active', 1) or 1),
            bw_producer_min_active=int(getattr(args, 'bw_producer_min_active', 4) or 4),
            bw_min_tripcount=int(getattr(args, 'bw_min_tripcount', 256) or 256),
            bw_min_bytes=int(getattr(args, 'bw_min_bytes', 262144) or 262144),
            bw_tripcount_cap=int(getattr(args, 'bw_tripcount_cap', 65536) or 65536),
            profile=profile_data,
            profile_alpha=float(getattr(args, 'profile_alpha', 1.0) or 1.0) if profile_data else 0.0,
            profile_scale=float(getattr(args, 'profile_scale', 1.0) or 1.0) if profile_data else 1.0,
            stability_base_map=stability_base_map,
            stability_lambda=float(getattr(args, 'stability_lambda', 0.0) or 0.0) if stability_base_map else 0.0,
            stability_meta=stability_meta,
            lowpar_active_max=int(getattr(args, 'lowpar_active_max', 2) or 2),
            lowpar_numwg_max=int(getattr(args, 'lowpar_numwg_max', 8) or 8),
            lowpar_bias_alpha=float(getattr(args, 'lowpar_bias_alpha', 1.0) or 1.0),
        )
        if pc_stats:
            global_dbg["producer_consumer"] = pc_stats
        if traffic_stats:
            global_dbg["traffic_scaling"] = traffic_stats
        plan_debug = {
            "placement_model": "affinity",
            "affinity": global_dbg,
        }
    else:
        items, page_costs, _total_pages, debug = build_page_costs_legacy(
            buffers=buffers,
            launches=launches,
            kernel_partition=kernel_partition,
            launch_buffer_traffic=launch_buffer_traffic,
            num_chiplets=chiplets,
            page_bytes=page_bytes,
            remote_penalty=args.remote_penalty,
            group_pages=max(1, int(args.group_pages)),
            force_ro_stripe=bool(args.readonly_stripe),
            profile=profile_data,
            profile_alpha=float(getattr(args, 'profile_alpha', 1.0) or 1.0) if profile_data else 0.0,
            profile_scale=float(getattr(args, 'profile_scale', 1.0) or 1.0) if profile_data else 1.0,
        )
        plan_debug = {
            "placement_model": "legacy",
        }

        if pc_stats:
            plan_debug["producer_consumer"] = pc_stats
        if traffic_stats:
            plan_debug["traffic_scaling"] = traffic_stats

    if launch_hints_debug:
        if plan_debug is None:
            plan_debug = {}
        plan_debug["launch_manager"] = launch_hints_debug

    chiplet_cap_pages = int(args.chiplet_cap_pages or 0)
    if chiplet_cap_pages <= 0:
        chiplet_cap_pages = int(hw_cfg.get('chiplet_cap_pages', 0) or 0)
    if chiplet_cap_pages <= 0:
        # Accept dram capacity in bytes and derive pages
        dram_bytes = int(hw_cfg.get('chiplet_dram_bytes', 0) or hw_cfg.get('dram_bytes_per_chiplet', 0) or 0)
        if dram_bytes > 0 and page_bytes > 0:
            chiplet_cap_pages = int(dram_bytes // int(page_bytes))
    chiplet_overflow_pages = int(getattr(args, 'chiplet_overflow_pages', 0) or 0)
    if chiplet_overflow_pages <= 0:
        chiplet_overflow_pages = int(hw_cfg.get('chiplet_overflow_pages', 0) or 0)
    chiplet_overflow_penalty = int(getattr(args, 'chiplet_overflow_penalty', 0) or 0)
    if chiplet_overflow_penalty <= 0:
        chiplet_overflow_penalty = int(hw_cfg.get('chiplet_overflow_penalty', 0) or 0)
    flow, min_cost = solve_assignment_min_cost_flow(
        items,
        page_costs,
        chiplets,
        chiplet_cap_pages if chiplet_cap_pages > 0 else None,
        chiplet_overflow_pages=int(chiplet_overflow_pages),
        chiplet_overflow_penalty=int(chiplet_overflow_penalty),
    )

    # Emit plan
    hardware_extra = {}
    if sa_per_chiplet > 0:
        hardware_extra["sa_per_chiplet"] = sa_per_chiplet
    cu_per_sa = int(cu_per_sa)
    if cu_per_sa > 0:
        hardware_extra["cu_per_sa"] = cu_per_sa
    banks_per_chiplet = int(banks_per_chiplet)
    if banks_per_chiplet > 0:
        hardware_extra["banks_per_chiplet"] = banks_per_chiplet
    if chiplet_cap_pages > 0:
        hardware_extra["chiplet_cap_pages"] = int(chiplet_cap_pages)
    if chiplet_overflow_pages > 0 and chiplet_overflow_penalty > 0:
        hardware_extra["chiplet_overflow_pages"] = int(chiplet_overflow_pages)
        hardware_extra["chiplet_overflow_penalty"] = int(chiplet_overflow_penalty)


    clap_group_pages = infer_clap_application_group_pages(
        args.global_json,
        buffers,
        int(page_bytes),
        requested=int(getattr(args, 'clap_group_pages', 0) or 0),
    )

    objective = f"min_cost_flow(placement_model={args.placement_model}, remote_penalty={args.remote_penalty}, dynamic_scale={args.dynamic_scale}, partition_policy={args.partition_policy}, util_alpha={args.partition_util_alpha}, readonly_stripe={args.readonly_stripe}, scheduling_policy=lasp-blockcu, affinity_max_blocks={args.affinity_max_blocks})"
    emit_plan_json(
        args.out,
        tool="mgvm-ilp-planner",
        version=8 if launch_hints else 5,
        objective=objective,
        chiplets=chiplets,
        page_bytes=int(page_bytes),
        hardware_extra=hardware_extra,
        kernel_partition=kernel_partition,
        kernel_policy=kernel_policy,
        buffers=buffers,
        items=items,
        item_flow=flow,
        debug_info=debug,
        launches=launches,
        launch_hints=launch_hints,
        launch_sequence_length=len(launches) if launch_hints else 0,
        plan_debug=plan_debug,
        stability_base_map=stability_base_map,
        stability_meta=stability_meta,
        graph_obj=(g.get("graph", {}) or {}),
        profile=profile_data,
        clap_group_pages=int(clap_group_pages),
        use_clap_explicit_postpass=(profile_data is not None and bool(launch_hints) and not bool(getattr(args, 'disable_clap_explicit_postpass', False))),
    )
    if args.debug_out:
        try:
            with open(args.debug_out, "w") as f:
                json.dump(plan_debug, f, indent=2, sort_keys=False)
        except Exception as e:
            print(f"WARNING: failed to write --debug-out {args.debug_out}: {e}")


    # Print a compact summary (for sanity / logs)
    print("=== MGVM Planner v5 summary ===")
    print(f"chiplets={chiplets} page_bytes={int(page_bytes)} group_pages={int(args.group_pages)}")
    print(f"dynamic_scale={args.dynamic_scale} remote_penalty={args.remote_penalty} readonly_stripe={args.readonly_stripe}")
    print(f"loop_tripcount_alpha={getattr(args, 'loop_tripcount_alpha', 1.0)} (min={getattr(args, 'loop_tripcount_min', 8)}, cap={getattr(args, 'loop_tripcount_cap', 65536)}, wg_max={getattr(args, 'loop_tripcount_wg_max', 64)}) bw_stripe={bool(getattr(args, 'bw_stripe', True))}")
    if getattr(args, 'profile_json', ''):
        pa = float(getattr(args, 'profile_alpha', 1.0) or 1.0)
        ps = float(getattr(args, 'profile_scale', 1.0) or 1.0)
        ppb = int((getattr(profile_data, 'stats', {}) or {}).get('profile_page_bytes', 0) or 0) if profile_data else 0
        pgp = int((getattr(profile_data, 'stats', {}) or {}).get('group_pages', 0) or 0) if profile_data else 0
        print(f"profile_json={args.profile_json} alpha={pa} scale={ps} profile_page_bytes={ppb} profile_group_pages={pgp}")
    print(f"pc_alpha={args.pc_alpha} pc_max_distance={args.pc_max_distance} pc_min_bytes={args.pc_min_bytes} pc_edges={int((pc_stats or {}).get('edges',0))}")
    print(f"scheduling_policy=lasp-blockcu (default_policy_alias={args.default_policy}, sa_per_chiplet={sa_per_chiplet})")
    print(f"min_cost={min_cost}")
    for k in kernel_names:
        print(f"  kernel={k:24s} partition={kernel_partition[k]:4s} policy={kernel_policy[k]}")

if __name__ == "__main__":
    main()
