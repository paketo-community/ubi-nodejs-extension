package main

import (
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/postal"

	ubinodejsextension "github.com/paketo-community/ubi-nodejs-extension"
)

func main() {
	dependencyManager := postal.NewService(cargo.NewTransport())

	packit.RunExtension(
		ubinodejsextension.Detect(),
		ubinodejsextension.Generate(dependencyManager),
	)
}
