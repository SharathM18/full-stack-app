package user

import "github.com/sharathm18/myapp/internal/server"

type UserHandler struct {
	AppServer   *server.APP
	UserService *UserService
}

func NewUserHandler(appServer *server.APP, userSvc *UserService) *UserHandler {
	return &UserHandler{
		AppServer:   appServer,
		UserService: userSvc,
	}
}
