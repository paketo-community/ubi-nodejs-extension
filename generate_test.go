package ubinodejsextension_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/packit/cargo"
	"github.com/paketo-buildpacks/packit/v2"
	ubinodejsextension "github.com/paketo-community/ubi-nodejs-extension"
	"github.com/paketo-community/ubi-nodejs-extension/internal/utils"
	"github.com/paketo-community/ubi-nodejs-extension/structs"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/packit/v2/scribe"

	"github.com/BurntSushi/toml"
	postal "github.com/paketo-buildpacks/packit/v2/postal"
)

func testGenerate(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect            = NewWithT(t).Expect
		workingDir        string
		imagesJsonTmpDir  string
		imagesJsonPath    string
		planPath          string
		testBuildPlan     packit.BuildpackPlan
		buf               = new(bytes.Buffer)
		generateResult    packit.GenerateResult
		err               error
		generate          packit.GenerateFunc
		buffer            *bytes.Buffer
		logger            scribe.Emitter
		dependencyManager postal.Service
	)

	it.Before(func() {
		buffer = bytes.NewBuffer(nil)
		logger = scribe.NewEmitter(buffer)
		dependencyManager = postal.NewService(cargo.NewTransport())
	})

	context("Generate called with NO node in build plan", func() {
		it.Before(func() {
			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				"/path/to/images.json")

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			workingDir := t.TempDir()
			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		it("the extension should exit with an error", func() {

			generateResult, err = generate(packit.GenerateContext{
				WorkingDir: workingDir,
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{},
				},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Node.js no longer requested by build plan"))
			Expect(generateResult).To(Equal(packit.GenerateResult{}))
		})
	}, spec.Sequential())

	context("Generate called with node in the build plan", func() {
		it.Before(func() {

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			workingDir = t.TempDir()

			planPath = filepath.Join(workingDir, "plan")
			t.Setenv("CNB_BP_PLAN_PATH", planPath)

			Expect(os.WriteFile(planPath, buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(imagesJsonTmpDir)).To(Succeed())
		})

		it("Specific version of node requested", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			versionTests := []struct {
				requestedNodeVersion string
				expectedNodeVersion  int
			}{
				{
					requestedNodeVersion: "16 - 18",
					expectedNodeVersion:  18,
				},
				{
					requestedNodeVersion: "16.0.0 - 18.0.0",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "<18.5.1",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: ">18.5.1",
					expectedNodeVersion:  18,
				},
				{
					requestedNodeVersion: "16 <18.5.1",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "<1.0.0 || >=2.5.2 <3.0.0 || >=2.3.1 <18.4.5",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "v18",
					expectedNodeVersion:  18,
				},
				{
					requestedNodeVersion: "16",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "18",
					expectedNodeVersion:  18,
				},
				{
					requestedNodeVersion: "~16",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "^18.0.x",
					expectedNodeVersion:  18,
				},
			}

			for _, tt := range versionTests {

				buildplan := packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node",
							Metadata: map[string]interface{}{
								"version":        tt.requestedNodeVersion,
								"version-source": "BP_NODE_VERSION",
							},
						},
					},
				}

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan:       buildplan,
					Stack:      "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerFileProps := structs.RunDockerfileProps{
					Source: fmt.Sprintf("paketocommunity/run-nodejs-%d-ubi-base", tt.expectedNodeVersion),
				}
				runDockerfileContent, _ := utils.GenerateRunDockerfile(runDockerFileProps)

				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := utils.GenerateBuildDockerfile(buildDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))

			}
		})

		it("should return the default when node version has NOT been requested", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{true, false}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			versionTests := []struct {
				requestedNodeVersion string
				versionSource        string
				Metadata             map[string]interface{}
				expectedNodeVersion  int
			}{
				{
					requestedNodeVersion: "",
					versionSource:        "",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "",
					versionSource:        "BP_NODE_VERSION",
					expectedNodeVersion:  16,
				},
				{
					requestedNodeVersion: "x",
					versionSource:        "BP_NODE_VERSION",
					expectedNodeVersion:  18,
				},
			}

			for _, tt := range versionTests {

				buildplan := packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node",
							Metadata: map[string]interface{}{
								"version":        tt.requestedNodeVersion,
								"version-source": tt.versionSource,
							},
						},
					},
				}

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan:       buildplan,
					Stack:      "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerFileProps := structs.RunDockerfileProps{
					Source: fmt.Sprintf("paketocommunity/run-nodejs-%d-ubi-base", tt.expectedNodeVersion),
				}

				runDockerfileContent, _ := utils.GenerateRunDockerfile(runDockerFileProps)
				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := utils.GenerateBuildDockerfile(buildDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}

		})

		it("should return the higher node version when it requests for >=nodeVersion", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			versionTests := []struct {
				requestedNodeVersion string
				expectedNodeVersion  int
			}{
				{
					requestedNodeVersion: ">16",
					expectedNodeVersion:  18,
				},
				{
					requestedNodeVersion: ">13",
					expectedNodeVersion:  18,
				},
			}

			for _, tt := range versionTests {
				buildplan := packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node",
							Metadata: map[string]interface{}{
								"version":        tt.requestedNodeVersion,
								"version-source": "BP_NODE_VERSION",
							},
						},
					},
				}

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan:       buildplan,
					Stack:      "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerFileProps := structs.RunDockerfileProps{
					Source: fmt.Sprintf("paketocommunity/run-nodejs-%d-ubi-base", tt.expectedNodeVersion),
				}
				runDockerfileContent, _ := utils.GenerateRunDockerfile(runDockerFileProps)

				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := utils.GenerateBuildDockerfile(buildDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}
		})

		it("Should error on below cases of requested node", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			versionTests := []struct {
				requestedNodeVersion string
			}{
				{
					requestedNodeVersion: "17 - 18.0.0",
				},
				{
					requestedNodeVersion: "15",
				},
				{
					requestedNodeVersion: "18.0.0",
				},
				{
					requestedNodeVersion: "v18.999.0",
				},
				{
					requestedNodeVersion: ">18",
				},
				{
					requestedNodeVersion: "~16.2",
				},
				{
					requestedNodeVersion: "16.5.x",
				},
			}

			for _, tt := range versionTests {

				buildplan := packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node",
							Metadata: map[string]interface{}{
								"version":        tt.requestedNodeVersion,
								"version-source": "BP_NODE_VERSION",
							},
						},
					},
				}

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan:       buildplan,
					Stack:      "io.buildpacks.stacks.ubi8",
				})

				Expect(err).To(HaveOccurred())
			}
		})

	}, spec.Sequential())

	context("Getting from detect phase the Node.js versions combined with the source", func() {

		it.Before(func() {

			workingDir = t.TempDir()

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			planPath = filepath.Join(workingDir, "plan")
			t.Setenv("CNB_BP_PLAN_PATH", planPath)

			Expect(os.WriteFile(planPath, buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(imagesJsonTmpDir)).To(Succeed())
		})

		it("Should respect the priorities and return the proper Node.js version", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			entriesTests := []struct {
				Entries            []packit.BuildpackPlanEntry
				RunDockerfileProps structs.RunDockerfileProps
			}{
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "<0", "version-source": ".node-version"},
						},
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "<0", "version-source": ".nvmrc"},
						},
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "<0", "version-source": "package.json"},
						},
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "=16", "version-source": "BP_NODE_VERSION"},
						},
					},
					RunDockerfileProps: structs.RunDockerfileProps{
						Source: "paketocommunity/run-nodejs-16-ubi-base",
					},
				},
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "<0", "version-source": ".node-version"},
						},
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "<0", "version-source": ".nvmrc"},
						},
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "=16", "version-source": "package.json"},
						},
					},
					RunDockerfileProps: structs.RunDockerfileProps{
						Source: "paketocommunity/run-nodejs-16-ubi-base",
					},
				},
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "=16", "version-source": ".node-version"},
						},
					},
					RunDockerfileProps: structs.RunDockerfileProps{
						Source: "paketocommunity/run-nodejs-16-ubi-base",
					},
				},
			}

			for _, tt := range entriesTests {

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerfileContent, _ := utils.GenerateRunDockerfile(tt.RunDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}

		})

	}, spec.Sequential())

	context("When BP_UBI_RUN_IMAGE_OVERRIDE env has been set", func() {

		it.Before(func() {
			workingDir = t.TempDir()

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			planPath = filepath.Join(workingDir, "plan")
			t.Setenv("CNB_BP_PLAN_PATH", planPath)

			Expect(os.WriteFile(planPath, buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		it("Should have the same value as the BP_UBI_RUN_IMAGE_OVERRIDE if is not empty string", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			entriesTests := []struct {
				Entries                   []packit.BuildpackPlanEntry
				BP_UBI_RUN_IMAGE_OVERRIDE string
			}{
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": ">0", "version-source": ".node-version"},
						},
					},
					BP_UBI_RUN_IMAGE_OVERRIDE: "testregistry/image-name",
				},
			}

			for _, tt := range entriesTests {
				t.Setenv("BP_UBI_RUN_IMAGE_OVERRIDE", tt.BP_UBI_RUN_IMAGE_OVERRIDE)

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				RunDockerfileProps := structs.RunDockerfileProps{
					Source: tt.BP_UBI_RUN_IMAGE_OVERRIDE,
				}

				runDockerfileContent, _ := utils.GenerateRunDockerfile(RunDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}
		})

		it("Should fallback to the run image which corresponds to the selected node version during build", func() {

			imagesJsonContent := utils.GenerateImagesJsonFile([]string{"16", "18"}, []bool{false, true}, false)
			imagesJsonTmpDir = t.TempDir()
			imagesJsonPath = filepath.Join(imagesJsonTmpDir, "images.json")
			Expect(os.WriteFile(imagesJsonPath, []byte(imagesJsonContent), 0600)).To(Succeed())

			generate = ubinodejsextension.Generate(
				dependencyManager,
				logger,
				structs.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000},
				imagesJsonPath,
			)

			entriesTests := []struct {
				Entries                   []packit.BuildpackPlanEntry
				selectedNodeVersion       int
				BP_UBI_RUN_IMAGE_OVERRIDE string
			}{
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "16.*", "version-source": ".node-version"},
						},
					},
					selectedNodeVersion:       16,
					BP_UBI_RUN_IMAGE_OVERRIDE: "",
				},
			}

			for _, tt := range entriesTests {
				t.Setenv("BP_UBI_RUN_IMAGE_OVERRIDE", tt.BP_UBI_RUN_IMAGE_OVERRIDE)

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				RunDockerfileProps := structs.RunDockerfileProps{
					Source: fmt.Sprintf("paketocommunity/run-nodejs-%d-ubi-base", tt.selectedNodeVersion),
				}

				runDockerfileContent, _ := utils.GenerateRunDockerfile(RunDockerfileProps)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}
		})
	}, spec.Sequential())

}
