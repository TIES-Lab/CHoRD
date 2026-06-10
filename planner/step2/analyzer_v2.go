package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"sort"
	"strconv"
	"strings"
)

type RSliceVal struct {
	Len      int64
	ElemType string
	Elems    map[int64]RValue
}

type RStructVal struct {
	TypeName string
	Fields   map[string]RValue
}

type RValue struct {
	Kind  string // int|float|string|arr3|ref|struct|slice|unknown
	Int   int64
	Float float64
	Str   string
	Arr3  [3]int64
	Ref   string
	St    *RStructVal
	Sl    *RSliceVal
	Expr  string
}

func (v RValue) Clone() RValue {
	out := v
	if v.St != nil {
		fields := make(map[string]RValue, len(v.St.Fields))
		for k, fv := range v.St.Fields {
			fields[k] = fv.Clone()
		}
		out.St = &RStructVal{TypeName: v.St.TypeName, Fields: fields}
	}
	if v.Sl != nil {
		elems := make(map[int64]RValue, len(v.Sl.Elems))
		for k, ev := range v.Sl.Elems {
			elems[k] = ev.Clone()
		}
		out.Sl = &RSliceVal{Len: v.Sl.Len, ElemType: v.Sl.ElemType, Elems: elems}
	}
	return out
}

func rValuesEqual(a, b RValue) bool {
	if a.Kind != b.Kind || a.Int != b.Int || a.Float != b.Float || a.Str != b.Str || a.Ref != b.Ref || a.Expr != b.Expr || a.Arr3 != b.Arr3 {
		return false
	}
	if (a.St == nil) != (b.St == nil) || (a.Sl == nil) != (b.Sl == nil) {
		return false
	}
	if a.St != nil {
		if a.St.TypeName != b.St.TypeName || len(a.St.Fields) != len(b.St.Fields) {
			return false
		}
		for k, va := range a.St.Fields {
			vb, ok := b.St.Fields[k]
			if !ok || !rValuesEqual(va, vb) {
				return false
			}
		}
	}
	if a.Sl != nil {
		if a.Sl.Len != b.Sl.Len || a.Sl.ElemType != b.Sl.ElemType || len(a.Sl.Elems) != len(b.Sl.Elems) {
			return false
		}
		for k, va := range a.Sl.Elems {
			vb, ok := b.Sl.Elems[k]
			if !ok || !rValuesEqual(va, vb) {
				return false
			}
		}
	}
	return true
}

func cloneREnv(src map[string]RValue) map[string]RValue {
	out := make(map[string]RValue, len(src))
	for k, v := range src {
		out[k] = v.Clone()
	}
	return out
}

func mergeREnvs(base, a, b map[string]RValue) map[string]RValue {
	out := cloneREnv(base)
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	for k := range keys {
		va, oka := a[k]
		vb, okb := b[k]
		switch {
		case oka && okb:
			if rValuesEqual(va, vb) {
				out[k] = va.Clone()
			} else {
				out[k] = RValue{Kind: "unknown", Expr: k}
			}
		case oka:
			out[k] = va.Clone()
		case okb:
			out[k] = vb.Clone()
		}
	}
	return out
}

func rvInt(i int64, expr string) RValue     { return RValue{Kind: "int", Int: i, Expr: expr} }
func rvFloat(f float64, expr string) RValue { return RValue{Kind: "float", Float: f, Expr: expr} }
func rvString(s, expr string) RValue        { return RValue{Kind: "string", Str: s, Expr: expr} }

func evalStringV2(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]RValue) (string, bool) {
	v := evalValueV2(e, fset, cfg, env)
	if v.Kind == "string" {
		return v.Str, true
	}
	return "", false
}

func evalIntV2(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]RValue) (int64, bool) {
	v := evalValueV2(e, fset, cfg, env)
	switch v.Kind {
	case "int":
		return v.Int, true
	case "float":
		if math.Trunc(v.Float) == v.Float && v.Float >= float64(math.MinInt64) && v.Float <= float64(math.MaxInt64) {
			return int64(v.Float), true
		}
	}
	return 0, false
}

