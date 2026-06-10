#!/usr/bin/env python3
from __future__ import annotations
import argparse, csv, io, json, math, os, sys, importlib.util, subprocess, tempfile
from dataclasses import dataclass
from pathlib import Path
from collections import defaultdict, deque
from copy import deepcopy
from typing import Dict, List, Optional, Tuple, Any, Set

# ---------------- utilities ----------------

def load_json(path: Optional[str]):
    if not path:
        return None
    return json.loads(Path(path).read_text())

def dump_json(path: str, obj):
    Path(path).write_text(json.dumps(obj, indent=2))

def ceil_div(a: int, b: int) -> int:
    return (a + max(1,b) - 1) // max(1,b)

def as_int(v, default: int = 0) -> int:
    if v is None:
        return int(default)
    try:
        return int(v)
    except Exception:
        return int(default)

def norm_vec(v: Optional[List[int]], n: int) -> List[int]:
    out = [0] * n
    if v:
        for i, x in enumerate(v[:n]):
            try:
                out[i] = int(x)
            except Exception:
                out[i] = 0
    return out

def sum_pos(v: List[int]) -> int:
    return sum(max(0, int(x)) for x in v)


def canonical_lasp_policy(policy: str = "") -> str:
    """Canonical scheduling policy emitted by the planner.

    Xdiv/Ydiv/Xblk/Yblk still encode WG->chiplet mapping.  The policy field is
    now intentionally single-valued: lasp-blockcu.  Old lasp/lasp-cu strings are
    accepted only as backward-compatible input aliases from old seed plans.
    """
    return "lasp-blockcu"


def normalize_plan_laspblockcu(plan: dict) -> dict:
    """Normalize all planner-owned scheduling-policy fields to lasp-blockcu."""
    if not isinstance(plan, dict):
        return plan
    for e in plan.get("kernel_scheduling") or []:
        if isinstance(e, dict):
            e["policy"] = canonical_lasp_policy(e.get("policy", "lasp-blockcu"))
    for h in plan.get("launch_hints") or []:
        if isinstance(h, dict):
            h["policy"] = canonical_lasp_policy(h.get("policy", "lasp-blockcu"))
    return plan

def unique_sorted(xs):
    return sorted(set(xs))

def max_int(a,b):
    return a if a>b else b

def min_int(a,b):
    return a if a<b else b

# ---------------- module loading / clap explicit placement import ----------------

def _load_module_from_candidates(module_name: str, candidates: List[Path]):
    for cand in candidates:
        try:
            if not cand.exists():
                continue
            spec = importlib.util.spec_from_file_location(module_name, cand)
            mod = importlib.util.module_from_spec(spec)
            assert spec and spec.loader
            sys.modules[spec.name] = mod
            spec.loader.exec_module(mod)
            return mod
        except Exception:
            continue
    return None


def _fallback_rr_buffer_placement(global_json: dict, profile: Optional[dict], page_bytes: int, num_chiplets: int) -> dict:
    sizes: Dict[str, int] = {}
    if profile:
        for b in profile.get("buffers") or []:
            tag = b.get("tag")
            if tag:
                size = int(b.get("size") or b.get("bytes") or 0)
                pages = int(b.get("pages") or 0)
                if size <= 0 and pages > 0:
                    size = int(pages) * int(page_bytes)
                sizes[tag] = max(sizes.get(tag, 0), int(size))
    for b in global_json.get("buffers") or []:
        tag = b.get("tag")
        if tag:
            size = int(b.get("size") or b.get("bytes") or 0)
            pages = int(b.get("pages") or 0)
            if size <= 0 and pages > 0:
                size = int(pages) * int(page_bytes)
            sizes[tag] = max(sizes.get(tag, 0), int(size))
    out = {}
    for tag, size in sorted(sizes.items()):
        pages = max(1, ceil_div(size, max(1, page_bytes)))
        out[tag] = {"policy": "explicit", "pages": [i % max(1, num_chiplets) for i in range(pages)]}
    return out


def _load_final_mod(script_path: Path):
    here = script_path.parent
    return _load_module_from_candidates(
        "ilp_planner_final",
        [
            here / "ilp_planner_final.py",
            Path("/mnt/data/ilp_planner_final.py"),
        ],
    )


def _load_clap_mod(script_path: Path):
    here = script_path.parent
    return _load_module_from_candidates(
        "clap_planner_current_sim_standalone",
        [
            here / "clap_planner_current_sim_standalone.py",
            here.parent / "step3_clap" / "clap_planner_current_sim_standalone.py",
            here.parent / "step3_clap" / "clap_planner.py",
            Path("/mnt/data/clap_planner_current_sim_standalone.py"),
            Path("/mnt/data/clap_planner.py"),
        ],
    )


def _load_clap_hardware_config(script_path: Path) -> dict:
    here = script_path.parent
    candidates = [
        here.parent / "step3_clap" / "hardware.config.clap.current_sim.json",
        here.parent / "step3_clap" / "hardware.config.clap.json",
        Path("/mnt/data/hardware.config.clap.current_sim.json"),
        Path("/mnt/data/hardware.config.clap.json"),
    ]
    for cand in candidates:
        try:
            if cand.exists():
                return json.loads(cand.read_text())
        except Exception:
            continue
    return {}


def build_clap_primary_placement(script_path: Path, profile: dict, hardware: dict, global_json: Optional[dict]) -> Tuple[dict, dict]:
    mod = _load_clap_mod(script_path)
    clap_hw = _load_clap_hardware_config(script_path)
    merged_hw = dict(clap_hw or {})
    merged_hw.update({k: v for k, v in (hardware or {}).items() if v is not None})
    num_chiplets = int((merged_hw or {}).get("chiplets") or profile.get("chiplets") or 1)
    page_bytes = int((merged_hw or {}).get("page_bytes") or profile.get("page_bytes") or 4096)
    if mod is None:
        return {
            "tool": "planner-clap-fallback",
            "version": 1,
            "buffer_placement": _fallback_rr_buffer_placement(global_json or {}, profile, page_bytes, num_chiplets),
        }, {"enabled": False, "reason": "clap module not found", "fallback": "rr-explicit"}

    cfg = dict(merged_hw or {})
    cfg.update({
        "chiplets": int(num_chiplets),
        "page_bytes": int(page_bytes),
        "chiplet_cap_pages": int((merged_hw or {}).get("chiplet_cap_pages") or 0),
        "overflow_pages": int((merged_hw or {}).get("overflow_pages") or (merged_hw or {}).get("chiplet_overflow_pages") or 0),
        "banks_per_chiplet": int((merged_hw or {}).get("banks_per_chiplet") or 0),
        "require_contiguous_regions": bool((merged_hw or {}).get("require_contiguous_regions", True)),
        "max_contiguous_group_pages": int((merged_hw or {}).get("max_contiguous_group_pages") or (merged_hw or {}).get("banks_per_chiplet") or 0),
        "candidate_group_pages": [1, 2, 4],
        "purity_threshold": 0.80,
        "diffuse_threshold": float((merged_hw or {}).get("clap_diffuse_threshold") or (merged_hw or {}).get("diffuse_threshold") or 0.45),
        "min_region_pages": int((merged_hw or {}).get("min_region_pages") or 1),
        "max_region_pages": int((merged_hw or {}).get("max_region_pages") or 1024),
        "group_selection_mode": "app",
        "fallback_policy": str((merged_hw or {}).get("fallback_policy") or "rr"),
    })
    try:
        planner_cls = getattr(mod, "ClapExplicitPlanner", None) or getattr(mod, "CLAPPlanner", None)
        if planner_cls is None:
            raise RuntimeError("CLAP planner class not found in loaded module")
        planner = planner_cls(
            profile=profile,
            hardware=cfg,
            global_json=global_json,
            use_global_sizes=bool(global_json),
            group_selection_mode="app",
        )
        plan_obj, debug_obj = planner.plan()
        if isinstance(debug_obj, dict):
            debug_obj = dict(debug_obj)
            debug_obj.setdefault("enabled", True)
            debug_obj.setdefault("mode", "placement-only-explicit-pages")
            debug_obj.setdefault("selection_rule", "integrated-safe-app-mode")
            debug_obj.setdefault("group_selection_mode", "app")
        return plan_obj, debug_obj
    except Exception as e:
        return {
            "tool": "planner-clap-fallback",
            "version": 1,
            "buffer_placement": _fallback_rr_buffer_placement(global_json or {}, profile, page_bytes, num_chiplets),
        }, {"enabled": False, "reason": f"clap planner failed: {e}", "fallback": "rr-explicit"}

