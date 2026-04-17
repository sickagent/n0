package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BaseConfig is embedded into every service-specific config.
type BaseConfig struct {
	AppName  string `mapstructure:"app_name"`
	LogLevel string `mapstructure:"log_level"`
	NATSURL  string `mapstructure:"nats_url"`
}

// InitCobra wires Viper with environment variables and flags on the provided cobra command.
func InitCobra(cmd *cobra.Command, _ string) error {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("bind flags: %w", err)
	}
	return nil
}

// Load populates the provided struct from Viper and then overrides with explicit env vars.
func Load(cfg interface{}) error {
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("config unmarshal: %w", err)
	}
	if err := loadFromEnv(reflect.ValueOf(cfg).Elem()); err != nil {
		return fmt.Errorf("config env override: %w", err)
	}
	return nil
}

func loadFromEnv(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		ft := t.Field(i)

		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := loadFromEnv(field); err != nil {
				return err
			}
			continue
		}

		tag := ft.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		envName := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))

		if val, ok := os.LookupEnv(envName); ok {
			switch field.Kind() {
			case reflect.String:
				field.SetString(val)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if n, err := strconv.Atoi(val); err == nil {
					field.SetInt(int64(n))
				}
			case reflect.Bool:
				if b, err := strconv.ParseBool(val); err == nil {
					field.SetBool(b)
				}
			default:
				return fmt.Errorf("unsupported field type %s for key %s", field.Kind(), name)
			}
		}
	}
	return nil
}
