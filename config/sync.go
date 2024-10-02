package config

var (
	// region Sync.

	// SyncMaxErrors specifies the maximum number of errors that can occur before the application exits.
	SyncMaxErrors = NewKey("sync.maxErrors",
		WithDefaultValue(5),
		WithValidInt())

	// SyncInterval specifies the interval at which images are synchronized.
	SyncInterval = NewKey("sync.interval",
		WithDefaultValue("30m"),
		WithValidDuration())

	// SyncRegistries specifies the repositories to use for pulling and pushing images.
	SyncRegistries = NewKey("sync.registries",
		WithDefaultValue([]map[string]interface{}{
			{
				"name": "Docker Hub",
				"url":  "docker.io",
				"auth": map[string]interface{}{
					"username": "",
					"password": "",
					"token":    "",
					"helper":   "",
				},
			},
		}),
		WithValidRepositories())

	// SyncImages specifies the images to synchronize.
	SyncImages = NewKey("sync.images",
		WithDefaultValue([]map[string]interface{}{
			{
				"source": "docker.io/library/ubuntu",
				"targets": []string{
					"docker.io/library/ubuntu",
				},
			},
		}),
		WithValidImages())

	// endregion.
)
