package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var settings struct {
	Buildpacks struct {
		NodeExtension struct {
			Online string
		}
		NodeEngine struct {
			Online string
			ID     string
			Name   string
		}
		BuildPlan struct {
			Online string
		}
		Processes struct {
			Online string
		}
	}

	Extension struct {
		ID   string
		Name string
	}

	Metadata struct {
	} `toml:"metadata"`

	Config struct {
		BuildPlan  string `json:"build-plan"`
		NodeEngine string `json:"node-engine"`
	}
}

func TestIntegration(t *testing.T) {
	Expect := NewWithT(t).Expect

	//reading the extension.toml file
	file, err := os.Open("../extension.toml")
	Expect(err).NotTo(HaveOccurred())

	_, err = toml.NewDecoder(file).Decode(&settings)
	Expect(err).NotTo(HaveOccurred())
	Expect(file.Close()).To(Succeed())

	//reading the integration.json file
	file, err = os.Open("../integration.json")
	Expect(err).NotTo(HaveOccurred())

	Expect(json.NewDecoder(file).Decode(&settings.Config)).To(Succeed())
	Expect(file.Close()).To(Succeed())

	buildpackStore := occam.NewBuildpackStore()

	root, err := filepath.Abs("./..")
	Expect(err).ToNot(HaveOccurred())

	settings.Buildpacks.NodeExtension.Online, err = buildpackStore.Get.
		WithVersion("1.2.3").
		Execute(root)
	Expect(err).NotTo(HaveOccurred())

	settings.Buildpacks.BuildPlan.Online, err = buildpackStore.Get.
		Execute(settings.Config.BuildPlan)
	Expect(err).NotTo(HaveOccurred())

	settings.Buildpacks.NodeEngine.Online, err = buildpackStore.Get.
		WithVersion("1.2.3").
		Execute(settings.Config.NodeEngine)
	Expect(err).NotTo(HaveOccurred())

	settings.Buildpacks.NodeEngine.ID = "paketo-buildpacks/node-engine"
	settings.Buildpacks.NodeEngine.Name = "Paketo Buildpack for Node Engine"

	settings.Buildpacks.Processes.Online = filepath.Join("testdata", "processes_buildpack")

	SetDefaultEventuallyTimeout(5 * time.Second)

	suite := spec.New("Integration", spec.Report(report.Terminal{}), spec.Parallel())
	suite("FetchRunImageFromEnv", testFetchRunImageFromEnv)
	suite("OpenSSL", testOpenSSL)
	suite("OptimizeMemory", testOptimizeMemory)
	suite("ProjectPath", testProjectPath)
	suite("Provides", testProvides)
	suite("Simple", testSimple)
	suite.Run(t)
}
