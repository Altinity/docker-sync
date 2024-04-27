package config

var (
	ECRRegion = NewKey("ecr.region",
		WithDefaultValue("us-east-1"),
		WithValidString())
)
