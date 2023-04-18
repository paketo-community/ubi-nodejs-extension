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

	ubinodejsextension "github.com/paketo-community/ubi-nodejs-extension"
	"github.com/paketo-community/ubi-nodejs-extension/fakes"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/packit/v2/cargo"

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

			buildDockerfileProps := BuildDockerfileProps{
				NODEJS_VERSION: 16,
				CNB_USER_ID:    1000,
				CNB_GROUP_ID:   1000,
				CNB_STACK_ID:   "",
				PACKAGES:       "make gcc gcc-c++ libatomic_ops git openssl-devel nodejs npm nodejs-nodemon nss_wrapper which",
			}

			output, err := ubinodejsextension.FillPropsToTemplate(buildDockerfileProps, buildDockerfileTemplate)

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(`ARG base_image
FROM ${base_image}

USER root

ARG build_id=0
RUN echo ${build_id}

RUN microdnf -y module enable nodejs:16
RUN microdnf --setopt=install_weak_deps=0 --setopt=tsflags=nodocs install -y make gcc gcc-c++ libatomic_ops git openssl-devel nodejs npm nodejs-nodemon nss_wrapper which && microdnf clean all

RUN echo uid:gid "1000:1000"
USER 1000:1000

RUN echo "CNB_STACK_ID: "`))

		})

		it("Should fill with properties the template/run.Dockerfile", func() {

			RunDockerfileProps := RunDockerfileProps{
				Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
			}

			output, err := ubinodejsextension.FillPropsToTemplate(RunDockerfileProps, runDockerfileTemplate)

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(`FROM paketo-buildpacks/ubi8-paketo-run-nodejs-18`))

		})
	})
}

