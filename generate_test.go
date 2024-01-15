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

func testFetchingPermissionsFromEtchPasswdFile(t *testing.T, context spec.G, it spec.S) {

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

			duringBuilderPermissions := ubinodejsextension.GetDuringBuildPermissions(path)

			Expect(duringBuilderPermissions).To(Equal(
				ubinodejsextension.DuringBuildPermissions{
					CNB_USER_ID:  1234,
					CNB_GROUP_ID: 2345},
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

			duringBuilderPermissions := ubinodejsextension.GetDuringBuildPermissions(path)

			Expect(duringBuilderPermissions).To(Equal(
				ubinodejsextension.DuringBuildPermissions{
					CNB_USER_ID:  ubinodejsextension.DEFAULT_USER_ID,
					CNB_GROUP_ID: ubinodejsextension.DEFAULT_GROUP_ID},
			))
		})
	})

	context("/etc/passwd does NOT exist", func() {

		it("It should return the default permissions", func() {
			tmpDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			duringBuilderPermissions := ubinodejsextension.GetDuringBuildPermissions(tmpDir)

			Expect(duringBuilderPermissions).To(Equal(
				ubinodejsextension.DuringBuildPermissions{
					CNB_USER_ID:  ubinodejsextension.DEFAULT_USER_ID,
					CNB_GROUP_ID: ubinodejsextension.DEFAULT_GROUP_ID},
			))
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
		BuildDockerfileProps = ubinodejsextension.BuildDockerfileProps{
			CNB_USER_ID:  1002,
			CNB_GROUP_ID: 1000,
			CNB_STACK_ID: "",
			PACKAGES:     ubinodejsextension.PACKAGES,
		}
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

	context("Generate called with NO node in buildplan", func() {
		it.Before(func() {

			workingDir = t.TempDir()
			Expect(err).NotTo(HaveOccurred())

			generate = ubinodejsextension.Generate(dependencyManager, logger, ubinodejsextension.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000})

			err = toml.NewEncoder(buf).Encode(testBuildPlan)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.WriteFile(filepath.Join(workingDir, "plan"), buf.Bytes(), 0600)).To(Succeed())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		it("Node no longer requested in buildplan", func() {

			generateResult, err = generate(packit.GenerateContext{
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

			generate = ubinodejsextension.Generate(dependencyManager, logger, ubinodejsextension.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000})

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

		it("Specific version of node requested", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = generate(packit.GenerateContext{
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
					Stack: "io.buildpacks.stacks.ubi8",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-16-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = generate(packit.GenerateContext{
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
					Stack: "io.buildpacks.stacks.ubi8",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
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
						Source: "paketocommunity/run-nodejs-18-ubi-base",
					},
					BuildDockerfileProps:                 BuildDockerfileProps,
					buildDockerfileExpectedNodejsVersion: 18,
				},
			}

			for _, tt := range versionTests {

				generateResult, err = generate(packit.GenerateContext{
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
					Stack: "io.buildpacks.stacks.ubi8",
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

				generateResult, err = generate(packit.GenerateContext{
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
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).To(HaveOccurred())
			}
		})

	}, spec.Sequential())

	context("Getting from detect phase the Node.js versions combined with the source", func() {

		it.Before(func() {

			workingDir = t.TempDir()
			cnbDir, err = os.MkdirTemp("", "cnb")

			generate = ubinodejsextension.Generate(dependencyManager, logger, ubinodejsextension.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000})

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

		it("Should respect the priorities and return the proper Node.js version", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

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
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
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
					RunDockerfileProps: ubinodejsextension.RunDockerfileProps{
						Source: "paketocommunity/run-nodejs-16-ubi-base",
					},
				},
			}

			for _, tt := range entriesTests {

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
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

		it("Should error in case there are no entries in the buildpack plan.", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			entriesTests := []struct {
				Entries []packit.BuildpackPlanEntry
			}{
				{
					Entries: []packit.BuildpackPlanEntry{},
				},
			}

			for _, tt := range entriesTests {

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).To(HaveOccurred())

			}

		})
	}, spec.Sequential())

	context("When BP_NODE_RUN_EXTENSION env has been set", func() {

		it.Before(func() {

			workingDir = t.TempDir()
			cnbDir, err = os.MkdirTemp("", "cnb")

			generate = ubinodejsextension.Generate(dependencyManager, logger, ubinodejsextension.DuringBuildPermissions{CNB_USER_ID: 1002, CNB_GROUP_ID: 1000})

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

		it("Should have the same value as the BP_NODE_RUN_EXTENSION if is not empty string", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			entriesTests := []struct {
				Entries               []packit.BuildpackPlanEntry
				BP_NODE_RUN_EXTENSION string
			}{
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": ">0", "version-source": ".node-version"},
						},
					},
					BP_NODE_RUN_EXTENSION: "testregistry/image-name",
				},
			}

			for _, tt := range entriesTests {
				t.Setenv("BP_NODE_RUN_EXTENSION", tt.BP_NODE_RUN_EXTENSION)

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				RunDockerfileProps := ubinodejsextension.RunDockerfileProps{
					Source: tt.BP_NODE_RUN_EXTENSION,
				}

				runDockerfileContent, _ := ubinodejsextension.FillPropsToTemplate(RunDockerfileProps, runDockerfileTemplate)

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, generateResult.RunDockerfile)
				Expect(buf.String()).To(Equal(runDockerfileContent))
			}
		})

		it("Should fallback to the run image which corresponds to the selected node version during build", func() {

			extensionToml, _ := readExtensionTomlTemplateFile()

			cnbDir, err = os.MkdirTemp("", "cnb")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(cnbDir+"/extension.toml", []byte(extensionToml), 0600)).To(Succeed())

			entriesTests := []struct {
				Entries               []packit.BuildpackPlanEntry
				selectedNodeVersion   int
				BP_NODE_RUN_EXTENSION string
			}{
				{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:     "node",
							Metadata: map[string]interface{}{"version": "16.*", "version-source": ".node-version"},
						},
					},
					selectedNodeVersion:   16,
					BP_NODE_RUN_EXTENSION: "",
				},
			}

			for _, tt := range entriesTests {
				t.Setenv("BP_NODE_RUN_EXTENSION", tt.BP_NODE_RUN_EXTENSION)

				generateResult, err = generate(packit.GenerateContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: tt.Entries,
					},
					Stack: "io.buildpacks.stacks.ubi8",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(generateResult).NotTo(Equal(nil))

				RunDockerfileProps := ubinodejsextension.RunDockerfileProps{
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
	stacks = ["io.buildpacks.stacks.ubi8"]
	source = "paketocommunity/run-nodejs-18-ubi-base"
	version = "18.1000"

  [[metadata.dependencies]]
	id = "node"
	name = "Ubi Node Extension"
	stacks = ["io.buildpacks.stacks.ubi8"]
	source = "paketocommunity/run-nodejs-16-ubi-base"
	version = "16.1000"
	`
	return fmt.Sprintf(template, version), nil
}
