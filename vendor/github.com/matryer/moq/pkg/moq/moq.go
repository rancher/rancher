package moq

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"go/types"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

// Mocker can generate mock structs.
type Mocker struct {
	srcPkg  *packages.Package
	tmpl    *template.Template
	pkgName string
	pkgPath string
	fmter   func(src []byte) ([]byte, error)

	importToAlias map[string]string
	importAliases map[string]bool
	importLines   map[string]bool
}

// Config specifies details about how interfaces should be mocked.
// SrcDir is the only field which needs be specified.
type Config struct {
	SrcDir    string
	PkgName   string
	Formatter string
}

// New makes a new Mocker for the specified package directory.
func New(conf Config) (*Mocker, error) {
	srcPkg, err := pkgInfoFromPath(conf.SrcDir, packages.NeedName|packages.NeedTypes|packages.NeedTypesInfo|packages.NeedSyntax)
	if err != nil {
		return nil, fmt.Errorf("couldn't load source package: %s", err)
	}

	pkgName := conf.PkgName
	if pkgName == "" {
		pkgName = srcPkg.Name
	}

	pkgPath, err := findPkgPath(conf.PkgName, srcPkg)
	if err != nil {
		return nil, fmt.Errorf("couldn't load mock package: %s", err)
	}

	tmpl, err := template.New("moq").Funcs(templateFuncs).Parse(moqTemplate)
	if err != nil {
		return nil, err
	}

	fmter := gofmt
	if conf.Formatter == "goimports" {
		fmter = goimports
	}

	importAliases := make(map[string]bool)
	importToAlias := make(map[string]string)

	// Attempt to preserve original aliases for prettiness
	for _, syntax := range srcPkg.Syntax {
		for _, importSpec := range syntax.Imports {
			if importSpec.Name != nil && importSpec.Path != nil {
				importAliases[importSpec.Name.Name] = true
				importToAlias[strings.Trim(importSpec.Path.Value, "\"")] = importSpec.Name.Name
			}
		}
	}

	return &Mocker{
		tmpl:          tmpl,
		srcPkg:        srcPkg,
		pkgName:       pkgName,
		pkgPath:       pkgPath,
		fmter:         fmter,
		importLines:   make(map[string]bool),
		importAliases: importAliases,
		importToAlias: importToAlias,
	}, nil
}

func findPkgPath(pkgInputVal string, srcPkg *packages.Package) (string, error) {
	if pkgInputVal == "" {
		return srcPkg.PkgPath, nil
	}
	if pkgInDir(".", pkgInputVal) {
		return ".", nil
	}
	if pkgInDir(srcPkg.PkgPath, pkgInputVal) {
		return srcPkg.PkgPath, nil
	}
	subdirectoryPath := filepath.Join(srcPkg.PkgPath, pkgInputVal)
	if pkgInDir(subdirectoryPath, pkgInputVal) {
		return subdirectoryPath, nil
	}
	return "", nil
}

func pkgInDir(pkgName, dir string) bool {
	currentPkg, err := pkgInfoFromPath(dir, packages.NeedName)
	if err != nil {
		return false
	}
	return currentPkg.Name == pkgName || currentPkg.Name+"_test" == pkgName
}

// Mock generates a mock for the specified interface name.
func (m *Mocker) Mock(w io.Writer, names ...string) error {
	if len(names) == 0 {
		return errors.New("must specify one interface")
	}

	doc := doc{
		PackageName: m.pkgName,
	}

	mocksMethods := false

	paramCache := make(map[string][]*param)

	tpkg := m.srcPkg.Types
	for _, name := range names {
		n, mockName := parseInterfaceName(name)
		iface := tpkg.Scope().Lookup(n)
		if iface == nil {
			return fmt.Errorf("cannot find interface %s", n)
		}
		if !types.IsInterface(iface.Type()) {
			return fmt.Errorf("%s (%s) not an interface", n, iface.Type().String())
		}
		iiface := iface.Type().Underlying().(*types.Interface).Complete()
		obj := obj{
			InterfaceName: n,
			MockName:      mockName,
		}
		for i := 0; i < iiface.NumMethods(); i++ {
			mocksMethods = true
			meth := iiface.Method(i)
			sig := meth.Type().(*types.Signature)
			method := &method{
				Name: meth.Name(),
			}
			obj.Methods = append(obj.Methods, method)
			method.Params, method.Returns = m.extractArgs(sig)

			for _, param := range method.Params {
				paramCache[param.Name] = append(paramCache[param.Name], param)
			}
		}
		doc.Objects = append(doc.Objects, obj)
	}

	if mocksMethods {
		_, importLine := m.qualifierAndImportLine("sync", "sync")
		doc.Imports = append(doc.Imports, importLine)
	}

	for pkgToImport := range m.importLines {
		doc.Imports = append(doc.Imports, pkgToImport)
	}

	if tpkg.Name() != m.pkgName {
		qualifier, importLine := m.qualifierAndImportLine(tpkg.Path(), tpkg.Name())
		doc.SourcePackagePrefix = qualifier + "."
		doc.Imports = append(doc.Imports, importLine)
	}

	for pkg := range m.importAliases {
		if params, hasConflict := paramCache[pkg]; hasConflict {
			for _, param := range params {
				param.LocalName = fmt.Sprintf("%sMoqParam", param.LocalName)
			}
		}
	}

	var buf bytes.Buffer
	err := m.tmpl.Execute(&buf, doc)
	if err != nil {
		return err
	}
	formatted, err := m.fmter(buf.Bytes())
	if err != nil {
		return err
	}
	if _, err := w.Write(formatted); err != nil {
		return err
	}
	return nil
}

