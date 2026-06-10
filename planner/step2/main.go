package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Benchmark  string            `json:"benchmark"`
	Params     map[string]int64  `json:"params"`
	PageBytes  int64             `json:"page_bytes,omitempty"`
	BufferTags map[string]string `json:"buffer_tags,omitempty"`
}

type Step1 struct {
	Tool    string        `json:"tool"`
	Version int           `json:"version"`
	Input   string        `json:"input"`
	Kernels []Step1Kernel `json:"kernels"`
}

type Step1Kernel struct {
	Name      string          `json:"name"`
	Args      []Step1Arg      `json:"args"`
	MemAccess []Step1MemAcc   `json:"mem_accesses"`
	Summary   json.RawMessage `json:"summary,omitempty"`
}

type Step1Arg struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsPointer bool   `json:"is_pointer"`
	AddrSpace int    `json:"addrspace"`
	ElemBytes int    `json:"elem_bytes"`
}

type Step1MemAcc struct {
	Kind         string          `json:"kind"` // load/store
	Inst         string          `json:"inst,omitempty"`
	IsGlobal     bool            `json:"is_global"`
	AddrSpace    int             `json:"addrspace"`
	ElemBytes    int             `json:"elem_bytes"`
	Buffer       string          `json:"buffer"`
	BufferArgIdx int             `json:"buffer_arg_index"`
	AddrSCEV     string          `json:"addr_scev"`
	AddRec       json.RawMessage `json:"addrec,omitempty"`
}

type CLKernelSig struct {
	Name string
	Args []string // argument names in order
}

type BufferDesc struct {
	Tag       string `json:"tag"`
	GoVar     string `json:"go_var"`
	BytesExpr string `json:"bytes_expr"`
	Bytes     int64  `json:"bytes"`
	Pages     int64  `json:"pages"`
	Unified   bool   `json:"unified"`
}

type KernelArgDesc struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	IRName    string `json:"ir_name,omitempty"`
	Type      string `json:"type,omitempty"`
	IsPointer bool   `json:"is_pointer"`
	AddrSpace int    `json:"addrspace,omitempty"`
	ElemBytes int    `json:"elem_bytes,omitempty"`
}

type KernelDesc struct {
	Name    string          `json:"name"`
	Args    []KernelArgDesc `json:"args"`
	Summary json.RawMessage `json:"summary,omitempty"`
}

type ArgBinding struct {
	ArgIndex   int     `json:"arg_index"`
	ArgName    string  `json:"arg_name"`
	Kind       string  `json:"kind"` // buffer|scalar|scalar_float|unknown
	Tag        string  `json:"tag,omitempty"`
	Value      int64   `json:"value,omitempty"`
	FloatValue float64 `json:"float_value,omitempty"`
	Expr       string  `json:"expr,omitempty"`
}

type LaunchDesc struct {
	ID              int          `json:"id"`
	Kernel          string       `json:"kernel"`
	GlobalSize      [3]int64     `json:"global_size"`
	LocalSize       [3]int64     `json:"local_size"`
	NumWG           [3]int64     `json:"num_wg,omitempty"`
	NumWGTotal      int64        `json:"num_wg_total,omitempty"`
	LaunchSignature string       `json:"launch_signature,omitempty"`
	ArgBindings     []ArgBinding `json:"arg_bindings"`
	// ScalarMap provides a stable name for integer scalar args that matches Step1 output.
	ScalarMap      map[string]int64   `json:"scalar_map,omitempty"`
	FloatScalarMap map[string]float64 `json:"float_scalar_map,omitempty"`
	Reads          []string           `json:"reads"`
	Writes         []string           `json:"writes"`
	BufferRoles    map[string]string  `json:"buffer_roles,omitempty"`
}

type Edge struct {
	From   int    `json:"from"`
	To     int    `json:"to"`
	Buffer string `json:"buffer"`
	Type   string `json:"type"` // RAW
}

type Graph struct {
	Edges []Edge `json:"edges"`
}

type GlobalManifest struct {
	Tool          string            `json:"tool"`
	Version       int               `json:"version"`
	Benchmark     string            `json:"benchmark"`
	Package       string            `json:"package"`
	SourceFiles   []string          `json:"source_files"`
	SourceDigests map[string]string `json:"source_digests,omitempty"`
	Params        map[string]int64  `json:"params"`
	PageBytes     int64             `json:"page_bytes"`
	Buffers       []BufferDesc      `json:"buffers"`
	Kernels       []KernelDesc      `json:"kernels"`
	Launches      []LaunchDesc      `json:"launches"`
	Graph         Graph             `json:"graph"`
	Notes         []string          `json:"notes,omitempty"`
}

