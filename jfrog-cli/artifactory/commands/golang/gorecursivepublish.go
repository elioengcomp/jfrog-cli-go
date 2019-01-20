package golang

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils/golang"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils/golang/project"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils/golang/project/dependencies"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"os"
	"strings"
)

func Execute(targetRepo, goModEditMessage string, details *config.ArtifactoryDetails) error {
	rgf := rootGoFiles{}
	modFileExists, err := fileutils.IsFileExists("go.mod", false)
	if err != nil {
		return err
	}
	shouldRevertMod := false
	wd, err := os.Getwd()
	if err != nil {
		return rgf.revert(wd, err)
	}
	if !modFileExists {
		shouldRevertMod, err = rgf.prepareModFile(wd, goModEditMessage)
		if err != nil {
			return err
		}
	} else {
		log.Debug("Using existing root mod file.")
		rgf.modFileContent, rgf.modFileStat, err = golang.GetFileDetails("go.mod")
		if err != nil {
			return err
		}
	}

	goProject, err := project.Load("-")
	if err != nil {
		return rgf.revert(wd, err)
	}

	err = goProject.DownloadFromVcsAndPublish(targetRepo, "", goModEditMessage, true, false, details)
	if err != nil {
		if !modFileExists {
			// Now lets revert to empty mod.
			modContent := string(rgf.modFileContent)
			lines := strings.Split(modContent, "\n")
			//result := lines[0] + "\n" + lines[1] + "\n" + lines[2] + "\n"
			mod := strings.Join(lines[:3], "\n")
			fmt.Println(mod)
			rgf.modFileContent = []byte(mod)
		}
		return rgf.revert(wd, err)
	}
	if shouldRevertMod {
		err = os.Chdir(wd)
		if err != nil {
			return rgf.revert(wd, err)
		}
		return rgf.revert(wd, err)
	}

	return nil
}

func (rgf *rootGoFiles) revert(wd string, err error) error {
	log.Debug("Reverting to original go.mod of the root project", )
	revertErr := ioutil.WriteFile("go.mod", rgf.modFileContent, rgf.modFileStat.Mode())
	if revertErr != nil && err != nil {
		log.Error(revertErr)
		return err
	}
	return revertErr
}

func (rgf *rootGoFiles) prepareModFile(wd, goModEditMessage string) (bool, error) {
	err := golang.RunGoModInit("", goModEditMessage)
	if err != nil {
		return false, err
	}
	regExp, err := dependencies.GetRegex()
	if err != nil {
		return false, err
	}
	notEmptyModRegex := regExp.GetNotEmptyModRegex()
	rgf.modFileContent, rgf.modFileStat, err = golang.GetFileDetails("go.mod")
	if err != nil {
		return false, err
	}
	projectPackage := dependencies.Package{}
	projectPackage.SetModContent(rgf.modFileContent)
	packageWithDep := dependencies.PackageWithDeps{Dependency: &projectPackage}
	shouldRevertMod := false
	if !packageWithDep.PatternMatched(notEmptyModRegex) {
		log.Debug("Root mod is empty, preparing to run 'go mod tidy'")
		err = golang.RunGoModTidy()
		if err != nil {
			return false, rgf.revert(wd, err)
		}
		shouldRevertMod = true
	} else {
		log.Debug("Root project mod not empty.")
	}

	return shouldRevertMod, nil
}

type rootGoFiles struct {
	modFileContent []byte
	modFileStat    os.FileInfo
}