func (m *Mocker) allocAlias(path string, pkgName string) string {
	suffix := 0
	attemptedName := pkgName
	for {
		if _, taken := m.importAliases[attemptedName]; taken {
			suffix++
			attemptedName = fmt.Sprintf("%s%d", pkgName, suffix)
			continue
		}

		m.importAliases[attemptedName] = true
		m.importToAlias[path] = attemptedName

		// Don't alias packages that don't require an alias
		if attemptedName == pkgName {
			m.importToAlias[path] = ""
			return ""
		}

		return attemptedName
	}
}

func (m *Mocker) getAlias(path string, pkgName string) string {
	alias, aliasSet := m.importToAlias[path]
	if !aliasSet {
		alias = m.allocAlias(path, pkgName)
	}
	return alias
}

func (m *Mocker) qualifierAndImportLine(pkg, pkgName string) (string, string) {
	pkg = stripVendorPath(pkg)
	alias := m.getAlias(pkg, pkgName)
	importLine := quoteImport(alias, pkg)
	if alias == "" {
		return pkgName, importLine
	}
	return alias, importLine
}

func quoteImport(alias, pkg string) string {
	if alias == "" {
		return fmt.Sprintf("\"%s\"", pkg)
	}
	return fmt.Sprintf("%s \"%s\"", alias, pkg)
}

func (m *Mocker) packageQualifier(pkg *types.Package) string {
	if m.pkgPath != "" && m.pkgPath == pkg.Path() {
		return ""
	}
	path := pkg.Path()
	if pkg.Path() == "." {
		wd, err := os.Getwd()
		if err == nil {
			path = stripGopath(wd)
		}
	}

	qualifier, importLine := m.qualifierAndImportLine(path, pkg.Name())
	m.importLines[importLine] = true
	return qualifier
}

func (m *Mocker) extractArgs(sig *types.Signature) (params, results []*param) {
	pp := sig.Params()
	for i := 0; i < pp.Len(); i++ {
		p := m.buildParam(pp.At(i), "in"+strconv.Itoa(i+1))
		// check for final variadic argument
		p.Variadic = sig.Variadic() && i == pp.Len()-1 && p.Type[0:2] == "[]"
		params = append(params, p)
	}

	rr := sig.Results()
	for i := 0; i < rr.Len(); i++ {
		results = append(results, m.buildParam(rr.At(i), "out"+strconv.Itoa(i+1)))
	}

	return
}

func (m *Mocker) buildParam(v *types.Var, fallbackName string) *param {
	name := v.Name()
	if name == "" {
		name = fallbackName
	}
	typ := types.TypeString(v.Type(), m.packageQualifier)
	return &param{Name: name, LocalName: name, Type: typ}
}

func pkgInfoFromPath(srcDir string, mode packages.LoadMode) (*packages.Package, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: mode,
		Dir:  srcDir,
	})
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, errors.New("No packages found")
	}
	if len(pkgs) > 1 {
		return nil, errors.New("More than one package was found")
	}
	return pkgs[0], nil
}

func parseInterfaceName(name string) (ifaceName, mockName string) {
	parts := strings.SplitN(name, ":", 2)
	ifaceName = parts[0]
	mockName = ifaceName + "Mock"
	if len(parts) == 2 {
		mockName = parts[1]
	}
	return
}

type doc struct {
	PackageName         string
	SourcePackagePrefix string
	Objects             []obj
	Imports             []string
}

type obj struct {
	InterfaceName string
	MockName      string
	Methods       []*method
}
type method struct {
	Name    string
	Params  []*param
	Returns []*param
}

func (m *method) Arglist() string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = p.String()
	}
	return strings.Join(params, ", ")
}

func (m *method) ArgCallList() string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = p.CallName()
	}
	return strings.Join(params, ", ")
}

func (m *method) ReturnArglist() string {
	params := make([]string, len(m.Returns))
	for i, p := range m.Returns {
		params[i] = p.TypeString()
	}
	if len(m.Returns) > 1 {
		return fmt.Sprintf("(%s)", strings.Join(params, ", "))
	}
	return strings.Join(params, ", ")
}

type param struct {
	Name      string
	LocalName string
	Type      string
	Variadic  bool
}

func (p param) String() string {
	return fmt.Sprintf("%s %s", p.LocalName, p.TypeString())
}

func (p param) CallName() string {
	if p.Variadic {
		return p.LocalName + "..."
	}
	return p.LocalName
}

func (p param) TypeString() string {
	if p.Variadic {
		return "..." + p.Type[2:]
	}
	return p.Type
}

var templateFuncs = template.FuncMap{
	"Exported": func(s string) string {
		if s == "" {
			return ""
		}
		for _, initialism := range golintInitialisms {
			if strings.ToUpper(s) == initialism {
				return initialism
			}
		}
		return strings.ToUpper(s[0:1]) + s[1:]
	},
}

// stripVendorPath strips the vendor dir prefix from a package path.
// For example we might encounter an absolute path like
// github.com/foo/bar/vendor/github.com/pkg/errors which is resolved
// to github.com/pkg/errors.
func stripVendorPath(p string) string {
	parts := strings.Split(p, "/vendor/")
	if len(parts) == 1 {
		return p
	}
	return strings.TrimLeft(path.Join(parts[1:]...), "/")
}

// stripGopath takes the directory to a package and removes the
// $GOPATH/src path to get the canonical package name.
func stripGopath(p string) string {
	for _, srcDir := range build.Default.SrcDirs() {
		rel, err := filepath.Rel(srcDir, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		return filepath.ToSlash(rel)
	}
	return p
}
