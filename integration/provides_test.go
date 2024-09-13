package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	. "github.com/paketo-buildpacks/occam/matchers"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testProvides(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when the buildpack is run with pack build", func() {
		var (
			image  occam.Image
			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("writes a buildplan requiring node and npm", func() {
			var err error

			source, err = occam.Source(filepath.Join("testdata", "needs_node_and_npm_app"))
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
				WithPullPolicy("always").
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			Expect(logs).To(ContainLines(
				fmt.Sprintf("%s 1.2.3", settings.Extension.Name),
				"  Resolving Node Engine version",
				"    Candidate version sources (in priority order):",
				"      <unknown> -> \"\""))

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
		})
	})
}
