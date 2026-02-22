package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type OpenAIConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	TimeoutSeconds int
	Temperature    *float64
	MaxTokens      *int
}

type Config struct {
	OpenAI OpenAIConfig
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		return fmt.Errorf("missing OPENAI_API_KEY")
	}
	if cfg.OpenAI.TimeoutSeconds < 0 {
		return fmt.Errorf("OPENAI_TIMEOUT_SECONDS cannot be negative")
	}
	return nil
}

func BindFlags(flags *pflag.FlagSet) error {
	if flags == nil {
		return fmt.Errorf("flags are required")
	}

	if err := readDotEnvFromCommonPaths(); err != nil {
		return err
	}

	flags.String("model", "", "model to use")
	flags.String("base-url", "", "OpenAI-compatible API base URL")
	flags.Int("timeout", 0, "request timeout in seconds (0 disables timeout)")
	flags.Float64("temperature", 0, "sampling temperature")
	flags.Int("max-tokens", -1, "max tokens for completion")

	if err := viper.BindPFlag("openai.model", flags.Lookup("model")); err != nil {
		return fmt.Errorf("bind model flag: %w", err)
	}
	if err := viper.BindPFlag("openai.baseURL", flags.Lookup("base-url")); err != nil {
		return fmt.Errorf("bind base-url flag: %w", err)
	}
	if err := viper.BindPFlag("openai.timeoutSeconds", flags.Lookup("timeout")); err != nil {
		return fmt.Errorf("bind timeout flag: %w", err)
	}
	if err := viper.BindPFlag("openai.temperature", flags.Lookup("temperature")); err != nil {
		return fmt.Errorf("bind temperature flag: %w", err)
	}
	if err := viper.BindPFlag("openai.maxTokens", flags.Lookup("max-tokens")); err != nil {
		return fmt.Errorf("bind max-tokens flag: %w", err)
	}

	if err := viper.BindEnv("openai.apiKey", "OPENAI_API_KEY"); err != nil {
		return fmt.Errorf("bind OPENAI_API_KEY: %w", err)
	}
	if err := viper.BindEnv("openai.model", "OPENAI_MODEL"); err != nil {
		return fmt.Errorf("bind OPENAI_MODEL: %w", err)
	}
	if err := viper.BindEnv("openai.baseURL", "OPENAI_BASE_URL"); err != nil {
		return fmt.Errorf("bind OPENAI_BASE_URL: %w", err)
	}
	if err := viper.BindEnv("openai.timeoutSeconds", "OPENAI_TIMEOUT_SECONDS"); err != nil {
		return fmt.Errorf("bind OPENAI_TIMEOUT_SECONDS: %w", err)
	}
	if err := viper.BindEnv("openai.temperature", "OPENAI_TEMPERATURE"); err != nil {
		return fmt.Errorf("bind OPENAI_TEMPERATURE: %w", err)
	}
	if err := viper.BindEnv("openai.maxTokens", "OPENAI_MAX_TOKENS"); err != nil {
		return fmt.Errorf("bind OPENAI_MAX_TOKENS: %w", err)
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
	timeoutSeconds := viper.GetInt("openai.timeoutSeconds")

	var temperature *float64
	if viper.IsSet("openai.temperature") {
		value := viper.GetFloat64("openai.temperature")
		temperature = &value
	}

	var maxTokens *int
	if viper.IsSet("openai.maxTokens") {
		value := viper.GetInt("openai.maxTokens")
		maxTokens = &value
	}

	return Config{
		OpenAI: OpenAIConfig{
			APIKey:         strings.TrimSpace(viper.GetString("openai.apiKey")),
			BaseURL:        strings.TrimSpace(viper.GetString("openai.baseURL")),
			Model:          strings.TrimSpace(viper.GetString("openai.model")),
			TimeoutSeconds: timeoutSeconds,
			Temperature:    temperature,
			MaxTokens:      maxTokens,
		},
	}
}
