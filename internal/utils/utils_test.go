package utils_test

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-community/ubi-nodejs-extension/internal/utils"
	utils_testdata "github.com/paketo-community/ubi-nodejs-extension/internal/utils/testdata"
	"github.com/sclevine/spec"
)

func testGetDefaultNodeVersion(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("When passing an array of stacks with nodejs images", func() {

		context("and there is a default run image", func() {
			it("should find the default node version", func() {
				defaultNodeVersion, err := utils.GetDefaultNodeVersion([]utils.StackImages{
					{
						Name:              "nodejs-22",
						IsDefaultRunImage: true,
						NodeVersion:       "22",
					},
					{
						Name:              "nodejs-20",
						IsDefaultRunImage: false,
						NodeVersion:       "20",
					},
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(defaultNodeVersion).To(Equal("22"))
			})
		})

		context("and there are no default run images", func() {
			it("should error", func() {
				defaultNodeVersion, err := utils.GetDefaultNodeVersion([]utils.StackImages{
					{
						Name:              "nodejs-22",
						IsDefaultRunImage: false,
						NodeVersion:       "22",
					},
					{
						Name:              "nodejs-20",
						IsDefaultRunImage: false,
						NodeVersion:       "20",
					},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("default node.js version not found"))
				Expect(defaultNodeVersion).To(Equal(""))
			})
		})

		context("and there are more than one default run images", func() {
			it("should error", func() {
				defaultNodeVersion, err := utils.GetDefaultNodeVersion([]utils.StackImages{
					{
						Name:              "nodejs-18",
						IsDefaultRunImage: true,
						NodeVersion:       "18",
					},
					{
						Name:              "nodejs-22",
						IsDefaultRunImage: false,
						NodeVersion:       "22",
					},
					{
						Name:              "nodejs-20",
						IsDefaultRunImage: true,
						NodeVersion:       "20",
					},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("multiple default node.js versions found"))
				Expect(defaultNodeVersion).To(Equal(""))
			})
		})
	})
}

func testCreateConfigTomlFileContent(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("When passing data properly to CreateConfigTomlFileContent function", func() {

		it("successfly create the content of config.toml", func() {
			configTomlFileContent, err := utils.CreateConfigTomlFileContent("22", []utils.StackImages{
				{
					Name:              "nodejs-22",
					IsDefaultRunImage: true,
					NodeVersion:       "22",
				},
				{
					Name:              "nodejs-20",
					IsDefaultRunImage: false,
				},
			}, "io.buildpacks.stacks.ubix")

			Expect(err).ToNot(HaveOccurred())
			Expect(configTomlFileContent.String()).To(ContainSubstring(`[metadata]
  [metadata.default-versions]
    node = "22.*.*"

  [[metadata.dependencies]]
    id = "node"
    source = "paketocommunity/run-nodejs-22-ubi-base"
    stacks = ["io.buildpacks.stacks.ubix"]
    version = "22.1000"

  [[metadata.dependencies]]
    id = "node"
    source = "paketocommunity/run-nodejs--ubi-base"
    stacks = ["io.buildpacks.stacks.ubix"]
    version = ".1000"`))
		})
	})
}

func testParseImagesJsonFile(t *testing.T, _ spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	var imagesJsonDir string
	it.Before(func() {
		imagesJsonDir = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(imagesJsonDir)).To(Succeed())
	})

	it("successfully parses images.json file", func() {
		cwd, err := os.Getwd()
		if err != nil {
			Expect(err).ToNot(HaveOccurred())
		}

		imagesJsonData, err := utils.ParseImagesJsonFile(filepath.Join(cwd, "/testdata/images_sample.json"))
		Expect(err).ToNot(HaveOccurred())

		imagesParsed := utils_testdata.GetParsedImages("images_sample.json")

		Expect(imagesJsonData).To(Equal(imagesParsed))
	})

	it("erros when images.json file does not exist", func() {
		imagesJsonData, err := utils.ParseImagesJsonFile("/does/not/exist")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		Expect(imagesJsonData).To(Equal(utils.ImagesJson{}))
	})

	it("erros when images.json file is corrupted", func() {
		cwd, err := os.Getwd()
		if err != nil {
			Expect(err).ToNot(HaveOccurred())
		}

		imagesJsonData, err := utils.ParseImagesJsonFile(filepath.Join(cwd, "/testdata/images_corrupted.json"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid character"))
		Expect(imagesJsonData).To(Equal(utils.ImagesJson{}))
	})
}

func testGetNodejsStackImages(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("When passing the array with all the stacks", func() {

		it("should return only the nodejs stacks", func() {
			allStacks := utils_testdata.GetParsedImages("images_sample.json")
			nodejsStacks, err := utils.GetNodejsStackImages(allStacks)
			Expect(err).ToNot(HaveOccurred())

			Expect(nodejsStacks).To(Equal([]utils.StackImages{
				{
					Name:              "nodejs-16",
					IsDefaultRunImage: false,
					NodeVersion:       "16",
				},
				{
					Name:              "nodejs-18",
					IsDefaultRunImage: false,
					NodeVersion:       "18",
				},
				{
					Name:              "nodejs-20",
					IsDefaultRunImage: true,
					NodeVersion:       "20",
				},
			}))
		})
	})

	context("When node version is malformed or does not exist", func() {

		it("should error with a message", func() {
			cwd, err := os.Getwd()
			if err != nil {
				Expect(err).ToNot(HaveOccurred())
			}

			for _, tt := range []struct {
				errorMessage   string
				imagesJsonPath string
			}{
				{
					errorMessage:   "extracted Node.js version [hello] for stack nodejs-hello is not an integer",
					imagesJsonPath: "/testdata/images_node_version_not_integer.json",
				},
				{
					errorMessage:   "extracted Node.js version [] for stack nodejs- is not an integer",
					imagesJsonPath: "/testdata/images_no_node_version.json",
				},
			} {
				imagesJsonData, err := utils.ParseImagesJsonFile(filepath.Join(cwd, tt.imagesJsonPath))
				Expect(err).ToNot(HaveOccurred())

				nodejsStacks, err := utils.GetNodejsStackImages(imagesJsonData)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.errorMessage))
				Expect(nodejsStacks).To(Equal([]utils.StackImages{}))
			}
		})
	})
}