# ---------------- data classes ----------------

@dataclass
class ActualLaunch:
    launch_id: int
    kernel: str
    start_sec: float
    end_sec: float
    latency_sec: float
    grid_size: List[int]
    local_size: List[int]
    num_wg: List[int]
    num_wg_total: int
    wg_count_by_chiplet: List[int]
    used_cus_by_chiplet: List[int]
    peak_cus_by_chiplet: List[int]
    seed_policy: str
    seed_partition: str
    reads: List[str]
    writes: List[str]
    launch_signature: str
    base_launch: dict
    split_safe_1d: bool = False
    split_safe_2d: bool = False

@dataclass
class ScheduleItem:
    id: int
    program_launch_id: int
    kernel: str
    policy: str
    partition: str
    depends_on: List[int]
    priority: int
    orig_num_wg_total: int
    final_num_wg_total: int
    est_cus: int
    chiplet_cus: List[int]
    est_duration_ps: int
    sub_wg_factor: int = 0
    note: str = ""

# ---------------- metrics/profile parsing ----------------

def parse_csv(path: str | Path) -> List[dict]:
    text = Path(path).read_text()
    return list(csv.DictReader(io.StringIO(text)))

def load_seed_kernel_scheduling(seed_plan: dict) -> Dict[str, Tuple[str, str]]:
    out = {}
    for e in seed_plan.get("kernel_scheduling") or []:
        k = e.get("kernel")
        if k:
            out[str(k)] = (canonical_lasp_policy(e.get("policy") or "lasp-blockcu"), str(e.get("partition") or "Xdiv"))
    return out


def metrics_rows_kernels(metrics_rows: List[dict]) -> List[dict]:
    out = []
    for idx, row in enumerate(metrics_rows):
        out.append({
            "launch_id": idx,
            "kernel": str(row["kernel_name"]),
            "start_sec": float(row["start_time_sec"]),
            "end_sec": float(row["end_time_sec"]),
            "latency_sec": float(row["latency_sec"]),
            "grid_size": [int(row["grid_x"]), int(row["grid_y"]), int(row["grid_z"])],
            "local_size": [int(row["wg_x"]), int(row["wg_y"]), int(row["wg_z"])],
            "num_wg": [int(row["num_wgx"]), int(row["num_wgy"]), int(row["num_wgz"])],
            "num_wg_total": int(row["num_wg_total"]),
        })
    return out


def profile_rows(profile: dict) -> List[dict]:
    out = []
    for idx, k in enumerate(profile.get("kernels") or []):
        rt = k.get("runtime") or {}
        gs = list(map(int, k.get("grid_size") or [1,1,1]))
        ws = list(map(int, k.get("wg_size") or [1,1,1]))
        nwg = [ceil_div(gs[i], ws[i] if i < len(ws) else 1) for i in range(3)]
        out.append({
            "launch_id": idx,
            "kernel": str(k.get("name") or ""),
            "peak_cus_by_chiplet": list(map(int, rt.get("peak_active_cus_by_chiplet") or [])),
            "used_cus_by_chiplet": list(map(int, rt.get("used_cus_by_chiplet") or [])),
            "wg_count_by_chiplet": list(map(int, rt.get("wg_count_by_chiplet") or [])),
            "profile_kernel": k,
        })
    return out


def assign_wgmap_to_launches(wg_rows: List[dict], mrows: List[dict]) -> Dict[int, List[dict]]:
    by_kernel = defaultdict(list)
    for idx, row in enumerate(wg_rows):
        rr = dict(row)
        rr["_assigned"] = False
        rr["_idx"] = idx
        by_kernel[str(row["kernel"])].append(rr)
    for k in by_kernel:
        by_kernel[k].sort(key=lambda r: (float(r["dispatch_time"]), r["_idx"]))
    out = {int(m["launch_id"]): [] for m in mrows}
    tol = 1e-9
    for m in mrows:
        k = m["kernel"]
        start = float(m["start_sec"]) - 1e-6
        end = float(m["end_sec"]) + 1e-6
        pool = by_kernel.get(k, [])
        chosen = []
        for r in pool:
            if r["_assigned"]:
                continue
            t = float(r["dispatch_time"])
            if start - tol <= t <= end + tol:
                chosen.append(r)
        if not chosen:
            for r in pool:
                if not r["_assigned"]:
                    chosen.append(r)
                    break
        for r in chosen:
            r["_assigned"] = True
        out[int(m["launch_id"])] = chosen
    return out


def extract_kernel_bodies(kernel_source_text: str) -> Dict[str, str]:
    bodies: Dict[str, str] = {}
    import re
    pat = re.compile(r'__kernel\s+void\s+(\w+)\s*\([^)]*\)\s*\{', re.M)
    for m in pat.finditer(kernel_source_text):
        name = m.group(1)
        i = m.end()-1
        depth = 0
        j = i
        while j < len(kernel_source_text):
            ch = kernel_source_text[j]
            if ch == '{':
                depth += 1
            elif ch == '}':
                depth -= 1
                if depth == 0:
                    bodies[name] = kernel_source_text[m.end():j]
                    break
            j += 1
    return bodies


def is_split_safe(body: str, grid_size: List[int], local_size: List[int]) -> Tuple[bool, bool]:
    body_l = body.lower()
    has_barrier = 'barrier(' in body_l or 'work_group_barrier' in body_l
    has_local = '__local' in body_l or 'local ' in body_l
    if has_barrier or has_local:
        return False, False
    gx, gy, gz = [max(1, int(v)) for v in (grid_size + [1,1,1])[:3]]
    lx, ly, lz = [max(1, int(v)) for v in (local_size + [1,1,1])[:3]]
    # The current simulator-level sub-WG implementation only preserves correct
    # local-id semantics when every original work-group has the same full shape.
    # If Step2 / runtime geometry would create partial edge work-groups, keep the
    # split path disabled so the planner and simulator stay aligned.
    if gx % lx != 0 or gy % ly != 0 or gz % lz != 0:
        return False, False
    safe_1d = (ly == 1 and lz == 1 and lx >= 4)
    safe_2d = (lz == 1 and ly > 1 and lx >= 2 and ly >= 2)
    return safe_1d, safe_2d


def pick_max_nonzero(primary: List[int], fallback: List[int]) -> List[int]:
    if sum_pos(primary) > 0:
        return list(primary)
    return list(fallback)


