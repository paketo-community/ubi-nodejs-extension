package utils_testdata

import "github.com/paketo-community/ubi-nodejs-extension/internal/utils"

func GetParsedImages(filename string) utils.ImagesJson {
	if filename == "images_sample.json" {
		return utils.ImagesJson{
			StackImages: []utils.StackImages{
				{
					Name:              "default",
					IsDefaultRunImage: false,
				},
				{
					Name:              "java-8",
					IsDefaultRunImage: false,
				},
				{
					Name:              "java-11",
					IsDefaultRunImage: false,
				},
				{
					Name:              "java-17",
					IsDefaultRunImage: false,
				},
				{
					Name:              "java-21",
					IsDefaultRunImage: false,
				},
				{
					Name:              "nodejs-16",
					IsDefaultRunImage: false,
				},
				{
					Name:              "nodejs-18",
					IsDefaultRunImage: false,
				},
				{
					Name:              "nodejs-20",
					IsDefaultRunImage: true,
				},
			},
		}
	}
	return utils.ImagesJson{}
}
