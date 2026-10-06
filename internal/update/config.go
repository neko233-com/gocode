package update

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

func jsonMarshal(config Config) ([]byte, error) { return json.MarshalIndent(config, "", "  ") }
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return config, err
	}
	if len(data) > 65536 {
		return config, errors.New("update config exceeds 64 KiB")
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	key, err := PublisherKey()
	if err != nil {
		return config, err
	}
	if err := (&Manager{Key: key, Config: config}).validate(); err != nil {
		return config, err
	}
	return config, nil
}
