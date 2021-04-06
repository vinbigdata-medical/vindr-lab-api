package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vindr-lab-api/account"
	"vindr-lab-api/annotation"
	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/label_group"
	"vindr-lab-api/mw"
	"vindr-lab-api/object"
	"vindr-lab-api/project"
	"vindr-lab-api/study"
	"vindr-lab-api/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StatsAPI struct {
	labelExportStore *LabelExportES
	labelGroupStore  *label_group.LabelGroupES
	labelStore       *annotation.LabelES
	projectStore     *project.ProjectES
	objectStore      *object.ObjectES
	antnStore        *annotation.AnnotationES
	studyStore       *study.StudyES
	taskStore        *study.TaskES
	kcStore          *account.KeycloakStore
	logger           *zap.Logger
	minioClient      *MinIOStorage
}

// NewLabelExportAPI it is going to be very huge
func NewLabelExportAPI(labelExportStore *LabelExportES, labelGroupStore *label_group.LabelGroupES, labelStore *annotation.LabelES, projectStore *project.ProjectES,
	antnStore *annotation.AnnotationES, objectStore *object.ObjectES, studyStore *study.StudyES, taskStore *study.TaskES,
	minioClient *MinIOStorage, kcStore *account.KeycloakStore, logger *zap.Logger) (app *StatsAPI) {
	app = &StatsAPI{
		labelExportStore: labelExportStore,
		labelGroupStore:  labelGroupStore,
		labelStore:       labelStore,
		projectStore:     projectStore,
		antnStore:        antnStore,
		objectStore:      objectStore,
		studyStore:       studyStore,
		taskStore:        taskStore,
		logger:           logger,
		minioClient:      minioClient,
		kcStore:          kcStore,
	}
	return app
}

func (app *StatsAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.POST("/label_exports", mw.ValidPerms("label_exports", mw.PERM_C), app.CreateExportLabel)
	group.GET("/label_exports", mw.ValidPerms("label_exports", mw.PERM_R), app.GetLabelExports)
	group.GET("/label_exports/download/:id", mw.ValidPerms("label_exports", mw.PERM_R), app.DownloadLabelExport)
	group.GET("/projects_by_role", mw.ValidPerms(path, mw.PERM_R), app.GetProjectsByRole)
	group.GET("/agg_labels", mw.ValidPerms(path, mw.PERM_R), app.GetStatsLabelsByAgg)
	group.GET("/studies/:id/assignee", mw.ValidPerms(path, mw.PERM_R), app.GetAssgineeOfStudy)
}

var existedTags = map[string]bool{}

