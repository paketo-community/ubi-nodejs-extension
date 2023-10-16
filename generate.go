package ubinodejsextension

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libnodejs"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/draft"
	postal "github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

var PACKAGES = "make gcc gcc-c++ libatomic_ops git openssl-devel nodejs npm nodejs-nodemon nss_wrapper which"

var DEFAULT_USER_ID = 1001
var DEFAULT_GROUP_ID = 1000

type DuringBuildPermissions struct {
	CNB_USER_ID, CNB_GROUP_ID int
}

//go:embed templates/build.Dockerfile
var buildDockerfileTemplate string

type BuildDockerfileProps struct {
	NODEJS_VERSION            uint64
	CNB_USER_ID, CNB_GROUP_ID int
	CNB_STACK_ID, PACKAGES    string
}

//go:embed templates/run.Dockerfile
var runDockerfileTemplate string

type RunDockerfileProps struct {
	Source string
}

//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Deliver(dependency postal.Dependency, cnbPath, layerPath, platformPath string) error
	GenerateBillOfMaterials(dependencies ...postal.Dependency) []packit.BOMEntry
}

type EtcPasswdFileContentGetter func() (string, error)

type DuringBuildPermissionsGetter struct {
	get_etc_passwd_file_content EtcPasswdFileContentGetter
}

func NewDuringBuildPermissionsGetter(epfg EtcPasswdFileContentGetter) DuringBuildPermissionsGetter {
	return DuringBuildPermissionsGetter{get_etc_passwd_file_content: epfg}
}

func Generate(dependencyManager DependencyManager, logger scribe.Emitter, p DuringBuildPermissionsGetter) packit.GenerateFunc {
	return func(context packit.GenerateContext) (packit.GenerateResult, error) {
		logger.Title("%s %s", context.Info.Name, context.Info.Version)
		logger.Process("Resolving Node Engine version")

		duringBuildPermissions, err := p.duringBuildPermissionsGetter()
		if err != nil {
			return packit.GenerateResult{}, err
		}

		entryResolver := draft.NewPlanner()
		entry, allEntries := libnodejs.ResolveNodeVersion(entryResolver.Resolve, context.Plan)
		if entry.Name == "" {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Node.js no longer requested by build plan")
		}

		logger.Candidates(allEntries)
		version, _ := entry.Metadata["version"].(string)
		extensionFilePath := filepath.Join(context.CNBPath, "extension.toml")
		dependency, err := dependencyManager.Resolve(extensionFilePath, entry.Name, version, context.Stack)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		sVersion, _ := semver.NewVersion(dependency.Version)

		NODEJS_VERSION := sVersion.Major()

		logger.Process("Selected Node Engine Major version %d", NODEJS_VERSION)

		// These variables have to be fetched from the env
		CNB_STACK_ID := os.Getenv("CNB_STACK_ID")

		// Generating build.Dockerfile
		buildDockerfileContent, err := FillPropsToTemplate(BuildDockerfileProps{
			NODEJS_VERSION: NODEJS_VERSION,
			CNB_USER_ID:    duringBuildPermissions.CNB_USER_ID,
			CNB_GROUP_ID:   duringBuildPermissions.CNB_GROUP_ID,
			CNB_STACK_ID:   CNB_STACK_ID,
			PACKAGES:       "make gcc gcc-c++ libatomic_ops git openssl-devel nodejs npm nodejs-nodemon nss_wrapper which",
		}, buildDockerfileTemplate)

		if err != nil {
			return packit.GenerateResult{}, err
		}

		// Generating run.Dockerfile
		runDockerfileContent, err := FillPropsToTemplate(RunDockerfileProps{
			Source: dependency.Source,
		}, runDockerfileTemplate)

		if err != nil {
			return packit.GenerateResult{}, err
		}

		return packit.GenerateResult{
			ExtendConfig:    packit.ExtendConfig{Build: packit.ExtendImageConfig{Args: []packit.ExtendImageConfigArg{}}},
			BuildDockerfile: strings.NewReader(buildDockerfileContent),
			RunDockerfile:   strings.NewReader(runDockerfileContent),
		}, nil
	}
}

func FillPropsToTemplate(properties interface{}, templateString string) (result string, Error error) {

	templ, err := template.New("template").Parse(templateString)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = templ.Execute(&buf, properties)
	if err != nil {
		panic(err)
	}

	return buf.String(), nil
}

func (d *DuringBuildPermissionsGetter) duringBuildPermissionsGetter() (DuringBuildPermissions, error) {

	re := regexp.MustCompile(`cnb:x:(\d+):(\d+)::`)

	etcPasswdContent, err := d.get_etc_passwd_file_content()
	if err != nil {
		return DuringBuildPermissions{}, err
	}

	matches := re.FindStringSubmatch(etcPasswdContent)

	//In case of no cnb user in the /etc/passwd file
	// return default values
	if len(matches) != 3 {
		return DuringBuildPermissions{
			CNB_USER_ID:  DEFAULT_USER_ID,
			CNB_GROUP_ID: DEFAULT_GROUP_ID,
		}, nil
	}

	CNB_USER_ID, err := strconv.Atoi(matches[1])

	if err != nil {
		return DuringBuildPermissions{}, err
	}

	CNB_GROUP_ID, err := strconv.Atoi(matches[2])

	if err != nil {
		return DuringBuildPermissions{}, err
	}

	return DuringBuildPermissions{
		CNB_USER_ID:  CNB_USER_ID,
		CNB_GROUP_ID: CNB_GROUP_ID,
	}, nil
}

func Get_etc_passwd_file_content() (string, error) {

	etcPasswdFile, err := os.ReadFile(filepath.Join("/etc/passwd"))
	if err != nil {
		return "", err
	}
	return string(etcPasswdFile), nil
}
