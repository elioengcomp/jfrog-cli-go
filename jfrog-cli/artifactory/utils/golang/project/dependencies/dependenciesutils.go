package dependencies

import (
	"bytes"
	"fmt"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils/golang"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/utils/config"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/utils/global"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	multifilereader "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils/checksum"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Collects the dependencies of the project
func CollectProjectNeededDependencies(targetRepo string, details *config.ArtifactoryDetails) (map[string]bool, error) {
	depSlice, err := golang.GetDependenciesGraph()
	if err != nil {
		return nil, err
	}

	projectDependencies, err := downloadDependencies(depSlice, details, targetRepo)
	if err != nil {
		return projectDependencies, err
	}
	return projectDependencies, nil
}

func GetRootDirAndUnsetProxyEnv(depsTidy bool) (string, error) {
	err := os.Unsetenv(golang.GOPROXY)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	var rootDir string
	if depsTidy {
		rootDir, err = golang.GetRootDir()
	}
	return rootDir, err
}

func downloadDependencies(depSlice map[string]bool, details *config.ArtifactoryDetails, targetRepo string) (map[string]bool, error) {
	client := httpclient.NewDefaultHttpClient()
	global := global.GetGlobalVariables()
	globalDependenciesMap := global.GetGlobalMap()
	dependenciesMap := map[string]bool{}
	for module := range depSlice {
		nameAndVersion := strings.Split(module, "@")
		resp, err := performHeadRequest(details, client, targetRepo, nameAndVersion[0], nameAndVersion[1])
		if err != nil {
			return dependenciesMap, err
		}

		if resp.StatusCode == 200 {
			globalDependenciesMap[getDependencyName(nameAndVersion[0])+":"+nameAndVersion[1]] = true
			err = downloadDependency(true, module, targetRepo, details)
			dependenciesMap[module] = true
		}

		if resp.StatusCode == 404 {
			globalDependenciesMap[getDependencyName(nameAndVersion[0])+":"+nameAndVersion[1]] = false
			err = downloadDependency(false, module, "", nil)
			dependenciesMap[module] = false
		}

		if err != nil {
			return dependenciesMap, err
		}
	}
	return dependenciesMap, nil
}

func performHeadRequest(details *config.ArtifactoryDetails, client *httpclient.HttpClient, targetRepo, module, version string) (*http.Response, error) {
	url := details.Url + "api/go/" + targetRepo + "/" + module + "/@v/" + version + ".mod"
	auth, err := details.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	resp, _, err := client.SendHead(url, auth.CreateHttpClientDetails())
	if err != nil {
		return nil, err
	}
	log.Debug("Artifactory head request response for", url, ":", resp.StatusCode)
	return resp, nil
}

// Creating dependency with the mod file in the temp directory
func createDependencyWithMod(tempDir string, dep Dependency) (path string, err error) {
	moduleId := dep.GetId()
	moduleInfo := strings.Split(moduleId, ":")

	moduleInfo[0] = replaceExclamationMarkWithUpperCase(moduleInfo[0])
	moduleId = strings.Join(moduleInfo, ":")
	modulePath := strings.Replace(moduleId, ":", "@", 1)
	path = filepath.Join(tempDir, modulePath, "go.mod")

	multiReader, err := multifilereader.NewMultiFileReaderAt([]string{dep.GetZipPath()})
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	err = fileutils.Unzip(multiReader, multiReader.Size(), tempDir)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	err = ioutil.WriteFile(path, dep.GetModContent(), 0700)
	if err != nil {
		return path, errorutils.CheckError(err)
	}
	return path, nil
}

func replaceExclamationMarkWithUpperCase(moduleName string) string {
	var str string
	for i := 0; i < len(moduleName); i++ {
		if string(moduleName[i]) == "!" {
			if i < len(moduleName)-1 {
				r := rune(moduleName[i+1])
				str += string(unicode.ToUpper(r))
				i++
			}
		} else {
			str += string(moduleName[i])
		}
	}
	return str
}

// Runs the go mod download command. Should set first the environment variable of GoProxy
func downloadDependency(downloadFromArtifactory bool, fullDependencyName, targetRepo string, details *config.ArtifactoryDetails) error {
	var err error
	if downloadFromArtifactory {
		log.Debug("Downloading dependency from Artifactory")
		err = golang.SetGoProxyEnvVar(details, targetRepo)
	} else {
		log.Debug("Downloading dependency from VCS", fullDependencyName)
		err = os.Unsetenv(golang.GOPROXY)
	}
	if errorutils.CheckError(err) != nil {
		return err
	}

	err = golang.DownloadDependency(fullDependencyName)
	return errorutils.CheckError(err)
}

func populateModAndGetDependenciesGraph(path string, shouldRunGoModCommand, shouldRunGoGraph bool) (output map[string]bool, err error) {
	err = os.Chdir(filepath.Dir(path))
	if errorutils.CheckError(err) != nil {
		return
	}
	log.Debug("Preparing to populate mod", filepath.Dir(path))
	// Remove go.sum file to avoid checksum conflicts with the old go.sum
	goSum := filepath.Join(filepath.Dir(path), "go.sum")
	exists, err := fileutils.IsFileExists(goSum, false)
	if err != nil {
		return
	}

	if exists {
		err = os.Remove(goSum)
		if errorutils.CheckError(err) != nil {
			return
		}
	}

	if shouldRunGoModCommand {
		// Running go mod tidy command
		err = golang.RunGoModTidy()
		if err != nil {
			return
		}
	}
	if shouldRunGoGraph {
		// Running go mod graph command
		output, err = golang.GetDependenciesGraph()
		if err != nil {
			return
		}
	}
	return
}

// Downloads the mod file from Artifactory to the Go cache
func overwriteModFileWithinCache(cachePath, targetRepo, name, version string, details *config.ArtifactoryDetails, client *httpclient.HttpClient) string {
	pathToModuleCache := filepath.Join(cachePath, name, "@v")
	dirExists, err := fileutils.IsDirExists(pathToModuleCache, false)
	if err != nil {
		log.Debug("Received and error:", err)
		return ""
	}

	if dirExists {
		url := details.Url + "api/go/" + targetRepo + "/" + name + "/@v/" + version + ".mod"
		log.Debug("Downloading mod file from Artifactory:", url)
		auth, err := details.CreateArtAuthConfig()
		if err != nil {
			log.Debug("Received and error:", err)
			return ""
		}
		downloadFileDetails := &httpclient.DownloadFileDetails{
			FileName: version + ".mod",
			// Artifactory URL
			DownloadPath:  url,
			LocalPath:     pathToModuleCache,
			LocalFileName: version + ".mod",
		}
		resp, err := client.DownloadFile(downloadFileDetails, "", auth.CreateHttpClientDetails(), 3, false)
		if err != nil {
			log.Debug("Received and error:", err)
			return ""
		}

		log.Debug(fmt.Sprintf("Received %d from Artifactory %s", resp.StatusCode, url))
		return filepath.Join(downloadFileDetails.LocalPath, downloadFileDetails.LocalFileName)
	}
	return ""
}

func GetRegex() (regExp *RegExp, err error) {
	emptyRegex, err := utils.GetRegExp(`^\s*require (?:[\(\w\.@:%_\+-.~#?&]?.+)`)
	if err != nil {
		return
	}

	indirectRegex, err := utils.GetRegExp(`(// indirect)$`)
	if err != nil {
		return
	}

	regExp = &RegExp{
		notEmptyModRegex: emptyRegex,
		indirectRegex:    indirectRegex,
	}
	return
}

func downloadAndCreateDependency(cachePath, name, version, fullDependencyName, targetRepo string, downloadedFromArtifactory bool, details *config.ArtifactoryDetails) (*Dependency, error) {
	// Dependency is missing within the cache!! need to download it...
	err := downloadDependency(downloadedFromArtifactory, fullDependencyName, targetRepo, details)
	if err != nil {
		return nil, err
	}
	// Now that this dependency in the cache, get the dependency object
	dep, err := createDependency(cachePath, name, version)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

func logErrorIfOccurred(err error) {
	if err != nil {
		log.Debug("Received and error:", err)
	}
}

func shouldDownloadFromArtifactory(module, version, targetRepo string, details *config.ArtifactoryDetails, client *httpclient.HttpClient) (bool, error) {
	res, err := performHeadRequest(details, client, targetRepo, module, version)
	if err != nil {
		return false, err
	}
	if res.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}

func loadDependencies(cachePath string) ([]Dependency, error) {
	modulesMap, err := golang.GetDependenciesGraph()
	if err != nil {
		return nil, err
	}
	if modulesMap == nil {
		return nil, nil
	}
	return GetDependencies(cachePath, modulesMap)
}

func GetDependencies(cachePath string, moduleSlice map[string]bool) ([]Dependency, error) {
	deps := []Dependency{}
	for module := range moduleSlice {
		moduleInfo := strings.Split(module, "@")
		name := getDependencyName(moduleInfo[0])
		dep, err := createDependency(cachePath, name, moduleInfo[1])
		if err != nil {
			return nil, err
		}
		if dep != nil {
			deps = append(deps, *dep)
		}
	}
	return deps, nil
}

// Returns the actual path to the dependency.
// If in the path there are capital letters, the Go convention is to use "!" before the letter.
// The letter itself in lowercase.
func getDependencyName(name string) string {
	path := ""
	for _, letter := range name {
		if unicode.IsUpper(letter) {
			path += "!" + strings.ToLower(string(letter))
		} else {
			path += string(letter)
		}
	}
	return path
}

// Creates a go dependency.
// Returns a nil value in case the dependency does not include a zip in the cache.
func createDependency(cachePath, dependencyName, version string) (*Dependency, error) {
	// We first check if the this dependency has a zip binary in the local go cache.
	// If it does not, nil is returned. This seems to be a bug in go.
	zipPath, err := getPackageZipLocation(cachePath, dependencyName, version)

	if err != nil {
		return nil, err
	}

	if zipPath == "" {
		return nil, nil
	}

	dep := Dependency{}

	dep.id = strings.Join([]string{dependencyName, version}, ":")
	dep.version = version
	dep.zipPath = zipPath
	dep.modContent, err = ioutil.ReadFile(filepath.Join(cachePath, dependencyName, "@v", version+".mod"))
	if err != nil {
		return &dep, errorutils.CheckError(err)
	}

	// Mod file dependency
	modDependency := buildinfo.Dependency{Id: dep.id}
	checksums, err := checksum.Calc(bytes.NewBuffer(dep.modContent))
	if err != nil {
		return &dep, err
	}
	modDependency.Checksum = &buildinfo.Checksum{Sha1: checksums[checksum.SHA1], Md5: checksums[checksum.MD5]}

	// Zip file dependency
	zipDependency := buildinfo.Dependency{Id: dep.id}
	fileDetails, err := fileutils.GetFileDetails(dep.zipPath)
	if err != nil {
		return &dep, err
	}
	zipDependency.Checksum = &buildinfo.Checksum{Sha1: fileDetails.Checksum.Sha1, Md5: fileDetails.Checksum.Md5}

	dep.buildInfoDependencies = append(dep.buildInfoDependencies, modDependency, zipDependency)
	return &dep, nil
}

// Returns the path to the package zip file if exists.
func getPackageZipLocation(cachePath, dependencyName, version string) (string, error) {
	zipPath, err := getPackagePathIfExists(cachePath, dependencyName, version)
	if err != nil {
		return "", err
	}

	if zipPath != "" {
		return zipPath, nil
	}

	zipPath, err = getPackagePathIfExists(filepath.Dir(cachePath), dependencyName, version)

	if err != nil {
		return "", err
	}

	return zipPath, nil
}

// Validates if the package zip file exists.
func getPackagePathIfExists(cachePath, dependencyName, version string) (zipPath string, err error) {
	zipPath = filepath.Join(cachePath, dependencyName, "@v", version+".zip")
	fileExists, err := fileutils.IsFileExists(zipPath, false)
	if err != nil {
		log.Warn(fmt.Sprintf("Could not find zip binary for dependency '%s' at %s.", dependencyName, zipPath))
		return "", err
	}
	// Zip binary does not exist, so we skip it by returning a nil dependency.
	if !fileExists {
		log.Debug("The following file is missing:", zipPath)
		return "", nil
	}
	return zipPath, nil
}

func getGOPATH() (string, error) {
	goCmd, err := golang.NewCmd()
	if err != nil {
		return "", err
	}
	goCmd.Command = []string{"env", "GOPATH"}
	output, err := utils.RunCmdOutput(goCmd)
	if err != nil {
		return "", fmt.Errorf("Could not find GOPATH env: %s", err.Error())
	}
	return strings.TrimSpace(string(output)), nil
}

type RegExp struct {
	notEmptyModRegex *regexp.Regexp
	indirectRegex    *regexp.Regexp
}

func (reg *RegExp) GetNotEmptyModRegex() *regexp.Regexp {
	return reg.notEmptyModRegex
}

func (reg *RegExp) GetIndirectRegex() *regexp.Regexp {
	return reg.indirectRegex
}