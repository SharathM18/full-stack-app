package user

import "github.com/sharathm18/myapp/internal/server"

type UserRepository struct {
	AppServer *server.APP
}

func NewUserRepository(appSrv *server.APP) *UserRepository {
	return &UserRepository{
		AppServer: appSrv,
	}
}
