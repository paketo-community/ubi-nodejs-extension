package ubinodejsextension_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitNode(t *testing.T) {
	suite := spec.New("node", spec.Report(report.Terminal{}))
	suite("Detect", testDetect)
	suite("Generate", testGenerate)
	suite("Dockerfile Creation", testFillPropsToTemplate)
	suite.Run(t)
}
