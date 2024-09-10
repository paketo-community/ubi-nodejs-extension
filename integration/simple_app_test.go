package integration

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testSimple(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when the buildpack is run with pack build", func() {
		var (
			image     occam.Image
			container occam.Container
			name      string
			source    string
			sbomDir   string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			sbomDir, err = os.MkdirTemp("", "sbom")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(sbomDir, os.ModePerm)).To(Succeed())
		})

		it.After(func() {
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
			Expect(os.RemoveAll(sbomDir)).To(Succeed())
		})

		context("simple app", func() {
			it.After(func() {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			})

			it("builds, logs and runs correctly", func() {
				var err error

				source, err = occam.Source(filepath.Join("testdata", "simple_app"))
				Expect(err).ToNot(HaveOccurred())

				var logs fmt.Stringer
				image, logs, err = pack.WithNoColor().Build.
					WithExtensions(
						settings.Buildpacks.NodeExtension.Online,
					).
					WithBuildpacks(
						settings.Buildpacks.NodeEngine.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithSBOMOutputDir(sbomDir).
					WithPullPolicy("always").
					Execute(name, source)
				Expect(err).ToNot(HaveOccurred(), logs.String)

				Expect(logs).To(ContainLines(
					fmt.Sprintf("%s 1.2.3", settings.Extension.Name),
					"  Resolving Node Engine version",
					"    Candidate version sources (in priority order):",
					"      <unknown> -> \"\"",
				))

				Expect(logs).To(ContainLines(MatchRegexp(`  Selected Node Engine Major version \d+`)))
				Expect(logs).To(ContainLines("===> RESTORING"))
				Expect(logs).To(ContainLines("===> EXTENDING (BUILD)"))
				Expect(logs).To(ContainLines(
					"[extender (build)] Enabling module streams:",
					MatchRegexp(`\[extender \(build\)\]     nodejs:\d+`)))

				// SBOM is not supported at the moment from UBI image
				// therefore there are no available logs to test/validate

				Expect(logs).To(ContainLines(
					"[extender (build)]   Configuring build environment",
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					`[extender (build)]   Configuring launch environment`,
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					"[extender (build)]     Writing exec.d/0-optimize-memory",
					"[extender (build)]       Calculates available memory based on container limits at launch time.",
					"[extender (build)]       Made available in the MEMORY_AVAILABLE environment variable.",
				))

				container, err = docker.Container.Run.
					WithCommand("echo NODE_ENV=$NODE_ENV && node server.js").
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("hello world"))

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(
					ContainSubstring("NODE_ENV=production"),
				)
			})
		})

		context("NODE_ENV, NODE_VERBOSE are set by user", func() {

			it.After(func() {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			})

			it("uses user-set value in build and buildpack-set value in launch phase", func() {
				var err error

				source, err = occam.Source(filepath.Join("testdata", "simple_app"))
				Expect(err).ToNot(HaveOccurred())

				var logs fmt.Stringer
				image, logs, err = pack.WithNoColor().Build.
					WithEnv(map[string]string{"NODE_ENV": "development", "NODE_VERBOSE": "true"}).
					WithExtensions(
						settings.Buildpacks.NodeExtension.Online,
					).
					WithBuildpacks(
						settings.Buildpacks.NodeEngine.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithPullPolicy("always").
					Execute(name, source)
				Expect(err).ToNot(HaveOccurred(), logs.String)

				Expect(logs).To(ContainLines(
					"[extender (build)]   Configuring build environment",
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					`[extender (build)]   Configuring launch environment`,
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					"[extender (build)]     Writing exec.d/0-optimize-memory",
					"[extender (build)]       Calculates available memory based on container limits at launch time.",
					"[extender (build)]       Made available in the MEMORY_AVAILABLE environment variable.",
				))

				container, err = docker.Container.Run.
					WithCommand("echo ENV=$NODE_ENV && echo VERBOSE=$NODE_VERBOSE && node server.js").
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("hello world"))

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(
					And(
						ContainSubstring("ENV=production"),
						ContainSubstring("VERBOSE=false"),
					),
				)
			})
		})

		context("simple app with .node-version", func() {

			it.After(func() {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			})

			it("builds, logs and runs correctly", func() {
				var err error

				source, err = occam.Source(filepath.Join("testdata", "node_version_app"))
				Expect(err).ToNot(HaveOccurred())

				var logs fmt.Stringer
				image, logs, err = pack.WithNoColor().Build.
					WithExtensions(
						settings.Buildpacks.NodeExtension.Online,
					).
					WithBuildpacks(
						settings.Buildpacks.NodeEngine.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithSBOMOutputDir(sbomDir).
					WithPullPolicy("always").
					Execute(name, source)
				Expect(err).ToNot(HaveOccurred(), logs.String)

				Expect(logs).To(ContainLines(
					fmt.Sprintf("%s 1.2.3", settings.Extension.Name),
					"  Resolving Node Engine version",
					"    Candidate version sources (in priority order):",
					"      .node-version -> \"16.*\"",
					"      <unknown>     -> \"\""))
				Expect(logs).To(ContainLines(
					"  Selected Node Engine Major version 16"))
				Expect(logs).To(ContainLines("===> RESTORING"))
				Expect(logs).To(ContainLines("===> EXTENDING (BUILD)"))
				Expect(logs).To(ContainLines("[extender (build)] Enabling module streams:",
					"[extender (build)]     nodejs:16"))

				// SBOM is not supported at the moment from UBI image
				// therefore there are no available logs to test/validate

				Expect(logs).To(ContainLines(
					"[extender (build)]   Configuring build environment",
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					`[extender (build)]   Configuring launch environment`,
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					"[extender (build)]     Writing exec.d/0-optimize-memory",
					"[extender (build)]       Calculates available memory based on container limits at launch time.",
					"[extender (build)]       Made available in the MEMORY_AVAILABLE environment variable.",
				))

				container, err = docker.Container.Run.
					WithCommand("echo NODE_ENV=$NODE_ENV && node server.js").
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("hello world"))

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(
					ContainSubstring("NODE_ENV=production"),
				)
			})
		})

		context("simple app with .nvmrc", func() {

			it.After(func() {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			})

			it("builds, logs and runs correctly", func() {
				var err error

				source, err = occam.Source(filepath.Join("testdata", "nvmrc_app"))
				Expect(err).ToNot(HaveOccurred())

				var logs fmt.Stringer
				image, logs, err = pack.WithNoColor().Build.
					WithExtensions(
						settings.Buildpacks.NodeExtension.Online,
					).
					WithBuildpacks(
						settings.Buildpacks.NodeEngine.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithSBOMOutputDir(sbomDir).
					WithPullPolicy("always").
					Execute(name, source)
				Expect(err).ToNot(HaveOccurred(), logs.String)

				Expect(logs).To(ContainLines(
					fmt.Sprintf("%s 1.2.3", settings.Extension.Name),
					"  Resolving Node Engine version",
					"    Candidate version sources (in priority order):",
					"      .nvmrc    -> \"16.*\"",
					"      <unknown> -> \"\"",
				))
				Expect(logs).To(ContainLines("  Selected Node Engine Major version 16"))
				Expect(logs).To(ContainLines("===> RESTORING"))
				Expect(logs).To(ContainLines("===> EXTENDING (BUILD)"))
				Expect(logs).To(ContainLines("[extender (build)] Enabling module streams:"))
				Expect(logs).To(ContainLines("[extender (build)]     nodejs:16"))

				// SBOM is not supported at the moment from UBI image
				// therefore there are no available logs to test/validate

				Expect(logs).To(ContainLines(
					"[extender (build)]   Configuring build environment",
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					`[extender (build)]   Configuring launch environment`,
					`[extender (build)]     NODE_ENV     -> "production"`,
					`[extender (build)]     NODE_HOME    -> ""`,
					`[extender (build)]     NODE_OPTIONS -> "--use-openssl-ca"`,
					`[extender (build)]     NODE_VERBOSE -> "false"`,
				))

				Expect(logs).To(ContainLines(
					"[extender (build)]     Writing exec.d/0-optimize-memory",
					"[extender (build)]       Calculates available memory based on container limits at launch time.",
					"[extender (build)]       Made available in the MEMORY_AVAILABLE environment variable.",
				))

				container, err = docker.Container.Run.
					WithCommand("echo NODE_ENV=$NODE_ENV && node server.js").
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("hello world"))

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(
					ContainSubstring("NODE_ENV=production"),
				)
			})
		})
	})
	context("when the node version specfied in the app is EOL'd", func() {

		var (
			name    string
			source  string
			sbomDir string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			sbomDir, err = os.MkdirTemp("", "sbom")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(sbomDir, os.ModePerm)).To(Succeed())
		})

		it("logs that the dependency is deprecated", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			_, logs, err = pack.WithNoColor().Build.
				WithExtensions(
					settings.Buildpacks.NodeExtension.Online,
				).
				WithBuildpacks(
					settings.Buildpacks.NodeEngine.Online,
					settings.Buildpacks.BuildPlan.Online,
				).
				WithEnv(map[string]string{"BP_NODE_VERSION": "~14"}).
				WithSBOMOutputDir(sbomDir).
				WithPullPolicy("always").
				Execute(name, source)

			Expect(err).To(HaveOccurred())

			Expect(logs).To(ContainLines(
				"  Resolving Node Engine version",
				"    Candidate version sources (in priority order):",
				"      BP_NODE_VERSION -> \"~14\"",
				"      <unknown>       -> \"\"",
			))
			Expect(logs).To(ContainLines(
				MatchRegexp(`failed to satisfy \"node\" dependency version constraint \"~14\": no compatible versions on \"io.buildpacks.stacks.ubi8\" stack. Supported versions are: \[(?:\d+\.\d+(?:, )?)*\d+\.\d+\]`),
			))
		})
	})

	// * Test context("BP_DISABLE_SBOM is set to true", func()
	// * is not supported at the * moment due to SBOM functionality is not yet implemented in UBI.

}
