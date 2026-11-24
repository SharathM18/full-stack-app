package user

import "github.com/sharathm18/myapp/internal/server"

type UserService struct {
	AppServer      *server.APP
	UserRepository *UserRepository
}

func NewUserService(appServer *server.APP, userRepo *UserRepository) *UserService {
	return &UserService{
		AppServer:      appServer,
		UserRepository: userRepo,
	}
}