func testGenerate(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect               = NewWithT(t).Expect
		workingDir           string
		planPath             string
		testBuildPlan        packit.BuildpackPlan
		buf                  = new(bytes.Buffer)
		generateResult       packit.GenerateResult
		err                  error
		cnbDir               string
		dependencyManager    *fakes.DependencyManager
		BuildDockerfileProps = ubinodejsextension.BuildDockerfileProps{
			CNB_USER_ID:  ubinodejsextension.CNB_USER_ID,
			CNB_GROUP_ID: ubinodejsextension.CNB_GROUP_ID,
			CNB_STACK_ID: "",
			PACKAGES:     ubinodejsextension.PACKAGES,
		}
	)

	context("Generate called with NO node in buildplan", func() {
		it.Before(func() {

			workingDir = t.TempDir()
			Expect(err).NotTo(HaveOccurred())

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it("Node no longer requested in buildplan", func() {
			dependencyManager = &fakes.DependencyManager{}
			dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{Name: "Node Engine", ID: "node", Version: "16.5.1"}

			generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
				WorkingDir: workingDir,
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{},
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(generateResult.BuildDockerfile).To(BeNil())
		})
	}, spec.Sequential())

	context("Generate called with node in the buildplan", func() {
		it.Before(func() {

			workingDir = t.TempDir()
			cnbDir, err = os.MkdirTemp("", "cnb")

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			planPath = filepath.Join(workingDir, "plan")
			t.Setenv("CNB_BP_PLAN_PATH", planPath)

			Expect(os.WriteFile(planPath, buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())

		})

		it("Specific version of node requested", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			versionTests := []struct {
				Name                                 string
				Metadata                             map[string]interface{}
				RunDockerfileProps                   ubinodejsextension.RunDockerfileProps
				BuildDockerfileProps                 ubinodejsextension.BuildDockerfileProps
				buildDockerfileExpectedNodejsVersion int
			}{
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "16 - 18",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "16.0.0 - 18.0.0",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "<18.5.1",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        ">18.5.1",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "16 <18.5.1",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "<1.0.0 || >=2.5.2 <3.0.0 || >=2.3.1 <18.4.5",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "v18",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "16",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "18",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "~16",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "^18.0.x",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:     tt.Name,
								Metadata: tt.Metadata,
							},
						},
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.RunDockerfileProps, runDockerfileTemplate)
				tt.BuildDockerfileProps.NODEJS_VERSION = uint64(tt.buildDockerfileExpectedNodejsVersion)
				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.BuildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}

		})

		it("should return the default when node version has NOT been requested", func() {

			extensionToml, _ := readExtensionTomlTemplateFile("16")

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			versionTests := []struct {
				Name                                 string
				Metadata                             map[string]interface{}
				RunDockerfileProps                   ubinodejsextension.RunDockerfileProps
				BuildDockerfileProps                 ubinodejsextension.BuildDockerfileProps
				buildDockerfileExpectedNodejsVersion int
			}{
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "",
						"version-source": "",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 16,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "x",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:     tt.Name,
								Metadata: tt.Metadata,
							},
						},
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.RunDockerfileProps, runDockerfileTemplate)
				tt.BuildDockerfileProps.NODEJS_VERSION = uint64(tt.buildDockerfileExpectedNodejsVersion)
				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.BuildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}

		})

		it("should return the higher node version when it requests for >=nodeVersion", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			versionTests := []struct {
				Name                                 string
				Metadata                             map[string]interface{}
				RunDockerfileProps                   ubinodejsextension.RunDockerfileProps
				BuildDockerfileProps                 ubinodejsextension.BuildDockerfileProps
				buildDockerfileExpectedNodejsVersion int
			}{
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        ">16",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        ">13",
						"version-source": "BP_NODE_VERSION",
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-18",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:     tt.Name,
								Metadata: tt.Metadata,
							},
						},
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.RunDockerfileProps, runDockerfileTemplate)
				tt.BuildDockerfileProps.NODEJS_VERSION = uint64(tt.buildDockerfileExpectedNodejsVersion)
				buildDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.BuildDockerfileProps, buildDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
				buf.Reset()
				_, _ = io.Copy(buf, generateResult.BuildDockerfile)
				Expect(buf.String()).To(Equal(buildDockerfileContent))
			}

		})

		it("Should error on below cases of requested node", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			versionTests := []struct {
				Name     string
				Metadata map[string]interface{}
			}{
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "17 - 18.0.0",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "15",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "18.0.0",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "v18.999.0",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        ">18",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "~16.2",
						"version-source": "BP_NODE_VERSION",
					},
				},
				{
					Name: "node",
					Metadata: map[string]interface{}{
						"version":        "16.5.x",
						"version-source": "BP_NODE_VERSION",
					},
				},
			}

			for _, tt := range versionTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:     tt.Name,
								Metadata: tt.Metadata,
							},
						},
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).To(HaveOccurred())
			}
		})

	}, spec.Sequential())

	context("Getting from detect phase the Node.js versions combined with the source", func() {

		it.Before(func() {

			workingDir = t.TempDir()
			cnbDir, err = os.MkdirTemp("", "cnb")

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			planPath = filepath.Join(workingDir, "plan")
			t.Setenv("CNB_BP_PLAN_PATH", planPath)

			Expect(os.WriteFile(planPath, buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it("Should respect the priorities and return the proper Node.js version", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			entriesTests := []struct {
				Entries            []packit.BuildpackPlanEntry
				RunDockerfileProps ubinodejsextension.RunDockerfileProps
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
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
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
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
				},
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "=16", "version-source": ".node-version"},
						},
					},
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketo-buildpacks/ubi8-paketo-run-nodejs-16",
					},
				},
			}

			for _, tt := range entriesTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(tt.RunDockerfileProps, runDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}

		})

		it("Should error in case there are no entries in the buildpack plan.", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			dependencyManager := postal.NewService(cargo.NewTransport())

			entriesTests := []struct {
				Entries []packit.BuildpackPlanEntry
			}{
				{
					Entries: []packit.BuildpackPlanEntry{},
				},
			}

			for _, tt := range entriesTests {

				generateResult, err = ubinodejsextension.Generate(dependencyManager)(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "ubi8-paketo",
				})

				Expect(err).To(HaveOccurred())

			}

		})
	}, spec.Sequential())

}

func readExtensionTomlTemplateFile(defaultNodeVersion ...string) (string, error) {
	var version string
	if len(defaultNodeVersion) == 0 {
		version = "18.*.*"
	} else {
		version = defaultNodeVersion[0]
	}

	template := `
api = "0.7"

[extension]
id = "redhat-runtimes/nodejs"
name = "RedHat Runtimes Node.js Dependency Extension"
version = "0.0.1"
description = "This extension installs the appropriate nodejs runtime via dnf"

[metadata]
  [metadata.default-versions]
	node = "%s"

  [[metadata.dependencies]]
	id = "node"
	name = "Ubi Node Extension"
	stacks = ["ubi8-paketo"]
	source = "paketo-buildpacks/ubi8-paketo-run-nodejs-18"
	version = "18.1000"

  [[metadata.dependencies]]
	id = "node"
	name = "Ubi Node Extension"
	stacks = ["ubi8-paketo"]
	source = "paketo-buildpacks/ubi8-paketo-run-nodejs-16"
	version = "16.1000"
	`
	return fmt.Sprintf(template, version), nil
}