func (app *StatsAPI) CreateExportLabel(c *gin.Context) {
	resp := entities.NewResponse()

	var labelExport LabelExport
	err := c.ShouldBindJSON(&labelExport)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	if len(existedTags) == 0 {
		labelExports, _, _ := app.labelExportStore.GetSlice(nil, "", 0, constants.DefaultLimit, "", nil)
		for _, item := range labelExports {
			existedTags[item.Tag] = true
		}
	}
	if len(existedTags) > 0 {
		if _, existedTag := existedTags[labelExport.Tag]; existedTag {
			app.logger.Info("Export tag is already existed")
			resp.ErrorCode = constants.ServerInvalidData
			c.JSON(http.StatusBadRequest, resp)
			return
		}
	}
	existedTags[labelExport.Tag] = true

	authInfo := mw.GetAuthInfoFromGin(c)
	labelExport.New()
	labelExport.CreatorID = authInfo.ID
	projectID := labelExport.ProjectID
	project, _, err := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))
	if projectID == "" || project == nil {
		app.logger.Info("ProjectID is nil")
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	utils.LogInfo("Create es object")
	app.labelExportStore.Create(labelExport)

	go func() {

		labelGroupIDs := project.LabelGroupIDs

		labelGroupsReturn, _, err := app.labelGroupStore.GetSlice(map[string][]string{
			"_id": labelGroupIDs,
		}, "", 0, constants.DefaultLimit, "", nil)
		if err != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		labelsReturn := make([]annotation.Label, 0)
		mapLabelImps := make(map[string]bool)
		mapLabelFind := make(map[string]bool)

		utils.LogInfo("Get labels")
		app.labelStore.Query(map[string][]string{"label_group_id.keyword": labelGroupIDs}, "", 0, constants.DefaultLimit, "", nil,
			func(labels []annotation.Label, esReturn entities.ESReturn) {
				for i, label := range labels {
					if _, found := utils.FindInSlice(labelGroupIDs, label.LabelGroupID); found {
						labelsReturn = append(labelsReturn, labels[i])
						switch label.Type {
						case "IMPRESSION":
							mapLabelImps[label.ID] = true
							break
						case "FINDING":
							mapLabelFind[label.ID] = true
							break
						}
					}
				}
			})

		utils.LogInfo("Get and map Annotations by types, Get comments")

		mapA8sImps := make(map[string]annotation.Annotation)
		mapA8sFind := make(map[string]annotation.Annotation)
		comments := make([]map[string]interface{}, 0)

		mapID2User, err := app.kcStore.GetAccountsAsMap("")
		if err != nil {
			utils.LogError(err)
		}

		totalTasks := 0
		app.studyStore.Query(nil, fmt.Sprintf("project_id.keyword:%s", projectID), 0, constants.DefaultLimit*10, "", nil, func(studies []study.Study, es entities.ESReturn) {
			utils.LogInfo("size of tasks: %d", totalTasks)
			for _, s := range studies {
				app.taskStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND study_id.keyword:%s AND type.keyword:%s AND status.keyword:%s",
					projectID, s.ID, constants.TaskTypeReview, constants.TaskStatusCompleted),
					0, constants.DefaultLimit, "", nil, func(tasks []study.Task, es entities.ESReturn) {

						totalTasks += len(tasks)
						taskIDs := make([]string, 0)

						for _, task := range tasks {
							taskIDs = append(taskIDs, task.ID)

							if task.Comment != "" {
								o, _, err := app.objectStore.Get(nil, fmt.Sprintf("study_id.keyword:%s", task.StudyID))
								if err != nil {
									utils.LogError(err)
								} else {
									comments = append(comments, map[string]interface{}{
										"id":         task.ID,
										"object_id":  o.ID,
										"creator_id": task.AssigneeID,
										"content":    task.Comment,
									})
								}
							}
						}

						if len(taskIDs) > 0 {
							app.antnStore.Query(map[string][]string{
								"task_id.keyword": taskIDs,
							}, "", 0, constants.DefaultLimit, "", nil,
								func(antns []annotation.Annotation, e entities.ESReturn) {
									for i := range antns {
										antn := antns[i]
										if mapID2User[antn.CreatorID] != nil && antn.CreatorName == "" {
											antn.CreatorName = mapID2User[antn.CreatorID].Username
										}

										for _, labelID := range antn.LabelIDs {
											if _, found := mapLabelImps[labelID]; found {
												mapA8sImps[antn.ID] = antn
											}
											if _, found := mapLabelFind[labelID]; found {
												mapA8sFind[antn.ID] = antn
											}
										}
									}
								})
						}
					})
			}

		})
		utils.LogInfo("size of tasks: %d", totalTasks)

		antnImps := make([]annotation.Annotation, 0)
		antnFind := make([]annotation.Annotation, 0)
		for _, v := range mapA8sImps {
			antnImps = append(antnImps, v)
		}
		for _, v := range mapA8sFind {
			antnFind = append(antnFind, v)
		}

		utils.LogInfo("Get and return objects")
		mapObjectTypeCount := make(map[string]int)
		objectsRet := make([]object.Object, 0)
		studyCount := 0
		studiesRet := make([]map[string]interface{}, 0)

		app.studyStore.Query(nil, fmt.Sprintf("project_id.keyword:%s", projectID),
			0, constants.DefaultLimit, "", nil, func(studies []study.Study, es entities.ESReturn) {
				studyCount += len(studies)

				for _, study := range studies {
					err := app.objectStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND study_id.keyword:%s", projectID, study.ID),
						0, constants.DefaultLimit, "", nil, func(objects []object.Object, es entities.ESReturn) {
							objectsRet = append(objectsRet, objects...)

							for _, object := range objects {
								mapObjectTypeCount[object.Type]++
								if object.Type == constants.ObjectTypeStudy {
									studiesRet = append(studiesRet, map[string]interface{}{
										"object_id": object.ID,
										"code":      study.Code,
										"status":    study.Status,
									})
								}
							}
						})

					if err != nil {
						utils.LogError(err)
					}
				}
			})
		utils.LogInfo("%d\t%v", studyCount, mapObjectTypeCount)

		utils.LogInfo("Get and return archived objects")
		mapArchives := make(map[string]bool)
		listArchives := make([]string, 0)
		acrhivedTask := 0
		app.taskStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND archived:%v", projectID, true), 0, constants.DefaultLimit,
			"", nil, func(tasks []study.Task, es entities.ESReturn) {
				acrhivedTask += len(tasks)
				for _, t := range tasks {
					app.objectStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND study_id.keyword:%s AND type.keyword:STUDY", projectID, t.StudyID), 0, 10, "", nil, func(objects []object.Object, es entities.ESReturn) {
						if len(objects) == 0 {
							utils.LogInfo("error")
						}
						for _, o := range objects {
							mapArchives[o.ID] = true
						}
					})
				}
			})
		for k := range mapArchives {
			listArchives = append(listArchives, k)
		}
		utils.LogInfo("%d-%d", acrhivedTask, len(listArchives))

		utils.LogInfo("Link result to return map")
		exportFile := make(map[string]interface{})
		exportFile["label_groups"] = labelGroupsReturn
		exportFile["impression"] = antnImps
		exportFile["finding"] = antnFind
		exportFile["labels"] = labelsReturn
		exportFile["objects"] = objectsRet
		exportFile["studies"] = studiesRet
		exportFile["comments"] = comments
		exportFile["archives"] = listArchives

		// fmt.Println(exportFile)
		bytes, _ := json.Marshal(exportFile)

		utils.LogInfo("Store file to minio")
		err1 := app.minioClient.StoreFile(labelExport.Tag, bytes)
		if err1 != nil {
			utils.LogError(err)
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		resp.Data = kvStr2Inf{
			constants.ParamID: labelExport.ID,
		}

		utils.LogInfo("Update es object")
		labelExport.Status = constants.ExportStatusDone
		app.labelExportStore.Create(labelExport)
	}()

	c.JSON(http.StatusOK, resp)

	return
}

