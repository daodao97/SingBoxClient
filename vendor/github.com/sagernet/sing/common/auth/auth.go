package auth

type Authenticator interface {
	Verify(user string, pass string) bool
	Users() []string
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type inMemoryAuthenticator struct {
	storage   map[string]string
	usernames []string
}

func (au *inMemoryAuthenticator) Verify(username string, password string) bool {
	realPass, ok := au.storage[username]
	return ok && realPass == password
}

func (au *inMemoryAuthenticator) Users() []string { return au.usernames }

func NewAuthenticator(users []User) Authenticator {
	if len(users) == 0 {
		return nil
	}
	au := &inMemoryAuthenticator{
		storage:   make(map[string]string),
		usernames: make([]string, 0, len(users)),
	}
	for _, user := range users {
		au.storage[user.Username] = user.Password
		au.usernames = append(au.usernames, user.Username)
	}
	return au
}
