package main

import (
	"flag"

	"github.com/dnonakolesax/cccad-locks/internal/application"
)

// @title OIDC API
// @version 1.0
// @description API for authorising users and storing their info

// @contact.name G
// @contact.email bg@dnk33.com

// @host oauth.dnk33.com
// @BasePath /api/v1/iam.
func main() {
	configsPath := flag.String("configs", "./configs", "Path to configs")
	flag.Parse()

	a, err := application.NewApp(*configsPath)
	if err != nil {
		panic(err)
	}

	a.Run()
}
