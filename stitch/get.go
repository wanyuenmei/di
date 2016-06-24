package stitch

import (
	"bufio"
	"fmt"
	"golang.org/x/tools/go/vcs"
	"os"
	"os/user"
	"path/filepath"
	"text/scanner"

	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"

	"github.com/spf13/afero"
)

// Break out the download and create functions for unit testing
var download = func(repo *vcs.RepoRoot, dir string) error {
	return repo.VCS.Download(dir)
}

var create = func(repo *vcs.RepoRoot, dir string) error {
	return repo.VCS.Create(dir, repo.Root)
}

// GetQuiltPath returns the user-defined QUILT_PATH, or the default absolute QUILT_PATH,
// which is ~/.quilt if the user did not specify a QUILT_PATH.
func GetQuiltPath() string {
	quiltPath := os.Getenv("QUILT_PATH")
	if quiltPath != "" {
		return quiltPath
	}

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	quiltPath = filepath.Join(usr.HomeDir, ".quilt")
	return quiltPath
}

// GetSpec takes in an import path repoName, and attempts to download the repository
// associated with that repoName.
func GetSpec(repoName string) error {
	path, err := getSpec(repoName)
	if err != nil {
		return err
	}
	return resolveSpecImports(path)
}

func getSpec(repoName string) (string, error) {
	repo, err := vcs.RepoRootForImportPath(repoName, true)
	if err != nil {
		return "", err
	}

	path := filepath.Join(GetQuiltPath(), repo.Root)
	if _, err := util.AppFs.Stat(path); os.IsNotExist(err) {
		log.Info(fmt.Sprintf("Cloning %s into %s", repo.Root, path))
		if err := create(repo, path); err != nil {
			return "", err
		}
	} else {
		log.Info(fmt.Sprintf("Updating %s in %s", repo.Root, path))
		download(repo, path)
	}
	return path, nil
}

func resolveSpecImports(folder string) error {
	return afero.Walk(util.AppFs, folder, checkSpec)
}

func checkSpec(file string, info os.FileInfo, err error) error {
	if filepath.Ext(file) != ".spec" {
		return nil
	}
	f, err := util.Open(file)

	if err != nil {
		return err
	}
	defer f.Close()

	sc := scanner.Scanner{
		Position: scanner.Position{
			Filename: file,
		},
	}
	_, err = New(*sc.Init(bufio.NewReader(f)), GetQuiltPath(), true)
	return err
}
