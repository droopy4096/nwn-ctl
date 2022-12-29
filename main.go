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

/*==================
  Class: fileList
*/
type fileList []string

func (e *fileList) String() string {
	return fmt.Sprint(*e)
}

func (e *fileList) Set(value string) error {
	for _, fn := range strings.Split(value, ",") {
		*e = append(*e, fn)
	}
	return nil
}

func (e *fileList) Contains(value string) bool {
	for _, fn := range *e {
		if value == fn {
			return true
		}
	}
	return false
}

/*
  End of fileList
  ===================*/

type ModuleInfo struct {
	Name              string   `json:"name"`
	ExtensionsDir     string   `json:"extensionsDir"`
	NwnDir            string   `json:"nwnDir"`
	BackupExtension   string   `json:"backupExtension"`
	OverwriteExisting bool     `json:"overwriteExisting"`
	Files             fileList `json:"files"`
	Installed         fileList `json:"installed"`
	Skipped           fileList `json:"skipped"`
	Excluded          fileList `json:"excluded"`
	Saved             fileList `json:"saved"`
}

func walkDir(path string, d fs.DirEntry, err error) (e error) {
	if err != nil {
		fmt.Println("Error accessing " + path)
		return err
	}
	if !d.IsDir() {
		shortPath := strings.Replace(path, moduleRootPath, "", 1)
		moduleInfo.Files = append(moduleInfo.Files, string(shortPath[1:]))
		// fmt.Println("..." + path)
	}
	return nil
}

/*=================
  Copy functions
*/

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

/*
  End of Copy Functions
  =====================*/

func fileExists(dst string) int {
	if _, err := os.Stat(dst); err == nil {
		// path/to/whatever exists
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		// unsure whether file exist or does not
		return -1
	}
	return 0
}

func install(dryRun, skipErrors bool) {
	var (
		basePath string
		checks   bool
		copy     CopyFunc
		exists   int
	)

	if tgtDir != "" {
		basePath = tgtDir
	} else {
		basePath, _ = filepath.Abs(moduleInfo.NwnDir)
	}
	filepath.WalkDir(moduleRootPath, walkDir)
	// filepath.Walk(filepath.Join(nwnDir, extensionsDir, moduleName), walker)
	fmt.Println("Run pre-install checks...")

	checks = true

	if dryRun {
		copy = copyDry
	} else {
		copy = copyFile
	}

	for _, fpath := range moduleInfo.Files {
		dst := filepath.Join(basePath, fpath)
		exists = fileExists(dst)
		excluded := moduleInfo.Excluded.Contains(fpath)
		if exists != 0 {
			// file exists
			fmt.Printf("%s seems to exist\n", fpath)
			checks = false
		} else if !excluded {
			// filed does not exist and is not excluded
			moduleInfo.Installed = append(moduleInfo.Installed, fpath)
		}
		if exists != 0 && moduleInfo.OverwriteExisting && !excluded {
			// copy(dst, dst+moduleInfo.BackupExtension)
			moduleInfo.Saved = append(moduleInfo.Saved, fpath+moduleInfo.BackupExtension)
			moduleInfo.Installed = append(moduleInfo.Installed, fpath)
		} else if exists != 0 && !excluded {
			moduleInfo.Skipped = append(moduleInfo.Skipped, fpath)
		}
	}

	if !checks && !skipErrors && !moduleInfo.OverwriteExisting {
		fmt.Printf("Exiting due to previous errors\n")
		os.Exit(1)
	}

	for _, fpath := range moduleInfo.Installed {
		// fmt.Println(fpath)
		if moduleInfo.Saved.Contains(fpath + moduleInfo.BackupExtension) {
			dst := filepath.Join(basePath, fpath)
			fmt.Printf("%s is being backed up\n", fpath)
			copy(dst, dst+moduleInfo.BackupExtension)
		}
		copy(filepath.Join(moduleRootPath, fpath), filepath.Join(basePath, fpath))
	}
	manifest, _ := json.Marshal(moduleInfo)
	// fmt.Println(string(manifest))
	manifestFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".json")
	fmt.Printf("Writing manifest to %s\n", manifestFile)
	ioutil.WriteFile(manifestFile, manifest, 0640)
}

func uninstall(dryRun, skipErrors bool) {
	manifestFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".json")
	uninstalledFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".uninstalled")
	jsonFile, err := os.Open(manifestFile)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var mi ModuleInfo
	err = json.Unmarshal(byteValue, &mi)
	if err != nil {
		panic(err)
	}
	for _, installed := range mi.Installed {
		filepath := filepath.Join(mi.NwnDir, installed)
		if !dryRun {
			os.Remove(filepath)
		} else {
			fmt.Printf("- %s\n", filepath)
		}
	}
	for _, saved := range mi.Saved {
		filepath := filepath.Join(mi.NwnDir, saved)
		// we need to rename filepath to original by removing suffix
		// strings.HasSuffix(xxx,mi.BackupExtension)
		originalFilepath := strings.TrimSuffix(filepath, mi.BackupExtension)
		if !dryRun {
			err := os.Rename(filepath, originalFilepath)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Printf("o %s <- %s\n", originalFilepath, filepath)
		}
	}
	if !dryRun {
		err = os.Rename(manifestFile, uninstalledFile)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func main() {
	var (
		skipErrors bool
		dryRun     bool
		command    string
	)

	flag.StringVar(&moduleInfo.Name, "module", "", "")
	flag.StringVar(&moduleInfo.ExtensionsDir, "extensions-dir", "Extensions", "")
	flag.StringVar(&moduleInfo.NwnDir, "nwn-dir", ".", "")
	flag.StringVar(&tgtDir, "target-dir", "", "")
	flag.StringVar(&moduleInfo.BackupExtension, "backup-extension", ".bak", "")
	flag.StringVar(&command, "command", "install", "")
	flag.BoolVar(&skipErrors, "skip-errors", false, "Skip file errors")
	flag.BoolVar(&moduleInfo.OverwriteExisting, "overwrite-existing", false, "Overwrite Existing files")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry Run")
	flag.Var(&moduleInfo.Excluded, "excluded", "excluded files")
	flag.Parse()

	moduleRootPath, _ = filepath.Abs(filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name))

	if command == "install" {
		install(dryRun, skipErrors)
	} else if command == "uninstall" {
		uninstall(dryRun, skipErrors)
	}

}
