package session

import (
	"fmt"
	"net/http"
	"time"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SessionAPI struct {
	store    *SessionES
	esClient *elasticsearch.Client
	logger   *zap.Logger
}

func NewSessionAPI(sessionS *SessionES, logger *zap.Logger) (app *SessionAPI) {
	app = &SessionAPI{
		store:  sessionS,
		logger: logger,
	}
	return app
}

func (app *SessionAPI) InitRoute(engine *gin.Engine, path string) {
	g := engine.Group(path, mw.WrapAuthInfo(app.logger))
	g.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateSession)
	g.GET("/:session_id", mw.ValidPerms(path, mw.PERM_R), app.GetSession)
}

func (app *SessionAPI) GetSession(c *gin.Context) {
	resp := entities.NewResponse()

	sessionID := c.Param(constants.ParamSessionID)
	session, _, err := app.store.Get(nil, fmt.Sprintf("session_id.keyword:%s", sessionID))
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = session.Data
	resp.Count = len(session.Data)

	c.JSON(http.StatusOK, resp)
}

func (app *SessionAPI) CreateSession(c *gin.Context) {

	resp := entities.NewResponse()

	var session Session
	err := c.ShouldBindJSON(&session)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
	} else {
		if !session.IsValidData() {
			resp.ErrorCode = constants.ServerInvalidData
		} else {
			newID := uuid.New().String()
			session.SessionID = newID
			session.Created = time.Now().UnixNano() / int64(time.Millisecond)
			err := app.store.Create(session)

			if err != nil {
				resp.ErrorCode = constants.ServerError
				c.JSON(http.StatusOK, resp)
				return
			}

			ret := make(map[string]interface{}, 0)
			ret[constants.ParamSessionID] = newID
			resp.Data = ret
		}
	}

	c.JSON(http.StatusOK, resp)
}
