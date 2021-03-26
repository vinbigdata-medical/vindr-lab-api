package annotation

import (
	"fmt"
	"net/http"

	"vindr-lab-api/account"
	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AnnotationAPI struct {
	antnStore     *AnnotationES
	labelStore    *LabelES
	keycloakStore *account.KeycloakStore
	Logger        *zap.Logger
}

func NewAnnotationAPI(antnStore *AnnotationES, labelStore *LabelES, keycloakStore *account.KeycloakStore, logger *zap.Logger) (app *AnnotationAPI) {
	app = &AnnotationAPI{
		antnStore:     antnStore,
		labelStore:    labelStore,
		keycloakStore: keycloakStore,
		Logger:        logger,
	}
	return app
}

func (app *AnnotationAPI) InitRoute(engine *gin.Engine, path string) {
	g := engine.Group(path, mw.WrapAuthInfo(app.Logger))
	g.GET("", mw.ValidPerms(path, mw.PERM_R), app.fetchAnnotations)
	g.POST("", mw.ValidPerms(path, mw.PERM_C), app.createNewAnnotation)
	g.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.updateAnnotation)
	g.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.deleteAnnotation)
}

func (app *AnnotationAPI) fetchAnnotations(c *gin.Context) {
	resp := entities.NewResponse()

	queries, qs, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)

	antns, _, err := app.antnStore.GetSlice(queries, qs, from, size, sort, aggs)

	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	mapUsers, err := app.keycloakStore.GetAccountsAsMap("")
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	mapLabelIDs := make(map[string]bool)
	for i, antn := range antns {
		userID := antn.CreatorID
		if creator, found := mapUsers[userID]; found {
			antns[i].CreatorName = creator.Username
		}

		for j := range antn.LabelIDs {
			mapLabelIDs[antn.LabelIDs[j]] = true
		}
	}
	queryLabels := make([]string, 0)
	for k := range mapLabelIDs {
		queryLabels = append(queryLabels, k)
	}

	mapLabels := make(map[string]*Label)
	err = app.labelStore.Query(map[string][]string{"_id": queryLabels}, "", 0, constants.DefaultLimit, "", nil, func(labels []Label, e entities.ESReturn) {
		for i, item := range labels {
			mapLabels[item.ID] = &labels[i]
		}
	})
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	antnsReturn := make(map[string][]Annotation)
	groupAnnotationsToMap(antns, mapLabels, antnsReturn)

	resp.Data = antnsReturn

	c.JSON(http.StatusOK, resp)
}

func groupAnnotationsToMap(antns []Annotation, mapLabels map[string]*Label, antnsReturn map[string][]Annotation) {
	for i, antn := range antns {
		labelType := ""
		for _, labelID := range antn.LabelIDs {
			labelData := mapLabels[labelID]
			if labelData == nil {
				utils.LogDebug("Invalid label: %s in annotation: %s", labelID, antn.ID)
			} else {
				labelType = labelData.Type
				if antn.Labels == nil {
					newLabels := make([]Label, 0)
					antn.Labels = &newLabels
				}
				*antn.Labels = append(*antn.Labels, *labelData)
			}
		}
		antns[i] = antn
		antnsReturn[labelType] = append(antnsReturn[labelType], antns[i])
	}
}

func (app *AnnotationAPI) createNewAnnotation(c *gin.Context) {
	resp := entities.NewResponse()

	var antn Annotation
	err := c.ShouldBindJSON(&antn)

	authInfo := mw.GetAuthInfoFromGin(c)
	antn.CreatorID = authInfo.ID

	if err != nil || !antn.IsValidAnnotation() {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	antn.NewAnnotation()
	err1 := app.antnStore.Create(antn)
	if err1 != nil {
		utils.LogError(err1)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = kvStr2Inf{
		constants.ParamID: antn.ID,
	}
	c.JSON(http.StatusOK, resp)
}

func (app *AnnotationAPI) updateAnnotation(c *gin.Context) {
	resp := entities.NewResponse()

	antnID := c.Param(constants.ParamID)
	if antnID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]interface{})
	err1 := c.ShouldBind(&updateMap)
	if err1 != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	app.antnStore.Update(Annotation{ID: antnID}, updateMap)

	c.JSON(http.StatusOK, resp)
}

func (app *AnnotationAPI) deleteAnnotation(c *gin.Context) {
	resp := entities.NewResponse()

	antnID := c.Param(constants.ParamID)
	if antnID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.antnStore.Delete(nil, fmt.Sprintf("_id:%s", antnID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}
