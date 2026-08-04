package config

import (
	slogavp "github.com/Anatoly8853/slog-avp/v2"

	"github.com/spf13/viper"
)

type Config struct {
	TGtoken       string `mapstructure:"TELEGRAM_TOKEN"`
	TGgrouptoken  string `mapstructure:"TELEGRAM_GROUPCHATID"`
	TGmesagetoken string `mapstructure:"TTELEGRAM_CHATID"`
	DBName        string `mapstructure:"DB_NAME"`
}

func LoadConfig(app *slogavp.Application) (cfg Config) {
	// Чтение файла app.env
	viper.AddConfigPath(".")
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	// Чтение переменных окружения
	viper.AutomaticEnv() // Автоматически читать переменные окружения

	if err := viper.MergeInConfig(); err != nil {
		app.Log.Printf("Error reading app.env file, %s", err)
	}

	if err := viper.ReadInConfig(); err != nil {
		app.Log.Fatalf("Error reading config file, %s", err)
	}

	err := viper.Unmarshal(&cfg)
	if err != nil {
		app.Log.Fatalf("unable to decode into struct, %v", err)
	}
	return cfg
}
