package golang

import (
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-go/jfrog-cli/artifactory/utils"
	"github.com/jfrog/jfrog-cli-go/jfrog-cli/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/mattn/go-shellwords"
)

const GOPROXY = "GOPROXY"

func NewCmd() (*Cmd, error) {
	execPath, err := exec.LookPath("go")
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &Cmd{Go: execPath}, nil
}

func (config *Cmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.Go)
	cmd = append(cmd, config.Command...)
	cmd = append(cmd, config.CommandFlags...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (config *Cmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *Cmd) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *Cmd) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

type Cmd struct {
	Go           string
	Command      []string
	CommandFlags []string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func SetGoProxyEnvVar(artifactoryDetails *config.ArtifactoryDetails, repoName string) error {
	rtUrl, err := url.Parse(artifactoryDetails.Url)
	if err != nil {
		return err
	}
	rtUrl.User = url.UserPassword(artifactoryDetails.User, artifactoryDetails.Password)
	rtUrl.Path += "api/go/" + repoName

	err = os.Setenv(GOPROXY, rtUrl.String())
	return err
}

func GetGoVersion() ([]byte, error) {
	goCmd, err := NewCmd()
	if err != nil {
		return nil, err
	}
	goCmd.Command = []string{"version"}
	output, err := utils.RunCmdOutput(goCmd)
	return output, err
}

func RunGo(goArg string) error {
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}

	goCmd.Command, err = shellwords.Parse(goArg)
	if err != nil {
		return errorutils.CheckError(err)
	}

	regExp, err := utils.GetRegExp(`((http|https):\/\/\w.*?:\w.*?@)`)
	if err != nil {
		return err
	}

	protocolRegExp := utils.CmdOutputPattern{
		RegExp: regExp,
	}
	protocolRegExp.ExecFunc = protocolRegExp.MaskCredentials

	regExp, err = utils.GetRegExp("(404 Not Found)")
	if err != nil {
		return err
	}

	notFoundRegExp := utils.CmdOutputPattern{
		RegExp: regExp,
	}
	notFoundRegExp.ExecFunc = notFoundRegExp.ErrorOnNotFound
	return utils.RunCmdWithOutputParser(goCmd, &protocolRegExp, &notFoundRegExp)
}

// Using go mod download {dependency} command to download the dependency
func DownloadDependency(dependencyName string) error {
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}
	log.Debug("Running go mod download -json", dependencyName)
	goCmd.Command = []string{"mod", "download", "-json", dependencyName}

	retryExecutor := clientutils.RetryExecutor{
		MaxRetries:      2,
		RetriesInterval: 10,
		ErrorMessage:    "Download module operation timed out",
		ExecutionHandler: func() (bool, error) {
			output, err := utils.RunCmdOutput(goCmd)
			if err != nil {
				// Retry on timeout
				strOutput := string(output)
				if strings.Contains(strOutput, "operation timed out") ||
					strings.Contains(strOutput, "i/o timeout") {
					return true, err
				} else {
					return false, err
				}
			}
			return false, nil
		},
	}

	return retryExecutor.Execute()
}

// Runs go mod graph command and returns slice of the dependencies
func GetDependenciesGraph() (map[string]bool, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	projectDir, err := GetProjectRoot()
	if err != nil {
		return nil, err
	}

	// Read and store the details of the go.mod and go.sum files,
	// because they may change by the "go mod graph" command.
	modFileContent, modFileStat, err := GetFileDetails(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return nil, err
	}
	sumFileContent, sumFileStat, err := GetSumContentAndRemove(projectDir)
	if len(sumFileContent) > 0 && sumFileStat != nil {
		defer RestoreSumFile(projectDir, sumFileContent, sumFileStat)
	}

	log.Info("Running 'go mod graph' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return nil, err
	}
	goCmd.Command = []string{"mod", "graph"}

	output, err := utils.RunCmdOutput(goCmd)
	if len(output) != 0 {
		log.Debug(string(output))
	}

	if err != nil {
		// If the command fails, the mod stays the same.
		return nil, err
	}

	// Restore the the go.mod and go.sum files, to make sure they stay the same as before
	// running the "go mod graph" command.
	err = ioutil.WriteFile(filepath.Join(projectDir, "go.mod"), modFileContent, modFileStat.Mode())
	if err != nil {
		return nil, err
	}
	return outputToMap(string(output)), errorutils.CheckError(err)
}

