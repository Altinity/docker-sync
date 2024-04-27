package config

import (
	"sync"
)

var configReloadMutex = &sync.Mutex{}

func Reload() []*ReloadedKey {
	configReloadMutex.Lock()
	defer configReloadMutex.Unlock()

	var reloadedKeys []*ReloadedKey

	for k := range keys {
		update := keys[k].Update()
		if update != nil {
			reloadedKeys = append(reloadedKeys, update)
		}
	}

	return reloadedKeys
}
