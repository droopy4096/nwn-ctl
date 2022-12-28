package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	moduleRootPath string
	tgtDir         string
	moduleInfo     ModuleInfo
)

type ModuleInfo struct {
	Name          string
	ExtensionsDir string
	NwnDir        string
	Installed     []string
	Skipped       []string
}

func walkDir(path string, d fs.DirEntry, err error) (e error) {
	if err != nil {
		fmt.Println("Error accessing " + path)
		return err
	}
	if !d.IsDir() {
		shortPath := strings.Replace(path, moduleRootPath, "", 1)
		moduleInfo.Installed = append(moduleInfo.Installed, string(shortPath[1:]))
		// fmt.Println("..." + path)
	}
	return nil
}

type CopyFunc func(src, dst string) (int64, error)

func copyDry(src, dst string) (int64, error) {
	fmt.Printf("copy %s -> %s\n", src, dst)
	return 0, nil
}

func copyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	// fmt.Println("Dir to create " + filepath.Dir(dst))
	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return 0, err
	}

	var destination *os.File
	destination, err = os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()

	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func main() {
	var skipErrors bool
	flag.StringVar(&moduleInfo.Name, "module", "", "")
	flag.StringVar(&moduleInfo.ExtensionsDir, "extensions-dir", "Extensions", "")
	flag.StringVar(&moduleInfo.NwnDir, "nwn-dir", ".", "")
	flag.StringVar(&tgtDir, "target-dir", "", "")
	flag.BoolVar(&skipErrors, "skip-errors", false, "Skip file errors")
	flag.Parse()
	moduleRootPath, _ = filepath.Abs(filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name))
	var basePath string
	if tgtDir != "" {
		basePath = tgtDir
	} else {
		basePath, _ = filepath.Abs(moduleInfo.NwnDir)
	}
	filepath.WalkDir(moduleRootPath, walkDir)
	// filepath.Walk(filepath.Join(nwnDir, extensionsDir, moduleName), walker)
	fmt.Println("Run pre-install checks...")
	var checks bool
	checks = true
	for _, fpath := range moduleInfo.Installed {
		dst := filepath.Join(basePath, fpath)
		if _, err := os.Stat(dst); err == nil {
			// path/to/whatever exists
			fmt.Printf("Can't overwrite existing %s\n", dst)
			checks = false
			moduleInfo.Skipped = append(moduleInfo.Skipped, fpath)
		} else if !errors.Is(err, os.ErrNotExist) {
			// unsure whether file exist or does not
			fmt.Printf("Uncertain existence of %s\n", dst)
			checks = false
			moduleInfo.Skipped = append(moduleInfo.Skipped, fpath)
		}
	}

	var copy CopyFunc
	if !checks && !skipErrors {
		fmt.Printf("Exiting due to previous errors\n")
		os.Exit(1)
		copy = copyDry
	} else {
		copy = copyDry
	}

	for _, fpath := range moduleInfo.Installed {
		// fmt.Println(fpath)
		copy(filepath.Join(moduleRootPath, fpath), filepath.Join(basePath, fpath))
	}
	manifest, _ := json.Marshal(moduleInfo)
	// fmt.Println(string(manifest))
	manifestFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".json")
	fmt.Printf("Writing manifest to %s\n", manifestFile)
	ioutil.WriteFile(manifestFile, manifest, 0640)
}
