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
  File functions
*/

type RemoveFunc func(src string) error

func removeDry(src string) error {
	fmt.Printf("remove %s\n", src)
	return nil
}

func removeFile(src string) error {
	return os.Remove(src)
}

type RenameFunc func(src, dst string) error

func renameDry(src, dst string) error {
	fmt.Printf("rename %s -> %s\n", src, dst)
	return nil
}

func renameFile(src, dst string) error {
	return os.Rename(src, dst)
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

	exists := fileExists(dst)
	if exists != 0 {
		return 0, errors.New(dst + " exists")
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
		basePath      string
		checks        bool
		copy          CopyFunc
		rename        RenameFunc
		exists        int
		criticalError bool
	)

	if tgtDir != "" {
		basePath = tgtDir
	} else {
		basePath, _ = filepath.Abs(moduleInfo.NwnDir)
	}
	filepath.WalkDir(moduleRootPath, walkDir)
	// filepath.Walk(filepath.Join(nwnDir, extensionsDir, moduleName), walker)
	fmt.Println("> Run pre-install checks...")

	checks = true

	if dryRun {
		copy = copyDry
		rename = renameDry
	} else {
		copy = copyFile
		rename = renameFile
	}

	criticalError = false
	for _, fpath := range moduleInfo.Files {
		dst := filepath.Join(basePath, fpath)
		exists = fileExists(dst)
		excluded := moduleInfo.Excluded.Contains(fpath)
		if exists != 0 {
			// file exists
			// fmt.Printf("%s seems to exist\n", fpath)
			checks = false
		} else if !excluded {
			// filed does not exist and is not excluded
			moduleInfo.Installed = append(moduleInfo.Installed, fpath)
		}
		if exists != 0 && moduleInfo.OverwriteExisting && !excluded {
			if fileExists(dst+moduleInfo.BackupExtension) == 0 {
				// Backup does not exist
				moduleInfo.Saved = append(moduleInfo.Saved, fpath+moduleInfo.BackupExtension)
				moduleInfo.Installed = append(moduleInfo.Installed, fpath)
			} else {
				// Backup exists already. Danger zone
				fmt.Printf("! %s already exist\n", fpath+moduleInfo.BackupExtension)
				criticalError = true
			}
		} else if exists != 0 && !excluded {
			fmt.Printf("! %s skipped\n", fpath)
			moduleInfo.Skipped = append(moduleInfo.Skipped, fpath)
		}
	}

	if (!checks && !skipErrors && !moduleInfo.OverwriteExisting) || criticalError {
		fmt.Printf("! Exiting due to previous errors\n")
		os.Exit(1)
	}

	for _, fpath := range moduleInfo.Installed {
		// fmt.Println(fpath)
		operation := "+"
		if moduleInfo.Saved.Contains(fpath + moduleInfo.BackupExtension) {
			// Save backup copy
			dst := filepath.Join(basePath, fpath)
			operation = "^"
			// copy(dst, dst+moduleInfo.BackupExtension)
			if fileExists(dst+moduleInfo.BackupExtension) == 0 {
				rename(dst, dst+moduleInfo.BackupExtension)
			}
		}
		fmt.Printf("%s %s\n", operation, fpath)
		copy(filepath.Join(moduleRootPath, fpath), filepath.Join(basePath, fpath))
	}
	manifest, _ := json.Marshal(moduleInfo)
	// fmt.Println(string(manifest))
	manifestFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".json")
	fmt.Printf("= Writing manifest to %s\n", manifestFile)
	if !dryRun {
		ioutil.WriteFile(manifestFile, manifest, 0640)
	}
}

func uninstall(dryRun, skipErrors bool) {
	var (
		rename RenameFunc
		remove RemoveFunc
	)
	manifestFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".json")
	uninstalledFile := filepath.Join(moduleInfo.NwnDir, moduleInfo.ExtensionsDir, moduleInfo.Name+".uninstalled")
	jsonFile, err := os.Open(manifestFile)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	if dryRun {
		rename = renameDry
		remove = removeDry
	} else {
		rename = renameFile
		remove = removeFile
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var mi ModuleInfo
	err = json.Unmarshal(byteValue, &mi)
	if err != nil {
		panic(err)
	}

	//XXX Override base vars with the one from CLI (?)
	mi.NwnDir = moduleInfo.NwnDir
	mi.ExtensionsDir = moduleInfo.ExtensionsDir
	mi.Name = moduleInfo.Name

	for _, installed := range mi.Installed {
		filepath := filepath.Join(mi.NwnDir, installed)
		fmt.Printf("- %s\n", filepath)
		remove(filepath)
	}
	for _, saved := range mi.Saved {
		filepath := filepath.Join(mi.NwnDir, saved)
		// we need to rename filepath to original by removing suffix
		// strings.HasSuffix(xxx,mi.BackupExtension)
		originalFilepath := strings.TrimSuffix(filepath, mi.BackupExtension)
		err := rename(filepath, originalFilepath)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("v %s\n", filepath)
	}

	err = rename(manifestFile, uninstalledFile)
	if err != nil {
		fmt.Println(err)
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
