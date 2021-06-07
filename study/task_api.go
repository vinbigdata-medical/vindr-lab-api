package study

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"vindr-lab-api/annotation"
	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/helper"
	"vindr-lab-api/mw"
	"vindr-lab-api/object"
	"vindr-lab-api/project"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type TaskAPI struct {
	taskStore    *TaskES
	studyStore   *StudyES
	projectStore *project.ProjectES
	antnStore    *annotation.AnnotationES
	labelStore   *annotation.LabelES
	idGenerator  *helper.IDGenerator
	objectStore  *object.ObjectES
	logger       *zap.Logger
}

func NewTaskAPI(taskStore *TaskES, studyStore *StudyES, projectStore *project.ProjectES, objectStore *object.ObjectES, antnStore *annotation.AnnotationES, labelStore *annotation.LabelES, idGenerator *helper.IDGenerator, logger *zap.Logger) (app *TaskAPI) {
	app = &TaskAPI{
		taskStore:    taskStore,
		studyStore:   studyStore,
		projectStore: projectStore,
		objectStore:  objectStore,
		idGenerator:  idGenerator,
		antnStore:    antnStore,
		labelStore:   labelStore,
		logger:       logger,
	}
	return app
}

func (app *TaskAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.GetTasks)
	group.POST("/assign", mw.ValidPerms(path, mw.PERM_C), app.CreateTask)
	group.POST("/delete_many", mw.ValidPerms(path, mw.PERM_D), app.DeleteTasks)
	group.POST("/update_status_many", mw.ValidPerms(path, mw.PERM_U), app.UpdateTasksStatus)
	group.GET("/:id", mw.ValidPerms(path, mw.PERM_R), app.GetTask)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateTask)
	group.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.DeleteTask)
	group.PUT("/:id/annotations", mw.ValidPerms(path, mw.PERM_U), app.SetManyAnnotationsV2)
	group.PUT("/:id/status", mw.ValidPerms(path, mw.PERM_U), app.UpdateTaskStatus)
	group.PUT("/:id/archive", mw.ValidPerms(path, mw.PERM_U), app.ChangeArchiveStatus)
}

func (app *TaskAPI) GetTask(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)

	task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s", taskID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = *task
	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) GetTasks(c *gin.Context) {
	resp := entities.NewResponse()

	role := c.Query("_role")
	queries, queryStr, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)
	delete(queries, "_role.keyword")

	if sort == "" && queries == nil && queryStr == "" {
		sort = "-created"
	}

	tasks := make([]Task, 0)
	esReturn := entities.ESReturn{}
	authInfo := mw.GetAuthInfoFromGin(c)
	switch role {
	case constants.ProjRoleProjectOwner:
		project, _, _ := app.projectStore.Get(nil, fmt.Sprintf("people.id.keyword:%s AND people.roles.keyword:%s", authInfo.ID, role))
		if project == nil {
			resp.ErrorCode = constants.ServerInvalidData
			c.JSON(http.StatusBadRequest, resp)
			return
		}

		tasks1, esReturn1, err := app.taskStore.GetSlice(queries, queryStr, from, size, sort, aggs)
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		tasks = tasks1
		esReturn = *esReturn1

		break
	case constants.ProjRoleAnnotator, constants.ProjRoleReviewer:

		if queryStr != "" {
			queryStr = fmt.Sprintf("%s AND assignee_id.keyword:%s", queryStr, authInfo.ID)
		} else {
			queryStr = "assignee_id.keyword:" + authInfo.ID
		}
		tasks1, esReturn1, err := app.taskStore.GetSlice(queries, queryStr, from, size, sort, aggs)
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		tasks = tasks1
		esReturn = *esReturn1
		break
	}

	for i, task := range tasks {
		study, _, _ := app.studyStore.Get(nil, fmt.Sprintf("_id:%s", task.StudyID))
		task.Study = study
		tasks[i] = task
	}

	if esReturn.Aggregations != nil {
		aggReturn := make(map[string]interface{})
		for _, agg := range aggs {
			v := *esReturn.Aggregations
			arrMap := make(map[string]interface{})
			for _, bucket := range v[agg].Buckets {
				arrMap[bucket.Key] = bucket.DocCount
			}
			aggReturn[agg] = arrMap
		}
		resp.Agg = &aggReturn
	}

	resp.Data = tasks
	resp.Count = esReturn.Hits.Total.Value
	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) CreateTask(c *gin.Context) {
	resp := entities.NewResponse()

	var ta2 TaskAssignment2
	err := c.ShouldBindJSON(&ta2)
	authInfo := mw.GetAuthInfoFromGin(c)

	if err != nil || !ta2.IsValidTaskAssignment2() {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	mapStudyID2Code := GetStudyIDsByAssignRequest(ta2, *app.studyStore)
	for assignType, assignees := range ta2.AssigneeIDs {
		tasks, err := DistributeTask(mapStudyID2Code, ta2, app.idGenerator, assignees,
			ta2.ProjectID, authInfo.ID, assignType)
		if err != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerInvalidData
			c.JSON(http.StatusBadRequest, resp)
			return
		}

		if len(tasks) > 0 {
			err = app.taskStore.Bulk(tasks)
			if err != nil {
				utils.LogError(err)
				resp.ErrorCode = constants.ServerError
				c.JSON(http.StatusInternalServerError, resp)
				return
			}

			for i := range tasks {
				studyID := tasks[i].StudyID
				utils.LogDebug(studyID)
				err1 := app.studyStore.Update(Study{ID: studyID}, kvStr2Inf{
					"status": constants.StudyStatusAssigned,
				})
				if err1 != nil {
					utils.LogError(err)
					resp.ErrorCode = constants.ServerError
					c.JSON(http.StatusInternalServerError, resp)
					return
				}
			}
		}
	}
	c.JSON(http.StatusOK, resp)
	return
}

