package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

const (
	traceAlias = "__arthastrace"
	tracePkg   = "github.com/songzhibin97/go-arthas/arthastrace"
)

// edit 表示对源码字节区间 [offset,end) 的一次替换（end==offset 为纯插入）
type edit struct {
	offset int
	end    int
	text   string
}

// instrumentSource 对 src 中 id 命中 targets 的函数注入 watch 钩子。
// 返回注入并格式化后的源码与被注入的 id 列表；无命中时返回原 src 与 nil。
//
// 采用基于 AST 定位 + 文本编辑的方式：用 AST 找到注入点的字节偏移，做文本
// 插入/替换，最后用 go/format 兜底语法正确性并美化——避免直接拼接 AST 节点
// 带来的 token.Pos 错乱问题。
func instrumentSource(filename, pkgPath string, src []byte, targets map[string]bool) ([]byte, []string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	offOf := func(p token.Pos) int { return fset.Position(p).Offset }

	var edits []edit
	var injected []string

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue // 跳过无函数体（如汇编实现）
		}
		if fn.Type.TypeParams != nil {
			continue // MVP 跳过泛型函数
		}
		id := funcID(pkgPath, fn)
		if !targets[id] {
			continue
		}

		params := collectParams(fn.Type.Params)
		results, sigEdit := planResults(src, offOf, fn.Type.Results)
		if sigEdit != nil {
			edits = append(edits, *sigEdit)
		}

		lbrace := offOf(fn.Body.Lbrace)
		edits = append(edits, edit{
			offset: lbrace + 1,
			end:    lbrace + 1,
			text:   "\n" + buildInjectionText(id, params, results) + "\n",
		})
		injected = append(injected, id)
	}

	if len(injected) == 0 {
		return src, nil, nil
	}

	if ie, ok := importEdit(src, offOf, file); ok {
		edits = append(edits, ie)
	}
	edits = append(edits, edit{offset: len(src), end: len(src), text: buildInitText(injected)})

	out := applyEdits(src, edits)
	formatted, err := format.Source(out)
	if err != nil {
		return nil, nil, fmt.Errorf("format injected %s: %w", filename, err)
	}
	return formatted, injected, nil
}

// funcID 计算函数的全局 id：pkg.Func 或 pkg.(Recv).Method
func funcID(pkgPath string, fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		return fmt.Sprintf("%s.(%s).%s", pkgPath, recvTypeName(fn.Recv.List[0].Type), fn.Name.Name)
	}
	return fmt.Sprintf("%s.%s", pkgPath, fn.Name.Name)
}

func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // 泛型 receiver（泛型已跳过，留作健壮）
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	default:
		return "?"
	}
}

// collectParams 收集可捕获的参数名（跳过 _ 与无名参数）
func collectParams(fields *ast.FieldList) []string {
	var names []string
	if fields == nil {
		return names
	}
	for _, f := range fields.List {
		for _, n := range f.Names {
			if n.Name != "_" {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// planResults 处理返回值：返回可引用的返回值名列表；若存在无名或 _ 返回值，
// 生成把整个返回值签名改写为命名版本的编辑（命名返回值才能在 defer 中读取）。
func planResults(src []byte, offOf func(token.Pos) int, results *ast.FieldList) ([]string, *edit) {
	if results == nil || len(results.List) == 0 {
		return nil, nil
	}

	needRewrite := false
	for _, f := range results.List {
		if len(f.Names) == 0 {
			needRewrite = true
			break
		}
		for _, n := range f.Names {
			if n.Name == "_" {
				needRewrite = true
			}
		}
	}

	if !needRewrite {
		var names []string
		for _, f := range results.List {
			for _, n := range f.Names {
				names = append(names, n.Name)
			}
		}
		return names, nil
	}

	var names, parts []string
	idx := 0
	for _, f := range results.List {
		typeText := string(src[offOf(f.Type.Pos()):offOf(f.Type.End())])
		count := len(f.Names)
		if count == 0 {
			count = 1
		}
		for k := 0; k < count; k++ {
			name := fmt.Sprintf("__arthas_ret%d", idx)
			parts = append(parts, name+" "+typeText)
			names = append(names, name)
			idx++
		}
	}
	newSig := "(" + strings.Join(parts, ", ") + ")"
	return names, &edit{offset: offOf(results.Pos()), end: offOf(results.End()), text: newSig}
}

// buildInjectionText 生成插入到函数体首部的注入代码（单行，交由 go/format 美化）
func buildInjectionText(id string, params, results []string) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "var __arthas_inv *%s.Invocation; ", traceAlias)
	fmt.Fprintf(&b, "if %s.Enabled(%s) { __arthas_inv = %s.Enter(%s, []%s.Arg{", traceAlias, q(id), traceAlias, q(id), traceAlias)
	for _, p := range params {
		fmt.Fprintf(&b, "{Name: %s, Value: %s.Format(%s)}, ", q(p), traceAlias, p)
	}
	b.WriteString("}) }; ")
	b.WriteString("defer func() { if __arthas_inv == nil { return }; ")
	b.WriteString("if __arthas_r := recover(); __arthas_r != nil { ")
	b.WriteString(exitCallText(results, "__arthas_r"))
	b.WriteString("; panic(__arthas_r) }; ")
	b.WriteString(exitCallText(results, "nil"))
	b.WriteString(" }();")
	return b.String()
}

func exitCallText(results []string, recovered string) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "__arthas_inv.Exit([]%s.Arg{", traceAlias)
	for i, r := range results {
		fmt.Fprintf(&b, "{Name: %s, Value: %s.Format(%s)}, ", q(fmt.Sprintf("ret%d", i)), traceAlias, r)
	}
	fmt.Fprintf(&b, "}, %s)", recovered)
	return b.String()
}

// importEdit 在 package 声明之后插入带别名的 arthastrace import（若尚未导入）
func importEdit(src []byte, offOf func(token.Pos) int, file *ast.File) (edit, bool) {
	for _, imp := range file.Imports {
		if imp.Path.Value == strconv.Quote(tracePkg) {
			return edit{}, false
		}
	}
	pkgEnd := offOf(file.Name.End())
	text := fmt.Sprintf("\nimport %s %s\n", traceAlias, strconv.Quote(tracePkg))
	return edit{offset: pkgEnd, end: pkgEnd, text: text}, true
}

// buildInitText 生成在文件末尾追加的 init()，注册所有被注入的 id 以便控制面发现
func buildInitText(ids []string) string {
	var b strings.Builder
	b.WriteString("\nfunc init() { ")
	for _, id := range ids {
		fmt.Fprintf(&b, "%s.Register(%s); ", traceAlias, strconv.Quote(id))
	}
	b.WriteString("}\n")
	return b.String()
}

// applyEdits 从后往前应用编辑，避免前面的插入使后面的偏移失效
func applyEdits(src []byte, edits []edit) []byte {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].offset > edits[j].offset })
	out := string(src)
	for _, e := range edits {
		out = out[:e.offset] + e.text + out[e.end:]
	}
	return []byte(out)
}