func (app *StatsAPI) GetStatsLabelsByAgg(c *gin.Context) {
	resp := entities.NewResponse()

	projectID := c.Query(constants.ParamProjectID)
	project, _, _ := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))
	studyStatus := c.Query(constants.ParamStudyStatus)
	taskStatus := c.Query(constants.ParamTaskStatus)

	if !study.IsValidTaskStatus(taskStatus) || !study.IsValidStatus(studyStatus) ||
		projectID == "" || project == nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	studyIDsCompleted := make([]string, 0)
	err := app.studyStore.Query(nil, fmt.Sprintf("project_id.keyword:%s AND status.keyword:%s", projectID, studyStatus), 0, constants.DefaultLimit, "", nil,
		func(studies []study.Study, es entities.ESReturn) {
			for i := range studies {
				studyIDsCompleted = append(studyIDsCompleted, studies[i].ID)
			}
		})
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	aggStatsLabels := make(map[string]map[string]int)
	studyIDsQueried := make([]string, 0)

	for i := range studyIDsCompleted {
		studyIDsQueried = append(studyIDsQueried, studyIDsCompleted[i])
		if len(studyIDsQueried) == 100 || i == len(studyIDsCompleted)-1 {
			err := app.taskStore.Query(map[string][]string{"study_id.keyword": studyIDsQueried},
				fmt.Sprintf("type.keyword:%s AND status.keyword:%s", constants.TaskTypeReview, taskStatus), 0, constants.DefaultLimit, "", nil,
				func(tasks []study.Task, es entities.ESReturn) {

					taskIDs := make([]string, 0)
					for i := range tasks {
						taskIDs = append(taskIDs, tasks[i].ID)
					}

					if len(taskIDs) > 0 {
						_, esReturn, err := app.antnStore.GetSlice(map[string][]string{"task_id.keyword": taskIDs}, "", 0, 0, "", []string{"label_ids"})
						if err != nil {
							resp.ErrorCode = constants.ServerError
							c.JSON(http.StatusInternalServerError, resp)
							return
						}

						for key, aggs := range *esReturn.Aggregations {
							for i, bucket := range aggs.Buckets {
								if aggStatsLabels[key] == nil {
									aggStatsLabels[key] = make(map[string]int)
								}
								aggStatsLabels[key][bucket.Key] += aggs.Buckets[i].DocCount
							}
						}
					}
				})

			if err != nil {
				resp.ErrorCode = constants.ServerError
				c.JSON(http.StatusInternalServerError, resp)
				return
			}

			studyIDsQueried = make([]string, 0)
		}
	}

	if len(aggStatsLabels) > 0 {
		labelIDs := make([]string, 0)
		for k := range aggStatsLabels["label_ids"] {
			labelIDs = append(labelIDs, k)
		}

		labels := make([]annotation.Label, 0)
		err := app.labelStore.Query(map[string][]string{
			"_id": labelIDs,
		}, "", 0, 10, "", nil, func(l []annotation.Label, e entities.ESReturn) {
			labels = append(labels, l...)
		})
		if err != nil {
			resp.ErrorCode = constants.ServerError
			c.JSON(http.StatusInternalServerError, resp)
			return
		}

		mapLabelsReturn := make(map[string][]annotation.Label, 0)
		for i := range labels {
			if mapLabelsReturn[labels[i].Type] == nil {
				mapLabelsReturn[labels[i].Type] = make([]annotation.Label, 0)
			}
			mapLabelsReturn[labels[i].Type] = append(mapLabelsReturn[labels[i].Type], labels[i])
		}

		resp.Data = mapLabelsReturn
		resp.Agg = &kvStr2Inf{
			"label_ids": aggStatsLabels["label_ids"],
		}
	}

	c.JSON(http.StatusOK, resp)
	return
}