func GetFileDetails(filePath string) (modFileContent []byte, modFileStat os.FileInfo, err error) {
	modFileStat, err = os.Stat(filePath)
	if errorutils.CheckError(err) != nil {
		return
	}
	modFileContent, err = ioutil.ReadFile(filePath)
	errorutils.CheckError(err)
	return
}

func outputToMap(output string) map[string]bool {
	lineOutput := strings.Split(output, "\n")
	var result []string
	mapOfDeps := map[string]bool{}
	for _, line := range lineOutput {
		splitLine := strings.Split(line, " ")
		if len(splitLine) == 2 {
			mapOfDeps[splitLine[1]] = true
			result = append(result, splitLine[1])
		}
	}
	return mapOfDeps
}

// Using go mod download command to download all the dependencies before publishing to Artifactory
func RunGoModTidy() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	log.Info("Running 'go mod tidy' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}

	goCmd.Command = []string{"mod", "tidy"}
	_, err = utils.RunCmdOutput(goCmd)
	return err
}

func RunGoModInit(moduleName string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	log.Info("Running 'go mod init' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}

	goCmd.Command = []string{"mod", "init", moduleName}
	_, err = utils.RunCmdOutput(goCmd)
	return err
}

// Returns the root dir where the go.mod located.
func GetProjectRoot() (string, error) {
	// Create a map to store all paths visited, to avoid running in circles.
	visitedPaths := make(map[string]bool)
	// Get the current directory.
	wd, err := os.Getwd()
	if err != nil {
		return wd, errorutils.CheckError(err)
	}
	defer os.Chdir(wd)

	// Get the OS root.
	osRoot := os.Getenv("SYSTEMDRIVE")
	if osRoot != "" {
		// If this is a Windows machine:
		osRoot += "\\"
	} else {
		// Unix:
		osRoot = "/"
	}

	// Check if the current directory includes the go.mod file. If not, check the parent directpry
	// and so on.
	for {
		// If the go.mod is found the current directory, return the path.
		exists, err := fileutils.IsFileExists(filepath.Join(wd, "go.mod"), false)
		if err != nil || exists {
			return wd, err
		}

		// If this the OS root, we can stop.
		if wd == osRoot {
			break
		}

		// Save this path.
		visitedPaths[wd] = true
		// CD to the parent directory.
		wd = filepath.Dir(wd)
		os.Chdir(wd)

		// If we already visited this directory, it means that there's a loop and we can stop.
		if visitedPaths[wd] {
			return "", errorutils.CheckError(errors.New("Could not find go.mod for project."))
		}
	}

	return "", errorutils.CheckError(errors.New("Could not find go.mod for project."))
}

func GetSumContentAndRemove(rootProjectDir string) (sumFileContent []byte, sumFileStat os.FileInfo, err error) {
	sumFileExists, err := fileutils.IsFileExists(filepath.Join(rootProjectDir, "go.sum"), false)
	if err != nil {
		return
	}
	if sumFileExists {
		log.Debug("Sum file exists:", rootProjectDir)
		sumFileContent, sumFileStat, err = GetFileDetails("go.sum")
		if err != nil {
			return
		}
		log.Debug("Removing file:", filepath.Join(rootProjectDir, "go.sum"))
		err = os.Remove(filepath.Join(rootProjectDir, "go.sum"))
		if err != nil {
			return
		}
		return
	} else {
		log.Debug("Sum file doesn't exists for:", rootProjectDir)
	}
	return
}

func RestoreSumFile(rootProjectDir string, sumFileContent []byte, sumFileStat os.FileInfo) error {
	log.Debug("Restoring file:", filepath.Join(rootProjectDir, "go.sum"))
	err := ioutil.WriteFile(filepath.Join(rootProjectDir, "go.sum"), sumFileContent, sumFileStat.Mode())
	if err != nil {
		return err
	}
	return nil
}
