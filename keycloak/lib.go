package keycloak

import (
	"context"
	"encoding/json"

	"github.com/Nerzal/gocloak/v7"
)

type JWTKeys struct {
	Keys []struct {
		Kid     string   `json:"kid"`
		Kty     string   `json:"kty"`
		Alg     string   `json:"alg"`
		Use     string   `json:"use"`
		N       string   `json:"n"`
		E       string   `json:"e"`
		X5C     []string `json:"x5c"`
		X5T     string   `json:"x5t"`
		X5TS256 string   `json:"x5t#S256"`
	} `json:"keys"`
	Realm           string `json:"realm"`
	PublicKey       string `json:"public_key"`
	TokenService    string `json:"token-service"`
	AccountService  string `json:"account-service"`
	TokensNotBefore int    `json:"tokens-not-before"`
}

func (jwtKeys *JWTKeys) String() string {
	b, _ := json.Marshal(jwtKeys)
	return string(b)
}

type KeycloakConfig struct {
	MasterRealm   string
	AdminUsername string
	AdminPassword string
	KeycloakURI   string
}

type UserModel struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

func (c *KeycloakConfig) NewKeycloakClient() gocloak.GoCloak {
	return gocloak.NewClient(c.KeycloakURI)
}

func (c *KeycloakConfig) NewKeycloakToken(client gocloak.GoCloak) (*gocloak.JWT, error) {
	ctx := context.Background()
	return client.LoginAdmin(ctx, c.AdminUsername, c.AdminPassword, c.MasterRealm)
}

func (c *KeycloakConfig) CreateNewUser(user gocloak.User, realm string) error {
	ctx := context.Background()
	kc := c.NewKeycloakClient()
	token, err := c.NewKeycloakToken(kc)

	if err != nil {
		return err
	}

	_, err = kc.CreateUser(ctx, token.AccessToken, realm, user)
	return err
}

func (c *KeycloakConfig) CreateRealm(realm gocloak.RealmRepresentation) error {
	ctx := context.Background()
	kc := c.NewKeycloakClient()
	token, err := c.NewKeycloakToken(kc)

	if err != nil {
		return err
	}

	_, err = kc.CreateRealm(ctx, token.AccessToken, realm)
	return err
}

func (c *KeycloakConfig) CreateRealmRole(role gocloak.Role, realm string) error {
	ctx := context.Background()
	kc := c.NewKeycloakClient()
	token, err := c.NewKeycloakToken(kc)

	if err != nil {
		return err
	}

	_, err = kc.CreateRealmRole(ctx, token.AccessToken, realm, role)
	return err
}

type KeycloakSession struct {
	Realm  string
	Token  *gocloak.JWT
	Client gocloak.GoCloak
}

func (s *KeycloakSession) CheckUserExistence(username string, ctx context.Context) (bool, error) {

	users, err := s.Client.GetUsers(ctx,
		s.Token.AccessToken,
		s.Realm,
		gocloak.GetUsersParams{
			Username: &username,
		},
	)
	if err != nil {
		return false, err
	}

	for _, user := range users {
		if *user.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (s *KeycloakSession) GetUsers(username string, ctx context.Context) ([]*gocloak.User, error) {
	users, err := s.Client.GetUsers(ctx,
		s.Token.AccessToken,
		s.Realm,
		gocloak.GetUsersParams{
			Username: &username,
		},
	)

	return users, err
}

func (s *KeycloakSession) GetUserRoles(user *gocloak.User, ctx context.Context) ([]*gocloak.Role, error) {
	roles, err := s.Client.GetRealmRolesByUserID(ctx, s.Token.AccessToken, s.Realm, *user.ID)
	return roles, err
}

func (s *KeycloakSession) GetUserDetail(user *gocloak.User, ctx context.Context) (*UserModel, error) {

	user0 := UserModel{}
	roles, err := s.GetUserRoles(user, ctx)
	if err != nil {
		return nil, err
	}
	user0.ID = *user.ID
	user0.Username = *user.Username
	user0.Roles = make([]string, 0)
	for _, role := range roles {
		switch *role.Name {
		case "uma_authorization", "offline_access":
			continue
		default:
			user0.Roles = append(user0.Roles, *role.Name)
		}
	}
	if err != nil {
		return nil, err
	}
	return &user0, nil
}