func evalTruthyV2(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]RValue) (bool, bool) {
	v := evalValueV2(e, fset, cfg, env)
	switch v.Kind {
	case "int":
		return v.Int != 0, true
	case "float":
		return v.Float != 0, true
	case "string":
		return v.Str != "", true
	}
	return false, false
}

func evalValueV2(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]RValue) RValue {
	switch ex := e.(type) {
	case *ast.BasicLit:
		switch ex.Kind {
		case token.INT:
			if i, err := strconv.ParseInt(ex.Value, 10, 64); err == nil {
				return rvInt(i, ex.Value)
			}
		case token.FLOAT:
			if f, err := strconv.ParseFloat(ex.Value, 64); err == nil {
				return rvFloat(f, ex.Value)
			}
		case token.STRING:
			if s, err := strconv.Unquote(ex.Value); err == nil {
				return rvString(s, s)
			}
			return rvString(ex.Value, ex.Value)
		}
		return RValue{Kind: "unknown", Expr: ex.Value}

	case *ast.Ident:
		if v, ok := env[ex.Name]; ok {
			out := v.Clone()
			if out.Expr == "" {
				out.Expr = ex.Name
			}
			return out
		}
		if pv, ok := cfg.Params[ex.Name]; ok {
			return rvInt(pv, ex.Name)
		}
		if ex.Name == "true" {
			return rvInt(1, ex.Name)
		}
		if ex.Name == "false" {
			return rvInt(0, ex.Name)
		}
		return RValue{Kind: "unknown", Expr: ex.Name}

	case *ast.SelectorExpr:
		key := exprToString(fset, ex)
		base := evalValueV2(ex.X, fset, cfg, env)
		if base.Kind == "struct" && base.St != nil {
			if fv, ok := base.St.Fields[ex.Sel.Name]; ok {
				out := fv.Clone()
				out.Expr = key
				return out
			}
		}
		if v, ok := env[key]; ok {
			out := v.Clone()
			out.Expr = key
			return out
		}
		if id, ok := ex.X.(*ast.Ident); ok && id.Name == "b" {
			if pv, ok := cfg.Params[ex.Sel.Name]; ok {
				return rvInt(pv, key)
			}
		}
		return RValue{Kind: "ref", Ref: key, Expr: key}

	case *ast.IndexExpr:
		key := exprToString(fset, ex)
		base := evalValueV2(ex.X, fset, cfg, env)
		idx, ok := evalIntV2(ex.Index, fset, cfg, env)
		if ok && base.Kind == "slice" && base.Sl != nil {
			if ev, ok2 := base.Sl.Elems[idx]; ok2 {
				out := ev.Clone()
				out.Expr = key
				return out
			}
			return RValue{Kind: "unknown", Expr: key}
		}
		if v, ok := env[key]; ok {
			out := v.Clone()
			out.Expr = key
			return out
		}
		return RValue{Kind: "unknown", Expr: key}

	case *ast.UnaryExpr:
		switch ex.Op {
		case token.AND:
			return evalValueV2(ex.X, fset, cfg, env)
		case token.SUB:
			v := evalValueV2(ex.X, fset, cfg, env)
			if v.Kind == "int" {
				return rvInt(-v.Int, exprToString(fset, e))
			}
			if v.Kind == "float" {
				return rvFloat(-v.Float, exprToString(fset, e))
			}
		}
		return RValue{Kind: "unknown", Expr: exprToString(fset, e)}

	case *ast.ParenExpr:
		return evalValueV2(ex.X, fset, cfg, env)

	case *ast.BinaryExpr:
		lv := evalValueV2(ex.X, fset, cfg, env)
		rv := evalValueV2(ex.Y, fset, cfg, env)
		// Prefer integer arithmetic when both operands are ints.
		if lv.Kind == "int" && rv.Kind == "int" {
			switch ex.Op {
			case token.ADD:
				return rvInt(lv.Int+rv.Int, exprToString(fset, e))
			case token.SUB:
				return rvInt(lv.Int-rv.Int, exprToString(fset, e))
			case token.MUL:
				return rvInt(lv.Int*rv.Int, exprToString(fset, e))
			case token.QUO:
				if rv.Int != 0 {
					return rvInt(lv.Int/rv.Int, exprToString(fset, e))
				}
			case token.REM:
				if rv.Int != 0 {
					return rvInt(lv.Int%rv.Int, exprToString(fset, e))
				}
			case token.SHL:
				return rvInt(lv.Int<<uint64(rv.Int), exprToString(fset, e))
			case token.SHR:
				return rvInt(lv.Int>>uint64(rv.Int), exprToString(fset, e))
			case token.LSS:
				if lv.Int < rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.LEQ:
				if lv.Int <= rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.GTR:
				if lv.Int > rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.GEQ:
				if lv.Int >= rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.EQL:
				if lv.Int == rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.NEQ:
				if lv.Int != rv.Int {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.LAND:
				if lv.Int != 0 && rv.Int != 0 {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.LOR:
				if lv.Int != 0 || rv.Int != 0 {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			}
		}
		// Fall back to float arithmetic if possible.
		var lf, rf float64
		okf := false
		if lv.Kind == "float" {
			lf, okf = lv.Float, true
		}
		if lv.Kind == "int" {
			lf, okf = float64(lv.Int), true
		}
		okg := false
		if rv.Kind == "float" {
			rf, okg = rv.Float, true
		}
		if rv.Kind == "int" {
			rf, okg = float64(rv.Int), true
		}
		if okf && okg {
			switch ex.Op {
			case token.ADD:
				return rvFloat(lf+rf, exprToString(fset, e))
			case token.SUB:
				return rvFloat(lf-rf, exprToString(fset, e))
			case token.MUL:
				return rvFloat(lf*rf, exprToString(fset, e))
			case token.QUO:
				if rf != 0 {
					return rvFloat(lf/rf, exprToString(fset, e))
				}
			case token.LSS:
				if lf < rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.LEQ:
				if lf <= rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.GTR:
				if lf > rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.GEQ:
				if lf >= rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.EQL:
				if lf == rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			case token.NEQ:
				if lf != rf {
					return rvInt(1, exprToString(fset, e))
				}
				return rvInt(0, exprToString(fset, e))
			}
		}
		return RValue{Kind: "unknown", Expr: exprToString(fset, e)}

	case *ast.CallExpr:
		fnName := ""
		switch fn := ex.Fun.(type) {
		case *ast.Ident:
			fnName = fn.Name
		case *ast.SelectorExpr:
			fnName = fn.Sel.Name
		}

		switch fnName {
		case "uint32", "uint64", "int32", "int64", "int", "uint", "uint16":
			if len(ex.Args) >= 1 {
				v := evalValueV2(ex.Args[0], fset, cfg, env)
				if v.Kind == "int" {
					return rvInt(v.Int, exprToString(fset, e))
				}
				if v.Kind == "float" && math.Trunc(v.Float) == v.Float {
					return rvInt(int64(v.Float), exprToString(fset, e))
				}
			}
		case "float32", "float64":
			if len(ex.Args) >= 1 {
				v := evalValueV2(ex.Args[0], fset, cfg, env)
				if v.Kind == "float" {
					return rvFloat(v.Float, exprToString(fset, e))
				}
				if v.Kind == "int" {
					return rvFloat(float64(v.Int), exprToString(fset, e))
				}
			}
		case "max", "min":
			if len(ex.Args) >= 2 {
				a := evalValueV2(ex.Args[0], fset, cfg, env)
				b := evalValueV2(ex.Args[1], fset, cfg, env)
				if a.Kind == "int" && b.Kind == "int" {
					if fnName == "max" {
						if a.Int >= b.Int {
							return a
						}
						return b
					}
					if a.Int <= b.Int {
						return a
					}
					return b
				}
			}
		case "len":
			if len(ex.Args) >= 1 {
				v := evalValueV2(ex.Args[0], fset, cfg, env)
				switch v.Kind {
				case "slice":
					if v.Sl != nil {
						return rvInt(v.Sl.Len, exprToString(fset, e))
					}
				case "string":
					return rvInt(int64(len(v.Str)), exprToString(fset, e))
				}
			}
		case "make":
			if len(ex.Args) >= 2 {
				n, ok := evalIntV2(ex.Args[1], fset, cfg, env)
				if ok {
					tstr := exprToString(fset, ex.Args[0])
					if strings.HasPrefix(tstr, "[]") {
						return RValue{Kind: "slice", Sl: &RSliceVal{Len: n, ElemType: strings.TrimPrefix(tstr, "[]"), Elems: map[int64]RValue{}}, Expr: exprToString(fset, e)}
					}
				}
			}
		case "Sprintf":
			if len(ex.Args) >= 1 {
				fmtStr, ok := evalStringV2(ex.Args[0], fset, cfg, env)
				if ok {
					vals := make([]any, 0, len(ex.Args)-1)
					okAll := true
					for _, a := range ex.Args[1:] {
						v := evalValueV2(a, fset, cfg, env)
						switch v.Kind {
						case "int":
							vals = append(vals, v.Int)
						case "float":
							vals = append(vals, v.Float)
						case "string":
							vals = append(vals, v.Str)
						default:
							okAll = false
						}
						if !okAll {
							break
						}
					}
					if okAll {
						return rvString(fmt.Sprintf(fmtStr, vals...), exprToString(fset, e))
					}
				}
			}
		}
		return RValue{Kind: "unknown", Expr: exprToString(fset, e)}

	case *ast.CompositeLit:
		tstr := ""
		if ex.Type != nil {
			tstr = exprToString(fset, ex.Type)
		}
		if strings.HasPrefix(tstr, "[3]") {
			var arr [3]int64
			for i := 0; i < 3 && i < len(ex.Elts); i++ {
				if v, ok := evalIntV2(ex.Elts[i], fset, cfg, env); ok {
					arr[i] = v
				}
			}
			return RValue{Kind: "arr3", Arr3: arr, Expr: exprToString(fset, e)}
		}
		if strings.HasPrefix(tstr, "[]") {
			sl := &RSliceVal{Len: int64(len(ex.Elts)), ElemType: strings.TrimPrefix(tstr, "[]"), Elems: map[int64]RValue{}}
			for i, elt := range ex.Elts {
				sl.Elems[int64(i)] = evalValueV2(elt, fset, cfg, env)
			}
			return RValue{Kind: "slice", Sl: sl, Expr: exprToString(fset, e)}
		}
		if id, ok := ex.Type.(*ast.Ident); ok {
			fields := map[string]RValue{}
			for _, elt := range ex.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					k := exprToString(fset, kv.Key)
					fields[k] = evalValueV2(kv.Value, fset, cfg, env)
				}
			}
			return RValue{Kind: "struct", St: &RStructVal{TypeName: id.Name, Fields: fields}, Expr: exprToString(fset, e)}
		}
		return RValue{Kind: "unknown", Expr: exprToString(fset, e)}

	default:
		return RValue{Kind: "unknown", Expr: exprToString(fset, e)}
	}

	return RValue{Kind: "unknown", Expr: exprToString(fset, e)}
}

func setExprValueV2(lhs ast.Expr, val RValue, fset *token.FileSet, cfg *Config, env map[string]RValue) {
	key := exprToString(fset, lhs)
	cloned := val.Clone()
	if cloned.Expr == "" {
		cloned.Expr = key
	}
	env[key] = cloned.Clone()

	switch ex := lhs.(type) {
	case *ast.Ident:
		env[ex.Name] = cloned.Clone()
		return

	case *ast.SelectorExpr:
		base := evalValueV2(ex.X, fset, cfg, env)
		if base.Kind == "struct" && base.St != nil {
			if base.St.Fields == nil {
				base.St.Fields = map[string]RValue{}
			}
			base.St.Fields[ex.Sel.Name] = cloned.Clone()
			setExprValueV2(ex.X, base, fset, cfg, env)
		}
		return

	case *ast.IndexExpr:
		idx, ok := evalIntV2(ex.Index, fset, cfg, env)
		if !ok {
			return
		}
		base := evalValueV2(ex.X, fset, cfg, env)
		if base.Kind == "slice" && base.Sl != nil {
			if base.Sl.Elems == nil {
				base.Sl.Elems = map[int64]RValue{}
			}
			base.Sl.Elems[idx] = cloned.Clone()
			if base.Sl.Len <= idx {
				base.Sl.Len = idx + 1
			}
			setExprValueV2(ex.X, base, fset, cfg, env)
		}
		return
	}
}

func parseSimpleForV2(fs *ast.ForStmt, fset *token.FileSet, cfg *Config, env map[string]RValue) (varName string, start, end int64, ok bool) {
	if fs.Init == nil || fs.Cond == nil || fs.Post == nil || fs.Body == nil {
		return "", 0, 0, false
	}
	as, ok := fs.Init.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return "", 0, 0, false
	}
	id, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		return "", 0, 0, false
	}
	varName = id.Name
	start, ok = evalIntV2(as.Rhs[0], fset, cfg, env)
	if !ok {
		return "", 0, 0, false
	}
	be, ok := fs.Cond.(*ast.BinaryExpr)
	if !ok {
		return "", 0, 0, false
	}
	lhsID, ok := be.X.(*ast.Ident)
	if !ok || lhsID.Name != varName {
		return "", 0, 0, false
	}
	end, ok = evalIntV2(be.Y, fset, cfg, env)
	if !ok {
		return "", 0, 0, false
	}
	switch be.Op {
	case token.LEQ:
		end++
	case token.LSS:
	default:
		return "", 0, 0, false
	}
	if inc, ok := fs.Post.(*ast.IncDecStmt); ok {
		if pid, ok := inc.X.(*ast.Ident); ok && pid.Name == varName && inc.Tok == token.INC {
			return varName, start, end, true
		}
	}
	return "", 0, 0, false
}

