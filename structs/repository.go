package structs

type RepositoryAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Token    string `json:"token" yaml:"token"`
	Helper   string `json:"helper" yaml:"helper"`
}

type Repository struct {
	Name string         `json:"name" yaml:"name"`
	URL  string         `json:"url" yaml:"url"`
	Auth RepositoryAuth `json:"auth" yaml:"auth"`
}