def build_actual_launches(global_json: dict, profile: dict, metrics_rows: List[dict], wgmap_rows: List[dict], seed_sched: Dict[str, Tuple[str, str]], num_chiplets: int, kernels_source_text: str) -> Tuple[List[ActualLaunch], dict]:
    g_launches = list(global_json.get("launches") or [])
    m_launches = metrics_rows_kernels(metrics_rows)
    p_launches = profile_rows(profile)
    wg_by_launch = assign_wgmap_to_launches(wgmap_rows, m_launches) if wgmap_rows else {}
    bodies = extract_kernel_bodies(kernels_source_text)
    n = min(len(g_launches), len(m_launches), len(p_launches))
    actuals: List[ActualLaunch] = []
    dbg = {"name_mismatches": [], "wgmap_assigned_rows": {}}
    for i in range(n):
        g = deepcopy(g_launches[i])
        m = m_launches[i]
        p = p_launches[i]
        if str(g.get("kernel")) != m["kernel"] or str(g.get("kernel")) != p["kernel"]:
            dbg["name_mismatches"].append({"launch_id": i, "global": g.get("kernel"), "metrics": m["kernel"], "profile": p["kernel"]})
        g["global_size"] = list(m["grid_size"])
        g["local_size"] = list(m["local_size"])
        g["num_wg"] = list(m["num_wg"])
        g["num_wg_total"] = int(m["num_wg_total"])
        pol, part = seed_sched.get(str(g.get("kernel") or ""), ("lasp-blockcu", "Xdiv"))
        pol = canonical_lasp_policy(pol)

        wg_rows = wg_by_launch.get(i, [])
        wg_count_by_chip = [0] * num_chiplets
        used_cus_by_chip = [0] * num_chiplets
        if wg_rows:
            wgset = [set() for _ in range(num_chiplets)]
            cuset = [set() for _ in range(num_chiplets)]
            for r in wg_rows:
                chip = int(r["chiplet"])
                if 0 <= chip < num_chiplets:
                    wgset[chip].add((int(r["wg_x"]), int(r["wg_y"]), int(r["wg_z"])))
                    cuset[chip].add((int(r["sa"]), int(r["cu"])))
            wg_count_by_chip = [len(s) for s in wgset]
            used_cus_by_chip = [len(s) for s in cuset]
        peak = norm_vec(p.get("peak_cus_by_chiplet"), num_chiplets)
        used = norm_vec(p.get("used_cus_by_chiplet"), num_chiplets)
        if sum_pos(used_cus_by_chip) > 0:
            used = pick_max_nonzero(used, used_cus_by_chip)
        if sum_pos(peak) <= 0:
            peak = list(used)
        else:
            peak = pick_max_nonzero(peak, used)
        if sum_pos(wg_count_by_chip) <= 0:
            wg_count_by_chip = norm_vec(p.get("wg_count_by_chiplet"), num_chiplets)
        body = bodies.get(m["kernel"], "")
        safe1d, safe2d = is_split_safe(body, list(m["grid_size"]), list(m["local_size"]))
        actuals.append(ActualLaunch(
            launch_id=i,
            kernel=m["kernel"],
            start_sec=float(m["start_sec"]),
            end_sec=float(m["end_sec"]),
            latency_sec=float(m["latency_sec"]),
            grid_size=list(m["grid_size"]),
            local_size=list(m["local_size"]),
            num_wg=list(m["num_wg"]),
            num_wg_total=int(m["num_wg_total"]),
            wg_count_by_chiplet=wg_count_by_chip,
            used_cus_by_chiplet=used,
            peak_cus_by_chiplet=peak,
            seed_policy=pol,
            seed_partition=part,
            reads=list(g.get("reads") or []),
            writes=list(g.get("writes") or []),
            launch_signature=str(g.get("launch_signature") or ""),
            base_launch=g,
            split_safe_1d=safe1d,
            split_safe_2d=safe2d,
        ))
        dbg["wgmap_assigned_rows"][str(i)] = len(wg_rows)
    dbg.update({"launch_count": n})
    return actuals, dbg

# ---------------- graph / motif ----------------

def path_exists(src: int, dst: int, succ: Dict[int, Set[int]]) -> bool:
    if src == dst:
        return True
    seen = set([src])
    stack = [src]
    while stack:
        u = stack.pop()
        for v in succ.get(u, ()):
            if v == dst:
                return True
            if v not in seen:
                seen.add(v)
                stack.append(v)
    return False


def build_graphs(global_json: dict, launch_count: int) -> Tuple[Dict[int, Set[int]], Dict[int, Set[int]], Dict[int, Set[int]], Dict[int, Set[int]]]:
    raw_succ = {i:set() for i in range(launch_count)}
    raw_pred = {i:set() for i in range(launch_count)}
    full_succ = {i:set() for i in range(launch_count)}
    full_pred = {i:set() for i in range(launch_count)}
    launches = list(global_json.get("launches") or [])[:launch_count]
    for e in (global_json.get("graph") or {}).get("edges") or []:
        try:
            u = int(e.get("from")); v = int(e.get("to"))
        except Exception:
            continue
        t = str(e.get("type") or "RAW").upper()
        if 0 <= u < launch_count and 0 <= v < launch_count:
            if t == "RAW":
                raw_succ[u].add(v); raw_pred[v].add(u)
            full_succ[u].add(v); full_pred[v].add(u)
    for i in range(launch_count):
        wi = set(launches[i].get("writes") or [])
        ri = set(launches[i].get("reads") or [])
        for j in range(i+1, launch_count):
            wj = set(launches[j].get("writes") or [])
            rj = set(launches[j].get("reads") or [])
            raw = bool(wi & rj)
            hazard = raw or bool(wi & wj) or bool(ri & wj)
            if raw:
                raw_succ[i].add(j); raw_pred[j].add(i)
            if hazard:
                full_succ[i].add(j); full_pred[j].add(i)
    return raw_succ, raw_pred, full_succ, full_pred


def detect_attention_motifs(actuals: List[ActualLaunch], raw_succ: Dict[int, Set[int]], raw_pred: Dict[int, Set[int]]) -> List[dict]:
    by_name = {a.launch_id: a.kernel for a in actuals}
    motifs = []
    for s in range(len(actuals)):
        kname = by_name.get(s)
        if "attention_scores" not in kname:
            continue
        preds = sorted(raw_pred[s])
        q = next((u for u in preds if "q_proj" in by_name.get(u, "")), None)
        k = next((u for u in preds if "k_proj" in by_name.get(u, "")), None)
        if q is None or k is None:
            continue
        shared = raw_pred[q].intersection(raw_pred[k])
        if not shared:
            continue
        parent = sorted(shared)[0]
        v = next((u for u in sorted(raw_succ[parent]) if "v_proj" in by_name.get(u, "")), None)
        soft = next((u for u in sorted(raw_succ[s]) if "softmax" in by_name.get(u, "")), None)
        if v is None or soft is None:
            continue
        ctx = None
        for cand in sorted(raw_succ[soft]):
            if "attention_context" in by_name.get(cand, "") and v in raw_pred[cand]:
                ctx = cand
                break
        if ctx is None:
            continue
        motifs.append({"parent": parent, "q": q, "k": k, "v": v, "scores": s, "softmax": soft, "context": ctx})
    return motifs

# ---------------- split helpers ----------------

def compatible_split_factor(local_size: List[int], factor: int, allow_2d: bool = False) -> bool:
    x, y, z = [max(1, int(v)) for v in (local_size + [1,1,1])[:3]]
    if z != 1:
        return False
    if y == 1:
        return x % factor == 0
    if not allow_2d:
        return False
    for fx in range(1, x+1):
        if x % fx != 0 or factor % fx != 0:
            continue
        rem = factor // fx
        if rem > 0 and y % rem == 0:
            return True
    return False


