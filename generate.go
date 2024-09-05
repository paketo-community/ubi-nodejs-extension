package ubinodejsextension

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
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

type StackImages struct {
	Name              string `json:"name"`
	Type              string `json:"type,omitempty"`
	IsDefaultRunImage bool   `json:"is_default_run_image,omitempty"`
	nodeVersion       string
}

type ImagesManager struct {
	imagesJsonPath string
}

type ImagesJson struct {
	StackImages []StackImages `json:"images"`
}

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

func Generate(dependencyManager DependencyManager, logger scribe.Emitter, duringBuildPermissions DuringBuildPermissions, imagesManager ImagesManager) packit.GenerateFunc {
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

		imagesJsonData, err := parseImagesJsonFile(imagesManager.imagesJsonPath)
		if err != nil {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Failed to parse images.json file: %s", err)
		}

		nodejsStacks, err := getNodejsStackImages(imagesJsonData)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		defaultNodeVersion := getDefaultNodeVersion(nodejsStacks)

		if defaultNodeVersion == "" {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Default Node.js version not found")
		}

		configTomlPath, err := createConfigTomlFile(defaultNodeVersion, nodejsStacks, context.Stack)
		if err != nil {
			return packit.GenerateResult{}, packit.Fail.WithMessage("Failed to create config.toml file: %s", err)
		}

		nodeVersion, _ := highestPriorityNodeVersion.Metadata["version"].(string)
		dependency, err := dependencyManager.Resolve(configTomlPath, highestPriorityNodeVersion.Name, nodeVersion, context.Stack)
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
		buildDockerfileContent, err := FillPropsToTemplate(BuildDockerfileProps{
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
		runDockerfileContent, err := FillPropsToTemplate(RunDockerfileProps{
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

func getDefaultNodeVersion(stacks []StackImages) string {
	for _, stack := range stacks {
		if stack.IsDefaultRunImage {
			return strings.Split(stack.Name, "-")[1]
		}
	}
	return ""
}

func GetDuringBuildPermissions(filepath string) DuringBuildPermissions {

	defaultPermissions := DuringBuildPermissions{
		CNB_USER_ID:  DEFAULT_USER_ID,
		CNB_GROUP_ID: DEFAULT_GROUP_ID,
	}
	re := regexp.MustCompile(`cnb:x:(\d+):(\d+)::`)

	etcPasswdFile, err := os.ReadFile(filepath)

	if err != nil {
		return defaultPermissions
	}
	etcPasswdContent := string(etcPasswdFile)

	matches := re.FindStringSubmatch(etcPasswdContent)

	if len(matches) != 3 {
		return defaultPermissions
	}

	CNB_USER_ID, err := strconv.Atoi(matches[1])

	if err != nil {
		return defaultPermissions
	}

	CNB_GROUP_ID, err := strconv.Atoi(matches[2])

	if err != nil {
		return defaultPermissions
	}

	return DuringBuildPermissions{
		CNB_USER_ID:  CNB_USER_ID,
		CNB_GROUP_ID: CNB_GROUP_ID,
	}
}

func NewImagesManager(imagesJsonPath string) ImagesManager {
	return ImagesManager{
		imagesJsonPath: imagesJsonPath,
	}
}

func parseImagesJsonFile(imagesJsonPath string) (ImagesJson, error) {
	filepath, err := os.Open(imagesJsonPath)
	if err != nil {
		return ImagesJson{}, err
	}

	defer filepath.Close()

	var imagesJsonData ImagesJson
	err = json.NewDecoder(filepath).Decode(&imagesJsonData)
	if err != nil {
		return ImagesJson{}, err
	}

	return imagesJsonData, nil
}

func createConfigTomlFile(defaultNodeVersion string, nodejsStacks []StackImages, stackId string) (string, error) {

	var dependencies []map[string]interface{}

	for _, stack := range nodejsStacks {
		dependency := map[string]interface{}{
			"id":      "node",
			"stacks":  []string{stackId},
			"version": fmt.Sprintf("%s.1000", stack.nodeVersion),
			"source":  fmt.Sprintf("paketocommunity/run-nodejs-%s-ubi-base", stack.nodeVersion),
		}
		dependencies = append(dependencies, dependency)
	}

	configTomlPath := "./config.toml"
	config := map[string]interface{}{
		"metadata": map[string]interface{}{
			"default-versions": map[string]string{
				"node": fmt.Sprintf("%s.*.*", defaultNodeVersion),
			},
			"dependencies": dependencies,
		},
	}

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(config); err != nil {
		return "", err
	}

	err := os.WriteFile(configTomlPath, buf.Bytes(), 0744)
	if err != nil {
		return "", err
	}

	return configTomlPath, nil
}

func getNodejsStackImages(imagesJsonData ImagesJson) ([]StackImages, error) {

	// Filter out the nodejs stacks based on the stack name
	nodejsRegex, _ := regexp.Compile("^nodejs")

	nodejsStacks := []StackImages{}
	for _, stack := range imagesJsonData.StackImages {

		if nodejsRegex.MatchString(stack.Name) {
			//Extract the node version from the stack name
			stack.nodeVersion = strings.Split(stack.Name, "-")[1]

			if stack.nodeVersion == "" {
				packit.Fail.WithMessage("Node.js version for stack %s not found", stack.Name)
			}

			nodejsStacks = append(nodejsStacks, stack)
		}
	}
	return nodejsStacks, nil
}
