package ubinodejsextension_test

import (
	"testing"

	ubinodejsextension "github.com/paketo-community/ubi-nodejs-extension"

	//	"github.com/paketo-buildpacks/packit/v2"
	"os"
	"path/filepath"

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

var expectedNotDetectBuildPlan packit.BuildPlan = packit.BuildPlan{
	Provides: nil,
}

func testDetect(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect        = NewWithT(t).Expect
		workingDir    string
		detectContext packit.DetectContext
		err           error
		detectResult  packit.DetectResult
	)

	context("when no application is detected", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("indicates it does not participate", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).To(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedNotDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when an application is auto detected in the default working dir", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "server.js"), nil, 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when an application is auto detected in directory set by BP_NODE_PROJECT_PATH", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			t.Setenv("BP_NODE_PROJECT_PATH", "./src")
			Expect(os.MkdirAll(filepath.Join(workingDir, "src"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "src", "server.js"), nil, 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when an application is detected based on BP_LAUNCHPOINT in the default working dir", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			t.Setenv("BP_LAUNCHPOINT", "not_a_known_name.js")
			Expect(os.WriteFile(filepath.Join(workingDir, "not_a_known_name.js"), nil, 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when an application is detected based on BP_LAUNCHPOINT in directory set by BP_NODE_PROJECT_PATH", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			t.Setenv("BP_NODE_PROJECT_PATH", "./src")
			t.Setenv("BP_LAUNCHPOINT", "./src/not_a_known_name.js")
			Expect(os.MkdirAll(filepath.Join(workingDir, "src"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "src", "not_a_known_name.js"), nil, 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when there is a package.json without a start script and no application", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"), []byte(`{
				"scripts": {
						"prestart":  "npm run lint",
						"poststart": "npm run test"
				}
			}`), 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("indicates it does not participate", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).To(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedNotDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when there is a package.json with start script in default directory", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"), []byte(`{
				"scripts": {
						"start":  "node server.js"
				}
			}`), 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when there is a package.json with start script in BP_NODE_PROJECT_PATH", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			t.Setenv("BP_NODE_PROJECT_PATH", "./src")
			Expect(os.MkdirAll(filepath.Join(workingDir, "src"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "src", "package.json"), []byte(`{
				"scripts": {
						"start":  "node server.js"
				}
			}`), 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

	context("when there is a package.json without a start script, with application", func() {
		it.Before(func() {
			workingDir = t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"), []byte(`{
				"scripts": {
						"prestart":  "npm run lint",
						"poststart": "npm run test"
				}
			}`), 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "server.js"), []byte(`dummy`), 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), nil, 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
			detectContext = packit.DetectContext{
				WorkingDir: workingDir,
			}
		})

		it("detects", func() {
			detectResult, err = ubinodejsextension.Detect()(detectContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectResult.Plan).To(Equal(expectedDetectBuildPlan))
		})
	}, spec.Sequential())

}
