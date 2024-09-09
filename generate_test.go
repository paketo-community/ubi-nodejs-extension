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

type RunDockerfileProps struct {
	Source string
}

//go:embed templates/run.Dockerfile
var runDockerfileTemplate string

type BuildDockerfileProps struct {
	NODEJS_VERSION            uint64
	CNB_USER_ID, CNB_GROUP_ID int
	CNB_STACK_ID, PACKAGES    string
}

//go:embed templates/build.Dockerfile
var buildDockerfileTemplate string

func testFillPropsToTemplate(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("Adding props on templates with FillPropsToTemplate", func() {

		it("Should fill with properties the template/build.Dockerfile", func() {

			output, err := ubinodejsextension.FillPropsToTemplate(BuildDockerfileProps{
				NODEJS_VERSION: 16,
				CNB_USER_ID:    1000,
				CNB_GROUP_ID:   1000,
				CNB_STACK_ID:   "",
				PACKAGES:       ubinodejsextension.PACKAGES,
			}, buildDockerfileTemplate)

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf(`ARG base_image
FROM ${base_image}

USER root

ARG build_id=0
RUN echo ${build_id}

RUN microdnf -y module enable nodejs:16
RUN microdnf --setopt=install_weak_deps=0 --setopt=tsflags=nodocs install -y %s && microdnf clean all

RUN echo uid:gid "1000:1000"
USER 1000:1000

RUN echo "CNB_STACK_ID: "`, ubinodejsextension.PACKAGES)))

		})

		it("Should fill with properties the template/run.Dockerfile", func() {

			RunDockerfileProps := RunDockerfileProps{
				Source: "paketocommunity/run-nodejs-18-ubi-base",
			}

			output, err := ubinodejsextension.FillPropsToTemplate(RunDockerfileProps, runDockerfileTemplate)

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(`FROM paketocommunity/run-nodejs-18-ubi-base`))

		})
	})
}

