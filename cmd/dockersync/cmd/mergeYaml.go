package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var yamlFiles []string

// mergeYamlCmd represents the mergeYaml command
var mergeYamlCmd = &cobra.Command{
	Use:   "mergeYaml",
	Short: "Merge two yaml files",
	PreRun: func(cmd *cobra.Command, args []string) {
		cmd.Annotations = make(map[string]string)
		cmd.Annotations["error"] = ""
	},
	Run: func(cmd *cobra.Command, args []string) {
		outFile := cmd.Flag("output").Value.String()

		base := make(map[string]interface{})
		currentMap := make(map[string]interface{})

		for _, fname := range yamlFiles {
			data, err := os.ReadFile(fname)
			if err != nil {
				cmd.Annotations["error"] = err.Error()
				return
			}
			if err := yaml.Unmarshal(data, &currentMap); err != nil {
				cmd.Annotations["error"] = err.Error()
				return
			}
			base = mergeMaps(base, currentMap)
		}

		config, err := yaml.Marshal(base)
		if err != nil {
			cmd.Annotations["error"] = err.Error()
			return
		}

		if outFile == "" {
			fmt.Println(string(config))
		} else {
			if err := os.WriteFile(outFile, config, 0644); err != nil {
				cmd.Annotations["error"] = err.Error()
				return
			}
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		// Wait for a second to allow for any pending log messages to be flushed
		time.Sleep(1 * time.Second)
		if cmd.Annotations["error"] != "" {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(mergeYamlCmd)

	mergeYamlCmd.Flags().StringP("output", "o", "", "File to write config to (default is stdout)")
	mergeYamlCmd.Flags().StringSliceVarP(&yamlFiles, "yamlFiles", "f", []string{}, "Yaml files to merge")
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
