package study

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type StudyAPI struct {
	studyStore   *StudyES
	taskStore    *TaskES
	projectStore *project.ProjectES
	objectStore  *object.ObjectES
	studyOrthanC *StudyOrthanC
	Logger       *zap.Logger
}

func NewStudyAPI(studyStore *StudyES, taskStore *TaskES, projectStore *project.ProjectES, objectStore *object.ObjectES, studyOrthanC *StudyOrthanC, logger *zap.Logger) (app *StudyAPI) {
	app = &StudyAPI{
		studyStore:   studyStore,
		taskStore:    taskStore,
		projectStore: projectStore,
		objectStore:  objectStore,
		studyOrthanC: studyOrthanC,
		Logger:       logger,
	}
	return app
}

func (app *StudyAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.Logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.FetchStudy)
	group.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateStudy)
	group.GET("/:id", mw.ValidPerms(path, mw.PERM_R), app.GetStudy)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateStudy)
	group.POST("/delete_many", mw.ValidPerms(path, mw.PERM_D), app.DeleteManyStudies)
}

func (app *StudyAPI) FetchStudy(c *gin.Context) {
	resp := entities.NewResponse()

	queries, searchQuery, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)

	studies := make([]Study, 0)
	studies, esReturn, err := app.studyStore.GetSlice(queries, searchQuery, from, size, sort, aggs)

	if searchQuery == "" && sort == "" {
		sort = "-created"
	}

	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if esReturn.Aggregations != nil {
		for _, agg := range aggs {
			v := *esReturn.Aggregations
			arrMap := make(map[string]interface{})
			for _, bucket := range v[agg].Buckets {
				arrMap[bucket.Key] = bucket.DocCount
			}
			resp.Agg = &kvStr2Inf{
				agg: arrMap,
			}
		}
	}

	resp.Data = studies
	resp.Count = esReturn.Hits.Total.Value

	c.JSON(http.StatusOK, resp)
}

func (app *StudyAPI) GetStudy(c *gin.Context) {
	resp := entities.NewResponse()

	studyID := c.Param(constants.ParamID)
	if studyID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	study, _, err := app.studyStore.Get(nil, fmt.Sprintf("_id:%s", studyID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	oStudyID, err := app.studyOrthanC.FindStudy(fmt.Sprintf("%s.%s", study.ProjectID, study.DICOMTags.StudyInstanceUID[0]))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	utils.LogInfo(oStudyID)

	resp.Data = study

	c.JSON(http.StatusOK, resp)
}

func (app *StudyAPI) CreateStudy(c *gin.Context) {

	resp := entities.NewResponse()

	var study Study
	err := c.ShouldBindJSON(&study)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	} else {
		if !IsValidStatus(study.Status) {
			resp.ErrorCode = constants.ServerInvalidData
			c.JSON(http.StatusBadRequest, resp)
			return
		}

		newID := uuid.New().String()
		study.ID = newID

		err := app.studyStore.Create(study)
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		ret := make(map[string]interface{}, 0)
		ret[constants.ParamID] = newID
		resp.Data = ret
	}

	c.JSON(http.StatusOK, resp)
}

func (app *StudyAPI) UpdateStudy(c *gin.Context) {
	resp := entities.NewResponse()

	studyID := c.Param(constants.ParamID)
	if studyID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]interface{})
	err := c.ShouldBind(&updateMap)
	if err != nil || updateMap["status"] != "" {
		utils.LogError(err)
		fmt.Println(updateMap)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	updateMap["modified"] = now
	err = app.studyStore.Update(Study{ID: studyID}, updateMap)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *StudyAPI) UpdateStudyStatus(c *gin.Context) {
	resp := entities.NewResponse()

	studyID := c.Param(constants.ParamID)
	if studyID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]interface{})
	err := c.ShouldBind(&updateMap)
	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	var study Study
	bytesData, _ := json.Marshal(updateMap)
	err = json.Unmarshal(bytesData, &study)
	if err != nil || !IsValidStatus(study.Status) {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	updateMap["modified"] = now
	err = app.studyStore.Update(Study{ID: studyID}, updateMap)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
	return
}

