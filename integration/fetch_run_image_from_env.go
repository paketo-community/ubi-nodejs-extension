package integration

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testFetchRunImageFromEnv(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		docker occam.Docker
		pack   occam.Pack

		image     occam.Image
		container occam.Container
		name      string
		source    string
	)

	it.Before(func() {
		docker = occam.NewDocker()
		pack = occam.NewPack()

		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when BP_UBI_RUN_IMAGE_OVERRIDE is set", func() {
		it("uses the run image specified from the BP_UBI_RUN_IMAGE_OVERRIDE", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			nodeRunImage := settings.Metadata.Dependencies[0].Source

			runNodeVersion, err := semver.NewVersion(settings.Metadata.Dependencies[0].Version)
			Expect(err).NotTo(HaveOccurred())
			runNodeMajorVersion := runNodeVersion.Major()

			buildNodeVersion, err := semver.NewVersion(settings.Metadata.Dependencies[len(settings.Metadata.Dependencies)-1].Version)
			Expect(err).NotTo(HaveOccurred())
			buildNodeMajorVersion := buildNodeVersion.Major()

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithExtensions(
					settings.Buildpacks.NodeExtension.Online,
				).
				WithBuildpacks(
					settings.Buildpacks.NodeEngine.Online,
					settings.Buildpacks.Processes.Online,
				).
				WithEnv(map[string]string{"BP_UBI_RUN_IMAGE_OVERRIDE": nodeRunImage, "BP_NODE_VERSION": fmt.Sprint(buildNodeMajorVersion)}).
				WithPullPolicy("always").
				Execute(name, source)

			Expect(err).NotTo(HaveOccurred(), logs.String())

			Expect(logs).To(ContainLines(fmt.Sprintf("%s 1.2.3", settings.Extension.Name)))
			Expect(logs).To(ContainLines("  Resolving Node Engine version"))
			Expect(logs).To(ContainLines("    Candidate version sources (in priority order):"))
			Expect(logs).To(ContainLines(fmt.Sprintf("      BP_NODE_VERSION -> \"%d\"", buildNodeMajorVersion)))
			Expect(logs).To(ContainLines("      <unknown>       -> \"\""))
			Expect(logs).To(ContainLines(fmt.Sprintf("  Using run image specified by BP_UBI_RUN_IMAGE_OVERRIDE %s", nodeRunImage)))

			Expect(logs).To(ContainLines(
				"[extender (build)] Enabling module streams:",
				fmt.Sprintf("[extender (build)]     nodejs:%d", buildNodeMajorVersion)))

			container, err = docker.Container.Run.
				WithPublish("8080").
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(container).Should(BeAvailable())

			response, err := http.Get(fmt.Sprintf("http://localhost:%s/version/major", container.HostPort("8080")))
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			content, err := io.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(fmt.Sprint(runNodeMajorVersion)))

		})
	})
}
