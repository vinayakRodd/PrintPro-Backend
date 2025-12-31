package app

import (
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/repositories"
)

// Repositories holds all repository instances
type Repositories struct {
	UserRepository            *repositories.UserRepository
	AccountRepository         *repositories.AccountRepository
	PartnerProfileRepository  *repositories.PartnerProfileRepository
	CustomerProfileRepository *repositories.CustomerProfileRepository
}

// setupRepositories initializes all repositories
func setupRepositories(postgresClient *infrastructure.PostgresClient) *Repositories {
	return &Repositories{
		UserRepository:            repositories.NewUserRepository(postgresClient.GetPool()),
		AccountRepository:         repositories.NewAccountRepository(postgresClient.GetPool()),
		PartnerProfileRepository:  repositories.NewPartnerProfileRepository(postgresClient.GetPool()),
		CustomerProfileRepository: repositories.NewCustomerProfileRepository(postgresClient.GetPool()),
	}
}

