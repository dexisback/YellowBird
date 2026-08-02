//using koanf


//both executables need ts -- cmd/api and cmd/worker 
package config 

import(
	"fmt"
	"github.com/joho/godotenv"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/koanf/providers/env"
	
)


type Config struct {
	Port   string 
	DatabaseURL   string 
	JWT_SECRET    string 
}

func Load() (*Config , error){
	_ = godotenv.Load()

	k := koanf.New(".")

	if err := k.Load(env.Provider("", ".", nil), nil); err != nil{
		return nil, fmt.Errorf("failed to load env variables %w", err)
	}

	cfg := &Config{
		Port : k.String("PORT"),
		DatabaseURL: k.String("DATABASE_URL"),
		JWT_SECRET: k.String("JWT_SECRET"),
	}


	return cfg, nil

}