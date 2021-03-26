package account

import (
	"context"
	"errors"
	"vindr-lab-api/keycloak"
)

type KeycloakStore struct {
	kc    *keycloak.KeycloakConfig
	realm string
}

func NewKeycloakStore(kc *keycloak.KeycloakConfig, realm string) *KeycloakStore {
	return &KeycloakStore{
		kc:    kc,
		realm: realm,
	}
}

func (app *KeycloakStore) getKeycloakSession() (*keycloak.KeycloakSession, error) {
	kc := app.kc.NewKeycloakClient()
	t, err := app.kc.NewKeycloakToken(kc)
	if err != nil {
		return nil, err
	}

	ks := &keycloak.KeycloakSession{
		Realm:  app.realm,
		Token:  t,
		Client: kc,
	}

	return ks, nil
}

func (app *KeycloakStore) GetAccount(username, id string) (*keycloak.UserModel, error) {
	users, err := app.GetAccounts(username)
	if err != nil {
		return nil, err
	}

	for i := range users {
		if users[i].ID == id {
			return users[i], nil
		}
	}

	return nil, errors.New("User is not existed")
}

func (app *KeycloakStore) GetAccountsAsMap(username string) (map[string]*keycloak.UserModel, error) {
	accounts, err := app.GetAccounts(username)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errors.New("Data is empty")
	}
	ret := make(map[string]*keycloak.UserModel)
	for i := range accounts {
		ret[accounts[i].ID] = accounts[i]
	}
	return ret, nil
}

func (app *KeycloakStore) GetAccounts(username string) ([]*keycloak.UserModel, error) {
	ks, err := app.getKeycloakSession()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	users, err := ks.GetUsers(username, ctx)
	if err != nil {
		return nil, err
	}

	data := make([]*keycloak.UserModel, 0)
	for _, user := range users {
		item, err := ks.GetUserDetail(user, ctx)
		if err != nil {
			return nil, err
		}

		data = append(data, item)
	}

	return data, nil
}
