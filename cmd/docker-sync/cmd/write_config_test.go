package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestWriteConfigCmd(t *testing.T) {
	// Load a sample configuration to Viper
	viper.SetConfigType("yaml")
	configData := []byte(`
ecr:
    region: us-east-1
logging:
    colors: true
    format: text
    level: INFO
    output: stdout
    timeformat: "15:04:05"
sync:
    images:
        - mutabletags:
            - latest
          source: altinity/docker-sync
          targets:
            - r2:30b2944b5174302fb2188bb48399c72b:cr-enam:altinity/docker-sync
    interval: 30m
    maxerrors: 0
    registries:
        - auth:
            password: foo
            username: bar
          name: R2
          url: r2:30b2944b5174302fb2188bb48399c72b:cr-enam
    s3:
        maxconcurrentuploads: 5
        objectcache:
            capacity: 100000
            enabled: true
            expirationtime: 120m
telemetry:
    enabled: true
    metrics:
        exporter: prometheus
        prometheus:
            address: 0.0.0.0:9090
            path: /metrics
        stdout:
            interval: 5s
`)
	viper.ReadConfig(bytes.NewBuffer(configData))

	// Capture stdout
	var stdout bytes.Buffer

	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)

	t.Run("Stdout", func(t *testing.T) {
		rootCmd.SetArgs([]string{"writeConfig"})
		assert.NoError(t, rootCmd.Execute())

		assert.Contains(t, stdout.String(), "- r2:30b2944b5174302fb2188bb48399c72b:cr-enam:altinity/docker-sync")

		m := make(map[string]interface{})
		assert.NoError(t, yaml.Unmarshal(stdout.Bytes(), &m))
	})

	t.Run("File", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "docker-sync")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		rootCmd.SetArgs([]string{"writeConfig", "-o", filepath.Join(tmpDir, "config.yaml")})
		assert.NoError(t, rootCmd.Execute())

		b, err := os.ReadFile(filepath.Join(tmpDir, "config.yaml"))
		assert.NoError(t, err)

		assert.Contains(t, string(b), "- r2:30b2944b5174302fb2188bb48399c72b:cr-enam:altinity/docker-sync")

		m := make(map[string]interface{})
		assert.NoError(t, yaml.Unmarshal(b, &m))
	})
}
