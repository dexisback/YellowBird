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
	CLOUDINARY_CLOUD_NAME   string 
	CLOUDINARY_API_KEY    string 
	CLOUDINARY_API_SECRET   string 
	RedisAddr     string 
	RedisPassword   string 
	RedisDB       int 
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

		CLOUDINARY_CLOUD_NAME: k.String("CLOUDINARY_CLOUD_NAME"),
		CLOUDINARY_API_KEY:    k.String("CLOUDINARY_API_KEY"),
		CLOUDINARY_API_SECRET:    k.String("CLOUDINARY_API_SECRET"),


		RedisAddr:   k.String("REDIS_ADDR"),
		RedisPassword: k.String("REDIS_PASSWORD"),
		RedisDB: k.Int("REDIS_DB"),
	}


	return cfg, nil

}