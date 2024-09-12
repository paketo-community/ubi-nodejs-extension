package utils_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitUtils(t *testing.T) {
	suite := spec.New("utils-ubi-nodejs-extension", spec.Report(report.Terminal{}))
	suite("GenerateConfigTomlContentFromImagesJson", testGenerateConfigTomlContentFromImagesJson)
	suite("GetDefaultNodeVersion", testGetDefaultNodeVersion)
	suite("CreateConfigTomlFileContent", testCreateConfigTomlFileContent)
	suite("ParseImagesJsonFile", testParseImagesJsonFile)
	suite("GetNodejsStackImages", testGetNodejsStackImages)
	suite("GetDuringBuildPermissions", testGetDuringBuildPermissions)
	suite("testGenerateBuildDockerfile", testGenerateBuildDockerfile)
	suite("testGenerateRunDockerfile", testGenerateRunDockerfile)
	suite.Run(t)
}
