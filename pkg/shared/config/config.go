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
	AppName     string `mapstructure:"app_name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
	NATSURL     string `mapstructure:"nats_url"`
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
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("config must be a non-nil pointer to struct")
	}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("config unmarshal: %w", err)
	}
	if err := loadFromEnv(rv.Elem()); err != nil {
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

		val, ok := os.LookupEnv("N0_" + envName)
		if !ok {
			val, ok = os.LookupEnv(envName)
		}
		if ok {
			switch field.Kind() {
			case reflect.String:
				field.SetString(val)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n, err := strconv.ParseInt(val, 10, field.Type().Bits())
				if err != nil {
					return fmt.Errorf("invalid integer for %s: %w", envName, err)
				}
				field.SetInt(n)
			case reflect.Bool:
				b, err := strconv.ParseBool(val)
				if err != nil {
					return fmt.Errorf("invalid boolean for %s: %w", envName, err)
				}
				field.SetBool(b)
			default:
				return fmt.Errorf("unsupported field type %s for key %s", field.Kind(), name)
			}
		}
	}
	return nil
}
