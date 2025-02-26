package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Altinity/docker-sync/structs"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

type Key struct {
	Name            string
	Default         interface{}
	Value           interface{}
	ValidationFuncs []func(interface{}) error
	mutex           sync.Mutex
}

type KeyOption func(*Key)

type ReloadedKey struct {
	Key      string
	Error    error
	OldValue interface{}
	NewValue interface{}
}

// Mimic the behavior of viper.Get???() calls.
func (k *Key) String() string {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToString(k.Value)
}

func (k *Key) Int() int {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToInt(k.Value)
}

func (k *Key) Int64() int64 {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToInt64(k.Value)
}

func (k *Key) UInt64() uint64 {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToUint64(k.Value)
}

func (k *Key) Duration() time.Duration {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToDuration(k.Value)
}

func (k *Key) Bool() bool {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToBool(k.Value)
}

func (k *Key) Float64() float64 {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToFloat64(k.Value)
}

func (k *Key) StringSlice() []string {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return cast.ToStringSlice(k.Value)
}

func (k *Key) Images() []*structs.Image {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var images []*structs.Image

	s, err := cast.ToSliceE(k.Value)
	if err != nil {
		return nil
	}

	for _, i := range s {
		m, err := cast.ToStringMapE(i)
		if err != nil {
			return nil
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}

		var image structs.Image

		if err := json.Unmarshal(b, &image); err != nil {
			return nil
		}

		images = append(images, &image)
	}

	return images
}

func (k *Key) Repositories() []*structs.Repository {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var repos []*structs.Repository

	s, err := cast.ToSliceE(k.Value)
	if err != nil {
		return nil
	}

	for _, i := range s {
		m, err := cast.ToStringMapE(i)
		if err != nil {
			return nil
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}

		var repo structs.Repository

		if err := json.Unmarshal(b, &repo); err != nil {
			return nil
		}

		repos = append(repos, &repo)
	}

	return repos
}

func (k *Key) Update() *ReloadedKey {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	r := &ReloadedKey{
		Key:      k.Name,
		OldValue: k.Value,
		NewValue: viper.Get(k.Name),
	}

	if fmt.Sprintf("%+v", r.OldValue) == fmt.Sprintf("%+v", r.NewValue) {
		return nil
	}

	for _, f := range k.ValidationFuncs {
		if err := f(r.NewValue); err != nil {
			r.Error = fmt.Errorf("validation failed: %v", err)
			return r
		}
	}

	k.Value = r.NewValue

	return r
}

func (k *Key) register() {
	keys[k.Name] = k

	if k.Default != nil {
		viper.SetDefault(k.Name, k.Default)
	}
}

// NewKey creates a new configuration key with the specified name and additional options.
func NewKey(name string, opts ...KeyOption) *Key {
	k := &Key{
		Name:  name,
		mutex: sync.Mutex{},
	}

	for _, opt := range opts {
		opt(k)
	}

	k.register()

	return k
}

// WithDefaultValue sets the default value for the configuration key.
func WithDefaultValue(defaultValue interface{}) KeyOption {
	return func(k *Key) {
		if defaultValue != nil {
			k.Default = defaultValue
			viper.SetDefault(k.Name, defaultValue)
		}
	}
}

// WithValidationFunc adds a validation function for the configuration key.
func WithValidationFunc(f func(interface{}) error) KeyOption {
	return func(k *Key) {
		k.ValidationFuncs = append(k.ValidationFuncs, f)
	}
}

// WithAllowedStrings sets the allowed values for the configuration key.
func WithAllowedStrings(values []string) KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToStringE(v)
		if err != nil {
			return err
		}

		for _, allowed := range values {
			if s == allowed {
				return nil
			}
		}

		return fmt.Errorf("value %q is not allowed, must be one of %v", s, values)
	})
}

// WithAllowedInts sets the allowed values for the configuration key.
func WithAllowedInts(values []int) KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		i, err := cast.ToIntE(v)
		if err != nil {
			return err
		}

		for _, allowed := range values {
			if i == allowed {
				return nil
			}
		}

		return fmt.Errorf("value %d is not allowed, must be one of %v", i, values)
	})
}

// WithValidString checks if the value is a string.
func WithValidString() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToStringE(v)
		return err
	})
}

// WithValidInt checks if the value is an integer.
func WithValidInt() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToIntE(v)
		return err
	})
}

