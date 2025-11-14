package main

import (
	"arunika_backend/config"
	"arunika_backend/registry"
	"arunika_backend/routes"
)

func main() {
	config.Init()
	db := config.DB
	redis := config.RDB

	services := registry.NewServiceRegistry(db, redis)

	r := routes.SetupRouter(services, redis)
	r.Run(":8080")
}