func collectBuffersV2(f *ast.File, fset *token.FileSet, cfg *Config, benchName string) (map[string]*BufferInfo, map[string]RValue) {
	out := map[string]*BufferInfo{}
	baseEnv := map[string]RValue{}
	for k, v := range cfg.Params {
		baseEnv[k] = rvInt(v, k)
		baseEnv["b."+k] = rvInt(v, "b."+k)
	}
	seedEnv := cloneREnv(baseEnv)
	uniq := 0

	parseAllocCall := func(ce *ast.CallExpr, env map[string]RValue) (isAlloc bool, unified bool, sizeExpr ast.Expr, tag string) {
		if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
			fn := sel.Sel.Name
			if fn == "alloc" {
				if len(ce.Args) < 1 {
					return false, false, nil, ""
				}
				sizeExpr = ce.Args[0]
				if len(ce.Args) >= 2 {
					if s, ok := evalStringV2(ce.Args[1], fset, cfg, env); ok {
						tag = s
					}
				}
				return true, false, sizeExpr, tag
			}
			if strings.HasPrefix(fn, "AllocateMemory") || strings.HasPrefix(fn, "AllocateUnifiedMemory") {
				unified = strings.Contains(fn, "Unified")
				if len(ce.Args) < 1 {
					return false, unified, nil, ""
				}
				sizeExpr = ce.Args[len(ce.Args)-1]
				if strings.Contains(fn, "WithTag") && len(ce.Args) >= 2 {
					sizeExpr = ce.Args[len(ce.Args)-2]
					if s, ok := evalStringV2(ce.Args[len(ce.Args)-1], fset, cfg, env); ok {
						tag = s
					}
				}
				return true, unified, sizeExpr, tag
			}
		}
		return false, false, nil, ""
	}

	recordAlloc := func(lhs ast.Expr, sizeExpr ast.Expr, tag string, unified bool, env map[string]RValue) {
		lhsKey := exprToString(fset, lhs)
		if tag == "" && cfg.BufferTags != nil {
			if t, ok := cfg.BufferTags[lhsKey]; ok {
				tag = t
			}
		}
		if tag == "" {
			tag = inferTag(benchName, lhsKey)
		}
		bytes, _ := evalIntV2(sizeExpr, fset, cfg, env)
		key := lhsKey
		if tag != "" {
			key = "__buf__::" + tag
		}
		if _, exists := out[key]; exists {
			uniq++
			key = fmt.Sprintf("%s#%d", key, uniq)
		}
		out[key] = &BufferInfo{GoVar: lhsKey, Tag: tag, BytesExpr: sizeExpr, BytesExprStr: exprToString(fset, sizeExpr), Bytes: bytes, Unified: unified}
		setExprValueV2(lhs, RValue{Kind: "ref", Ref: key, Expr: lhsKey}, fset, cfg, env)
	}

	var execStmts func(stmts []ast.Stmt, env map[string]RValue)
	execStmts = func(stmts []ast.Stmt, env map[string]RValue) {
		for _, st := range stmts {
			switch s := st.(type) {
			case *ast.AssignStmt:
				rhsVals := make([]RValue, len(s.Rhs))
				for i := range s.Rhs {
					rhsVals[i] = evalValueV2(s.Rhs[i], fset, cfg, env)
				}
				n := len(s.Lhs)
				if len(rhsVals) < n {
					n = len(rhsVals)
				}
				for i := 0; i < n; i++ {
					if ce, ok := s.Rhs[i].(*ast.CallExpr); ok {
						if isAlloc, unified, sizeExpr, tag := parseAllocCall(ce, env); isAlloc && sizeExpr != nil {
							recordAlloc(s.Lhs[i], sizeExpr, tag, unified, env)
							continue
						}
					}
					setExprValueV2(s.Lhs[i], rhsVals[i], fset, cfg, env)
				}

			case *ast.DeclStmt:
				gd, ok := s.Decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, sp := range gd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if ce, ok := vs.Values[i].(*ast.CallExpr); ok {
							if isAlloc, unified, sizeExpr, tag := parseAllocCall(ce, env); isAlloc && sizeExpr != nil {
								recordAlloc(name, sizeExpr, tag, unified, env)
								continue
							}
						}
						setExprValueV2(name, evalValueV2(vs.Values[i], fset, cfg, env), fset, cfg, env)
					}
				}

			case *ast.BlockStmt:
				execStmts(s.List, env)

			case *ast.ForStmt:
				if varName, start, end, ok := parseSimpleForV2(s, fset, cfg, env); ok {
					trip := end - start
					if trip < 0 {
						trip = 0
					}
					if trip > 4096 {
						if s.Body != nil {
							execStmts(s.Body.List, env)
						}
						continue
					}
					old, hasOld := env[varName]
					for i := start; i < end; i++ {
						env[varName] = rvInt(i, varName)
						execStmts(s.Body.List, env)
					}
					if hasOld {
						env[varName] = old
					} else {
						delete(env, varName)
					}
					continue
				}
				if s.Body != nil {
					execStmts(s.Body.List, env)
				}

			case *ast.IfStmt:
				if take, ok := evalTruthyV2(s.Cond, fset, cfg, env); ok {
					if take {
						if s.Body != nil {
							execStmts(s.Body.List, env)
						}
					} else if s.Else != nil {
						if bs, ok := s.Else.(*ast.BlockStmt); ok {
							execStmts(bs.List, env)
						}
					}
					continue
				}
				base := cloneREnv(env)
				thenEnv := cloneREnv(base)
				if s.Body != nil {
					execStmts(s.Body.List, thenEnv)
				}
				elseEnv := cloneREnv(base)
				if s.Else != nil {
					if bs, ok := s.Else.(*ast.BlockStmt); ok {
						execStmts(bs.List, elseEnv)
					}
				}
				merged := mergeREnvs(base, thenEnv, elseEnv)
				for k := range env {
					delete(env, k)
				}
				for k, v := range merged {
					env[k] = v
				}
			}
		}
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		env := cloneREnv(baseEnv)
		execStmts(fd.Body.List, env)
		if fd.Name != nil && fd.Name.Name == "initMem" {
			seedEnv = cloneREnv(env)
		}
	}

	// Also seed direct Go-var aliases from collected buffers.
	for ref, bi := range out {
		if bi != nil && bi.GoVar != "" {
			seedEnv[bi.GoVar] = RValue{Kind: "ref", Ref: ref, Expr: bi.GoVar}
		}
	}

	return out, seedEnv
}