func testFetchingPermissionsFromEtcPasswdFile(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
		tmpDir string
		path   string
		err    error
	)

	context("/etc/passwd exists and has the cnb user", func() {

		it("It should return the permissions specified for the cnb user", func() {
			tmpDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			path = filepath.Join(tmpDir, "/passwd")

			Expect(os.WriteFile(path, []byte(`root:x:0:0:root:/root:/bin/bash
bin:x:1:1:bin:/bin:/sbin/nologin
daemon:x:2:2:daemon:/sbin:/sbin/nologin
adm:x:3:4:adm:/var/adm:/sbin/nologin
lp:x:4:7:lp:/var/spool/lpd:/sbin/nologin
sync:x:5:0:sync:/sbin:/bin/sync
shutdown:x:6:0:shutdown:/sbin:/sbin/shutdown
halt:x:7:0:halt:/sbin:/sbin/halt
mail:x:8:12:mail:/var/spool/mail:/sbin/nologin
operator:x:11:0:operator:/root:/sbin/nologin
games:x:12:100:games:/usr/games:/sbin/nologin
ftp:x:14:50:FTP User:/var/ftp:/sbin/nologin
cnb:x:1234:2345::/home/cnb:/bin/bash
nobody:x:65534:65534:Kernel Overflow User:/:/sbin/nologin
`), 0600)).To(Succeed())

			duringBuilderPermissions := utils.GetDuringBuildPermissions(path)

			Expect(duringBuilderPermissions).To(Equal(
				structs.DuringBuildPermissions{
					CNB_USER_ID:  1234,
					CNB_GROUP_ID: 2345,
				},
			))
		})
	})

	context("/etc/passwd exists and does NOT have the cnb user", func() {

		it("It should return the default permissions", func() {
			tmpDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			path = filepath.Join(tmpDir, "/passwd")

			Expect(os.WriteFile(path, []byte(`root:x:0:0:root:/root:/bin/bash
bin:x:1:1:bin:/bin:/sbin/nologin
daemon:x:2:2:daemon:/sbin:/sbin/nologin
adm:x:3:4:adm:/var/adm:/sbin/nologin
lp:x:4:7:lp:/var/spool/lpd:/sbin/nologin
sync:x:5:0:sync:/sbin:/bin/sync
shutdown:x:6:0:shutdown:/sbin:/sbin/shutdown
halt:x:7:0:halt:/sbin:/sbin/halt
mail:x:8:12:mail:/var/spool/mail:/sbin/nologin
operator:x:11:0:operator:/root:/sbin/nologin
games:x:12:100:games:/usr/games:/sbin/nologin
ftp:x:14:50:FTP User:/var/ftp:/sbin/nologin
nobody:x:65534:65534:Kernel Overflow User:/:/sbin/nologin
`), 0600)).To(Succeed())

			duringBuilderPermissions := utils.GetDuringBuildPermissions(path)

			Expect(duringBuilderPermissions).To(Equal(
				structs.DuringBuildPermissions{
					CNB_USER_ID:  ubinodejsextension.DEFAULT_USER_ID,
					CNB_GROUP_ID: ubinodejsextension.DEFAULT_GROUP_ID},
			))
		})
	})

	context("/etc/passwd does NOT exist", func() {

		it("It should return the default permissions", func() {
			tmpDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			duringBuilderPermissions := utils.GetDuringBuildPermissions(tmpDir)

			Expect(duringBuilderPermissions).To(Equal(
				structs.DuringBuildPermissions{
					CNB_USER_ID:  ubinodejsextension.DEFAULT_USER_ID,
					CNB_GROUP_ID: ubinodejsextension.DEFAULT_GROUP_ID},
			))
		})
	})
}

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

			imagesJsonContent := generateImagesJsonFile("18")
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
				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(runDockerFileProps, runDockerfileTemplate)

				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(buildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))

			}
		})

		it("should return the default when node version has NOT been requested", func() {

			imagesJsonContent := generateImagesJsonFile("16")
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

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(runDockerFileProps, runDockerfileTemplate)
				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(buildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}

		})

		it("should return the higher node version when it requests for >=nodeVersion", func() {

			imagesJsonContent := generateImagesJsonFile("18")
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
				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(runDockerFileProps, runDockerfileTemplate)

				buildDockerfileProps := structs.BuildDockerfileProps{
					CNB_USER_ID:    1002,
					CNB_GROUP_ID:   1000,
					CNB_STACK_ID:   "io.buildpacks.stacks.ubi8",
					PACKAGES:       ubinodejsextension.PACKAGES,
					NODEJS_VERSION: uint64(tt.expectedNodeVersion),
				}

				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(buildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}
		})

		it("Should error on below cases of requested node", func() {

			imagesJsonContent := generateImagesJsonFile("18")
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

			imagesJsonContent := generateImagesJsonFile("18")
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

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.RunDockerfileProps, runDockerfileTemplate)

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

			imagesJsonContent := generateImagesJsonFile("18")
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

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(RunDockerfileProps, runDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}
		})

		it("Should fallback to the run image which corresponds to the selected node version during build", func() {

			imagesJsonContent := generateImagesJsonFile("18")
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

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(RunDockerfileProps, runDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}
		})
	}, spec.Sequential())

}

func generateImagesJsonFile(defaultNodeVersion string) string {

	var isDefaultNodeRunImage16 bool
	var isDefaultNodeRunImage18 bool

	if defaultNodeVersion == "16" {
		isDefaultNodeRunImage16 = true
		isDefaultNodeRunImage18 = false
	} else if defaultNodeVersion == "18" {
		isDefaultNodeRunImage16 = false
		isDefaultNodeRunImage18 = true
	}
	return fmt.Sprintf(`{
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
      },
      {
        "name": "nodejs-16",
        "is_default_run_image": %t,
        "config_dir": "stack-nodejs-16",
        "output_dir": "build-nodejs-16",
        "build_image": "build-nodejs-16",
        "run_image": "run-nodejs-16",
        "build_receipt_filename": "build-nodejs-16-receipt.cyclonedx.json",
        "run_receipt_filename": "run-nodejs-16-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/nodejs-16-minimal"
      },
      {
        "name": "nodejs-18",
        "is_default_run_image": %t,
        "config_dir": "stack-nodejs-18",
        "output_dir": "build-nodejs-18",
        "build_image": "build-nodejs-18",
        "run_image": "run-nodejs-18",
        "build_receipt_filename": "build-nodejs-18-receipt.cyclonedx.json",
        "run_receipt_filename": "run-nodejs-18-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/nodejs-18-minimal"
      }
    ]
  }
`, isDefaultNodeRunImage16, isDefaultNodeRunImage18)
}