def max_supported_factor(local_size: List[int], min_sub_threads: int, cap_factor: int, allow_2d: bool=False) -> int:
    threads = max(1, int(local_size[0]) * int(local_size[1]) * int(local_size[2]))
    max_by_size = max(1, threads // max(1, min_sub_threads))
    if cap_factor > 0:
        max_by_size = min(max_by_size, cap_factor)
    for f in range(max_by_size, 1, -1):
        if compatible_split_factor(local_size, f, allow_2d=allow_2d):
            return f
    return 1


def scale_duration_ps(latency_sec: float, base_total: int, new_total: int) -> int:
    if base_total <= 0 or new_total <= 0:
        return int(max(1, round(latency_sec * 1e12)))
    return int(max(1, round(latency_sec * (base_total / float(new_total)) * 1e12)))


def choose_subwg_shape(local_size: List[int], max_factor: int, *, allow_2d: bool, min_1d_x: int = 4, min_2d_x: int = 2, min_2d_y: int = 2) -> Optional[dict]:
    lx, ly, lz = [max(1, int(v)) for v in (local_size + [1,1,1])[:3]]
    if lz != 1:
        return None
    max_factor = min(max(1, int(max_factor)), lx * ly)
    if max_factor <= 1:
        return None
    best = None
    for f in range(max_factor, 1, -1):
        cands = []
        if ly == 1:
            if lx % f == 0:
                subx = lx // f
                if subx >= max(1, int(min_1d_x)):
                    cands.append({"factor": f, "x_parts": f, "y_parts": 1, "sub_x": subx, "sub_y": 1, "kind": "1d"})
        elif allow_2d:
            for fx in range(1, min(f, lx) + 1):
                if f % fx != 0 or lx % fx != 0:
                    continue
                fy = f // fx
                if fy <= 0 or ly % fy != 0:
                    continue
                subx = lx // fx
                suby = ly // fy
                if subx < max(1, int(min_2d_x)) or suby < max(1, int(min_2d_y)):
                    continue
                cands.append({"factor": f, "x_parts": fx, "y_parts": fy, "sub_x": subx, "sub_y": suby, "kind": "2d"})
        if not cands:
            continue
        cands.sort(key=lambda c: (
            min(int(c["sub_x"]), int(c["sub_y"])),
            -abs(int(c["sub_x"]) - int(c["sub_y"])),
            int(c["sub_x"]),
            int(c["sub_y"]),
        ), reverse=True)
        best = cands[0]
        break
    return best


def hint_windows_total(h: dict, default_total: int) -> int:
    wins = list(h.get("wg_windows") or [])
    if not wins:
        return int(default_total)
    total = 0
    for w in wins:
        try:
            total += max(0, int((w or {}).get("count", 0) or 0))
        except Exception:
            continue
    return max(1, int(total))


def peak_partner_chiplet_usage(timeline: List[dict], item_id: int, num_chiplets: int) -> List[int]:
    item = next((t for t in (timeline or []) if int(t.get("launch_id", -1)) == int(item_id)), None)
    if item is None:
        return [0] * max(1, int(num_chiplets))
    try:
        start_i = int(item.get("start_ps", 0) or 0)
        end_i = int(item.get("end_ps", 0) or 0)
    except Exception:
        return [0] * max(1, int(num_chiplets))
    if end_i <= start_i:
        return [0] * max(1, int(num_chiplets))
    overlaps = []
    bounds = {start_i, end_i}
    for other in (timeline or []):
        try:
            oid = int(other.get("launch_id", -1))
        except Exception:
            oid = -1
        if oid == int(item_id):
            continue
        s = max(start_i, int(other.get("start_ps", 0) or 0))
        e = min(end_i, int(other.get("end_ps", 0) or 0))
        if e <= s:
            continue
        vec = norm_vec(other.get("chiplet_cus") or [], max(1, int(num_chiplets)))
        overlaps.append((s, e, vec))
        bounds.add(s)
        bounds.add(e)
    if not overlaps:
        return [0] * max(1, int(num_chiplets))
    peak = [0] * max(1, int(num_chiplets))
    ordered = sorted(bounds)
    for i in range(len(ordered) - 1):
        a = ordered[i]
        b = ordered[i + 1]
        if b <= a:
            continue
        cur = [0] * max(1, int(num_chiplets))
        for s, e, vec in overlaps:
            if s < b and e > a:
                for c in range(min(len(cur), len(vec))):
                    cur[c] += int(vec[c])
        for c in range(len(cur)):
            if cur[c] > peak[c]:
                peak[c] = cur[c]
    return peak


def legacy_launch_hints_from_items(items: List[ScheduleItem]) -> List[dict]:
    launch_hints = []
    for it in items:
        h = {
            "id": int(it.id),
            "program_launch_id": int(it.program_launch_id),
            "kernel": it.kernel,
            "depends_on": [int(x) for x in sorted(unique_sorted(it.depends_on))],
            "priority": int(it.priority),
            "est_cus": int(it.est_cus),
            "chiplet_cus": [int(x) for x in it.chiplet_cus],
            "est_duration_ps": int(it.est_duration_ps),
            "policy": canonical_lasp_policy(it.policy),
            "partition": it.partition,
        }
        if it.sub_wg_factor > 1:
            h["sub_wg_factor"] = int(it.sub_wg_factor)
        launch_hints.append(h)
    return launch_hints


def apply_low_parallel_subwg_postpass(actuals: List[ActualLaunch], launch_hints: List[dict], *, final_mod, base_launches: List[dict], num_chiplets: int, sa_per_chiplet: int, cu_per_sa: int, min_subwg_threads: int) -> Tuple[List[dict], dict]:
    hints = [deepcopy(h) for h in (launch_hints or [])]
    if not hints or final_mod is None:
        return hints, {"enabled": False, "reason": "no-hints-or-final-mod"}

    total_cus = max(1, int(num_chiplets) * max(1, int(sa_per_chiplet) * int(cu_per_sa)))
    cap_per_chip = max(1, int(sa_per_chiplet) * int(cu_per_sa))

    try:
        eval_hints, schedule, _nodes, _preds, _succs, _dur = final_mod._evaluate_hint_plan(
            base_launches,
            hints,
            total_cus=int(total_cus),
            num_chiplets=int(num_chiplets),
        )
    except Exception as e:
        return hints, {"enabled": False, "reason": f"schedule-eval-failed: {e}"}

    actual_by_launch = {int(a.launch_id): a for a in (actuals or [])}
    timeline = list((schedule or {}).get("timeline") or [])
    debug_rows = []

    for h in hints:
        hid = as_int(h.get("id", -1), -1)
        pid = as_int(h.get("program_launch_id", hid), hid)
        a = actual_by_launch.get(pid)
        if a is None:
            continue
        if h.get("adaptive_wg_chunks") and not h.get("wg_windows"):
            # Keep adaptive chunk-only launches conservative; sub-WG split on top
            # of runtime-chunking is possible but much harder to reason about.
            continue
        if int(h.get("sub_wg_factor", 0) or 0) > 1:
            continue

        allow_2d = bool(a.split_safe_2d)
        allow_1d = bool(a.split_safe_1d)
        if not allow_1d and not allow_2d:
            continue

        base_chip = norm_vec(h.get("chiplet_cus") or a.peak_cus_by_chiplet or a.used_cus_by_chiplet, int(num_chiplets))
        active = [c for c, v in enumerate(base_chip) if int(v) > 0]
        base_total = sum_pos(base_chip)
        if not active or base_total <= 0 or base_total >= total_cus:
            continue

        partner_peak = peak_partner_chiplet_usage(timeline, hid, int(num_chiplets))
        max_factor = None
        for c in active:
            headroom = max(0, int(cap_per_chip) - int(partner_peak[c]))
            base_c = max(1, int(base_chip[c]))
            f = max(0, headroom // base_c)
            max_factor = f if max_factor is None else min(max_factor, f)
        if max_factor is None or int(max_factor) <= 1:
            continue

        shape = choose_subwg_shape(
            list(a.local_size),
            int(max_factor),
            allow_2d=bool(allow_2d and not allow_1d),
            min_1d_x=max(1, int(min_subwg_threads)),
            min_2d_x=2,
            min_2d_y=2,
        )
        if not shape or int(shape.get("factor", 1) or 1) <= 1:
            continue
        factor = int(shape["factor"])
        new_chip = [int(v) * factor if int(v) > 0 else 0 for v in base_chip]
        if any(int(new_chip[c]) + int(partner_peak[c]) > int(cap_per_chip) for c in active):
            continue
        new_total = sum_pos(new_chip)
        if new_total <= base_total:
            continue

        old_est = max(1, int(h.get("est_cus", base_total) or base_total))
        old_dur = max(1, int(h.get("est_duration_ps", max(1, round(a.latency_sec * 1e12))) or max(1, round(a.latency_sec * 1e12))))
        h["sub_wg_factor"] = int(factor)
        h["chiplet_cus"] = [int(x) for x in new_chip]
        h["est_cus"] = int(new_total)
        h["est_duration_ps"] = max(1, int(round(float(old_dur) * float(old_est) / float(max(1, new_total)))))
        debug_rows.append({
            "dispatch_id": int(hid),
            "program_launch_id": int(pid),
            "kernel": a.kernel,
            "split_kind": str(shape.get("kind", "1d")),
            "local_size": list(a.local_size),
            "sub_wg_factor": int(factor),
            "shape": shape,
            "base_chiplet_cus": list(base_chip),
            "partner_peak_chiplet_cus": list(partner_peak),
            "new_chiplet_cus": list(new_chip),
            "base_est_cus": int(old_est),
            "new_est_cus": int(new_total),
            "effective_num_wg_total": int(hint_windows_total(h, a.num_wg_total) * factor),
        })

    try:
        _eh2, schedule2, _n2, _p2, _s2, _d2 = final_mod._evaluate_hint_plan(
            base_launches,
            hints,
            total_cus=int(total_cus),
            num_chiplets=int(num_chiplets),
        )
    except Exception:
        schedule2 = schedule

    return hints, {
        "enabled": True,
        "min_subwg_threads_1d": int(min_subwg_threads),
        "min_subwg_shape_2d": [2, 2],
        "applied": debug_rows,
        "schedule_before": schedule,
        "schedule_after": schedule2,
    }

# ---------------- scheduling ----------------

def can_overlap_as_is(a: ActualLaunch, b: ActualLaunch, cap_per_chip: int, full_succ: Dict[int, Set[int]]) -> bool:
    if path_exists(a.launch_id, b.launch_id, full_succ) or path_exists(b.launch_id, a.launch_id, full_succ):
        return False
    va = pick_max_nonzero(a.peak_cus_by_chiplet, a.used_cus_by_chiplet)
    vb = pick_max_nonzero(b.peak_cus_by_chiplet, b.used_cus_by_chiplet)
    if sum_pos(va) + sum_pos(vb) > cap_per_chip * len(va):
        return False
    for i in range(len(va)):
        if int(va[i]) + int(vb[i]) > cap_per_chip:
            return False
    return True


def build_schedule_items(actuals: List[ActualLaunch], raw_succ, raw_pred, full_succ, full_pred, *, num_chiplets: int, sa_per_chiplet: int, cu_per_sa: int, enable_low_parallel_split: bool, min_subwg_threads: int) -> Tuple[List[ScheduleItem], dict]:
    cap_per_chip = sa_per_chiplet * cu_per_sa
    total_cus = cap_per_chip * num_chiplets
    items: List[ScheduleItem] = []
    for a in actuals:
        chip = pick_max_nonzero(a.peak_cus_by_chiplet, a.used_cus_by_chiplet)
        if sum_pos(chip) <= 0:
            chip = [1 if c > 0 else 0 for c in a.wg_count_by_chiplet]
            if sum_pos(chip) <= 0 and a.num_wg_total > 0:
                chip = [1] + [0]*(num_chiplets-1)
        # sequential default in actual seed order
        items.append(ScheduleItem(
            id=a.launch_id,
            program_launch_id=a.launch_id,
            kernel=a.kernel,
            policy=a.seed_policy,
            partition=a.seed_partition,
            depends_on=[a.launch_id - 1] if a.launch_id > 0 else [],
            priority=0,
            orig_num_wg_total=a.num_wg_total,
            final_num_wg_total=a.num_wg_total,
            est_cus=sum_pos(chip),
            chiplet_cus=chip,
            est_duration_ps=int(max(1, round(a.latency_sec*1e12))),
            sub_wg_factor=0,
            note="seed-actual",
        ))
    dbg = {"policy": "sequential-by-default", "parallel_overlaps": [], "low_parallel_split": []}

    motifs = detect_attention_motifs(actuals, raw_succ, raw_pred)
    for m in motifs:
        q = actuals[m["q"]]; k = actuals[m["k"]]
        if can_overlap_as_is(q, k, cap_per_chip, full_succ):
            # Both after common parent, no chain dependency between them.
            items[m["q"]].depends_on = sorted(unique_sorted((set(items[m["q"]].depends_on) - {m["q"]-1}) | {m["parent"]} | set(full_pred[m["q"]])))
            items[m["k"]].depends_on = sorted(unique_sorted((set(items[m["k"]].depends_on) - {m["k"]-1}) | {m["parent"]} | set(full_pred[m["k"]])))
            dbg["parallel_overlaps"].append({"type": "qk", "pair": [m["q"], m["k"]], "reason": "actual-fits-as-is"})
        v = actuals[m["v"]]; soft = actuals[m["softmax"]]
        if can_overlap_as_is(v, soft, cap_per_chip, full_succ):
            items[m["v"]].depends_on = sorted(unique_sorted((set(items[m["v"]].depends_on) - {m["v"]-1}) | {m["scores"]} | set(full_pred[m["v"]])))
            items[m["softmax"]].depends_on = sorted(unique_sorted((set(items[m["softmax"]].depends_on) - {m["softmax"]-1}) | {m["scores"]} | set(full_pred[m["softmax"]])))
            items[m["context"]].depends_on = sorted(unique_sorted(set(items[m["context"]].depends_on) | {m["v"], m["softmax"]}))
            dbg["parallel_overlaps"].append({"type": "v_softmax", "pair": [m["v"], m["softmax"]], "reason": "actual-fits-as-is"})

    if enable_low_parallel_split:
        partner_budget = {i: [0]*num_chiplets for i in range(len(actuals))}
        for rec in dbg["parallel_overlaps"]:
            a, b = rec["pair"]
            partner_budget[a] = list(items[b].chiplet_cus)
            partner_budget[b] = list(items[a].chiplet_cus)
        for it in items:
            a = actuals[it.id]
            # only split safe 1D kernels for now
            if not a.split_safe_1d:
                continue
            base_chip = list(it.chiplet_cus)
            base_total = sum_pos(base_chip)
            if base_total <= 0 or base_total >= total_cus:
                continue
            active = [c for c,v in enumerate(base_chip) if v > 0]
            if not active:
                continue
            max_cap_factor = None
            for c in active:
                headroom = max(0, cap_per_chip - int(partner_budget[it.id][c]))
                if headroom <= 0:
                    max_cap_factor = 1
                    break
                base_c = max(1, int(base_chip[c]))
                f = max(1, headroom // base_c)
                max_cap_factor = f if max_cap_factor is None else min(max_cap_factor, f)
            if max_cap_factor is None or max_cap_factor <= 1:
                continue
            factor = max_supported_factor(a.local_size, min_subwg_threads, max_cap_factor, allow_2d=False)
            if factor <= 1:
                continue
            new_chip = []
            for c in range(num_chiplets):
                if base_chip[c] <= 0:
                    new_chip.append(0)
                else:
                    headroom = max(0, cap_per_chip - int(partner_budget[it.id][c]))
                    new_chip.append(min(headroom, int(base_chip[c]) * factor))
            if sum_pos(new_chip) <= base_total:
                continue
            it.sub_wg_factor = factor
            it.final_num_wg_total = a.num_wg_total * factor
            it.chiplet_cus = new_chip
            it.est_cus = sum_pos(new_chip)
            it.est_duration_ps = scale_duration_ps(a.latency_sec, base_total, it.est_cus)
            it.note += f";subwg={factor}"
            dbg["low_parallel_split"].append({
                "launch_id": it.id,
                "kernel": it.kernel,
                "orig_wg_total": a.num_wg_total,
                "orig_chiplet_cus": base_chip,
                "new_chiplet_cus": new_chip,
                "sub_wg_factor": factor,
                "final_num_wg_total": it.final_num_wg_total,
            })
    return items, dbg

# ---------------- seed plan ----------------

def collect_buffer_sizes(global_json: dict, profile: Optional[dict], page_bytes: int) -> Dict[str, int]:
    """Collect buffer sizes for explicit RR seed placement.

    Step2 metadata uses bytes/pages, while some profile JSONs use size.  Keep all
    three forms so the seed plan really emits complete explicit RR pages instead of
    accidentally relying on simulator fallback.
    """
    sizes: Dict[str, int] = {}
    if profile:
        for b in profile.get("buffers") or []:
            tag = b.get("tag")
            if not tag:
                continue
            size = int(b.get("size") or b.get("bytes") or 0)
            pages = int(b.get("pages") or 0)
            if size <= 0 and pages > 0:
                size = int(pages) * int(page_bytes)
            sizes[str(tag)] = max(int(sizes.get(str(tag), 0)), int(size))
    for b in global_json.get("buffers") or []:
        tag = b.get("tag")
        if not tag:
            continue
        size = int(b.get("size") or b.get("bytes") or 0)
        pages = int(b.get("pages") or 0)
        if size <= 0 and pages > 0:
            size = int(pages) * int(page_bytes)
        sizes[str(tag)] = max(int(sizes.get(str(tag), 0)), int(size))
    return sizes


def build_seed_plan(global_json: dict, hardware: dict, profile: Optional[dict], script_path: Path) -> Tuple[dict, dict]:
    # Delegate static partition choice to sibling ilp_planner_final.py, then neutralize placement.
    final_py = script_path.parent / 'ilp_planner_final.py'
    if not final_py.exists():
        raise SystemExit(f"ERROR: missing sibling static planner {final_py}")
    num_chiplets = int((hardware or {}).get("chiplets") or 1)
    page_bytes = int((hardware or {}).get("page_bytes") or 4096)
    with tempfile.TemporaryDirectory() as td:
        outp = Path(td) / 'seed_static_tmp.plan.json'
        dbg = Path(td) / 'seed_static_tmp.debug.json'
        cmd = [sys.executable, '-S', str(final_py), '--kernels', args.kernels, '--global', args.global_json, '--hardware-config', args.hardware_config, '--out', str(outp), '--debug-out', str(dbg)]
        subprocess.run(cmd, check=True)
        plan = normalize_plan_laspblockcu(load_json(str(outp)))
        debug = load_json(str(dbg)) if dbg.exists() else {}
    sizes = collect_buffer_sizes(global_json, profile, page_bytes)
    buffer_placement = {tag: {"policy": "explicit", "pages": [i % num_chiplets for i in range(max(1, ceil_div(size, page_bytes)))]} for tag, size in sorted(sizes.items())}
    seed = {
        "tool": "planner-profiled-reqfinal",
        "version": 1,
        "objective": "seed-static-scheduling + neutral-rr-placement",
        "hardware": {"chiplets": num_chiplets, "page_bytes": page_bytes},
        "default_sched_partition": plan.get("default_sched_partition") or 'Xdiv',
        "launch_sequence_length": 0,
        "dispatch_sequence_length": 0,
        "launch_hints": [],
        "kernel_scheduling": plan.get("kernel_scheduling") or [],
        "buffer_placement": buffer_placement,
    }
    return seed, {"seed_static_debug": debug, "seed_buffer_placement": "rr-explicit"}

# ---------------- profiled plan assembly ----------------

def build_profiled_plan(global_json: dict, hardware: dict, profile: dict, profile_path: str, launch_metrics_csv: str, metrics_rows: List[dict], wgmap_rows: List[dict], seed_plan: dict, clap_primary_placement: bool, enable_low_parallel_split: bool, min_subwg_threads: int, script_path: Path, kernels_source_text: str) -> Tuple[dict, dict]:
    num_chiplets = int((hardware or {}).get("chiplets") or profile.get("chiplets") or 1)
    page_bytes = int((hardware or {}).get("page_bytes") or profile.get("page_bytes") or 4096)
    sa_per_chiplet = int((hardware or {}).get("sa_per_chiplet") or 1)
    cu_per_sa = int((hardware or {}).get("cu_per_sa") or 1)
    seed_sched = load_seed_kernel_scheduling(seed_plan)
    actuals, actual_dbg = build_actual_launches(global_json, profile, metrics_rows, wgmap_rows, seed_sched, num_chiplets, kernels_source_text)

    launch_hints: List[dict] = []
    overlap_dbg: dict = {"enabled": False, "reason": "not-built"}
    legacy_dbg: dict = {}

    final_mod = _load_final_mod(script_path)
    if final_mod is not None:
        try:
            profile_data = final_mod.load_profile_json(
                str(profile_path),
                planner_chiplets=int(num_chiplets),
                planner_page_bytes=int(page_bytes),
            )
            kernel_partition = {str(a.kernel): str(a.seed_partition) for a in actuals}
            kernel_policy = {str(a.kernel): canonical_lasp_policy(a.seed_policy) for a in actuals}
            placement_proxy = {}
            for tag, info in (seed_plan.get("buffer_placement") or {}).items():
                if not isinstance(info, dict):
                    continue
                pages = info.get("pages") or []
                if isinstance(pages, list) and pages:
                    placement_proxy[str(tag)] = [int(x) for x in pages]
            launch_hints, overlap_dbg = final_mod.build_launch_hints(
                launches=list(global_json.get("launches") or [])[:len(actuals)],
                kernel_partition=kernel_partition,
                kernel_policy=kernel_policy,
                num_chiplets=int(num_chiplets),
                sa_per_chiplet=int(sa_per_chiplet),
                cu_per_sa=int(cu_per_sa),
                launch_metrics_csv=str(launch_metrics_csv or ""),
                profile=profile_data,
                global_graph=global_json.get("graph"),
                placement_proxy=placement_proxy,
                allow_partition_overrides=False,
            )
        except Exception as e:
            overlap_dbg = {"enabled": False, "reason": f"final build_launch_hints failed: {e}", "fallback": "legacy-seed-freeze"}
            launch_hints = []
    else:
        overlap_dbg = {"enabled": False, "reason": "ilp_planner_final.py not found", "fallback": "legacy-seed-freeze"}

    if not launch_hints:
        raw_succ, raw_pred, full_succ, full_pred = build_graphs(global_json, len(actuals))
        items, legacy_dbg = build_schedule_items(actuals, raw_succ, raw_pred, full_succ, full_pred, num_chiplets=num_chiplets, sa_per_chiplet=sa_per_chiplet, cu_per_sa=cu_per_sa, enable_low_parallel_split=False, min_subwg_threads=min_subwg_threads)
        launch_hints = legacy_launch_hints_from_items(items)

    if enable_low_parallel_split:
        launch_hints, split_dbg = apply_low_parallel_subwg_postpass(
            actuals,
            launch_hints,
            final_mod=final_mod,
            base_launches=list(global_json.get("launches") or [])[:len(actuals)],
            num_chiplets=int(num_chiplets),
            sa_per_chiplet=int(sa_per_chiplet),
            cu_per_sa=int(cu_per_sa),
            min_subwg_threads=int(min_subwg_threads),
        )
    else:
        split_dbg = {"enabled": False, "reason": "disabled-by-flag"}

    launch_hints.sort(key=lambda h: (int(h.get("id", -1)), int(h.get("program_launch_id", -1))))
    for h in launch_hints:
        if isinstance(h, dict):
            h["policy"] = canonical_lasp_policy(h.get("policy", "lasp-blockcu"))

    if clap_primary_placement:
        clap_plan, clap_dbg = build_clap_primary_placement(script_path, profile, hardware, global_json)
        buffer_placement = clap_plan.get("buffer_placement") or {}
    else:
        clap_dbg = {"enabled": False}
        buffer_placement = _fallback_rr_buffer_placement(global_json, profile, page_bytes, num_chiplets)

    dispatch_sequence_length = max(
        int(len(actuals)),
        max((int(h.get("id", -1)) for h in (launch_hints or [])), default=-1) + 1,
    )
    kernel_scheduling = deepcopy(seed_plan.get("kernel_scheduling") or [])
    for e in kernel_scheduling:
        if isinstance(e, dict):
            e["policy"] = canonical_lasp_policy(e.get("policy", "lasp-blockcu"))

    plan = {
        "tool": "planner-profiled-reqfinal",
        "version": 1,
        "objective": "seed-truth branch-aware overlap + post-overlap in-chiplet subwg split + clap explicit placement",
        "hardware": {"chiplets": num_chiplets, "page_bytes": page_bytes},
        "default_sched_partition": seed_plan.get("default_sched_partition") or "Xdiv",
        "launch_sequence_length": len(actuals),
        "dispatch_sequence_length": int(dispatch_sequence_length),
        "launch_hints": launch_hints,
        "kernel_scheduling": kernel_scheduling,
        "buffer_placement": buffer_placement,
    }
    debug = {
        "actual_launch_source": actual_dbg,
        "overlap_debug": overlap_dbg,
        "legacy_schedule_debug": legacy_dbg,
        "split_postpass_debug": split_dbg,
        "clap_debug": clap_dbg,
    }
    plan = normalize_plan_laspblockcu(plan)
    return plan, debug

# ---------------- static no-profile plan assembly ----------------

def _workgroup_dims_from_launch(l: dict) -> List[int]:
    nwg = list(l.get("num_wg") or [])
    if len(nwg) >= 3 and all(int(x or 0) > 0 for x in nwg[:3]):
        return [max(1, int(x)) for x in nwg[:3]]
    gs = [max(1, int(x)) for x in (list(l.get("global_size") or []) + [1, 1, 1])[:3]]
    ls = [max(1, int(x)) for x in (list(l.get("local_size") or []) + [1, 1, 1])[:3]]
    return [ceil_div(gs[i], ls[i]) for i in range(3)]


def _static_chiplet_budget(final_mod, launch: dict, est_cus: int, partition: str, num_chiplets: int) -> List[int]:
    if final_mod is not None and hasattr(final_mod, "_balanced_chiplet_budget_for_launch"):
        try:
            return [int(x) for x in final_mod._balanced_chiplet_budget_for_launch(launch, int(est_cus), str(partition), int(num_chiplets))]
        except Exception:
            pass
    active = max(1, min(int(num_chiplets), int(est_cus)))
    per = int(est_cus) // active
    rem = int(est_cus) % active
    out = [0] * int(num_chiplets)
    for c in range(active):
        out[c] = per + (1 if c < rem else 0)
    return out


def build_static_actual_launches(global_json: dict, plan: dict, final_mod, num_chiplets: int, sa_per_chiplet: int, cu_per_sa: int, kernels_source_text: str) -> List[ActualLaunch]:
    sched = load_seed_kernel_scheduling(plan)
    bodies = extract_kernel_bodies(kernels_source_text or "")
    hint_by_pid: Dict[int, dict] = {}
    for h in plan.get("launch_hints") or []:
        if not isinstance(h, dict):
            continue
        pid = as_int(h.get("program_launch_id", h.get("id", -1)), -1)
        if pid >= 0 and pid not in hint_by_pid:
            hint_by_pid[pid] = h
    actuals: List[ActualLaunch] = []
    for i, gl in enumerate(global_json.get("launches") or []):
        l = deepcopy(gl)
        kernel = str(l.get("kernel") or "")
        gs = [max(1, int(x)) for x in (list(l.get("global_size") or []) + [1, 1, 1])[:3]]
        ls = [max(1, int(x)) for x in (list(l.get("local_size") or []) + [1, 1, 1])[:3]]
        nwg = _workgroup_dims_from_launch(l)
        num_wg_total = max(1, int(l.get("num_wg_total") or (nwg[0] * nwg[1] * nwg[2])))
        pol, part = sched.get(kernel, ("lasp-blockcu", "Xdiv"))
        pol = canonical_lasp_policy(pol)
        h = hint_by_pid.get(i, {})
        est_cus = as_int(h.get("est_cus"), 0) if h else 0
        if est_cus <= 0 and final_mod is not None and hasattr(final_mod, "estimate_launch_cus"):
            try:
                est_cus = int(final_mod.estimate_launch_cus(l, part, num_chiplets=int(num_chiplets), sa_per_chiplet=int(sa_per_chiplet), cu_per_sa=int(cu_per_sa)))
            except Exception:
                est_cus = 0
        if est_cus <= 0:
            est_cus = max(1, min(num_wg_total, int(num_chiplets) * int(sa_per_chiplet) * int(cu_per_sa)))
        chip = norm_vec((h.get("chiplet_cus") if h else None) or [], int(num_chiplets))
        if sum_pos(chip) <= 0:
            chip = _static_chiplet_budget(final_mod, l, int(est_cus), str(part), int(num_chiplets))
        dur_ps = as_int(h.get("est_duration_ps"), 0) if h else 0
        if dur_ps <= 0 and final_mod is not None and hasattr(final_mod, "static_launch_duration_ps"):
            try:
                dur_ps = int(final_mod.static_launch_duration_ps(l))
            except Exception:
                dur_ps = 0
        if dur_ps <= 0:
            dur_ps = max(1, num_wg_total)
        safe1d, safe2d = is_split_safe(bodies.get(kernel, ""), gs, ls)
        actuals.append(ActualLaunch(
            launch_id=i, kernel=kernel, start_sec=0.0, end_sec=float(dur_ps) / 1e12, latency_sec=float(dur_ps) / 1e12,
            grid_size=gs, local_size=ls, num_wg=nwg, num_wg_total=int(num_wg_total),
            wg_count_by_chiplet=[0] * int(num_chiplets), used_cus_by_chiplet=list(chip), peak_cus_by_chiplet=list(chip),
            seed_policy=pol, seed_partition=str(part), reads=list(l.get("reads") or []), writes=list(l.get("writes") or []),
            launch_signature=str(l.get("launch_signature") or ""), base_launch=l, split_safe_1d=bool(safe1d), split_safe_2d=bool(safe2d),
        ))
    return actuals



def build_static_sequential_split_only_hints(global_json: dict, plan: dict, final_mod, *, num_chiplets: int, sa_per_chiplet: int, cu_per_sa: int) -> Tuple[List[dict], dict]:
    """Generate no-profile static launch hints without enabling kernel overlap.

    Static hints are intentionally host-order sequential.  They exist only to carry
    simulator-compatible scheduling metadata and optional sub_wg_factor generated by
    the split postpass.  They must not introduce wg_windows/adaptive chunks or alter
    the kernel_scheduling partition selected by the static planner.
    """
    launches = list(global_json.get("launches") or [])
    sched = load_seed_kernel_scheduling(plan)
    default_partition = str(plan.get("default_sched_partition") or "Xdiv")
    total_cus = max(1, int(num_chiplets) * max(1, int(sa_per_chiplet) * int(cu_per_sa)))

    hints: List[dict] = []
    debug_rows: List[dict] = []
    chiplet_budget_filled = 0
    for i, launch in enumerate(launches):
        kernel = str(launch.get("kernel") or f"launch_{i}")
        policy, partition = sched.get(kernel, ("lasp-blockcu", default_partition))
        policy = canonical_lasp_policy(policy)
        partition = str(partition or default_partition or "Xdiv")

        est_cus = 0
        if final_mod is not None and hasattr(final_mod, "estimate_launch_cus"):
            try:
                est_cus = int(final_mod.estimate_launch_cus(
                    launch,
                    partition,
                    num_chiplets=int(num_chiplets),
                    sa_per_chiplet=int(sa_per_chiplet),
                    cu_per_sa=int(cu_per_sa),
                ))
            except Exception:
                est_cus = 0
        if est_cus <= 0:
            num_wg_total = max(1, int(launch.get("num_wg_total") or math.prod(_workgroup_dims_from_launch(launch))))
            est_cus = max(1, min(int(num_wg_total), int(total_cus)))

        chiplet_cus = _static_chiplet_budget(final_mod, launch, int(est_cus), str(partition), int(num_chiplets))
        if sum_pos(chiplet_cus) <= 0:
            chiplet_cus = [0 for _ in range(int(num_chiplets))]
            chiplet_cus[0] = int(est_cus)
        chiplet_budget_filled += 1

        dur_ps = 0
        if final_mod is not None and hasattr(final_mod, "static_launch_duration_ps"):
            try:
                dur_ps = int(final_mod.static_launch_duration_ps(launch))
            except Exception:
                dur_ps = 0
        if dur_ps <= 0:
            dur_ps = max(1, int(launch.get("num_wg_total") or math.prod(_workgroup_dims_from_launch(launch))))

        hint = {
            "id": int(i),
            "program_launch_id": int(i),
            "kernel": kernel,
            # Static no-profile branch forbids inter-kernel overlap: preserve host launch order.
            "depends_on": [int(i - 1)] if i > 0 else [],
            "priority": int(len(launches) - i),
            "est_cus": int(max(1, est_cus)),
            "chiplet_cus": [int(x) for x in chiplet_cus],
            "est_duration_ps": int(max(1, dur_ps)),
            "policy": policy,
            "partition": partition,
        }
        hints.append(hint)
        debug_rows.append({
            "id": int(i),
            "program_launch_id": int(i),
            "kernel": kernel,
            "depends_on": list(hint["depends_on"]),
            "partition": partition,
            "est_cus": int(hint["est_cus"]),
            "chiplet_cus": list(hint["chiplet_cus"]),
            "est_duration_ps": int(hint["est_duration_ps"]),
        })

    return hints, {
        "mode": "static-sequential-split-only",
        "kernel_overlap_enabled": False,
        "reason": "static branch has no seed/profile latency or runtime occupancy truth; hints preserve host order and only carry split metadata",
        "num_launches": int(len(launches)),
        "chiplet_budget_filled_hints": int(chiplet_budget_filled),
        "no_wg_windows": True,
        "no_adaptive_wg_chunks": True,
        "hints": debug_rows,
    }

def build_static_plan(global_json: dict, hardware: dict, *, emit_launch_hints: bool, enable_low_parallel_split: bool, min_subwg_threads: int, script_path: Path, kernels_source_text: str) -> Tuple[dict, dict]:
    """Build a no-profile static plan.

    This path intentionally does not call CLAP because CLAP-style placement needs a
    runtime profile.  It delegates Xdiv/Ydiv selection and static min-cost-flow page
    placement to ilp_planner_final.py.  If launch hints are requested, this wrapper
    emits split-only sequential hints so the static branch never introduces
    static-estimated inter-kernel overlap.
    """
    final_py = script_path.parent / 'ilp_planner_final.py'
    if not final_py.exists():
        raise SystemExit(f"ERROR: missing sibling static planner {final_py}")
    num_chiplets = int((hardware or {}).get("chiplets") or 1)
    page_bytes = int((hardware or {}).get("page_bytes") or 4096)
    sa_per_chiplet = int((hardware or {}).get("sa_per_chiplet") or 1)
    cu_per_sa = int((hardware or {}).get("cu_per_sa") or 1)
    with tempfile.TemporaryDirectory() as td:
        outp = Path(td) / 'static_tmp.plan.json'
        dbg = Path(td) / 'static_tmp.debug.json'
        # Do not pass --emit-launch-hints to ilp_planner_final.py in the static branch.
        # That path may generate static-estimated overlap/itemization.  Static only
        # uses final.py for chiplet partition selection and static page placement;
        # split-only launch hints are generated below as a strict host-order chain.
        cmd = [sys.executable, '-S', str(final_py), '--kernels', args.kernels, '--global', args.global_json, '--hardware-config', args.hardware_config, '--out', str(outp), '--debug-out', str(dbg), '--default-policy', 'lasp-blockcu']
        subprocess.run(cmd, check=True)
        plan = normalize_plan_laspblockcu(load_json(str(outp)))
        final_debug = load_json(str(dbg)) if dbg.exists() else {}

    final_mod = _load_final_mod(script_path)
    split_dbg = {"enabled": False, "reason": "not-requested"}
    sequential_hint_dbg = {"enabled": False, "reason": "--emit-launch-hints not set"}
    chiplet_budget_filled = 0
    launches = list(global_json.get("launches") or [])
    if emit_launch_hints:
        hints, sequential_hint_dbg = build_static_sequential_split_only_hints(
            global_json,
            plan,
            final_mod,
            num_chiplets=int(num_chiplets),
            sa_per_chiplet=int(sa_per_chiplet),
            cu_per_sa=int(cu_per_sa),
        )
        chiplet_budget_filled = int(sequential_hint_dbg.get("chiplet_budget_filled_hints", 0) or 0)
        plan["launch_hints"] = hints
        plan["launch_sequence_length"] = len(launches)
        plan["dispatch_sequence_length"] = max(len(launches), max((as_int(h.get("id"), -1) for h in hints), default=-1) + 1)

        if enable_low_parallel_split:
            actuals = build_static_actual_launches(global_json, plan, final_mod, int(num_chiplets), int(sa_per_chiplet), int(cu_per_sa), kernels_source_text)
            hints, split_dbg = apply_low_parallel_subwg_postpass(
                actuals, plan.get("launch_hints") or [], final_mod=final_mod, base_launches=launches,
                num_chiplets=int(num_chiplets), sa_per_chiplet=int(sa_per_chiplet), cu_per_sa=int(cu_per_sa),
                min_subwg_threads=max(1, int(min_subwg_threads)),
            )
            for h in hints:
                if isinstance(h, dict):
                    h["policy"] = canonical_lasp_policy(h.get("policy", "lasp-blockcu"))
                    # Keep static hints split-only: no window/itemization fields are allowed.
                    h.pop("wg_windows", None)
                    h.pop("adaptive_wg_chunks", None)
                    h["depends_on"] = [as_int(h.get("id", 0), 0) - 1] if as_int(h.get("id", 0), 0) > 0 else []
            plan["launch_hints"] = list(sorted(hints, key=lambda h: (as_int(h.get("id"), -1), as_int(h.get("program_launch_id"), -1))))
            plan["dispatch_sequence_length"] = max(len(launches), max((as_int(h.get("id"), -1) for h in plan.get("launch_hints") or []), default=-1) + 1)

    plan["tool"] = "planner-static-reqfinal"
    plan["objective"] = str(plan.get("objective") or "static") + " + static-no-profile scheduling/placement"
    plan["hardware"] = dict(plan.get("hardware") or {"chiplets": num_chiplets, "page_bytes": page_bytes})
    plan["hardware"].setdefault("chiplets", int(num_chiplets))
    plan["hardware"].setdefault("page_bytes", int(page_bytes))
    plan = normalize_plan_laspblockcu(plan)
    debug = {
        "static_line": {
            "enabled": True,
            "profile_required": False,
            "chiplet_mapping": "static Xdiv/Ydiv/Xblk/Yblk from Step1/Step2",
            "scheduling_policy": "lasp-blockcu",
            "kernel_overlap": "disabled; static launch_hints are a strict host-order chain",
            "launch_hints_mode": "static-sequential-split-only" if emit_launch_hints else "disabled",
            "kernel_parallelism": "chiplet-preserving sub_wg_factor postpass" if emit_launch_hints and enable_low_parallel_split else "disabled",
            "data_placement": "static min-cost-flow/affinity placement from ilp_planner_final.py; no CLAP profile is used",
            "chiplet_budget_filled_hints": int(chiplet_budget_filled),
        },
        "static_sequential_hint_debug": sequential_hint_dbg,
        "final_static_debug": final_debug,
        "split_postpass_debug": split_dbg,
    }
    return plan, debug

# ---------------- CLI ----------------

def build_argparser():
    ap = argparse.ArgumentParser(description="Profile-guided planner aligned with seed actuals and current simulator semantics")
    ap.add_argument("--kernels")
    ap.add_argument("--global", dest="global_json", required=True)
    ap.add_argument("--hardware-config", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--debug-out")
    ap.add_argument("--profile-seed-plan", action="store_true")
    ap.add_argument("--seed-placement", default="rr")
    ap.add_argument("--profile-json")
    ap.add_argument("--freeze-kernel-scheduling-from")
    ap.add_argument("--emit-launch-hints", action="store_true")
    ap.add_argument("--launch-metrics-csv")
    ap.add_argument("--disable-launch-partition-overrides", action="store_true")
    ap.add_argument("--clap-primary-placement", action="store_true")
    ap.add_argument("--no-low-parallel-subwg-split", action="store_true")
    ap.add_argument("--low-parallel-subwg-min-subsize", type=int, default=4)
    return ap

def main():
    global args
    args = build_argparser().parse_args()
    global_json = load_json(args.global_json)
    hardware = load_json(args.hardware_config)
    profile = load_json(args.profile_json) if args.profile_json else None
    if args.profile_seed_plan:
        plan, debug = build_seed_plan(global_json, hardware, profile, Path(__file__))
    elif not args.profile_json:
        kernels_source_text = Path(args.kernels).with_name('kernels.cl').read_text() if args.kernels and Path(args.kernels).with_name('kernels.cl').exists() else ''
        plan, debug = build_static_plan(
            global_json=global_json,
            hardware=hardware,
            emit_launch_hints=bool(args.emit_launch_hints),
            enable_low_parallel_split=not bool(args.no_low_parallel_subwg_split),
            min_subwg_threads=max(1, int(args.low_parallel_subwg_min_subsize)),
            script_path=Path(__file__),
            kernels_source_text=kernels_source_text,
        )
    else:
        if not args.launch_metrics_csv:
            raise SystemExit("--launch-metrics-csv is required for profiled plan generation")
        if not args.freeze_kernel_scheduling_from:
            raise SystemExit("--freeze-kernel-scheduling-from is required for profiled plan generation")
        seed_plan = load_json(args.freeze_kernel_scheduling_from)
        metrics_rows = parse_csv(args.launch_metrics_csv)
        wgmap_path = Path(args.launch_metrics_csv).with_name("metrics_wgmap.csv")
        wgmap_rows = parse_csv(wgmap_path) if wgmap_path.exists() else []
        kernels_source_text = Path(args.kernels).with_name('kernels.cl').read_text() if args.kernels and Path(args.kernels).with_name('kernels.cl').exists() else ''
        plan, debug = build_profiled_plan(
            global_json=global_json,
            hardware=hardware,
            profile=profile,
            profile_path=str(args.profile_json),
            launch_metrics_csv=str(args.launch_metrics_csv),
            metrics_rows=metrics_rows,
            wgmap_rows=wgmap_rows,
            seed_plan=seed_plan,
            clap_primary_placement=bool(args.clap_primary_placement),
            enable_low_parallel_split=not bool(args.no_low_parallel_subwg_split),
            min_subwg_threads=max(1, int(args.low_parallel_subwg_min_subsize)),
            script_path=Path(__file__),
            kernels_source_text=kernels_source_text,
        )
    plan = normalize_plan_laspblockcu(plan)
    dump_json(args.out, plan)
    if args.debug_out:
        dump_json(args.debug_out, debug)
    print(f"[planner-profiled-reqfinal] wrote {args.out}")

if __name__ == "__main__":
    main()
