package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testFetchRunImageFromEnv(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		pack   occam.Pack
		name   string
		source string
	)

	it.Before(func() {
		pack = occam.NewPack()

		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when BP_UBI_RUN_IMAGE_OVERRIDE is set", func() {
		it("uses the run image specified from the BP_UBI_RUN_IMAGE_OVERRIDE", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			nodeRunImage := "this-is-a-run-image"

			var logs fmt.Stringer
			_, logs, err = pack.WithNoColor().Build.
				WithExtensions(
					settings.Buildpacks.NodeExtension.Online,
				).
				WithBuildpacks(
					settings.Buildpacks.NodeEngine.Online,
					settings.Buildpacks.Processes.Online,
				).
				WithEnv(map[string]string{"BP_UBI_RUN_IMAGE_OVERRIDE": nodeRunImage}).
				WithPullPolicy("always").
				Execute(name, source)

			Expect(err).To(HaveOccurred())

			Expect(logs).To(ContainLines(fmt.Sprintf("%s 1.2.3", settings.Extension.Name)))
			Expect(logs).To(ContainLines("  Resolving Node Engine version"))
			Expect(logs).To(ContainLines("    Candidate version sources (in priority order):"))
			Expect(logs).To(ContainLines("      <unknown> -> \"\""))
			Expect(logs).To(ContainLines(fmt.Sprintf("  Using run image specified by BP_UBI_RUN_IMAGE_OVERRIDE %s", nodeRunImage)))
			Expect(logs).To(ContainLines(MatchRegexp(`  Selected Node Engine Major version \d+`)))
		})
	})
}