func main() {
	var (
		step1Path = flag.String("kernels-json", "", "Step1 kernels.json (from mgvm-kernel-analyzer v3)")
		benchGo   = flag.String("benchmark-go", "", "benchmark.go path (host code)")
		kernelsCL = flag.String("kernels-cl", "", "OpenCL source (.cl) path (optional, for arg names)")
		cfgPath   = flag.String("config", "", "kernels.config.json path (params like NX/NY)")
		outPath   = flag.String("output", "global.json", "output global.json path")
		entryName = flag.String("entry", "", "entry function to simulate (default: exec, then Run, then first LaunchKernel-containing func)")
	)
	flag.Parse()

	if *step1Path == "" || *benchGo == "" || *cfgPath == "" {
		fmt.Fprintf(os.Stderr, "error: --kernels-json, --benchmark-go, --config are required\n")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := readConfig(*cfgPath)
	if err != nil {
		die(err)
	}
	if cfg.PageBytes == 0 {
		cfg.PageBytes = 4096
	}

	step1, err := readStep1(*step1Path)
	if err != nil {
		die(err)
	}

	var clSigs map[string]CLKernelSig
	if *kernelsCL != "" {
		clSigs, err = parseCL(*kernelsCL)
		if err != nil {
			// Keep going; arg names are optional.
			fmt.Fprintf(os.Stderr, "warning: failed to parse kernels-cl: %v\n", err)
		}
	}

	astFile, fset, err := parseGoFile(*benchGo)
	if err != nil {
		die(err)
	}
	pkg := astFile.Name.Name
	benchName := cfg.Benchmark
	if benchName == "" {
		benchName = pkg
	}

	structTypes := collectStructTypes(astFile, fset)
	kernelHandleMap := collectKernelHandles(astFile, fset)
	buffers, seedEnv := collectBuffersV2(astFile, fset, cfg, benchName)

	entryFunc := pickEntryFunc(astFile, fset, *entryName)
	if entryFunc == nil {
		die(fmt.Errorf("cannot find entry function (searched: %q, exec, Run, or any func with LaunchKernel)", *entryName))
	}

	launches, notes := simulateLaunchesV2(entryFunc, fset, cfg, benchName, structTypes, kernelHandleMap, buffers, seedEnv)

	kernels := buildKernelDescs(step1, clSigs)

	// Derive per-launch read/write sets using step1 mem_accesses
	kernelMap := map[string]Step1Kernel{}
	for _, k := range step1.Kernels {
		kernelMap[k.Name] = k
	}
	for i := range launches {
		if kmeta, ok := kernelMap[launches[i].Kernel]; ok {
			reads, writes := deriveRW(kmeta, launches[i].ArgBindings)
			launches[i].Reads = sortedKeys(reads)
			launches[i].Writes = sortedKeys(writes)
		} else {
			notes = append(notes, fmt.Sprintf("warning: launch %d kernel %q not found in kernels-json", launches[i].ID, launches[i].Kernel))
		}
		launches[i].NumWG = computeNumWG(launches[i].GlobalSize, launches[i].LocalSize)
		launches[i].NumWGTotal = launches[i].NumWG[0] * launches[i].NumWG[1] * launches[i].NumWG[2]
		launches[i].FloatScalarMap = makeFloatScalarMap(launches[i].ArgBindings)
		launches[i].BufferRoles = deriveBufferRoles(launches[i].Reads, launches[i].Writes)
		launches[i].LaunchSignature = makeLaunchSignature(launches[i])
	}

	graph := buildGraph(launches)
	notes = append(notes, detectDuplicateTagNotes(buffersToList(buffers, cfg.PageBytes))...)

	sourceFiles := []string{filepath.Clean(*benchGo), filepath.Clean(*step1Path), filepath.Clean(*cfgPath)}
	if *kernelsCL != "" {
		sourceFiles = append(sourceFiles, filepath.Clean(*kernelsCL))
	}
	sort.Strings(sourceFiles)
	notes = append(notes,
		"freshness: regenerate this global json whenever benchmark.go, kernels.cl, kernels.json, or kernels.config.json changes",
	)

	manifest := GlobalManifest{
		Tool:          "mgvm-host-analyzer",
		Version:       4,
		Benchmark:     benchName,
		Package:       pkg,
		SourceFiles:   sourceFiles,
		SourceDigests: computeSourceDigests(sourceFiles),
		Params:        cfg.Params,
		PageBytes:     cfg.PageBytes,
		Buffers:       buffersToList(buffers, cfg.PageBytes),
		Kernels:       kernels,
		Launches:      launches,
		Graph:         graph,
		Notes:         notes,
	}

	outBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		die(err)
	}
	if err := ioutil.WriteFile(*outPath, outBytes, 0644); err != nil {
		die(err)
	}
	fmt.Printf("wrote %s (kernels=%d buffers=%d launches=%d edges=%d)\n",
		*outPath, len(manifest.Kernels), len(manifest.Buffers), len(manifest.Launches), len(manifest.Graph.Edges))
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func computeSourceDigests(paths []string) map[string]string {
	out := make(map[string]string)
	for _, path := range paths {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}
		h := sha256.Sum256(b)
		out[path] = hex.EncodeToString(h[:])
	}
	return out
}

func readConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.Params == nil {
		cfg.Params = map[string]int64{}
	}
	if cfg.BufferTags == nil {
		cfg.BufferTags = map[string]string{}
	}
	return &cfg, nil
}

func readStep1(path string) (*Step1, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Step1
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func parseGoFile(path string) (*ast.File, *token.FileSet, error) {
	src, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return f, fset, nil
}

func parseCL(path string) (map[string]CLKernelSig, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(b)

	// Very lightweight parser; good enough for PolyBench-style kernels.
	re := regexp.MustCompile(`(?s)__kernel\s+void\s+([A-Za-z_]\w*)\s*\((.*?)\)\s*\{`)
	matches := re.FindAllStringSubmatch(src, -1)
	out := map[string]CLKernelSig{}
	for _, m := range matches {
		name := m[1]
		argList := m[2]
		args := splitArgs(argList)
		out[name] = CLKernelSig{Name: name, Args: args}
	}
	return out, nil
}

func splitArgs(argList string) []string {
	parts := strings.Split(argList, ",")
	var args []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Remove any attribute blocks like __attribute__((...))
		p = regexp.MustCompile(`__attribute__\s*\(\(.*?\)\)`).ReplaceAllString(p, "")
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		name := fields[len(fields)-1]
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, "*")
		name = strings.TrimPrefix(name, "&")
		name = strings.Trim(name, ")(")
		// In case of "int nx" it's fine; for "*A" handled above.
		args = append(args, name)
	}
	return args
}

func collectStructTypes(f *ast.File, fset *token.FileSet) map[string][]string {
	out := map[string][]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			var fields []string
			for _, field := range st.Fields.List {
				// field.Names may be nil for embedded fields; ignore those.
				for _, n := range field.Names {
					fields = append(fields, n.Name)
				}
			}
			out[ts.Name.Name] = fields
		}
	}
	return out
}

func exprToString(fset *token.FileSet, e ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, e)
	return buf.String()
}

