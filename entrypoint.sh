#!/bin/sh

# check if /config.yaml exists and first argument isn't 'sync'
if [ ! -f /config.yaml ] && [ "$1" != "sync" ]; then
  /docker-sync mergeYaml -o /config.yaml -f /config_map.yaml -f /secret.yaml
  if [ $? -ne 0 ]; then
    echo "Error merging YAML files. Exiting."
    exit 1
  fi
fi

/docker-sync "$@"
