package ubinodejsextension_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitUbiNodejsExtension(t *testing.T) {
	suite := spec.New("ubi-nodejs-extension", spec.Report(report.Terminal{}))
	suite("Detect", testDetect)
	suite("Generate", testGenerate)
	suite.Run(t)
}