func collectKernelHandles(f *ast.File, fset *token.FileSet) map[string]string {
	m := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		lhs := exprToString(fset, as.Lhs[0])

		ce, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}

		// Case 1: selector-based loaders (e.g., kernels.LoadProgramFromFile(...,"kernel"))
		if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
			fn := sel.Sel.Name
			if strings.Contains(fn, "LoadProgram") || fn == "GetKernel" {
				for _, arg := range ce.Args {
					if s, ok := arg.(*ast.BasicLit); ok && s.Kind == token.STRING {
						name, _ := strconv.Unquote(s.Value)
						m[lhs] = name
						return true
					}
				}
			}
		}

		// Case 2: helper loader (e.g., mustLoadKernel(hsacoBytes, "kernel"))
		if id, ok := ce.Fun.(*ast.Ident); ok {
			fn := id.Name
			if fn == "mustLoadKernel" || strings.Contains(strings.ToLower(fn), "loadkernel") {
				if len(ce.Args) > 0 {
					if s, ok := ce.Args[len(ce.Args)-1].(*ast.BasicLit); ok && s.Kind == token.STRING {
						name, _ := strconv.Unquote(s.Value)
						m[lhs] = name
						return true
					}
				}
			}
		}

		return true
	})
	return m
}

type BufferInfo struct {
	GoVar        string
	Tag          string
	BytesExpr    ast.Expr
	BytesExprStr string
	Bytes        int64
	Unified      bool
}

func collectBuffers(f *ast.File, fset *token.FileSet, cfg *Config, benchName string) map[string]*BufferInfo {
	out := map[string]*BufferInfo{}

	parseAllocCall := func(ce *ast.CallExpr) (isAlloc bool, unified bool, sizeExpr ast.Expr, tag string) {
		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return false, false, nil, ""
		}
		fn := sel.Sel.Name

		// Helper wrapper (used by transformer benchmark): b.alloc(bytes, "tag")
		if fn == "alloc" {
			if len(ce.Args) < 1 {
				return false, false, nil, ""
			}
			sizeExpr = ce.Args[0]
			if len(ce.Args) >= 2 {
				if s, ok := ce.Args[1].(*ast.BasicLit); ok && s.Kind == token.STRING {
					tag, _ = strconv.Unquote(s.Value)
				}
			}
			// Whether it is unified depends on runtime flags; treat as non-unified here.
			return true, false, sizeExpr, tag
		}

		// Driver allocators:
		//   AllocateMemory(ctx, bytes)
		//   AllocateMemoryWithTag(ctx, bytes, "tag")
		//   AllocateUnifiedMemory(ctx, bytes)
		//   AllocateUnifiedMemoryWithTag(ctx, bytes, "tag")
		if !(strings.HasPrefix(fn, "AllocateMemory") || strings.HasPrefix(fn, "AllocateUnifiedMemory")) {
			return false, false, nil, ""
		}
		unified = strings.Contains(fn, "Unified")
		if len(ce.Args) < 1 {
			return false, unified, nil, ""
		}

		sizeExpr = ce.Args[len(ce.Args)-1]
		if strings.Contains(fn, "WithTag") && len(ce.Args) >= 2 {
			sizeExpr = ce.Args[len(ce.Args)-2]
			if s, ok := ce.Args[len(ce.Args)-1].(*ast.BasicLit); ok && s.Kind == token.STRING {
				tag, _ = strconv.Unquote(s.Value)
			}
		}
		return true, unified, sizeExpr, tag
	}

	record := func(goVar string, sizeExpr ast.Expr, tag string, unified bool, env map[string]Value) {
		if tag == "" && cfg.BufferTags != nil {
			if t, ok := cfg.BufferTags[goVar]; ok {
				tag = t
			}
		}
		if tag == "" {
			tag = inferTag(benchName, goVar)
		}

		bytes, _ := evalInt(sizeExpr, fset, cfg, env)
		out[goVar] = &BufferInfo{
			GoVar:        goVar,
			Tag:          tag,
			Bytes:        bytes,
			BytesExpr:    sizeExpr,
			BytesExprStr: exprToString(fset, sizeExpr),
			Unified:      unified,
		}

		// Seed env with a ref so later statements can reference this pointer.
		env[goVar] = Value{Kind: "ref", Ref: goVar, Expr: goVar}
	}

	// Sequential interpretation inside each function body so allocation sizes like
	// "bytesX" can be resolved from earlier assignments.
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}

		env := map[string]Value{}
		for k, v := range cfg.Params {
			env[k] = Value{Kind: "int", Int: v}
			env["b."+k] = Value{Kind: "int", Int: v}
		}

		var execStmts func(stmts []ast.Stmt)
		execStmts = func(stmts []ast.Stmt) {
			for _, st := range stmts {
				switch s := st.(type) {

				case *ast.AssignStmt:
					// Evaluate RHS first (important for swap-style assignments).
					rhsVals := make([]Value, len(s.Rhs))
					for i := range s.Rhs {
						rhsVals[i] = evalValue(s.Rhs[i], fset, cfg, env)
					}

					n := len(s.Lhs)
					if len(rhsVals) < n {
						n = len(rhsVals)
					}
					for i := 0; i < n; i++ {
						lhsKey := exprToString(fset, s.Lhs[i])
						rhsExpr := s.Rhs[i]

						if ce, ok := rhsExpr.(*ast.CallExpr); ok {
							if isAlloc, unified, sizeExpr, tag := parseAllocCall(ce); isAlloc && sizeExpr != nil {
								record(lhsKey, sizeExpr, tag, unified, env)
								continue
							}
						}

						env[lhsKey] = rhsVals[i]
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
							rhsExpr := vs.Values[i]

							if ce, ok := rhsExpr.(*ast.CallExpr); ok {
								if isAlloc, unified, sizeExpr, tag := parseAllocCall(ce); isAlloc && sizeExpr != nil {
									record(name.Name, sizeExpr, tag, unified, env)
									continue
								}
							}

							env[name.Name] = evalValue(rhsExpr, fset, cfg, env)
						}
					}

				case *ast.BlockStmt:
					execStmts(s.List)

				case *ast.ForStmt:
					// Avoid unrolling (could be huge); execute once so we don't miss trivial allocs.
					if s.Body != nil {
						execStmts(s.Body.List)
					}

				case *ast.IfStmt:
					if take, ok := evalTruthy(s.Cond, fset, cfg, env); ok {
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
					baseEnv := cloneEnv(env)
					thenEnv := cloneEnv(baseEnv)
					env = thenEnv
					if s.Body != nil {
						execStmts(s.Body.List)
					}
					thenEnv = cloneEnv(env)
					elseEnv := cloneEnv(baseEnv)
					env = elseEnv
					if s.Else != nil {
						if bs, ok := s.Else.(*ast.BlockStmt); ok {
							execStmts(bs.List)
						}
					}
					elseEnv = cloneEnv(env)
					env = mergeEnvs(baseEnv, thenEnv, elseEnv)
				}
			}
		}

		execStmts(fd.Body.List)
	}

	return out
}

