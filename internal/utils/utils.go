package utils

import (
	_ "embed"

	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/paketo-community/ubi-nodejs-extension/structs"

	"github.com/BurntSushi/toml"
)

var DEFAULT_USER_ID = 1002
var DEFAULT_GROUP_ID = 1000

//go:embed templates/build.Dockerfile
var buildDockerfileTemplate string

//go:embed templates/run.Dockerfile
var runDockerfileTemplate string

type StackImages struct {
	Name              string `json:"name"`
	IsDefaultRunImage bool   `json:"is_default_run_image,omitempty"`
	NodeVersion       string
}

type ImagesJson struct {
	StackImages []StackImages `json:"images"`
}

func GetDefaultNodeVersion(stacks []StackImages) (string, error) {
	var defaultNodeVersionsFound []string
	for _, stack := range stacks {
		if stack.IsDefaultRunImage {
			defaultNodeVersionsFound = append(defaultNodeVersionsFound, strings.Split(stack.Name, "-")[1])
		}
	}
	if len(defaultNodeVersionsFound) == 1 {
		return defaultNodeVersionsFound[0], nil
	} else if len(defaultNodeVersionsFound) > 1 {
		return "", errors.New("multiple default node.js versions found")
	} else {
		return "", errors.New("default node.js version not found")
	}
}

func CreateConfigTomlFileContent(defaultNodeVersion string, nodejsStacks []StackImages, stackId string) (bytes.Buffer, error) {

	var dependencies []map[string]interface{}

	for _, stack := range nodejsStacks {
		dependency := map[string]interface{}{
			"id":      "node",
			"stacks":  []string{stackId},
			"version": fmt.Sprintf("%s.1000", stack.NodeVersion),
			"source":  fmt.Sprintf("paketocommunity/run-nodejs-%s-ubi-base", stack.NodeVersion),
		}
		dependencies = append(dependencies, dependency)
	}

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
		return bytes.Buffer{}, err
	}

	return *buf, nil
}

func GetNodejsStackImages(imagesJsonData ImagesJson) ([]StackImages, error) {

	// Filter out the nodejs stacks based on the stack name
	nodejsRegex, _ := regexp.Compile("^nodejs")

	nodejsStacks := []StackImages{}
	for _, stack := range imagesJsonData.StackImages {

		if nodejsRegex.MatchString(stack.Name) {
			//Extract the node version from the stack name
			extractedNodeVersion := strings.Split(stack.Name, "-")[1]

			_, err := strconv.Atoi(extractedNodeVersion)
			if err != nil {
				return []StackImages{}, fmt.Errorf("extracted Node.js version [%s] for stack %s is not an integer", extractedNodeVersion, stack.Name)
			}

			stack.NodeVersion = extractedNodeVersion

			nodejsStacks = append(nodejsStacks, stack)
		}
	}
	if len(nodejsStacks) == 0 {
		return []StackImages{}, errors.New("no nodejs stacks found")
	}

	return nodejsStacks, nil
}

func ParseImagesJsonFile(imagesJsonPath string) (ImagesJson, error) {
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

func GetDuringBuildPermissions(filepath string) structs.DuringBuildPermissions {

	defaultPermissions := structs.DuringBuildPermissions{
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

	return structs.DuringBuildPermissions{
		CNB_USER_ID:  CNB_USER_ID,
		CNB_GROUP_ID: CNB_GROUP_ID,
	}
}

func GenerateBuildDockerfile(buildProps structs.BuildDockerfileProps) (result string, Error error) {

	result, err := fillPropsToTemplate(buildProps, buildDockerfileTemplate)

	if err != nil {
		return "", err
	}

	return result, nil
}

func GenerateRunDockerfile(runProps structs.RunDockerfileProps) (result string, Error error) {

	result, err := fillPropsToTemplate(runProps, runDockerfileTemplate)

	if err != nil {
		return "", err
	}
	return result, nil
}

func fillPropsToTemplate(properties interface{}, templateString string) (result string, Error error) {

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

func GenerateImagesJsonFile(nodeVersions []string, isDefault []bool, isCorrupted bool) string {
	addStacks := ""

	for i, nodeVersion := range nodeVersions {

		addStacks += fmt.Sprintf(`,
      {
        "name": "nodejs-%s",
        "is_default_run_image": %t,
        "config_dir": "stack-nodejs-%s",
        "output_dir": "build-nodejs-%s",
        "build_image": "build-nodejs-%s",
        "run_image": "run-nodejs-%s",
        "build_receipt_filename": "build-nodejs-%s-receipt.cyclonedx.json",
        "run_receipt_filename": "run-nodejs-%s-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/nodejs-%s-runtime"
      }`, nodeVersion, isDefault[i], nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion)
	}

	if isCorrupted {
		addStacks += `,
		{
			"name": "nodejs-18",}
			not a valid json
		}`
	}

	stacks := fmt.Sprintf(`{
    "support_usns": false,
    "update_on_new_image": true,
    "receipts_show_limit": 16,
    "images": [
      {
        "name": "default",
        "config_dir": "stack",
        "output_dir": "build",
        "build_image": "build",
        "run_image": "run",
        "build_receipt_filename": "build-receipt.cyclonedx.json",
        "run_receipt_filename": "run-receipt.cyclonedx.json",
        "create_build_image": true,
        "base_build_container_image": "docker://registry.access.redhat.com/ubi8/ubi-minimal",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/ubi-minimal"
      },
      {
        "name": "java-17",
        "config_dir": "stack-java-17",
        "output_dir": "build-java-17",
        "build_image": "build-java-17",
        "run_image": "run-java-17",
        "build_receipt_filename": "build-java-17-receipt.cyclonedx.json",
        "run_receipt_filename": "run-java-17-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/openjdk-17-runtime"
      },
      {
        "name": "java-21",
        "config_dir": "stack-java-21",
        "output_dir": "build-java-21",
        "build_image": "build-java-21",
        "run_image": "run-java-21",
        "build_receipt_filename": "build-java-21-receipt.cyclonedx.json",
        "run_receipt_filename": "run-java-21-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/openjdk-21-runtime"
      }%s
    ]
  }
`, addStacks)

	return stacks
}
