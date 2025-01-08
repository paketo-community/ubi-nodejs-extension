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

	"github.com/paketo-buildpacks/ubi-nodejs-extension/constants"
	"github.com/paketo-buildpacks/ubi-nodejs-extension/structs"

	"github.com/BurntSushi/toml"
)

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

func GenerateConfigTomlContentFromImagesJson(imagesJsonPath string, stackId string) ([]byte, error) {
	imagesJsonData, err := ParseImagesJsonFile(imagesJsonPath)
	if err != nil {
		return []byte{}, err
	}

	nodejsStacks, err := GetNodejsStackImages(imagesJsonData)
	if err != nil {
		return []byte{}, err
	}

	defaultNodeVersion, err := GetDefaultNodeVersion(nodejsStacks)
	if err != nil {
		return []byte{}, err
	}

	configTomlContent, err := CreateConfigTomlFileContent(defaultNodeVersion, nodejsStacks, stackId)
	if err != nil {
		return []byte{}, err
	}

	configTomlContentString := configTomlContent.Bytes()
	return configTomlContentString, nil
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
		CNB_USER_ID:  constants.DEFAULT_USER_ID,
		CNB_GROUP_ID: constants.DEFAULT_GROUP_ID,
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
