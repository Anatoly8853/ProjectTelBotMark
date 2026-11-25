package config

import (
	slogavp "github.com/Anatoly8853/slog-avp/v2"

	"github.com/spf13/viper"
)

type Config struct {
	TGtoken       string `mapstructure:"TELEGRAM_TOKEN"`
	TGgrouptoken  string `mapstructure:"TELEGRAM_GROUPCHATID"`
	TGmesagetoken string `mapstructure:"TELEGRAM_CHATID"`
	DBName        string `mapstructure:"DB_NAME"`
	AdminUsername string `mapstructure:"ADMIN_USERNAME"`
	AdminPassword string `mapstructure:"ADMIN_PASSWORD"`
	WebAddr       string `mapstructure:"WEB_ADDR"`
	TLSCertFile   string `mapstructure:"TLS_CERT_FILE"`
	TLSKeyFile    string `mapstructure:"TLS_KEY_FILE"`
}

func LoadConfig(app *slogavp.Application) (cfg Config) {
	// Чтение файла app.env
	viper.AddConfigPath(".")
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	// Чтение переменных окружения. Явно связываем параметры администратора,
	// чтобы они загружались и из app.env, и из системного окружения.
	viper.AutomaticEnv()
	_ = viper.BindEnv("ADMIN_USERNAME")
	_ = viper.BindEnv("ADMIN_PASSWORD")
	_ = viper.BindEnv("WEB_ADDR")
	_ = viper.BindEnv("TLS_CERT_FILE")
	_ = viper.BindEnv("TLS_KEY_FILE")

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
