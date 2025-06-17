#!/bin/sh

# check if /config.yaml exists
if [ ! -f /config.yaml ]; then
  /docker-sync mergeYaml -o /config.yaml -f /config_map.yaml -f /secret.yaml
  if [ $? -ne 0 ]; then
    echo "Error merging YAML files. Exiting."
    exit 1
  fi
fi

/docker-sync "$@"
