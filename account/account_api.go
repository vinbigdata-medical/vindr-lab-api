package account

import (
	"fmt"
	"net/http"
	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AccountAPI struct {
	kcs    *KeycloakStore
	logger *zap.Logger
}

func NewAccountAPI(kcs *KeycloakStore, logger *zap.Logger) (app *AccountAPI) {
	app = &AccountAPI{
		kcs:    kcs,
		logger: logger,
	}
	return app
}

var mapRoleRank = map[string]int{
	"PO":         0,
	"PO_PARTNER": 1,
	"REVIEWER":   2,
	"ANNOTATOR":  4,
	"GUEST":      8,
}

var mapScope = map[string]string{
	"C": "create",
	"R": "read",
	"U": "update",
	"D": "delete",
}

func (app *AccountAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("/userinfo", mw.ValidPerms(path, mw.PERM_R), app.GetAccounts)
	group.GET("/userinfo/:id", mw.ValidPerms(path, mw.PERM_R), app.GetAccount)
	group.GET("/permissions", app.GetPermission)
}

func (app *AccountAPI) GetPermission(c *gin.Context) {
	resp := entities.NewResponse()

	authInfo := mw.GetAuthInfoFromGin(c)
	role := ""
	roleRank := 10000
	for i := range authInfo.SystemRoles {
		if rank, ok := mapRoleRank[authInfo.SystemRoles[i]]; ok {
			if rank < roleRank {
				roleRank = rank
				role = authInfo.SystemRoles[i]
			}
		}
	}

	index := -1
	data := make([]string, 0)
	err := utils.ReadCSVByLines("conf/permissions.csv", func(items []string) {
		if items[0] == "X" {
			index, _ = utils.FindInSlice(items, role)
		} else {
			if index != -1 {
				runes := []rune(items[index])
				for i := range runes {
					scope := mapScope[string(runes[i])]
					data = append(data, fmt.Sprintf("%s#%s", items[0], scope))
				}
			}
		}
	})

	utils.LogDebug("%v\t%s\t%v", authInfo, role, data)

	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = data
	c.JSON(http.StatusOK, resp)
}

func (app *AccountAPI) GetAccounts(c *gin.Context) {
	resp := entities.NewResponse()

	username := c.Query("username")
	data, err := app.kcs.GetAccounts(username)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = data
	c.JSON(http.StatusOK, resp)
}

func (app *AccountAPI) GetAccount(c *gin.Context) {
	resp := entities.NewResponse()

	userID := c.Param(constants.ParamID)
	username := c.Query("username")
	if userID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	data, err := app.kcs.GetAccount(username, userID)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = data
	c.JSON(http.StatusOK, resp)
}
