package annotation

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/object"
	"vindr-lab-api/project"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type LabelAPI struct {
	labelStore   *LabelES
	antnStore    *AnnotationES
	projectStore *project.ProjectES
	objectStore  *object.ObjectES
	logger       *zap.Logger
}

func NewLabelAPI(labelStore *LabelES, antnStore *AnnotationES, projectStore *project.ProjectES, logger *zap.Logger) (app *LabelAPI) {
	app = &LabelAPI{
		labelStore:   labelStore,
		antnStore:    antnStore,
		projectStore: projectStore,
		logger:       logger,
	}
	return app
}

func (app *LabelAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.GetLabels)
	group.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateLabel)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateLabel)
	group.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.DeleteLabel)
}

func (app *LabelAPI) GetLabels(c *gin.Context) {
	resp := entities.NewResponse()

	projectID := c.Query(constants.ParamProjectID)
	labelGroupID := c.Query(constants.ParamLabelGroupID)
	sort := c.Query(constants.ParamSort)

	if projectID == "" && labelGroupID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	if sort == "" {
		sort = "name"
	}

	labelsGroupIDs := make([]string, 0)
	if projectID != "" {
		projects, _, err := app.projectStore.GetSlice(nil, fmt.Sprintf("_id:%s", projectID), 0, 1, "", nil)
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		if len(projects) > 0 {
			project := projects[0]
			if project.LabelGroupIDs != nil {
				labelsGroupIDs = append(labelsGroupIDs, project.LabelGroupIDs...)
			}
		}
	} else if labelGroupID != "" {
		labelsGroupIDs = append(labelsGroupIDs, labelGroupID)
	}

	labels := make([]Label, 0)
	if len(labelsGroupIDs) > 0 {
		search := strings.Join(labelsGroupIDs, " OR label_group_id.keyword:")
		search = "label_group_id.keyword:" + search
		err := app.labelStore.Query(nil, search, 0, 10, sort, nil, func(labels1 []Label, e entities.ESReturn) {
			labels = append(labels, labels1...)
		})
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
	}

	labelsRetMap := ReMapLabelParentAndChildren(labels)
	resp.Data = labelsRetMap

	c.JSON(http.StatusOK, resp)
}

func (app *LabelAPI) CreateLabel(c *gin.Context) {
	resp := entities.NewResponse()

	var label Label
	err := c.ShouldBindJSON(&label)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	label.ID = uuid.New().String()
	label.Created = time.Now().UnixNano() / int64(time.Millisecond)
	authInfo := mw.GetAuthInfoFromGin(c)
	label.CreatorID = authInfo.ID

	if !label.IsValidLabel() {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	if label.ParentLabelID != "" {
		parentLabel, _, err := app.labelStore.Get(nil, fmt.Sprintf("_id:%s", label.ParentLabelID))
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		if parentLabel.Scope != label.Scope || parentLabel.AnnotationType != label.AnnotationType || parentLabel.Type != label.Type {
			resp.ErrorCode = constants.ServerInvalidData
			c.JSON(http.StatusBadRequest, resp)
			return
		}
	}

	err1 := app.labelStore.Create(label)
	if err1 != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *LabelAPI) UpdateLabel(c *gin.Context) {
	resp := entities.NewResponse()

	labelID := c.Param(constants.ParamID)
	if labelID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]interface{})
	err := c.ShouldBind(&updateMap)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	delete(updateMap, "sub_labels")
	delete(updateMap, "id")

	err = app.labelStore.Update(Label{ID: labelID}, updateMap)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *LabelAPI) DeleteLabel(c *gin.Context) {
	resp := entities.NewResponse()

	labelID := c.Param(constants.ParamID)
	if labelID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	_, esReturn, err := app.antnStore.Get(nil, fmt.Sprintf("label_id:%s", labelID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if esReturn != nil && esReturn.Hits.Total.Value > 0 {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusNotAcceptable, resp)
		return
	}

	err = app.labelStore.Delete(nil, fmt.Sprintf("_id:%s", labelID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ReMapLabelParentAndChildren
func ReMapLabelParentAndChildren(labels []Label) *map[string][]Label {
	sortLabels(labels)

	labelsRetMap := make(map[string][]Label, 0)
	mapParent2Children := make(map[string][]Label, 0)
	tmpLabels := make([]Label, 0)

	for i, label := range labels {
		parentID := label.ParentLabelID
		if parentID != "" {
			mapParent2Children[parentID] = append(mapParent2Children[parentID], labels[i])
		} else {
			tmpLabels = append(tmpLabels, labels[i])
		}
	}

	for i := range tmpLabels {
		labelID := tmpLabels[i].ID
		if _, found := mapParent2Children[labelID]; found && labelID != "" {
			tmpLabels[i].SubLabels = mapParent2Children[labelID]
		}
	}

	for _, label := range tmpLabels {
		labelsRetMap[label.Type] = append(labelsRetMap[label.Type], label)
	}

	return &labelsRetMap
}

func sortLabels(labels []Label) {
	sort.SliceStable(labels, func(i, j int) bool {
		if labels[i].Order == labels[j].Order {
			return strings.ToLower(labels[i].Name) < strings.ToLower(labels[j].Name)
		}
		return labels[i].Order < labels[j].Order
	})
}