func (app *StatsAPI) GetLabelExports(c *gin.Context) {
	resp := entities.NewResponse()

	queries, qs, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)
	if sort == "" {
		sort = "-created"
	}

	labelExports, esReturn, err := app.labelExportStore.GetSlice(queries, qs, from, size, sort, aggs)
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = labelExports
	resp.Count = esReturn.Hits.Total.Value

	c.JSON(http.StatusOK, resp)
}

func (app *StatsAPI) DownloadLabelExport(c *gin.Context) {
	resp := entities.NewResponse()

	labelExportID := c.Param(constants.ParamID)
	if labelExportID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	labelExports, _, err := app.labelExportStore.GetSlice(nil, fmt.Sprintf("_id:%s", labelExportID), 0, 1, "", nil)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if len(labelExports) > 0 {
		labelExport := labelExports[0]
		file, err := app.minioClient.DownloadFile(labelExport)
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		localFile, err := os.Create(fmt.Sprintf("%s.%d", labelExport.Tag, time.Now().Unix()))
		if err != nil {
			log.Fatalln(err)
		}
		defer localFile.Close()

		fileInfo, _ := file.Stat()
		utils.LogInfo("%d", fileInfo.Size)
		if err != nil {
			log.Fatalln(err)
		}
		var data []byte
		n, _ := localFile.Read(data)
		utils.LogInfo("%d\t%d", n, data)

		if _, err := io.CopyN(localFile, file, fileInfo.Size); err != nil {
			log.Fatalln(err)
		}

		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+labelExport.FilePath)
		c.Header("Content-Type", "application/octet-stream")
		c.File(localFile.Name())
		os.Remove(localFile.Name())
	}
}