func inferTag(benchName, goVar string) string {
	// Common mgpusim style: b.dA -> bench.A
	// Strip common prefixes.
	short := goVar
	short = strings.TrimPrefix(short, "b.")
	short = strings.TrimPrefix(short, "b.d")
	short = strings.TrimPrefix(short, "d")
	short = strings.TrimPrefix(short, "&")
	short = strings.TrimPrefix(short, "*")
	short = strings.TrimSpace(short)
	// If still contains '.', take last selector.
	if idx := strings.LastIndex(short, "."); idx >= 0 {
		short = short[idx+1:]
	}
	// If short begins with 'd' (like dA), drop it.
	if strings.HasPrefix(short, "d") && len(short) >= 2 && strings.ToUpper(short[1:2]) == short[1:2] {
		short = short[1:]
	}
	return fmt.Sprintf("%s.%s", benchName, short)
}

func pickEntryFunc(f *ast.File, fset *token.FileSet, want string) *ast.FuncDecl {
	// 1) exact requested name
	if want != "" {
		if fd := findFuncByName(f, want); fd != nil {
			return fd
		}
	}
	// 2) exec
	if fd := findFuncByName(f, "exec"); fd != nil {
		return fd
	}
	// 3) Run
	if fd := findFuncByName(f, "Run"); fd != nil {
		return fd
	}
	// 4) first function that contains LaunchKernel
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		if funcContainsCall(fd.Body, "LaunchKernel") {
			return fd
		}
	}
	return nil
}

func findFuncByName(f *ast.File, name string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name != nil && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

func funcContainsCall(body *ast.BlockStmt, selName string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel != nil && sel.Sel.Name == selName {
			found = true
			return false
		}
		return true
	})
	return found
}

// Value is an interpreter value for a small subset of Go expressions.
type Value struct {
	Kind string // "int", "arr3", "ref", "struct", "unknown"
	Int  int64
	Arr3 [3]int64
	Ref  string
	St   *StructVal
	Expr string
}

type StructVal struct {
	TypeName string
	Fields   map[string]Value // by field name
}