func (app *StudyAPI) DeleteStudy(c *gin.Context) {
	resp := entities.NewResponse()

	studyID := c.Param(constants.ParamID)
	if studyID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	study, _, err := app.studyStore.Get(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", studyID, constants.StudyStatusUnassigned))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if study == nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err = app.studyStore.Delete(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", studyID, constants.StudyStatusUnassigned))
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.objectStore.Delete(nil, fmt.Sprintf("study_id.keyword:%s", studyID))
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	// go func() {
	// 	//delete dicom file from orthanc
	// }()
	err = app.studyOrthanC.DeleteDicomFile(app.studyOrthanC, *study)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *StudyAPI) DeleteManyStudies(c *gin.Context) {
	resp := entities.NewResponse()

	deleteStudies := make(map[string][]string)
	err := c.ShouldBind(&deleteStudies)
	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	studyIDs, found := deleteStudies[constants.ParamIDs]
	if !found {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	//maximum size of list is 10, prevent risks
	if len(studyIDs) > constants.DefaultLimit {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	// _, esReturn, err := app.studyStore.GetSlice(map[string][]string{
	// 	"_id": studyIDs,
	// }, fmt.Sprintf("status.keyword:%s", constants.StudyStatusUnassigned), 0, 0, "", nil)
	// deleted := esReturn.Hits.Total.Value

	// err = app.studyStore.Query(map[string][]string{"_id": studyIDs}, fmt.Sprintf("status.keyword:%s", constants.StudyStatusUnassigned), 0, 10, "", nil,
	// 	func(studies []Study, es entities.ESReturn) {
	// 		if len(studies) > 0 {
	// 			deleteStudyIDs := make([]string, 0)
	// 			for i := range studies {
	// 				deleteStudyIDs = append(deleteStudyIDs, studies[i].ID)
	// 				err := DeleteDicomFile(app.studyOrthanC, studies[i])
	// 				utils.LogError(err)
	// 			}

	// 			err = app.objectStore.Delete(map[string][]string{"study_id.keyword": deleteStudyIDs}, "")

	// 			if err != nil {
	// 				resp.ErrorCode = constants.ServerError
	// 				c.JSON(http.StatusInternalServerError, resp)
	// 				return
	// 			}
	// 		}
	// 	})

	// err = app.studyStore.Delete(map[string][]string{
	// 	"_id": studyIDs,
	// }, fmt.Sprintf("status.keyword:%s", constants.StudyStatusUnassigned))
	// if err != nil {
	// 	resp.ErrorCode = constants.ServerError
	// 	c.JSON(http.StatusInternalServerError, resp)
	// 	return
	// }

	deleted, err := DeleteStudies(studyIDs, app.studyStore, app.taskStore, app.objectStore, app.studyOrthanC)

	resp.Meta = &kvStr2Inf{
		"deleted":     deleted,
		"not_deleted": len(studyIDs) - deleted,
	}

	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func DeleteStudies(studyIDs []string, studyStore *StudyES, taskStore *TaskES, objectStore *object.ObjectES, studyOrthanC *StudyOrthanC) (int, error) {
	deleted := 0

	for i := range studyIDs {
		studyID := studyIDs[i]

		s, getStudyESReturn, err := studyStore.Get(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", studyID, constants.StudyStatusUnassigned))
		if err != nil {
			utils.LogError(err)
			continue
		}

		_, esReturn, err := taskStore.GetSlice(nil, fmt.Sprintf("study_id.keyword:%s", studyID), 0, 1, "", nil)
		if err != nil {
			utils.LogError(err)
			return deleted, err
		}
		if esReturn.Hits.Total.Value > 0 {
			continue
		}

		err = objectStore.Delete(nil, fmt.Sprintf("study_id.keyword:%s", studyID))
		if err != nil {
			return deleted, err
		}

		err = studyStore.Delete(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", studyID, constants.StudyStatusUnassigned))
		if err != nil {
			return deleted, err
		}

		err = studyOrthanC.DeleteDicomFile(studyOrthanC, *s)
		if err != nil {
			utils.LogError(err)
		}

		deleted += getStudyESReturn.Hits.Total.Value
	}

	return deleted, nil
}
