package golang

import (
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
	wd, err := os.Getwd()
	if err != nil {
		return rgf.revert(wd, err)
	}
	if !modFileExists {
		err = rgf.prepareModFile(wd, goModEditMessage)
		if err != nil {
			return err
		}
	} else {
		log.Debug("Using existing root mod file.")
		rgf.modFileContent, rgf.modFileStat, err = golang.GetFileDetails("go.mod")
		if err != nil {
			return err
		}
		rgf.shouldRevertSumFile, err = fileutils.IsFileExists("go.sum", false)
		if err != nil {
			return err
		}
		if rgf.shouldRevertSumFile {
			rgf.sumFileContent, rgf.sumFileStat, err = golang.GetFileDetails("go.sum")
			if err != nil {
				return err
			}
		}
	}

	goProject, err := project.Load("-")
	if err != nil {
		return rgf.revert(wd, err)
	}

	err = goProject.DownloadFromVcsAndPublish(targetRepo, "", goModEditMessage, true, false, details)
	if err != nil {
		if !modFileExists {
			log.Debug("Graph failed, preparing to run go mod tidy on the root project since got the following error:", err.Error())
			err = rgf.prepareAndRunTidyOnFailedGraph(wd, goProject, targetRepo, goModEditMessage, details)
			if err != nil {
				return rgf.revert(wd, err)
			}
		} else {
			return rgf.revert(wd, err)
		}
	}

	if rgf.shouldRevertModFile || rgf.shouldRevertSumFile {
		err = os.Chdir(wd)
		if err != nil {
			return rgf.revert(wd, err)
		}
		return rgf.revert(wd, nil)
	}
	return nil
}

func (rgf *rootGoFiles) prepareAndRunTidyOnFailedGraph(wd string, goProject project.Go, targetRepo, goModEditMessage string, details *config.ArtifactoryDetails) error {
	// First revert the mod to an empty mod that includes only module name
	lines := strings.Split(string(rgf.modFileContent), "\n")
	emptyMod := strings.Join(lines[:3], "\n")
	rgf.modFileContent = []byte(emptyMod)
	rgf.shouldRevertModFile = true
	err := rgf.revert(wd, nil)
	if err != nil {
		log.Error(err)
	}
	// Run go mod tidy.
	err = golang.RunGoModTidy()
	if err != nil {
		return err
	}
	// Perform collection again after tidy finished successfully.
	err = goProject.DownloadFromVcsAndPublish(targetRepo, "", goModEditMessage, true, false, details)
	if err != nil {
		return rgf.revert(wd, err)
	}
	return nil
}

func (rgf *rootGoFiles) revert(wd string, err error) error {
	if rgf.shouldRevertModFile {
		log.Debug("Reverting to original go.mod of the root project", )
		revertErr := ioutil.WriteFile("go.mod", rgf.modFileContent, rgf.modFileStat.Mode())
		if revertErr != nil {
			if err != nil {
				log.Error(revertErr)
				return err
			} else {
				return revertErr
			}
		}
	}


	if rgf.shouldRevertSumFile {
		log.Debug("Reverting to original go.sum of the root project", )
		revertErr := ioutil.WriteFile("go.sum", rgf.sumFileContent, rgf.sumFileStat.Mode())
		if revertErr != nil {
			if err != nil {
				log.Error(revertErr)
				return err
			} else {
				return revertErr
			}
		}
	}
	return nil
}

func (rgf *rootGoFiles) prepareModFile(wd, goModEditMessage string) error {
	err := golang.RunGoModInit("", goModEditMessage)
	if err != nil {
		return err
	}
	regExp, err := dependencies.GetRegex()
	if err != nil {
		return err
	}
	notEmptyModRegex := regExp.GetNotEmptyModRegex()
	rgf.modFileContent, rgf.modFileStat, err = golang.GetFileDetails("go.mod")
	if err != nil {
		return err
	}
	projectPackage := dependencies.Package{}
	projectPackage.SetModContent(rgf.modFileContent)
	packageWithDep := dependencies.PackageWithDeps{Dependency: &projectPackage}
	if !packageWithDep.PatternMatched(notEmptyModRegex) {
		log.Debug("Root mod is empty, preparing to run 'go mod tidy'")
		err = golang.RunGoModTidy()
		if err != nil {
			return rgf.revert(wd, err)
		}
		rgf.shouldRevertModFile = true
	} else {
		log.Debug("Root project mod not empty.")
	}

	return nil
}

type rootGoFiles struct {
	modFileContent      []byte
	modFileStat         os.FileInfo
	shouldRevertModFile bool
	shouldRevertSumFile bool
	sumFileContent      []byte
	sumFileStat         os.FileInfo
}
