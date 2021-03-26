package mw

import (
	"encoding/json"
)

type AuthClaim struct {
	Exp          int64  `json:"exp"`
	Iat          int    `json:"iat"`
	AuthTime     int    `json:"auth_time"`
	Jti          string `json:"jti"`
	Iss          string `json:"iss"`
	Aud          string `json:"aud"`
	Sub          string `json:"sub"`
	Typ          string `json:"typ"`
	Azp          string `json:"azp"`
	SessionState string `json:"session_state"`
	Acr          string `json:"acr"`
	RealmAccess  struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	ResourceAccess map[string]interface{} `json:"resource_access"`
	Authorization  struct {
		Permissions []struct {
			Scopes []string `json:"scopes"`
			Rsid   string   `json:"rsid"`
			Rsname string   `json:"rsname"`
		} `json:"permissions"`
	} `json:"authorization"`
	Scope             string   `json:"scope"`
	RealmRoles        []string `json:"realm_roles"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
}

type Account struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	SystemRoles []string `json:"system_roles"`
	ExpTime     int64    `json:"exp_time"`
}

func (object *Account) String() string {
	b, _ := json.Marshal(object)
	return string(b)
}

func (object *AuthClaim) String() string {
	b, _ := json.Marshal(object)
	return string(b)
}

func (authClaim *AuthClaim) ConvertAuthClaimToAccount() *Account {
	return &Account{
		ID:          authClaim.Sub,
		SystemRoles: authClaim.RealmRoles,
		ExpTime:     authClaim.Exp,
		Username:    authClaim.PreferredUsername,
	}
}
