//builds go run ./cmd/api



package main 

import (
	"log"
	"github.com/dexisback/YellowBird/internal/config"
)



func main(){
	cfg, err := config.Load()
	if err != nil{
		log.Fatal(err)
	}

	//else:
	log.Println("config loaded succesfullyuh ✅")
	log.Println("up and runnin, on ", cfg.Port)
}