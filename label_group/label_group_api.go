package label_group

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vindr-lab-api/annotation"
	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type LabelGroupAPI struct {
	labelGroupStore *LabelGroupES
	labelStore      *annotation.LabelES
	logger          *zap.Logger
}

func NewLabelGroupAPI(labelGroupStore *LabelGroupES, labelStore *annotation.LabelES, logger *zap.Logger) (app *LabelGroupAPI) {
	app = &LabelGroupAPI{
		labelGroupStore: labelGroupStore,
		labelStore:      labelStore,
		logger:          logger,
	}
	return app

}

func (app *LabelGroupAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.GetLabelGroups)
	group.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateNewLabelGroup)
	group.POST("/:id/labels", mw.ValidPerms(path, mw.PERM_C), app.ImportLabels)
	group.GET("/:id/labels", mw.ValidPerms(path, mw.PERM_R), app.ExportLabels)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateLabelGroup)
	group.PUT("/:id/update_order", mw.ValidPerms(path, mw.PERM_U), app.UpdateLabelsOrder)
	group.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.DeleteLabelGroup)
}

func (app *LabelGroupAPI) GetLabelGroups(c *gin.Context) {
	resp := entities.NewResponse()

	mapQuery, _, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)

	authInfo := mw.GetAuthInfoFromGin(c)
	labelGroups, _, err := app.labelGroupStore.GetSlice(mapQuery, fmt.Sprintf("owner_ids.keyword:%s", authInfo.ID),
		from, size, sort, aggs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	utils.LogInfo("%v", labelGroups)
	// retLabelGroups := make([]LabelGroup, 0)
	// for i := range labelGroups {
	// 	if labelGroups[i].CreatorID == authInfo.ID {
	// 		retLabelGroups = append(retLabelGroups, labelGroups[i])
	// 	}
	// }
	// utils.LogInfo("%v", retLabelGroups)

	resp.Data = labelGroups
	c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) CreateNewLabelGroup(c *gin.Context) {
	resp := entities.NewResponse()

	var labelGroup LabelGroup
	err := c.ShouldBindJSON(&labelGroup)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	authInfo := mw.GetAuthInfoFromGin(c)
	labelGroup.CreatorID = authInfo.ID
	if labelGroup.CreatorID == "" || labelGroup.Name == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	newID := uuid.New().String()
	labelGroup.ID = newID
	labelGroup.Created = time.Now().UnixNano() / int64(time.Millisecond)
	labelGroup.OwnerIDs = []string{authInfo.ID}

	err = app.labelGroupStore.CreateLabelGroup(labelGroup)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = labelGroup
	c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) ImportLabels(c *gin.Context) {
	resp := entities.NewResponse()

	labelGroupID := c.Param(constants.ParamID)
	if labelGroupID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	authInfo := mw.GetAuthInfoFromGin(c)

	form, err := c.MultipartForm()
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	files := form.File["files"]

	for _, file := range files {
		f, _ := file.Open()
		b, _ := ioutil.ReadAll(f)
		fileContents := string(b)

		lines := strings.Split(fileContents, "\n")
		for i := range lines {
			if lines[i] != "" {
				mapLabelName2ID := make(map[string]string)
				record := strings.Split(lines[i], ",")
				l := ConvertLineToLabel(record, mapLabelName2ID, labelGroupID, authInfo.ID)
				if l != nil {
					err := app.labelStore.Create(*l)
					if err != nil {
						utils.LogError(err)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) ExportLabels(c *gin.Context) {
	resp := entities.NewResponse()

	labelGroupID := c.Param(constants.ParamID)
	if labelGroupID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	now := time.Now().UnixNano() / int64(time.Second)
	fileOut := fmt.Sprintf("%s_%d.csv", labelGroupID, now)

	lines := make([]string, 0)
	lines = append(lines, "Name,Type,Scope,AnnotationType,ShortName,Description,Color,ChildrenSelectType,ParentName,Order")

	app.labelStore.Query(nil, fmt.Sprintf("label_group_id.keyword:%s", labelGroupID), 0, 10, "", nil, func(ls []annotation.Label, e entities.ESReturn) {
		for _, label := range ls {
			line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%.1f",
				label.Name, label.Type, label.Scope, label.AnnotationType, label.ShortName, label.Description, label.Color,
				label.ChildrenSelectType, label.ParentLabelID, label.Order)
			lines = append(lines, line)
		}
	})

	contents := strings.Join(lines, "\n")
	utils.LogInfo(contents)

	utils.WriteAppend(fileOut, contents)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileOut)
	c.Header("Content-Type", "text/csv")
	c.File(fileOut)
	os.Remove(fileOut)

	// c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) UpdateLabelGroup(c *gin.Context) {
	resp := entities.NewResponse()

	labelGroupID := c.Param(constants.ParamID)
	if labelGroupID == "" {
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
	app.labelGroupStore.Update(LabelGroup{ID: labelGroupID}, updateMap)

	c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) UpdateLabelsOrder(c *gin.Context) {
	resp := entities.NewResponse()

	labelGroupID := c.Param(constants.ParamID)
	if labelGroupID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	mapLabels := make(map[string][]annotation.Label, 0)
	err := c.ShouldBind(&mapLabels)
	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	if labels, found := mapLabels["labels"]; found {
		for i := range labels {
			app.labelStore.Update(annotation.Label{ID: labels[i].ID}, map[string]interface{}{
				"order": labels[i].Order,
			})
		}
	} else {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	// updateMap := make(map[string]interface{})
	// err1 := c.ShouldBind(&updateMap)
	// if err1 != nil {
	// 	resp.ErrorCode = constants.ServerInvalidData
	// 	c.JSON(http.StatusBadRequest, resp)
	// 	return
	// }
	// app.labelGroupStore.Update(LabelGroup{ID: labelGroupID}, updateMap)

	c.JSON(http.StatusOK, resp)
}

func (app *LabelGroupAPI) DeleteLabelGroup(c *gin.Context) {
	resp := entities.NewResponse()

	labelGroupID := c.Param(constants.ParamID)
	if labelGroupID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.labelGroupStore.Delete(LabelGroup{
		ID: labelGroupID,
	})
	if err != nil {
		fmt.Println(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	app.labelStore.Delete(nil, fmt.Sprintf("label_group_id.keyword:%s", labelGroupID))

	c.JSON(http.StatusOK, resp)
}

func ConvertLineToLabel(record []string, mapLabelName2ID map[string]string, labelGroupID string, creatorID string) *annotation.Label {
	if record[0] != "Name" {
		labelID := uuid.New().String()

		now := time.Now().UnixNano() / int64(time.Millisecond)
		var label annotation.Label

		label.ID = labelID
		label.Name = record[0]
		label.Type = record[1]
		label.Scope = record[2]
		label.AnnotationType = record[3]
		label.ShortName = record[4]
		label.Description = record[5]
		label.Color = record[6]
		label.ChildrenSelectType = record[7]
		parentLabelName := record[8]
		label.ParentLabelID = mapLabelName2ID[parentLabelName]
		order, _ := strconv.ParseFloat(record[9], 32)
		label.Order = float32(order)

		label.Created = now
		label.CreatorID = creatorID
		label.LabelGroupID = labelGroupID
		mapLabelName2ID[label.Name] = label.ID

		return &label
	}

	return nil
}