type SetManyAnnotationsBody struct {
	Annotations []annotation.Annotation `json:"annotations"`
	Comment     string                  `json:"comment"`
}

func (app *TaskAPI) SetManyAnnotations(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s", taskID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if task.Status == constants.TaskStatusCompleted {
		utils.LogError(errors.New("Task is completed, cannot overwrite its annotations"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	var setManyBody SetManyAnnotationsBody
	err = c.ShouldBindJSON(&setManyBody)
	authInfo := mw.GetAuthInfoFromGin(c)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	annotations := setManyBody.Annotations
	taskComment := setManyBody.Comment

	err = app.taskStore.Update(Task{ID: taskID}, kvStr2Inf{"comment": taskComment})
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if len(annotations) > 0 {

		unauthorizedAntns := 0
		mapAntnType2Antns := make(map[string][]annotation.Annotation)
		deleteIDs := make([]string, 0)

		for i, annotation := range annotations {

			if annotation.ID != "" && annotation.CreatorID != authInfo.ID {
				unauthorizedAntns++
				continue
			}

			switch annotation.Event {
			case constants.EventCreate, constants.EventUpdate:
				if annotation.ID == "" && annotation.Event == constants.EventCreate {
					annotations[i].NewAnnotation()
					annotations[i].CreatorID = authInfo.ID
				}

				if annotations[i].IsValidAnnotation() {
					annotations[i].Labels = nil
					// newAnnotations = append(newAnnotations, annotations[i])
					mapAntnType2Antns[annotations[i].Type] = append(mapAntnType2Antns[annotations[i].Type], annotations[i])
				}
				break
			case constants.EventDelete:
				if annotation.ID != "" {
					deleteIDs = append(deleteIDs, annotations[i].ID)
				}
				break
			default:
				break
			}

		}
		utils.LogInfo("Unable to access %d annotations", unauthorizedAntns)

		for k, newAnnotations := range mapAntnType2Antns {
			if newAnnotations != nil && len(newAnnotations) > 0 {
				utils.LogInfo("%s\t%d", k, len(newAnnotations))
				err := app.antnStore.BulkCreate(newAnnotations)
				if err != nil {
					utils.LogError(err)
					resp.ErrorCode = constants.ServerError
					c.JSON(http.StatusInternalServerError, resp)
					return
				}
			}
		}

		if len(deleteIDs) > 0 {
			utils.LogDebug("%v", deleteIDs)
			for i := range deleteIDs {
				err := app.antnStore.Delete(nil, fmt.Sprintf("_id:%s", deleteIDs[i]))
				if err != nil {
					utils.LogError(err)
					resp.ErrorCode = constants.ServerError
					c.JSON(http.StatusInternalServerError, resp)
					return
				}
			}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) SetManyAnnotationsV2(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s", taskID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if task.Status == constants.TaskStatusCompleted {
		utils.LogError(errors.New("Task is completed, cannot overwrite its annotations"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	var setManyBody SetManyAnnotationsBody
	err = c.ShouldBindJSON(&setManyBody)
	authInfo := mw.GetAuthInfoFromGin(c)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	annotations := setManyBody.Annotations
	taskComment := setManyBody.Comment

	err = app.taskStore.Update(Task{ID: taskID}, kvStr2Inf{"comment": taskComment})
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if len(annotations) > 0 {

		unauthorizedAntns := 0
		mapAntnType2Antns := make(map[string][]annotation.Annotation)
		deleteIDs := make([]string, 0)

		for i := range annotations {
			a := annotations[i]

			if a.ID != "" && a.CreatorID != authInfo.ID {
				unauthorizedAntns++
				continue
			}

			switch a.Event {
			case constants.EventCreate, constants.EventUpdate:
				if a.ID == "" && a.Event == constants.EventCreate {
					a.NewAnnotation()
					a.CreatorID = authInfo.ID

					if a.Type != constants.AntnType3DBox {
						labelID := a.LabelIDs[0]
						l, _, err := app.labelStore.Get(nil, fmt.Sprintf("_id:%s", labelID))
						if err == nil {
							objectID, err := getObjectIDFromUID(a, l.Scope, *app.objectStore)
							if err == nil {
								a.ObjectID = objectID
							}
						}
					} else {
						objectID, err := getObjectIDFromUID(a, constants.ObjectTypeSeries, *app.objectStore)
						if err == nil {
							a.ObjectID = objectID
						}
					}
				}

				if a.IsValidAnnotation() {
					a.Labels = nil
					// newAnnotations = append(newAnnotations, annotations[i])
					mapAntnType2Antns[a.Type] = append(mapAntnType2Antns[a.Type], a)
				}
				break
			case constants.EventDelete:
				if a.ID != "" {
					deleteIDs = append(deleteIDs, a.ID)
				}
				break
			default:
				break
			}

		}
		utils.LogInfo("Unable to access %d annotations", unauthorizedAntns)

		for k, newAnnotations := range mapAntnType2Antns {
			if newAnnotations != nil && len(newAnnotations) > 0 {
				utils.LogInfo("%s\t%d", k, len(newAnnotations))
				err := app.antnStore.BulkCreate(newAnnotations)
				if err != nil {
					utils.LogError(err)
					resp.ErrorCode = constants.ServerError
					c.JSON(http.StatusInternalServerError, resp)
					return
				}
			}
		}

		if len(deleteIDs) > 0 {
			utils.LogDebug("%v", deleteIDs)
			for i := range deleteIDs {
				err := app.antnStore.Delete(nil, fmt.Sprintf("_id:%s", deleteIDs[i]))
				if err != nil {
					utils.LogError(err)
					resp.ErrorCode = constants.ServerError
					c.JSON(http.StatusInternalServerError, resp)
					return
				}
			}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func getObjectIDFromUID(a annotation.Annotation, objectType string, objectES object.ObjectES) (string, error) {

	uid := ""
	keySearch := ""
	projectID := a.ProjectID

	switch objectType {
	case constants.ObjectTypeImage:
		keySearch = "sop_instance_uid"
		uid = fmt.Sprintf("%v", a.Meta["masked_sop_instance_uid"])
		break
	case constants.ObjectTypeSeries:
		keySearch = "series_instance_uid"
		uid = fmt.Sprintf("%v", a.Meta["masked_series_instance_uid"])
		break
	case constants.ObjectTypeStudy:
		keySearch = "study_instance_uid"
		uid = fmt.Sprintf("%v", a.Meta["masked_study_instance_uid"])
		break
	}

	if strings.Contains(uid, projectID) {
		uid = strings.ReplaceAll(uid, projectID, "")
		uid = uid[1:]
	}

	o, _, err := objectES.Get(nil, fmt.Sprintf("project_id:%s AND type.keyword:%s AND meta.%s.keyword:%s", projectID, objectType, keySearch, uid))
	if err != nil {
		return "", err
	}
	// utils.LogInfo("%s %s %s", uid, keySearch, uid)
	return o.ID, nil
}

func (app *TaskAPI) UpdateTask(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]interface{})
	c.ShouldBind(&updateMap)

	if _, found := updateMap["status"]; found {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.taskStore.Update(Task{ID: taskID}, updateMap)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) UpdateTaskStatus(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		utils.LogError(fmt.Errorf("taskID is empty"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]string)
	err := c.Bind(&updateMap)
	if err != nil {
		utils.LogError(fmt.Errorf("map input is invalid"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	if !IsValidTaskStatus(updateMap["status"]) {
		utils.LogError(fmt.Errorf("status is invalid"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err = app.taskStore.Update(Task{ID: taskID}, kvStr2Inf{
		"status": updateMap["status"],
	})
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	//then check and update study's status
	task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s", taskID))
	if err == nil && task != nil {
		err1 := app.UpdateStudyStatus(task.ProjectID, map[string]bool{
			task.StudyID: true,
		})
		if err1 != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

type UpdateTasksStatusBody struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

func (app *TaskAPI) UpdateTasksStatus(c *gin.Context) {
	resp := entities.NewResponse()

	updateRequest := UpdateTasksStatusBody{}
	err := c.Bind(&updateRequest)
	if err != nil {
		utils.LogError(fmt.Errorf("input is invalid"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	newStatus := updateRequest.Status
	if !IsValidTaskStatus(newStatus) {
		utils.LogError(fmt.Errorf("status is invalid"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	utils.LogInfo(newStatus, updateRequest)

	tasks := make([]Task, 0)
	mapStudies := make(map[string]bool)
	for i := range updateRequest.IDs {
		taskID := updateRequest.IDs[i]
		task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s", taskID))
		if err != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		task.Status = newStatus

		mapStudies[task.StudyID] = true
		tasks = append(tasks, *task)
	}

	if len(tasks) > 0 {
		err := app.taskStore.Bulk(tasks)
		if err != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		err = app.UpdateStudyStatus(tasks[0].ProjectID, mapStudies)
		if err != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) ChangeArchiveStatus(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	updateMap := make(map[string]bool)
	c.ShouldBind(&updateMap)

	archived, found := updateMap["archive"]
	if !found {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.taskStore.Update(Task{ID: taskID}, kvStr2Inf{
		"archived": archived,
	})
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) DeleteTask(c *gin.Context) {
	resp := entities.NewResponse()

	taskID := c.Param(constants.ParamID)
	if taskID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	mapStudyIDs := make(map[string]bool)
	err := app.taskStore.Query(nil, fmt.Sprintf("_id:%s", taskID), 0, 10, "", nil,
		func(tasks []Task, es entities.ESReturn) {
			for _, task := range tasks {
				mapStudyIDs[task.StudyID] = true
			}
		})

	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	task, _, err := app.taskStore.Get(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", taskID, constants.TaskStatusNew))
	if task == nil || err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.taskStore.Delete(nil, fmt.Sprintf("_id:%s AND status.keyword:%s", taskID, constants.TaskStatusNew))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.UpdateStudyStatus(task.ProjectID, mapStudyIDs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.DeleteAnnotationsOfTasks([]string{taskID})
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (app *TaskAPI) DeleteTasks(c *gin.Context) {
	resp := entities.NewResponse()

	deleteTaskIDs := make(map[string][]string)
	err := c.ShouldBind(&deleteTaskIDs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	taskIDs, found := deleteTaskIDs[constants.ParamIDs]
	if !found {
		utils.LogError(fmt.Errorf("IDs of tasks not found"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	//maximum size of list is 100, prevent risks
	if len(taskIDs) > constants.DefaultLimit {
		utils.LogError(fmt.Errorf("Limit size of input is %d, input size %d", constants.DefaultLimit, len(taskIDs)))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	mapStudyIDs := make(map[string]bool)
	projectID := ""
	app.taskStore.Query(map[string][]string{
		"_id": taskIDs,
	}, "", 0, 10, "", nil, func(tasks []Task, es entities.ESReturn) {
		for _, task := range tasks {
			mapStudyIDs[task.StudyID] = true
			projectID = task.ProjectID
		}
	})

	err = app.taskStore.Delete(map[string][]string{
		"_id": taskIDs,
	}, fmt.Sprintf("status.keyword:%s", constants.TaskStatusNew))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.UpdateStudyStatus(projectID, mapStudyIDs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	err = app.DeleteAnnotationsOfTasks(taskIDs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateStudyStatus updtes study's status following its tasks's status
func (app *TaskAPI) UpdateStudyStatus(projectID string, mapStudyIDs map[string]bool) error {
	// mapStudyIDs := make(map[string]bool)

	for studyID := range mapStudyIDs {
		mapTaskStatusCount := make(map[string]int)
		tasksOfStudy := 0
		err := app.taskStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND study_id.keyword:%s", projectID, studyID),
			0, constants.DefaultLimit, "", nil,
			func(tasks []Task, es entities.ESReturn) {
				for i := range tasks {
					key := tasks[i].Status
					if _, found := mapTaskStatusCount[key]; !found {
						mapTaskStatusCount[key] = 0
					}
					mapTaskStatusCount[key]++
					tasksOfStudy++
				}
			})
		if err != nil {
			return err
		}

		studyStatus := ""
		completedTask := mapTaskStatusCount[constants.TaskStatusCompleted]
		if completedTask > 0 {
			if completedTask == tasksOfStudy {
				studyStatus = constants.StudyStatusCompleted
			} else if completedTask < tasksOfStudy {
				studyStatus = constants.StudyStatusAssigned
			} else {
				return fmt.Errorf("Unexpected completed tasks, required %d got %d", tasksOfStudy, completedTask)
			}
		} else {
			if tasksOfStudy == 0 {
				studyStatus = constants.StudyStatusUnassigned
			} else {
				studyStatus = constants.StudyStatusAssigned
			}
		}

		if studyStatus != "" {
			err1 := app.studyStore.Update(Study{ID: studyID}, kvStr2Inf{
				"status": studyStatus,
			})
			if err1 != nil {
				return err
			}
		}
	}

	return nil
}

func (app *TaskAPI) DeleteAnnotationsOfTasks(taskIDs []string) error {
	for i := range taskIDs {
		taskID := taskIDs[i]
		err := app.antnStore.Delete(nil, fmt.Sprintf("task_id.keyword:%s", taskID))
		if err != nil {
			return err
		}
	}

	return nil
}

func GetStudyIDsByAssignRequest(ta TaskAssignment2, studyStore StudyES) map[string]string {
	mapStudyID2Code := make(map[string]string)

	switch ta.SourceType {
	case constants.ASSIGN_SOURCE_FILE:
		studyUIDs := ta.StudyInstanceUIDs
		for i := range studyUIDs {
			s, _, err := studyStore.Get(nil, fmt.Sprintf("project_id.keyword:%s AND dicom_tags.StudyInstanceUID.keyword:%s", ta.ProjectID, studyUIDs[i]))
			if err == nil && s != nil {
				mapStudyID2Code[s.ID] = s.Code
			}
		}
		break
	case constants.ASSIGN_SOURCE_SELECTED:
		for i := range ta.StudyIDs {
			s, _, err := studyStore.Get(nil, fmt.Sprintf("_id:%s", ta.StudyIDs[i]))
			if err == nil && s != nil {
				mapStudyID2Code[s.ID] = s.Code
			}
		}
		break
	case constants.ASSIGN_SOURCE_SEARCH:
		out := false
		status := ta.SearchQuery.Status
		queries := make([]string, 0)
		if ta.SearchQuery.Query != "" {
			queries = append(queries, ta.SearchQuery.Query)
		}
		queries = append(queries, "status.keyword:"+status, "project_id.keyword:"+ta.ProjectID)
		search := strings.Join(queries, " AND ")
		studyStore.Query(nil, search, 0, constants.DefaultLimit, "", nil, func(studies []Study, es entities.ESReturn) {
			for i := range studies {
				s := studies[i]
				if len(mapStudyID2Code) < ta.SearchQuery.Size {
					mapStudyID2Code[s.ID] = s.Code
				} else {
					out = true
					break
				}
			}

			if out {
				return
			}
		})
		break
	}

	return mapStudyID2Code
}

func DistributeTask(mapStudyID2Code map[string]string, ta2 TaskAssignment2, idGen *helper.IDGenerator,
	assignees []string, projectID, creatorID, assignType string) ([]Task, error) {
	tasks := make([]Task, 0)

	switch ta2.Strategy {
	case constants.ASSIGN_STRATEGY_ALL:
		for _, assignee := range assignees {
			for studyID, code := range mapStudyID2Code {
				task, _ := CreateTask(idGen, code, assignee, ta2.ProjectID, studyID, creatorID, assignType)
				tasks = append(tasks, *task)
			}
		}
		break
	case constants.ASSIGN_STRATEGY_EQUALLY:
		sizeAssignee := len(assignees)
		count := 0
		for studyID, code := range mapStudyID2Code {
			assignee := assignees[count%sizeAssignee]
			count++

			task, _ := CreateTask(idGen, code, assignee, ta2.ProjectID, studyID, creatorID, assignType)
			tasks = append(tasks, *task)
		}
		break
	}

	return tasks, nil
}

func CreateTask(idGen *helper.IDGenerator, code, assigneeID, projectID, studyID, creatorID, taskType string) (*Task, error) {
	task := Task{}
	task.CreatorID = creatorID
	task.StudyCode = code
	task.NewTask(assigneeID, studyID, projectID, taskType)
	key := "task_" + projectID
	counter, err := idGen.GenNew(key)
	if err != nil {
		return nil, err
	}
	task.Code = fmt.Sprintf("TSK-%d", counter)

	return &task, nil
}
