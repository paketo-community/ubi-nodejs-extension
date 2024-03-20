package ubinodejsextension_test

import (
	"testing"

	ubinodejsextension "github.com/paketo-community/ubi-nodejs-extension"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/sclevine/spec"
)

var expectedDetectBuildPlan packit.BuildPlan = packit.BuildPlan{
	Provides: []packit.BuildPlanProvision{
		{Name: "node"},
	},
	Or: []packit.BuildPlan{
		{
			Provides: []packit.BuildPlanProvision{
				{Name: "node"},
				{Name: "npm"},
			},
		},
	},
}

func testDetect(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect       = NewWithT(t).Expect
		err          error
		detectResult packit.DetectResult
	)

	it("it returns a plan that provides node and/or npm", func() {
		detectResult, err = ubinodejsextension.Detect()(packit.DetectContext{
			WorkingDir: "/working-dir",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
	})
}
