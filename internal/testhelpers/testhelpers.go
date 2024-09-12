package testhelpers

import "fmt"

func GenerateImagesJsonFile(nodeVersions []string, isDefault []bool, isCorrupted bool) string {
	addStacks := ""

	for i, nodeVersion := range nodeVersions {

		addStacks += fmt.Sprintf(`,
      {
        "name": "nodejs-%s",
        "is_default_run_image": %t,
        "config_dir": "stack-nodejs-%s",
        "output_dir": "build-nodejs-%s",
        "build_image": "build-nodejs-%s",
        "run_image": "run-nodejs-%s",
        "build_receipt_filename": "build-nodejs-%s-receipt.cyclonedx.json",
        "run_receipt_filename": "run-nodejs-%s-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/nodejs-%s-runtime"
      }`, nodeVersion, isDefault[i], nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion, nodeVersion)
	}

	if isCorrupted {
		addStacks += `,
		{
			"name": "nodejs-18",}
			not a valid json
		}`
	}

	stacks := fmt.Sprintf(`{
    "support_usns": false,
    "update_on_new_image": true,
    "receipts_show_limit": 16,
    "images": [
      {
        "name": "default",
        "config_dir": "stack",
        "output_dir": "build",
        "build_image": "build",
        "run_image": "run",
        "build_receipt_filename": "build-receipt.cyclonedx.json",
        "run_receipt_filename": "run-receipt.cyclonedx.json",
        "create_build_image": true,
        "base_build_container_image": "docker://registry.access.redhat.com/ubi8/ubi-minimal",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/ubi-minimal"
      },
      {
        "name": "java-17",
        "config_dir": "stack-java-17",
        "output_dir": "build-java-17",
        "build_image": "build-java-17",
        "run_image": "run-java-17",
        "build_receipt_filename": "build-java-17-receipt.cyclonedx.json",
        "run_receipt_filename": "run-java-17-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/openjdk-17-runtime"
      },
      {
        "name": "java-21",
        "config_dir": "stack-java-21",
        "output_dir": "build-java-21",
        "build_image": "build-java-21",
        "run_image": "run-java-21",
        "build_receipt_filename": "build-java-21-receipt.cyclonedx.json",
        "run_receipt_filename": "run-java-21-receipt.cyclonedx.json",
        "base_run_container_image": "docker://registry.access.redhat.com/ubi8/openjdk-21-runtime"
      }%s
    ]
  }
`, addStacks)

	return stacks
}
