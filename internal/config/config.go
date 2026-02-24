package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

type Config struct {
}

func (cfg Config) Validate() error {
	return nil
}

func BindFlags(flags *pflag.FlagSet) error {
	if flags == nil {
		return fmt.Errorf("flags are required")
	}

	if err := readDotEnvFromCommonPaths(); err != nil {
		return err
	}

	return nil
}

func readDotEnvFromCommonPaths() error {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
		filepath.Join("tmp", ".env"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}

		if err := godotenv.Load(candidate); err != nil {
			return fmt.Errorf("read %s: %w", candidate, err)
		}
		return nil
	}

	return nil
}

func Load() Config {
	return Config{}
}
