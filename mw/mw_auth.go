package mw

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"vindr-lab-api/keycloak"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var GIN_CONTEXT_AUTHINFO = "AuthInfo"
var DEFAULT_API_KEY = "YOUR_SECRET_API_KEY"
var JWT_KEYS *keycloak.JWTKeys

func getJWTKeysFromKeycloak(uri string) keycloak.JWTKeys {
	utils.LogDebug(uri)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	res, err := http.Get(uri)
	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	jwtKeys := keycloak.JWTKeys{}
	err = json.Unmarshal(body, &jwtKeys)
	if err != nil {
		panic(err)
	}

	return jwtKeys
}

func ParseJWTAccessToken(apiKey, token string) (*Account, error) {

	if JWT_KEYS == nil {
		JWT_KEYS = &keycloak.JWTKeys{}
		*JWT_KEYS = getJWTKeysFromKeycloak(fmt.Sprintf("%s/auth/realms/%s",
			viper.GetString("keycloak.uri"), viper.GetString("keycloak.app_realm")))
		utils.LogDebug(JWT_KEYS.String())
	}

	isValid, err := VerifyTokenWithPubkey(token, JWT_KEYS.PublicKey)
	if err != nil {
		return nil, err
	}

	if !isValid {
		fmt.Println("The token is invalid")
		return nil, errors.New("The token is invalid")
	}

	_token, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return nil, nil
	})

	var authClaim AuthClaim

	if claims, ok := _token.Claims.(jwt.MapClaims); ok {
		jsonString, _ := json.Marshal(claims)
		json.Unmarshal(jsonString, &authClaim)
		account := authClaim.ConvertAuthClaimToAccount()

		now := time.Now().Unix()

		if apiKey != "" && apiKey == DEFAULT_API_KEY {
			return account, nil
		}

		if now > authClaim.Exp {
			return nil, errors.New("Token expired")
		}

		if account.Username != "" {
			return account, nil
		}
		return nil, err
	}
	return nil, err

}

func WrapAuthInfo(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			auth  Account
			_auth []byte
			err   error
		)

		apiKey := c.GetHeader("x-api-key")
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		splitted := strings.Split(authHeader, " ")
		logger.Debug("Request headers", zap.String("Authorization", authHeader))
		xUserinfoHeader := c.GetHeader("X-USERINFO")
		logger.Debug("X-USERINFO headers", zap.String("X-USERINFO", xUserinfoHeader))

		switch {
		case splitted[0] == "Bearer":
			if len(splitted) == 2 {
				var authp *Account
				authp, err = ParseJWTAccessToken(apiKey, (splitted[1]))
				if err != nil {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				auth = *authp
			} else {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		case xUserinfoHeader != "":
			_auth, err = base64.StdEncoding.DecodeString(xUserinfoHeader)
			err = json.Unmarshal(_auth, &auth)

		default:
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}
		c.Set(GIN_CONTEXT_AUTHINFO, &auth)
		c.Next()
	}
}

func GetAuthInfoFromGin(c *gin.Context) *Account {
	if inf, exists := c.Get(GIN_CONTEXT_AUTHINFO); exists {
		var account Account
		bytes, err := json.Marshal(inf)
		if err != nil {
			return nil
		}
		json.Unmarshal(bytes, &account)
		return &account
	}
	return nil
}

func ValidPerms(rResource, rScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		splitted := strings.Split(authHeader, " ")
		token := splitted[1]

		p, err := ParseJWTAccessTokenToObject(token)
		if err != nil {
			return
		}

		for _, perm := range p.Authorization.Permissions {
			for _, scope := range perm.Scopes {
				if scope == rScope && perm.Rsname == rResource {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatus(http.StatusForbidden)
		return
	}
}

func ParseJWTAccessTokenToObject(token string) (*AuthClaim, error) {
	_token, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return nil, nil
	})

	parsedJWT := &AuthClaim{}
	if claims, ok := _token.Claims.(jwt.MapClaims); ok {
		bytes, _ := json.Marshal(claims)
		json.Unmarshal(bytes, parsedJWT)
		return parsedJWT, nil
	}
	return nil, err
}

func VerifyTokenWithPubkey(token, keyData string) (bool, error) {
	if !strings.Contains(keyData, "BEGIN PUBLIC KEY") {
		keyData = fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", keyData)
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(keyData))
	if err != nil {
		return false, err
	}

	parts := strings.Split(token, ".")
	err = jwt.SigningMethodRS256.Verify(strings.Join(parts[0:2], "."), parts[2], key)
	if err != nil {
		return false, nil
	}
	return true, nil
}
