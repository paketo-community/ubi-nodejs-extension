package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/paketo-community/ubi-nodejs-extension/structs"

	"github.com/BurntSushi/toml"
)

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
		CNB_USER_ID:  1002,
		CNB_GROUP_ID: 1000,
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