func buildArgBindingsV2(argVal RValue, structTypes map[string][]string, buffers map[string]*BufferInfo) []ArgBinding {
	if argVal.Kind != "struct" || argVal.St == nil {
		return nil
	}
	st := argVal.St
	fieldsOrder, ok := structTypes[st.TypeName]
	if !ok {
		for k := range st.Fields {
			fieldsOrder = append(fieldsOrder, k)
		}
		sort.Strings(fieldsOrder)
	}

	var bindings []ArgBinding
	for idx, fname := range fieldsOrder {
		v, ok := st.Fields[fname]
		if !ok {
			bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: "unknown"})
			continue
		}
		switch v.Kind {
		case "ref":
			if bi, ok := buffers[v.Ref]; ok {
				bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: "buffer", Tag: bi.Tag, Expr: v.Expr})
			} else {
				bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: "buffer", Expr: v.Expr})
			}
		case "int":
			bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: "scalar", Value: v.Int})
		case "float":
			bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: "scalar_float", FloatValue: v.Float})
		default:
			bindings = append(bindings, ArgBinding{ArgIndex: idx, ArgName: fname, Kind: v.Kind, Expr: v.Expr})
		}
	}
	return bindings
}

func simulateLaunchesV2(entry *ast.FuncDecl, fset *token.FileSet, cfg *Config, benchName string, structTypes map[string][]string, kernelHandleMap map[string]string, buffers map[string]*BufferInfo, seedEnv map[string]RValue) ([]LaunchDesc, []string) {
	env := cloneREnv(seedEnv)
	if env == nil {
		env = map[string]RValue{}
	}
	for k, v := range cfg.Params {
		env[k] = rvInt(v, k)
		env["b."+k] = rvInt(v, "b."+k)
	}
	for ref, bi := range buffers {
		if bi != nil && bi.GoVar != "" {
			env[bi.GoVar] = RValue{Kind: "ref", Ref: ref, Expr: bi.GoVar}
		}
	}

	notes := []string{}
	launches := []LaunchDesc{}
	launchID := 0

	roundUp := func(total, align int64) int64 {
		if align <= 0 {
			return total
		}
		r := total % align
		if r == 0 {
			return total
		}
		return total + (align - r)
	}

	resolveArgStruct := func(argExpr ast.Expr) (RValue, bool) {
		v := evalValueV2(argExpr, fset, cfg, env)
		if v.Kind == "struct" {
			return v, true
		}
		return RValue{}, false
	}

	handleLaunch := func(kernelName string, globalArr [3]int64, localArr [3]int64, argExpr ast.Expr) {
		argVal, ok := resolveArgStruct(argExpr)
		if !ok {
			notes = append(notes, fmt.Sprintf("args not resolved for %s (expr=%s)", kernelName, exprToString(fset, argExpr)))
			return
		}
		argBindings := buildArgBindingsV2(argVal, structTypes, buffers)
		launches = append(launches, LaunchDesc{ID: launchID, Kernel: kernelName, GlobalSize: globalArr, LocalSize: localArr, ArgBindings: argBindings, ScalarMap: makeScalarMap(argBindings)})
		launchID++
	}

	handleLaunchKernelCall := func(ce *ast.CallExpr) {
		if len(ce.Args) < 5 {
			notes = append(notes, fmt.Sprintf("LaunchKernel skipped (<5 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[1])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
		}
		global := evalValueV2(ce.Args[2], fset, cfg, env)
		local := evalValueV2(ce.Args[3], fset, cfg, env)
		if global.Kind != "arr3" || local.Kind != "arr3" {
			notes = append(notes, fmt.Sprintf("LaunchKernel sizes not constant for %s", kernelName))
			return
		}
		handleLaunch(kernelName, global.Arr3, local.Arr3, ce.Args[4])
	}

	handleLaunch1DCall := func(ce *ast.CallExpr) {
		if len(ce.Args) < 4 {
			notes = append(notes, fmt.Sprintf("launch1D skipped (<4 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[0])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
		}
		total, okT := evalIntV2(ce.Args[1], fset, cfg, env)
		localX, okL := evalIntV2(ce.Args[2], fset, cfg, env)
		if !okT || !okL {
			notes = append(notes, fmt.Sprintf("launch1D sizes not constant for %s", kernelName))
			return
		}
		if localX <= 0 {
			localX = 1
		}
		handleLaunch(kernelName, [3]int64{roundUp(total, localX), 1, 1}, [3]int64{localX, 1, 1}, ce.Args[3])
	}

	handleLaunch2DCall := func(ce *ast.CallExpr) {
		if len(ce.Args) < 6 {
			notes = append(notes, fmt.Sprintf("launch2D skipped (<6 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[0])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
		}
		x, okX := evalIntV2(ce.Args[1], fset, cfg, env)
		y, okY := evalIntV2(ce.Args[2], fset, cfg, env)
		lx, okLX := evalIntV2(ce.Args[3], fset, cfg, env)
		ly, okLY := evalIntV2(ce.Args[4], fset, cfg, env)
		if !okX || !okY || !okLX || !okLY {
			notes = append(notes, fmt.Sprintf("launch2D sizes not constant for %s", kernelName))
			return
		}
		if lx <= 0 {
			lx = 1
		}
		if ly <= 0 {
			ly = 1
		}
		handleLaunch(kernelName, [3]int64{roundUp(x, lx), roundUp(y, ly), 1}, [3]int64{lx, ly, 1}, ce.Args[5])
	}

	var execStmts func(stmts []ast.Stmt)
	execStmts = func(stmts []ast.Stmt) {
		for _, st := range stmts {
			switch s := st.(type) {
			case *ast.AssignStmt:
				rhsVals := make([]RValue, len(s.Rhs))
				for i := range s.Rhs {
					rhsVals[i] = evalValueV2(s.Rhs[i], fset, cfg, env)
				}
				n := len(s.Lhs)
				if len(rhsVals) < n {
					n = len(rhsVals)
				}
				for i := 0; i < n; i++ {
					setExprValueV2(s.Lhs[i], rhsVals[i], fset, cfg, env)
				}

			case *ast.DeclStmt:
				gd, ok := s.Decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, sp := range gd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						setExprValueV2(name, evalValueV2(vs.Values[i], fset, cfg, env), fset, cfg, env)
					}
				}

			case *ast.ExprStmt:
				ce, ok := s.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "LaunchKernel":
						handleLaunchKernelCall(ce)
					case "launch1D":
						handleLaunch1DCall(ce)
					case "launch2D":
						handleLaunch2DCall(ce)
					}
				}

			case *ast.ForStmt:
				if varName, start, end, ok := parseSimpleForV2(s, fset, cfg, env); ok {
					trip := end - start
					if trip < 0 {
						trip = 0
					}
					if trip > 4096 {
						notes = append(notes, fmt.Sprintf("loop %s not unrolled (trip=%d > 4096); executing body once", varName, trip))
						if s.Body != nil {
							execStmts(s.Body.List)
						}
						continue
					}
					old, hasOld := env[varName]
					for i := start; i < end; i++ {
						env[varName] = rvInt(i, varName)
						execStmts(s.Body.List)
					}
					if hasOld {
						env[varName] = old
					} else {
						delete(env, varName)
					}
					continue
				}
				notes = append(notes, "unrecognized for-loop shape; executing body once")
				if s.Body != nil {
					execStmts(s.Body.List)
				}

			case *ast.BlockStmt:
				execStmts(s.List)

			case *ast.IfStmt:
				if take, ok := evalTruthyV2(s.Cond, fset, cfg, env); ok {
					if take {
						if s.Body != nil {
							execStmts(s.Body.List)
						}
					} else if s.Else != nil {
						if bs, ok := s.Else.(*ast.BlockStmt); ok {
							execStmts(bs.List)
						}
					}
					continue
				}
				base := cloneREnv(env)
				thenEnv := cloneREnv(base)
				oldEnv := env
				env = thenEnv
				if s.Body != nil {
					execStmts(s.Body.List)
				}
				thenEnv = cloneREnv(env)
				elseEnv := cloneREnv(base)
				env = elseEnv
				if s.Else != nil {
					if bs, ok := s.Else.(*ast.BlockStmt); ok {
						execStmts(bs.List)
					}
				}
				elseEnv = cloneREnv(env)
				env = mergeREnvs(base, thenEnv, elseEnv)
				_ = oldEnv
			}
		}
	}

	execStmts(entry.Body.List)
	if len(launches) == 0 {
		notes = append(notes, "warning: no LaunchKernel/launch1D found in entry function body; check --entry or benchmark.go format")
	}
	return launches, notes
}
