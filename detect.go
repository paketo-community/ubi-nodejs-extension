package ubinodejsextension

import (
	"os"
	"path/filepath"

	nodestart "github.com/paketo-buildpacks/node-start"
	npmstart "github.com/paketo-buildpacks/npm-start"
	"github.com/paketo-buildpacks/packit/v2"
)

// functionality from npm-start buildpack, also some overlap with npm-install
func packageJSONWithStartExists(workingDir string, projectPathParser npmstart.PathParser) (path string, err error) {

	projectPath, err := projectPathParser.Get(workingDir)
	if err != nil {
		return "", err
	}

	packageJSONPath := filepath.Join(projectPath, "package.json")
	_, err = os.Stat(packageJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			// no package.json
			return "", nil
		}
		return "", err
	}

	var pkg *npmstart.PackageJson
	pkg, err = npmstart.NewPackageJsonFromPath(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return "", err
	}

	if pkg.Scripts.Start != "" {
		// package.json has start command
		return packageJSONPath, nil
	}

	return "", nil
}

// functionality from node-start
func nodeApplicationExists(workingDir string, applicationFinder nodestart.ApplicationFinder) (path string, err error) {
	return applicationFinder.Find(workingDir, os.Getenv("BP_LAUNCHPOINT"), os.Getenv("BP_NODE_PROJECT_PATH"))
}

func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		// likely move these to main.go ?
		workingDir := context.WorkingDir
		projectPathParser := npmstart.NewProjectPathParser()
		nodeApplicationFinder := nodestart.NewNodeApplicationFinder()

		packageJSON, err := packageJSONWithStartExists(workingDir, projectPathParser)
		if err != nil {
			return packit.DetectResult{}, err
		}

		if packageJSON == "" {
			// no package.json so look for Node.js application files
			path, err := nodeApplicationExists(workingDir, nodeApplicationFinder)
			if err != nil {
				return packit.DetectResult{}, err
			}
			// if no application was found then we don't need to provide node
			if path == "" {
				return packit.DetectResult{}, packit.Fail.WithMessage("Node a Node.js application")
			}
		}

		// if we get here we either found a pacakge.json or Node.js application file
		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: "node"},
				},
				Or: []packit.BuildPlan{
					{
						Provides: []packit.BuildPlanProvision{
							{Name: "node"},
							{Name: "npm"},
						},
					},
				},
			},
		}, nil
	}
}