// WithValidDuration checks if the value is a duration.
func WithValidDuration() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToDurationE(v)
		return err
	})
}

// WithValidStringSlice checks if the value is a string slice.
func WithValidStringSlice() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToStringSliceE(v)
		return err
	})
}

// WithValidBool checks if the value is a boolean.
func WithValidBool() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToBoolE(v)
		return err
	})
}

// WithValidFloat64 checks if the value is a float64.
func WithValidFloat64() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToFloat64E(v)
		return err
	})
}

// WithValidExistingPathOrEmpty checks if the value is an existing path or if it is empty.
func WithValidExistingPathOrEmpty() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		if v == nil {
			return nil
		}

		s, err := cast.ToStringE(v)
		if err != nil {
			return err
		}

		if s == "" {
			return nil
		}

		if _, err := os.Stat(s); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path %q does not exist", s)
			}

			return err
		}

		return nil
	})
}

// WithValidPositiveInt checks if the value is a positive integer.
func WithValidPositiveInt() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		i, err := cast.ToIntE(v)
		if err != nil {
			return err
		}

		if i < 0 {
			return fmt.Errorf("value must be positive")
		}

		return nil
	})
}

// WithValidNetHostPort checks if the value is a valid host:port string.
func WithValidNetHostPort() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToStringE(v)
		if err != nil {
			return err
		}

		host, port, err := net.SplitHostPort(s)
		if err != nil {
			return fmt.Errorf("invalid host:port %q", s)
		}

		if addrs, err := net.LookupHost(host); err != nil || len(addrs) == 0 {
			return fmt.Errorf("invalid host %q: %w", host, err)
		}

		p, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port %q: %w", port, err)
		}

		if p < 0 || p > 65535 {
			return fmt.Errorf("port %q is out of range", port)
		}

		return nil
	})
}

// WithValidURL checks if the value is a valid URL.
func WithValidURL() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToStringE(v)
		if err != nil {
			return err
		}

		u, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("invalid URL %q: %w", s, err)
		}

		if addrs, err := net.LookupHost(u.Hostname()); err != nil || len(addrs) == 0 {
			return fmt.Errorf("invalid host %q: %w", u.Hostname(), err)
		}

		port := u.Port()
		if port == "" {
			switch u.Scheme {
			case "http":
				port = "80"
			case "https":
				port = "443"
			}
		}

		p, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port %q: %w", u.Port(), err)
		}

		if p < 0 || p > 65535 {
			return fmt.Errorf("port %q is out of range", u.Port())
		}

		return nil
	})
}

// WithValidURI checks if the value is a valid URI.
func WithValidURI() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToStringE(v)
		if err != nil {
			return err
		}

		if _, err := url.ParseRequestURI(s); err != nil {
			return fmt.Errorf("invalid URL path %q: %w", s, err)
		}

		return nil
	})
}

// WithValidMap checks if the value is a map.
func WithValidMap() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		_, err := cast.ToStringMapE(v)
		return err
	})
}

// WithValidImages checks if the value is a valid image.
func WithValidImages() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToSliceE(v)
		if err != nil {
			return err
		}

		for _, i := range s {
			m, err := cast.ToStringMapE(i)
			if err != nil {
				return err
			}

			b, err := json.Marshal(m)
			if err != nil {
				return err
			}

			var image structs.Image

			if err := json.Unmarshal(b, &image); err != nil {
				return err
			}

			if image.Source == "" {
				return fmt.Errorf("source is required")
			}

			if len(image.Targets) == 0 {
				return fmt.Errorf("at least one target is required")
			}

		}

		return nil
	})
}

// WithValidRepositories checks if the value is a valid repository.
func WithValidRepositories() KeyOption {
	return WithValidationFunc(func(v interface{}) error {
		s, err := cast.ToSliceE(v)
		if err != nil {
			return err
		}

		for _, i := range s {
			m, err := cast.ToStringMapE(i)
			if err != nil {
				return err
			}

			b, err := json.Marshal(m)
			if err != nil {
				return err
			}

			var repo structs.Repository

			if err := json.Unmarshal(b, &repo); err != nil {
				return err
			}

			if repo.Name == "" {
				return fmt.Errorf("name is required")
			}

			if repo.URL == "" {
				return fmt.Errorf("url is required")
			}
		}

		return nil
	})
}
