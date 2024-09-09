package ubinodejsextension

import (
	"bytes"
	_ "embed"
	"os"
	"strings"
	"text/template"

	"github.com/paketo-community/ubi-nodejs-extension/internal/utils"
	"github.com/paketo-community/ubi-nodejs-extension/structs"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libnodejs"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/draft"
	postal "github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

const PACKAGES = "make gcc gcc-c++ libatomic_ops git openssl-devel nodejs npm nodejs-nodemon nss_wrapper which python3"
const DEFAULT_USER_ID = 1002
const DEFAULT_GROUP_ID = 1000
const CONFIG_TOML_PATH = "/tmp/config.toml"

//go:embed templates/build.Dockerfile
var buildDockerfileTemplate string

//go:embed templates/run.Dockerfile
var runDockerfileTemplate string

//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Deliver(dependency postal.Dependency, cnbPath, layerPath, platformPath string) error
	GenerateBillOfMaterials(dependencies ...postal.Dependency) []packit.BOMEntry
}

func Generate(dependencyManager DependencyManager, logger scribe.Emitter, duringBuildPermissions structs.DuringBuildPermissions, imagesJsonPath string) packit.GenerateFunc {
	return func(context packit.GenerateContext) (packit.GenerateResult, error) {

		logger.Title("%s %s", context.Info.Name, context.Info.Version)
		logger.Process("Resolving Node Engine version")

		// Find the version with the highest priority
		entryResolver := draft.NewPlanner()
		highestPriorityNodeVersion, allNodeVersionsInPriorityOrder := libnodejs.ResolveNodeVersion(entryResolver.Resolve, context.Plan)
		if highestPriorityNodeVersion.Name == "" {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Node.js no longer requested by build plan")
		}

		logger.Candidates(allNodeVersionsInPriorityOrder)

		imagesJsonData, err := utils.ParseImagesJsonFile(imagesJsonPath)
		if err != nil {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Failed to parse images.json file: %s", err)
		}

		nodejsStacks, err := utils.GetNodejsStackImages(imagesJsonData)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		defaultNodeVersion, err := utils.GetDefaultNodeVersion(nodejsStacks)

		if err != nil {
			return packit.GenerateResult{}, err
		}

		configTomlFileContent, err := utils.CreateConfigTomlFileContent(defaultNodeVersion, nodejsStacks, context.Stack)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		//save config.toml file
		err = os.WriteFile(CONFIG_TOML_PATH, configTomlFileContent.Bytes(), 0644)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		nodeVersion, _ := highestPriorityNodeVersion.Metadata["version"].(string)
		dependency, err := dependencyManager.Resolve(CONFIG_TOML_PATH, highestPriorityNodeVersion.Name, nodeVersion, context.Stack)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		selectedNodeVersion, err := semver.NewVersion(dependency.Version)
		if err != nil {
			return packit.GenerateResult{}, err
		}
		selectedNodeMajorVersion := selectedNodeVersion.Major()

		var selectedNodeRunImage string

		bpNodeRunExtension, bpNodeRunExtensionEnvExists := os.LookupEnv("BP_UBI_RUN_IMAGE_OVERRIDE")
		if !bpNodeRunExtensionEnvExists || bpNodeRunExtension == "" {
			selectedNodeRunImage = dependency.Source
		} else {
			logger.Process("Using run image specified by BP_UBI_RUN_IMAGE_OVERRIDE %s", bpNodeRunExtension)
			selectedNodeRunImage = bpNodeRunExtension
		}

		logger.Process("Selected Node Engine Major version %d", selectedNodeMajorVersion)

		// Generating build.Dockerfile
		buildDockerfileContent, err := FillPropsToTemplate(structs.BuildDockerfileProps{
			NODEJS_VERSION: selectedNodeMajorVersion,
			CNB_USER_ID:    duringBuildPermissions.CNB_USER_ID,
			CNB_GROUP_ID:   duringBuildPermissions.CNB_GROUP_ID,
			CNB_STACK_ID:   context.Stack,
			PACKAGES:       PACKAGES,
		}, buildDockerfileTemplate)

		if err != nil {
			return packit.GenerateResult{}, err
		}

		// Generating run.Dockerfile
		runDockerfileContent, err := FillPropsToTemplate(structs.RunDockerfileProps{
			Source: selectedNodeRunImage,
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