func (app *StatsAPI) GetProjectsByRole(c *gin.Context) {
	resp := entities.NewResponse()

	roles := c.QueryArray("role")
	if len(roles) == 0 {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	size, err := strconv.Atoi(c.Query(constants.ParamLimit))
	if err != nil {
		size = constants.DefaultLimit
	}
	from, err := strconv.Atoi(c.Query(constants.ParamOffset))
	if err != nil {
		from = constants.DefaultOffset
	}

	authInfo := mw.GetAuthInfoFromGin(c)
	userID := authInfo.ID
	projectsRet := make([]project.Project, 0)
	rolesQ := make([]string, 0)

	for _, role := range roles {
		if strings.Contains(role, "PO") {
			role = "PO"
		}
		rolesQ = append(rolesQ, fmt.Sprintf("roles_mapping.%s.keyword:%s", role, userID))
	}

	projects, esReturn, err := app.projectStore.GetSlice(nil, strings.Join(rolesQ, " OR "), from, size, "-created", nil)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	for _, project := range projects {
		projectsRet = append(projectsRet, project)
	}

	aggs := []string{"status"}
	switch roles[0] {
	case constants.ProjRoleProjectOwner:
		for i, project := range projectsRet {
			_, esReturn, err := app.studyStore.GetSlice(nil, fmt.Sprintf("project_id:%s", project.ID), 0, 0, "", aggs)
			if err != nil {
				utils.LogError(err)
				resp.ErrorCode = constants.ServerError
				c.JSON(http.StatusInternalServerError, resp)
				return
			}

			utils.LogDebug("%s\t%s\t%s", project.ID, project.Name, esReturn.Aggregations)
			if esReturn.Aggregations != nil {
				for _, agg := range aggs {
					v := *esReturn.Aggregations
					arrMap := make(map[string]interface{})
					for _, bucket := range v[agg].Buckets {
						arrMap[bucket.Key] = bucket.DocCount
					}
					projectsRet[i].Meta = arrMap
				}
			}
		}
		break
	case constants.ProjRoleReviewer, constants.ProjRoleAnnotator:
		for i, project := range projectsRet {
			_, esReturn, err := app.taskStore.GetSlice(nil, fmt.Sprintf("assignee_id:%s AND project_id:%s", userID, project.ID), 0, 0, "", aggs)
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

					projectsRet[i].Meta = arrMap
				}
			}
		}
		break
	}

	resp.Count = esReturn.Hits.Total.Value
	resp.Data = projectsRet
	c.JSON(http.StatusOK, resp)
}

func (app *StatsAPI) GetAssgineeOfStudy(c *gin.Context) {
	resp := entities.NewResponse()

	studyID := c.Param(constants.ParamID)
	if studyID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	s, _, err := app.studyStore.Get(nil, fmt.Sprintf("_id:%s", studyID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if s == nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	p, _, err := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", s.ProjectID))
	if err != nil {
		utils.LogError(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	mapUserOfStudy := make(map[string]bool)
	app.taskStore.Query(nil, fmt.Sprintf("study_id.keyword:%s", studyID), 0, 10, "", nil, func(tasks []study.Task, es entities.ESReturn) {
		for _, t := range tasks {
			mapUserOfStudy[t.AssigneeID] = true
		}
	})

	listAssignee := make([]project.ProjectPerson, 0)
	for _, person := range p.People {
		if _, ok := mapUserOfStudy[person.ID]; ok {
			listAssignee = append(listAssignee, person)
		}
	}

	resp.Data = kvStr2Inf{
		"assignees": listAssignee,
	}

	c.JSON(http.StatusOK, resp)
}
