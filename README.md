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

To run a one-off synchronization, you can run `dist/docker-sync sync`.

```sh
dist/docker-sync --source docker.io/library/ubuntu --target r2:blablabla:docker-sync-test:ubuntu --source-username foo --source-password bar --target-username foo --target-password bar
```

Run `dist/docker-sync sync --help` for more configuration options.

## Configuration

Write the default config file:

```console
dist/docker-sync writeConfig -o config.yaml`
```

Edit the config file accordingly, then run:

```console
dist/docker-sync
```

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

#### R2 (target only)

```yaml
sync:
    images:
        - source: docker.io/library/ubuntu
          targets:
            - r2:f6934f56ce237241104dbe9302cee786:docker-sync-test:ubuntu # r2:<endpoint>:<bucket>:<image>
      registries:
            - auth:
                helper: ""
                password: "SECRET_ACCES_KEY"
                token: ""
                username: "ACCESS_KEY_ID"
              name: R2
              url: r2:f6934f56ce237241104dbe9302cee786:docker-sync-test # r2:<endpoint>:<bucket>
```

Note that pulls should be performed against the bucket's public url. Check [the docs](https://developers.cloudflare.com/r2/buckets/public-buckets/#enable-managed-public-access) for more information.

Don't use the standard `r2.dev` domain, as some rules need to be created and they won't work without a custom domain.

To match the [official spec](https://github.com/openshift/docker-distribution/blob/master/docs/spec/api.md#api-version-check), some rules need to be created.

Use the Cloudflare UI to create the rules by going to Rules > Transform Rules.

##### V2 Ping Fix - /v2/ requires `200 OK`

Create a **Rewrite URL** rule.

```
**If incoming requests match...**: Custom filter expression

**URI Path**: `equals` `/v2/`

**Expression Preview**: `(http.request.uri.path eq "/v2/")` (optionally also add your hostname for a better match)

**Then...**: Path > Rewrite to... > Static > `/v2` (without the trailing slash)
```

##### V2 Ping Fix - /v2/ requires `Docker-Distribution-API-Version` header

Create a **Modify Response Header** rule.

```
**If incoming requests match...**: Custom filter expression

**URI Path**: `starts with` `/v2/`

**Expression Preview**: `(starts_with(http.request.uri.path, "/v2"))` (optionally also add your hostname for a better match)

**Then...**: Set static > `Docker-Distribution-API-Version` > `registry/2.0`
```

#### S3 (target only)

```yaml
sync:
    images:
        - source: docker.io/library/ubuntu
          targets:
            - r3:us-east-1:docker-sync-test:ubuntu # s3:<region>:<bucket>:<image>
      registries:
            - auth:
                helper: ""
                password: "SECRET_ACCES_KEY"
                token: ""
                username: "ACCESS_KEY_ID"
              name: S3
              url: s3:us-east-1:docker-sync-test # s3:<region>:<bucket>
```
