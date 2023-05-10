package ubinodejsextension

import (
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/libnodejs"
	nodestart "github.com/paketo-buildpacks/node-start"
	"github.com/paketo-buildpacks/packit/v2"
)

// functionality from node-start
func nodeApplicationExists(workingDir string, applicationFinder nodestart.ApplicationFinder) (path string, err error) {
	return applicationFinder.Find(workingDir, os.Getenv("BP_LAUNCHPOINT"), os.Getenv("BP_NODE_PROJECT_PATH"))
}

func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		// likely move these to main.go ?
		workingDir := context.WorkingDir
		nodeApplicationFinder := nodestart.NewNodeApplicationFinder()

		projectPath, err := libnodejs.FindProjectPath(context.WorkingDir)
		if err != nil {
			return packit.DetectResult{}, err
		}

		pkg, err := libnodejs.ParsePackageJSON(filepath.Join(projectPath))
		if err != nil && !os.IsNotExist(err) {
			return packit.DetectResult{}, packit.Fail
		}

		if err != nil || !pkg.HasStartScript() {
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
