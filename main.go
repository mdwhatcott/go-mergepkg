package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

func combinedImports(dirs []string) (results []string) {
	all := make(map[string]struct{})
	for _, dir := range dirs {
		stat, err := os.Stat(dir)
		if os.IsNotExist(err) || !stat.IsDir() {
			log.Fatal("Specified directory does not exist:", dir)
		}
		imported, err := build.Default.ImportDir(dir, build.ImportComment)
		if err != nil {
			log.Fatal(err)
		}
		for _, imp := range imported.Imports {
			all[imp] = struct{}{}
		}
	}
	for imp := range all {
		results = append(results, imp)
	}
	sort.Strings(results)
	return results
}

type Config struct {
	Dirs   []string
	Header string
}

func parseConfig() (result Config) {
	var dirs string
	flag.StringVar(&result.Header, "header", "",
		"A file header to go in the package-level comment of the output file. (Maybe a version number?)")
	flag.StringVar(&dirs, "dirs", "",
		strings.Join([]string{
			"The colon-separated list of directories containing go files to merge into a single file.",
			"The package name of the first directory listed will be used as the package name of the output file.",
			"It should go without saying that each of the packages should be independent of all the others.",
			"It should also go without saying that there must be no name collisions between any of the packages.",
			"All source code comments will be absent in the output file.",
			"Go test files (those ending in '_test.go') are NOT included in output file.",
		}, " "),
	)
	flag.Parse()
	if len(dirs) == 0 {
		log.Fatal("Usage: mergepkg -dirs '<colon-separated-listing>'")
	}
	result.Dirs = strings.Split(dirs, ":")
	result.Header = strings.ReplaceAll(strings.TrimSpace(result.Header), "\n", "\n// ")
	return result
}

func main() {
	config := parseConfig()
	output := new(bytes.Buffer)
	emit := func(args ...any) { _, _ = fmt.Fprintln(output, args...) }
	imports := combinedImports(config.Dirs)
	for d, dir := range config.Dirs {
		stat, err := os.Stat(dir)
		if os.IsNotExist(err) || !stat.IsDir() {
			log.Fatal("Specified directory does not exist:", dir)
		}
		listing, err := os.ReadDir(dir)
		if err != nil {
			log.Fatal("Cannot read specified directory.")
		}
		if d == 0 {
			imported, err := build.Default.ImportDir(dir, build.ImportComment)
			if err != nil {
				log.Fatal(err)
			}
			emit("// Package", imported.Name, "info:", config.Header)
			emit("// AUTO-GENERATED:", time.Now())
			emit("package", imported.Name)
			emit()
			emit("import (")
			for _, pkg := range imports {
				emit(fmt.Sprintf("%#v", pkg))
			}
			emit(")")
			emit()
		}

		for _, file := range listing {
			if file.IsDir() {
				continue
			}
			name := file.Name()
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			emit()
			emit("// FILE:", name)
			emit()

			filepath := path.Join(dir, name)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath, nil, 0)
			if err != nil {
				log.Println(err)
				continue
			}
			for _, declaration := range f.Decls {
				g, ok := declaration.(*ast.GenDecl)
				if ok && g.Tok == token.IMPORT {
					continue
				}
				err = format.Node(output, fset, declaration)
				emit()
				if err != nil {
					log.Println(err)
					break
				}
			}
		}
	}

	out, err := format.Source(output.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	_, _ = os.Stdout.Write(out)
}
