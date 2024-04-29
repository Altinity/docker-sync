# docker-sync

Simple tool to keep images in sync between different registries.

It uses the OCI distribution spec to pull and push images, so it should work with any compliant registry.

Currently, it doesn't support the deprecated v1 manifests, and there are no plans to support them.

## Usage

Clone the repository:

```console
git clone https://github.com/Altinity/docker-sync.git
```

Change to the project directory:

```console
cd docker-sync
```

Build the project:

```console
make
```

Write the default config file:

```console
dist/dockersync writeConfig -o config.yaml`
```

Edit the config file accordingly, then run:

```console
dist/dockersync
```

## Configuration

The default configuration looks like this: 

```yaml
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
        - source: docker.io/library/ubuntu
          targets:
            - docker.io/kamushadenes/ubuntu
    interval: 5m
    maxerrors: 5
    registries:
        - auth:
            helper: ""
            password: ""
            token: ""
            username: ""
          name: Docker Hub
          url: docker.io
telemetry:
    enabled: false
    metrics:
        exporter: prometheus
        prometheus:
            address: 127.0.0.1:9090
            path: /metrics
        stdout:
            interval: 5s
```

The `sync` section is where you define the images you want to keep in sync. The `interval` is the time between syncs, and `maxerrors` is the maximum number of errors before the sync is stopped and the program exits.

### Authentication

To provide authentication for registries, put them under `sync.registries` in the following format:

```yaml
sync:
      registries:
            - auth:
                helper: "" 
                password: ""
                token: ""
                username: ""
              name: Docker Hub
              url: docker.io
```

#### ECR

To authenticate against ECR, you can leave `password`, `token` and `username` empty, and set `helper` to `ecr`:

```yaml
sync:
      registries:
            - auth:
                helper: ecr
                password: ""
                token: ""
                username: ""
              name: ECR
              url: 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

The same applies to ecr-public, since it uses the url prefix to differentiate between the two.

Now, any image under `123456789012.dkr.ecr.us-east-1.amazonaws.com` will be authenticated using the default AWS credentials.

#### GCR

To authenticate against ECR, get either a access token or service account key.

##### Access Token

Check the [GCR documentation](https://cloud.google.com/artifact-registry/docs/docker/authentication#token) for more information.

Note that access tokens are short lived.

```yaml
sync:
  registries:
    - auth:
        helper: ""
        password: "PASSWORD"
        token: ""
        username: "oauth2accesstoken"
      name: GCR / GAR
      url: gcr.io
```

##### Service Account Key

Check the [GCR documentation](https://cloud.google.com/artifact-registry/docs/docker/authentication#json-key) for more information.

```yaml
sync:
  registries:
    - auth:
        helper: ""
        password: "BASE64_ENCODED_JSON_KEY"
        token: ""
        username: "_json_key_base64"
      name: GCR / GAR
      url: gcr.io
```