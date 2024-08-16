package ubinodejsextension

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// TODO
// Same struct as in images.json is on the stacks
// Also we dont need all these fields, remove the unused ones.
type StackImages struct {
	Name                    string `json:"name"`
	ConfigDir               string `json:"config_dir"`
	OutputDir               string `json:"output_dir"`
	BuildImage              string `json:"build_image"`
	RunImage                string `json:"run_image"`
	BuildReceiptFilename    string `json:"build_receipt_filename"`
	RunReceiptFilename      string `json:"run_receipt_filename"`
	CreateBuildImage        bool   `json:"create_build_image,omitempty"`
	BaseBuildContainerImage string `json:"base_build_container_image,omitempty"`
	BaseRunContainerImage   string `json:"base_run_container_image"`
	Type                    string `json:"type,omitempty"`
}

type ImagesJson struct {
	SupportUsns       bool          `json:"support_usns"`
	UpdateOnNewImage  bool          `json:"update_on_new_image"`
	ReceiptsShowLimit int           `json:"receipts_show_limit"`
	StackImages       []StackImages `json:"images"`
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

func Generate(dependencyManager DependencyManager, logger scribe.Emitter, duringBuildPermissions DuringBuildPermissions) packit.GenerateFunc {
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

		imagesJsonPath, err := os.Open("/etc/buildpacks/images.json")
		if err != nil {
			return packit.GenerateResult{}, err
		}

		var imagesJson ImagesJson
		json.NewDecoder(imagesJsonPath).Decode(&imagesJson)
		err = imagesJsonPath.Close()
		if err != nil {
			return packit.GenerateResult{}, err
		}

		nodeVersion, _ := highestPriorityNodeVersion.Metadata["version"].(string)

		nodejsRegex, _ := regexp.Compile("^nodejs")

		var dependencies []map[string]interface{}

		for _, stack := range imagesJson.StackImages {
			if !nodejsRegex.MatchString(stack.Name) {
				continue
			}

			//TODO fetch the stacks from the images.json
			nodeVersion := strings.Split(stack.Name, "-")[1]
			dependency := map[string]interface{}{
				"id":      "node",
				"stacks":  []string{"io.buildpacks.stacks.ubi8"},
				"version": fmt.Sprintf("%s.1000", nodeVersion),
				"source":  fmt.Sprintf("paketocommunity/run-nodejs-%s-ubi-base", nodeVersion),
			}
			dependencies = append(dependencies, dependency)

		}

		config := map[string]interface{}{
			"metadata": map[string]interface{}{
				"default-versions": map[string]string{
					"node": "20.*.*",
				},
				"dependencies": dependencies,
			},
		}

		buf := new(bytes.Buffer)
		if err := toml.NewEncoder(buf).Encode(config); err != nil {
			log.Fatal(err)
		}
		fmt.Println(buf.String())

		err = os.WriteFile("./config.toml", buf.Bytes(), 0744)
		if err != nil {
			return packit.GenerateResult{}, err
		}

		// Search and fetch the version from the config.toml
		configFilePath := filepath.Join("./config.toml")
		dependency, err := dependencyManager.Resolve(configFilePath, highestPriorityNodeVersion.Name, nodeVersion, context.Stack)
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

		// These variables have to be fetched from the env
		CNB_STACK_ID := os.Getenv("CNB_STACK_ID")

		// Generating build.Dockerfile
		buildDockerfileContent, err := FillPropsToTemplate(BuildDockerfileProps{
			NODEJS_VERSION: selectedNodeMajorVersion,
			CNB_USER_ID:    duringBuildPermissions.CNB_USER_ID,
			CNB_GROUP_ID:   duringBuildPermissions.CNB_GROUP_ID,
			CNB_STACK_ID:   CNB_STACK_ID,
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
