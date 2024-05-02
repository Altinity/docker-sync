#!/bin/sh

# check if /config.yaml exists
if [ ! -f /config.yaml ]; then
  /docker-sync mergeYaml -o /config.yaml -f /config_map.yaml -f /secret.yaml
fi

/docker-sync "$@"