func simulateLaunches(entry *ast.FuncDecl, fset *token.FileSet, cfg *Config, benchName string, structTypes map[string][]string, kernelHandleMap map[string]string, buffers map[string]*BufferInfo) ([]LaunchDesc, []string) {
	env := map[string]Value{}
	for k, v := range cfg.Params {
		env[k] = Value{Kind: "int", Int: v}
		env["b."+k] = Value{Kind: "int", Int: v}
	}
	for goVar := range buffers {
		env[goVar] = Value{Kind: "ref", Ref: goVar, Expr: goVar}
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

	parseSimpleFor := func(fs *ast.ForStmt) (varName string, start, end int64, ok bool) {
		// Parse: for i := 0; i < N; i++ { ... }
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
		start, ok = evalInt(as.Rhs[0], fset, cfg, env)
		if !ok {
			return "", 0, 0, false
		}

		// cond: i < N or i <= N
		be, ok := fs.Cond.(*ast.BinaryExpr)
		if !ok {
			return "", 0, 0, false
		}
		lhsID, ok := be.X.(*ast.Ident)
		if !ok || lhsID.Name != varName {
			return "", 0, 0, false
		}
		end, ok = evalInt(be.Y, fset, cfg, env)
		if !ok {
			return "", 0, 0, false
		}
		if be.Op == token.LEQ {
			end = end + 1
		} else if be.Op != token.LSS {
			return "", 0, 0, false
		}

		// post: i++
		if inc, ok := fs.Post.(*ast.IncDecStmt); ok {
			if pid, ok := inc.X.(*ast.Ident); ok && pid.Name == varName && inc.Tok == token.INC {
				return varName, start, end, true
			}
		}
		return "", 0, 0, false
	}

	resolveArgStruct := func(argExpr ast.Expr) (Value, bool) {
		v := evalValue(argExpr, fset, cfg, env)
		if v.Kind == "struct" {
			return v, true
		}
		return Value{}, false
	}

	handleLaunch := func(kernelName string, globalArr [3]int64, localArr [3]int64, argExpr ast.Expr) {
		argVal, ok := resolveArgStruct(argExpr)
		if !ok {
			notes = append(notes, fmt.Sprintf("args not resolved for %s (expr=%s)", kernelName, exprToString(fset, argExpr)))
			return
		}

		argBindings := buildArgBindings(argVal, structTypes, buffers, fset, cfg, env)

		desc := LaunchDesc{
			ID:          launchID,
			Kernel:      kernelName,
			GlobalSize:  globalArr,
			LocalSize:   localArr,
			ArgBindings: argBindings,
			ScalarMap:   makeScalarMap(argBindings),
		}
		launchID++
		launches = append(launches, desc)
	}

	handleLaunchKernelCall := func(ce *ast.CallExpr) {
		// Signature: LaunchKernel(ctx, kernel, global, local, &Args)
		if len(ce.Args) < 5 {
			notes = append(notes, fmt.Sprintf("LaunchKernel skipped (<5 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[1])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
			notes = append(notes, fmt.Sprintf("kernel handle %q not resolved; using %q", kernelKey, kernelName))
		}

		global := evalValue(ce.Args[2], fset, cfg, env)
		local := evalValue(ce.Args[3], fset, cfg, env)
		if global.Kind != "arr3" || local.Kind != "arr3" {
			notes = append(notes, fmt.Sprintf("LaunchKernel sizes not constant for %s", kernelName))
			return
		}

		handleLaunch(kernelName, global.Arr3, local.Arr3, ce.Args[4])
	}

	handleLaunch1DCall := func(ce *ast.CallExpr) {
		// Signature: launch1D(kernelHandle, total, local, &Args{...})
		if len(ce.Args) < 4 {
			notes = append(notes, fmt.Sprintf("launch1D skipped (<4 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[0])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
			notes = append(notes, fmt.Sprintf("kernel handle %q not resolved; using %q", kernelKey, kernelName))
		}

		total, okT := evalInt(ce.Args[1], fset, cfg, env)
		localX, okL := evalInt(ce.Args[2], fset, cfg, env)
		if !okT || !okL {
			notes = append(notes, fmt.Sprintf("launch1D sizes not constant for %s", kernelName))
			return
		}
		if localX <= 0 {
			localX = 1
		}
		globalX := roundUp(total, localX)

		globalArr := [3]int64{globalX, 1, 1}
		localArr := [3]int64{localX, 1, 1}

		handleLaunch(kernelName, globalArr, localArr, ce.Args[3])
	}

	handleLaunch2DCall := func(ce *ast.CallExpr) {
		// Signature: launch2D(kernelHandle, x, y, localX, localY, &Args{...})
		if len(ce.Args) < 6 {
			notes = append(notes, fmt.Sprintf("launch2D skipped (<6 args): %s", exprToString(fset, ce)))
			return
		}
		kernelKey := exprToString(fset, ce.Args[0])
		kernelName, ok := kernelHandleMap[kernelKey]
		if !ok {
			kernelName = kernelKey
			notes = append(notes, fmt.Sprintf("kernel handle %q not resolved; using %q", kernelKey, kernelName))
		}

		x, okX := evalInt(ce.Args[1], fset, cfg, env)
		y, okY := evalInt(ce.Args[2], fset, cfg, env)
		localX, okLX := evalInt(ce.Args[3], fset, cfg, env)
		localY, okLY := evalInt(ce.Args[4], fset, cfg, env)
		if !okX || !okY || !okLX || !okLY {
			notes = append(notes, fmt.Sprintf("launch2D sizes not constant for %s", kernelName))
			return
		}
		if localX <= 0 {
			localX = 1
		}
		if localY <= 0 {
			localY = 1
		}
		globalX := roundUp(x, localX)
		globalY := roundUp(y, localY)

		globalArr := [3]int64{globalX, globalY, 1}
		localArr := [3]int64{localX, localY, 1}

		handleLaunch(kernelName, globalArr, localArr, ce.Args[5])
	}

	var execStmts func(stmts []ast.Stmt)
	execStmts = func(stmts []ast.Stmt) {
		for _, st := range stmts {
			switch s := st.(type) {
			case *ast.AssignStmt:
				// Evaluate RHS first (important for swaps).
				rhsVals := make([]Value, len(s.Rhs))
				for i := range s.Rhs {
					rhsVals[i] = evalValue(s.Rhs[i], fset, cfg, env)
				}
				n := len(s.Lhs)
				if len(rhsVals) < n {
					n = len(rhsVals)
				}
				for i := 0; i < n; i++ {
					lhsKey := exprToString(fset, s.Lhs[i])
					env[lhsKey] = rhsVals[i]
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
						env[name.Name] = evalValue(vs.Values[i], fset, cfg, env)
					}
				}

			case *ast.ExprStmt:
				ce, ok := s.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				sel, ok := ce.Fun.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				fn := sel.Sel.Name
				if fn == "LaunchKernel" {
					handleLaunchKernelCall(ce)
				} else if fn == "launch1D" {
					handleLaunch1DCall(ce)
				} else if fn == "launch2D" {
					handleLaunch2DCall(ce)
				}

			case *ast.ForStmt:
				// Try to unroll small constant-bounded for-loops (e.g., for l := 0; l < b.Layers; l++).
				if varName, start, end, ok := parseSimpleFor(s); ok {
					trip := end - start
					if trip < 0 {
						trip = 0
					}
					if trip > 1024 {
						notes = append(notes, fmt.Sprintf("loop %s not unrolled (trip=%d > 1024); executing body once", varName, trip))
						if s.Body != nil {
							execStmts(s.Body.List)
						}
						continue
					}

					old, hasOld := env[varName]
					for i := start; i < end; i++ {
						env[varName] = Value{Kind: "int", Int: i}
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
				if take, ok := evalTruthy(s.Cond, fset, cfg, env); ok {
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
				baseEnv := cloneEnv(env)
				thenEnv := cloneEnv(baseEnv)
				env = thenEnv
				if s.Body != nil {
					execStmts(s.Body.List)
				}
				thenEnv = cloneEnv(env)
				elseEnv := cloneEnv(baseEnv)
				env = elseEnv
				if s.Else != nil {
					if bs, ok := s.Else.(*ast.BlockStmt); ok {
						execStmts(bs.List)
					}
				}
				elseEnv = cloneEnv(env)
				env = mergeEnvs(baseEnv, thenEnv, elseEnv)
			}
		}
	}

	execStmts(entry.Body.List)

	if len(launches) == 0 {
		notes = append(notes, "warning: no LaunchKernel/launch1D found in entry function body; check --entry or benchmark.go format")
	}
	return launches, notes
}

func makeScalarMap(bindings []ArgBinding) map[string]int64 {
	var out map[string]int64
	for _, b := range bindings {
		if b.Kind != "scalar" {
			continue
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[fmt.Sprintf("param_arg%d", b.ArgIndex)] = b.Value
	}
	return out
}

func makeFloatScalarMap(bindings []ArgBinding) map[string]float64 {
	var out map[string]float64
	for _, b := range bindings {
		if b.Kind != "scalar_float" {
			continue
		}
		if out == nil {
			out = map[string]float64{}
		}
		out[fmt.Sprintf("param_arg%d", b.ArgIndex)] = b.FloatValue
	}
	return out
}

func computeNumWG(global, local [3]int64) [3]int64 {
	out := [3]int64{1, 1, 1}
	for i := 0; i < 3; i++ {
		g := global[i]
		l := local[i]
		if g <= 0 {
			g = 1
		}
		if l <= 0 {
			l = 1
		}
		out[i] = (g + l - 1) / l
		if out[i] <= 0 {
			out[i] = 1
		}
	}
	return out
}

func deriveBufferRoles(reads, writes []string) map[string]string {
	role := map[string]string{}
	for _, t := range reads {
		role[t] = "read_only"
	}
	for _, t := range writes {
		if prev, ok := role[t]; ok && prev == "read_only" {
			role[t] = "read_write"
		} else {
			role[t] = "write_only"
		}
	}
	if len(role) == 0 {
		return nil
	}
	return role
}

func makeLaunchSignature(l LaunchDesc) string {
	parts := []string{l.Kernel}
	parts = append(parts, fmt.Sprintf("g=%d,%d,%d", l.GlobalSize[0], l.GlobalSize[1], l.GlobalSize[2]))
	parts = append(parts, fmt.Sprintf("l=%d,%d,%d", l.LocalSize[0], l.LocalSize[1], l.LocalSize[2]))
	if len(l.ScalarMap) > 0 {
		keys := make([]string, 0, len(l.ScalarMap))
		for k := range l.ScalarMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		vals := make([]string, 0, len(keys))
		for _, k := range keys {
			vals = append(vals, fmt.Sprintf("%s=%d", k, l.ScalarMap[k]))
		}
		parts = append(parts, "si="+strings.Join(vals, ";"))
	}
	if len(l.FloatScalarMap) > 0 {
		keys := make([]string, 0, len(l.FloatScalarMap))
		for k := range l.FloatScalarMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		vals := make([]string, 0, len(keys))
		for _, k := range keys {
			vals = append(vals, fmt.Sprintf("%s=%g", k, l.FloatScalarMap[k]))
		}
		parts = append(parts, "sf="+strings.Join(vals, ";"))
	}
	if len(l.Reads) > 0 {
		parts = append(parts, "r="+strings.Join(l.Reads, ","))
	}
	if len(l.Writes) > 0 {
		parts = append(parts, "w="+strings.Join(l.Writes, ","))
	}
	return strings.Join(parts, "|")
}

func detectDuplicateTagNotes(buffers []BufferDesc) []string {
	count := map[string]int{}
	for _, b := range buffers {
		if b.Tag != "" {
			count[b.Tag]++
		}
	}
	var notes []string
	var dups []string
	for tag, n := range count {
		if n > 1 {
			dups = append(dups, fmt.Sprintf("%s(x%d)", tag, n))
		}
	}
	sort.Strings(dups)
	if len(dups) > 0 {
		notes = append(notes, "warning: duplicate buffer tags detected: "+strings.Join(dups, ", "))
	}
	return notes
}

func buildArgBindings(argVal Value,
	structTypes map[string][]string,
	buffers map[string]*BufferInfo,
	fset *token.FileSet,
	cfg *Config,
	env map[string]Value,
) []ArgBinding {
	if argVal.Kind != "struct" || argVal.St == nil {
		return nil
	}
	st := argVal.St
	fieldsOrder, ok := structTypes[st.TypeName]
	if !ok {
		// Fallback: sort by field name for determinism (not perfect but better than empty).
		for k := range st.Fields {
			fieldsOrder = append(fieldsOrder, k)
		}
		sort.Strings(fieldsOrder)
	}

	var bindings []ArgBinding
	for idx, fname := range fieldsOrder {
		v, ok := st.Fields[fname]
		if !ok {
			// missing field assignment; keep placeholder
			bindings = append(bindings, ArgBinding{
				ArgIndex: idx,
				ArgName:  fname,
				Kind:     "unknown",
				Expr:     "",
			})
			continue
		}

		// Heuristic: if v is a ref and matches known buffer vars -> buffer binding.
		if v.Kind == "ref" {
			if bi, ok := buffers[v.Ref]; ok {
				bindings = append(bindings, ArgBinding{
					ArgIndex: idx,
					ArgName:  fname,
					Kind:     "buffer",
					Tag:      bi.Tag,
					Expr:     v.Ref,
				})
			} else {
				bindings = append(bindings, ArgBinding{
					ArgIndex: idx,
					ArgName:  fname,
					Kind:     "buffer",
					Tag:      cfg.BufferTags[v.Ref],
					Expr:     v.Ref,
				})
			}
			continue
		}
		if v.Kind == "int" {
			bindings = append(bindings, ArgBinding{
				ArgIndex: idx,
				ArgName:  fname,
				Kind:     "scalar",
				Value:    v.Int,
			})
			continue
		}

		// Try evaluating if unknown
		if v.Kind == "unknown" && v.Expr != "" {
			// If expression resolves to int in current env, convert.
			if i, ok := tryResolveInt(v.Expr, env); ok {
				bindings = append(bindings, ArgBinding{
					ArgIndex: idx,
					ArgName:  fname,
					Kind:     "scalar",
					Value:    i,
					Expr:     v.Expr,
				})
			} else {
				bindings = append(bindings, ArgBinding{
					ArgIndex: idx,
					ArgName:  fname,
					Kind:     "unknown",
					Expr:     v.Expr,
				})
			}
			continue
		}

		bindings = append(bindings, ArgBinding{
			ArgIndex: idx,
			ArgName:  fname,
			Kind:     v.Kind,
			Expr:     v.Expr,
		})
	}
	return bindings
}

func tryResolveInt(expr string, env map[string]Value) (int64, bool) {
	if v, ok := env[expr]; ok && v.Kind == "int" {
		return v.Int, true
	}
	return 0, false
}

func cloneEnv(src map[string]Value) map[string]Value {
	out := make(map[string]Value, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func mergeEnvs(base map[string]Value, a map[string]Value, b map[string]Value) map[string]Value {
	out := cloneEnv(base)
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
		if oka && okb {
			if va.Kind == vb.Kind && va.Int == vb.Int && va.Ref == vb.Ref && va.Expr == vb.Expr {
				out[k] = va
			} else {
				out[k] = Value{Kind: "unknown", Expr: k}
			}
		} else if oka {
			out[k] = va
		} else if okb {
			out[k] = vb
		}
	}
	return out
}

func evalTruthy(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]Value) (bool, bool) {
	v := evalValue(e, fset, cfg, env)
	if v.Kind == "int" {
		return v.Int != 0, true
	}
	return false, false
}

func evalValue(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]Value) Value {
	switch ex := e.(type) {
	case *ast.BasicLit:
		if ex.Kind == token.INT {
			i, err := strconv.ParseInt(ex.Value, 10, 64)
			if err == nil {
				return Value{Kind: "int", Int: i}
			}
		}
		return Value{Kind: "unknown", Expr: ex.Value}
	case *ast.Ident:
		if v, ok := env[ex.Name]; ok {
			return v
		}
		// if ident matches config param
		if pv, ok := cfg.Params[ex.Name]; ok {
			return Value{Kind: "int", Int: pv}
		}
		return Value{Kind: "unknown", Expr: ex.Name}
	case *ast.SelectorExpr:
		key := exprToString(fset, ex)
		if v, ok := env[key]; ok {
			return v
		}
		// If selector is b.NX style, resolve to param
		if id, ok := ex.X.(*ast.Ident); ok && id.Name == "b" {
			if pv, ok := cfg.Params[ex.Sel.Name]; ok {
				return Value{Kind: "int", Int: pv, Expr: key}
			}
		}
		// Otherwise treat as ref (pointer/buffer handle)
		return Value{Kind: "ref", Ref: key, Expr: key}
	case *ast.UnaryExpr:
		if ex.Op == token.AND {
			v := evalValue(ex.X, fset, cfg, env)
			// Keep underlying ref/struct
			return v
		}
		if ex.Op == token.SUB {
			v := evalValue(ex.X, fset, cfg, env)
			if v.Kind == "int" {
				return Value{Kind: "int", Int: -v.Int}
			}
		}
		return Value{Kind: "unknown", Expr: exprToString(fset, e)}
	case *ast.ParenExpr:
		return evalValue(ex.X, fset, cfg, env)
	case *ast.BinaryExpr:
		li := evalValue(ex.X, fset, cfg, env)
		ri := evalValue(ex.Y, fset, cfg, env)
		if li.Kind == "int" && ri.Kind == "int" {
			switch ex.Op {
			case token.ADD:
				return Value{Kind: "int", Int: li.Int + ri.Int}
			case token.SUB:
				return Value{Kind: "int", Int: li.Int - ri.Int}
			case token.MUL:
				return Value{Kind: "int", Int: li.Int * ri.Int}
			case token.QUO:
				if ri.Int != 0 {
					return Value{Kind: "int", Int: li.Int / ri.Int}
				}
			case token.REM:
				if ri.Int != 0 {
					return Value{Kind: "int", Int: li.Int % ri.Int}
				}
			case token.SHL:
				return Value{Kind: "int", Int: li.Int << uint64(ri.Int)}
			case token.SHR:
				return Value{Kind: "int", Int: li.Int >> uint64(ri.Int)}
			case token.LSS:
				if li.Int < ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.LEQ:
				if li.Int <= ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.GTR:
				if li.Int > ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.GEQ:
				if li.Int >= ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.EQL:
				if li.Int == ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.NEQ:
				if li.Int != ri.Int {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.LAND:
				if li.Int != 0 && ri.Int != 0 {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			case token.LOR:
				if li.Int != 0 || ri.Int != 0 {
					return Value{Kind: "int", Int: 1}
				}
				return Value{Kind: "int", Int: 0}
			}
		}
		return Value{Kind: "unknown", Expr: exprToString(fset, e)}
	case *ast.CallExpr:
		// Handle casts like uint32(x), int32(x), uint64(x), int(x)
		fnName := ""
		switch fn := ex.Fun.(type) {
		case *ast.Ident:
			fnName = fn.Name
		case *ast.SelectorExpr:
			// ignore package-qualified calls for now
			fnName = fn.Sel.Name
		}
		if (fnName == "uint32" || fnName == "uint64" || fnName == "int32" || fnName == "int64" || fnName == "int" || fnName == "uint" || fnName == "uint16") && len(ex.Args) >= 1 {
			return evalValue(ex.Args[0], fset, cfg, env)
		}
		if (fnName == "max" || fnName == "min") && len(ex.Args) >= 2 {
			a := evalValue(ex.Args[0], fset, cfg, env)
			b := evalValue(ex.Args[1], fset, cfg, env)
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
		return Value{Kind: "unknown", Expr: exprToString(fset, e)}
	case *ast.CompositeLit:
		// Arrays or structs
		// If Type is array, try parse [3]uint32{...}
		tstr := ""
		if ex.Type != nil {
			tstr = exprToString(fset, ex.Type)
		}
		// array literal
		if strings.HasPrefix(tstr, "[3]") {
			var arr [3]int64
			for i := 0; i < 3 && i < len(ex.Elts); i++ {
				v := evalValue(ex.Elts[i], fset, cfg, env)
				if v.Kind == "int" {
					arr[i] = v.Int
				} else {
					// try treat unknown as 0
					arr[i] = 0
				}
			}
			return Value{Kind: "arr3", Arr3: arr, Expr: exprToString(fset, e)}
		}
		// struct literal
		if id, ok := ex.Type.(*ast.Ident); ok {
			typeName := id.Name
			fields := map[string]Value{}
			for _, elt := range ex.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					k := exprToString(fset, kv.Key)
					fields[k] = evalValue(kv.Value, fset, cfg, env)
				}
			}
			return Value{Kind: "struct", St: &StructVal{TypeName: typeName, Fields: fields}, Expr: exprToString(fset, e)}
		}
		return Value{Kind: "unknown", Expr: exprToString(fset, e)}
	default:
		return Value{Kind: "unknown", Expr: exprToString(fset, e)}
	}
}

func evalInt(e ast.Expr, fset *token.FileSet, cfg *Config, env map[string]Value) (int64, bool) {
	v := evalValue(e, fset, cfg, env)
	if v.Kind == "int" {
		return v.Int, true
	}
	return 0, false
}

func buffersToList(buffers map[string]*BufferInfo, pageBytes int64) []BufferDesc {
	// Stable output ordering.
	var keys []string
	for k := range buffers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []BufferDesc
	for _, k := range keys {
		bi := buffers[k]
		bytesExpr := bi.BytesExprStr
		pages := int64(0)
		if pageBytes > 0 && bi.Bytes > 0 {
			pages = (bi.Bytes + pageBytes - 1) / pageBytes
		}
		out = append(out, BufferDesc{
			Tag:       bi.Tag,
			GoVar:     bi.GoVar,
			BytesExpr: bytesExpr,
			Bytes:     bi.Bytes,
			Pages:     pages,
			Unified:   bi.Unified,
		})
	}
	return out
}

func buildKernelDescs(step1 *Step1, clSigs map[string]CLKernelSig) []KernelDesc {
	var out []KernelDesc
	for _, k := range step1.Kernels {
		var args []KernelArgDesc
		var clArgs []string
		if clSigs != nil {
			if sig, ok := clSigs[k.Name]; ok {
				clArgs = sig.Args
			}
		}
		for _, a := range k.Args {
			name := a.Name
			irName := a.Name
			if len(clArgs) > a.Index {
				name = clArgs[a.Index]
			}
			args = append(args, KernelArgDesc{
				Index:     a.Index,
				Name:      name,
				IRName:    irName,
				Type:      a.Type,
				IsPointer: a.IsPointer,
				AddrSpace: a.AddrSpace,
				ElemBytes: a.ElemBytes,
			})
		}
		out = append(out, KernelDesc{
			Name:    k.Name,
			Args:    args,
			Summary: k.Summary,
		})
	}
	return out
}

func deriveRW(k Step1Kernel, bindings []ArgBinding) (map[string]bool, map[string]bool) {
	reads := map[string]bool{}
	writes := map[string]bool{}
	// Map arg_index -> tag for buffer bindings
	argToTag := map[int]string{}
	for _, b := range bindings {
		if b.Kind == "buffer" && b.Tag != "" {
			argToTag[b.ArgIndex] = b.Tag
		}
	}

	for _, ma := range k.MemAccess {
		if !ma.IsGlobal {
			continue
		}
		if ma.BufferArgIdx < 0 {
			continue
		}
		tag, ok := argToTag[ma.BufferArgIdx]
		if !ok || tag == "" {
			continue
		}
		switch ma.Kind {
		case "load":
			reads[tag] = true
		case "store":
			writes[tag] = true
		}
	}
	return reads, writes
}

func sortedKeys(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildGraph(launches []LaunchDesc) Graph {
	lastWrite := map[string]int{}
	readsSinceWrite := map[string]map[int]bool{}
	var edges []Edge
	edgeSeen := map[string]bool{}

	addEdge := func(from, to int, tag, typ string) {
		if from < 0 || to < 0 || from == to || tag == "" {
			return
		}
		key := fmt.Sprintf("%d->%d:%s:%s", from, to, tag, typ)
		if edgeSeen[key] {
			return
		}
		edgeSeen[key] = true
		edges = append(edges, Edge{From: from, To: to, Buffer: tag, Type: typ})
	}

	for _, l := range launches {
		// RAW: a read depends on the most recent write to the same buffer.
		for _, tag := range l.Reads {
			if w, ok := lastWrite[tag]; ok {
				addEdge(w, l.ID, tag, "RAW")
			}
		}

		// WAW/WAR: a write must stay ordered after the previous writer, and
		// after all readers since that writer. This conservatively preserves
		// sequential scratch-buffer reuse and prevents unsafe pipelining when a
		// later kernel overwrites a buffer that an earlier kernel still reads.
		for _, tag := range l.Writes {
			if w, ok := lastWrite[tag]; ok {
				addEdge(w, l.ID, tag, "WAW")
			}
			if rs, ok := readsSinceWrite[tag]; ok {
				var readers []int
				for rid := range rs {
					readers = append(readers, rid)
				}
				sort.Ints(readers)
				for _, rid := range readers {
					addEdge(rid, l.ID, tag, "WAR")
				}
			}
		}

		// Update reader sets before writes so readers participate in later WARs
		// only until the next write to the same buffer.
		for _, tag := range l.Reads {
			if tag == "" {
				continue
			}
			if _, ok := readsSinceWrite[tag]; !ok {
				readsSinceWrite[tag] = map[int]bool{}
			}
			readsSinceWrite[tag][l.ID] = true
		}

		for _, tag := range l.Writes {
			if tag == "" {
				continue
			}
			lastWrite[tag] = l.ID
			readsSinceWrite[tag] = map[int]bool{}
		}
	}
	return Graph{Edges: edges}
}
