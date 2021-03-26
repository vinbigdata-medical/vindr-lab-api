package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/utils"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ProjectAPI struct {
	projectStore *ProjectES
	esClient     *elasticsearch.Client
	logger       *zap.Logger
}

func NewProjectAPI(storeProject *ProjectES, logger *zap.Logger) (app *ProjectAPI) {
	app = &ProjectAPI{
		projectStore: storeProject,
		logger:       logger,
	}
	return app
}

func (app *ProjectAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.GetProjects)
	group.GET("/:id", mw.ValidPerms(path, mw.PERM_R), app.GetProject)
	group.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateProject)
	group.POST("/:id/people", mw.ValidPerms(path, mw.PERM_C), app.AddPeopleToProject)
	group.PUT("/:id/people", mw.ValidPerms(path, mw.PERM_U), app.UpdatePeopleOfProject)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateProject)
	group.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.DeleteProject)
}

func (app *ProjectAPI) GetProject(c *gin.Context) {
	resp := entities.NewResponse()

	projectID := c.Param(constants.ParamID)
	project, _, err := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Data = project
	c.JSON(http.StatusOK, resp)
}

func (app *ProjectAPI) GetProjects(c *gin.Context) {
	resp := entities.NewResponse()

	queries, queryStr, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)

	projects, esReturn, err := app.projectStore.GetSlice(queries, queryStr, from, size, sort, aggs)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	resp.Count = esReturn.Hits.Total.Value

	resp.Data = projects
	c.JSON(http.StatusOK, resp)
}

func (app *ProjectAPI) CreateProject(c *gin.Context) {
	resp := entities.NewResponse()

	var project Project
	err := c.ShouldBindJSON(&project)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	authInfo := mw.GetAuthInfoFromGin(c)
	project.CreatorID = authInfo.ID
	project.People = append(project.People, ProjectPerson{
		Username: authInfo.Username,
		ID:       authInfo.ID,
		Roles:    []string{constants.ProjRoleProjectOwner},
	})
	project.retrieveRolesMapFromPeople()

	if !project.IsValidProject() {
		utils.LogError(fmt.Errorf(project.String()))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	newID := uuid.New().String()
	project.ID = newID
	project.Created = time.Now().UnixNano() / int64(time.Millisecond)

	if project.Key == "" {
		utils.LogError(fmt.Errorf("Project Key is empty"))
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err = app.projectStore.Create(project)
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	ret := make(map[string]interface{}, 0)
	ret[constants.ParamID] = newID
	resp.Data = ret

	c.JSON(http.StatusOK, resp)
}

func (app *ProjectAPI) UpdateProject(c *gin.Context) {
	resp := entities.NewResponse()

	projectID := c.Param(constants.ParamID)
	if projectID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	_, esReturn, err := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	project := Project{}
	if len(esReturn.Hits.Hits) > 0 {
		mapData := esReturn.Hits.Hits[0].Source
		bytesData, _ := json.Marshal(mapData)
		json.Unmarshal(bytesData, &project)
	}

	updateMap := make(map[string]interface{})
	err1 := c.ShouldBind(&updateMap)
	if err1 != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	err2 := app.projectStore.Update(project, updateMap)
	if err2 != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	fmt.Println(updateMap)

	c.JSON(http.StatusOK, resp)
}

type AddProjectPeopleRequestBody struct {
	People []ProjectPerson `json:"people"`
}

func (app *ProjectAPI) AddPeopleToProject(c *gin.Context) {
	var resp = entities.NewResponse()

	projectID := c.Param("id")
	var b AddProjectPeopleRequestBody
	if err := c.BindJSON(&b); err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.String(http.StatusBadRequest, resp.String())
		return
	}

	project, esReturn, _ := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))

	currentPeople := project.People
	if currentPeople == nil {
		currentPeople = make([]ProjectPerson, 0)
	}

	newPeople := b.People
	for _, newP := range newPeople {
		added := false
		for ith, currentP := range currentPeople {
			if newP.ID == currentP.ID {
				roles := newP.Roles
				roles = append(roles, currentP.Roles...)
				temp := make(map[string]bool)
				newRoles := make([]string, 0)
				for _, r := range roles {
					if _, found := temp[r]; !found {
						newRoles = append(newRoles, r)
						temp[r] = true
					}
				}
				currentPeople[ith].Roles = newRoles
				added = true
				break
			}
		}
		if !added {
			currentPeople = append(currentPeople, newP)
		}
	}

	project.People = currentPeople
	project.retrieveRolesMapFromPeople()

	hit := esReturn.Hits.Hits[0]
	update := ProjectUpdateRequest{
		ID:      hit.ID,
		Index:   hit.Index,
		Project: *project,
	}
	ctx := context.TODO()
	app.projectStore.Persist(ctx, &update)

	resp.Data = currentPeople
	c.JSON(http.StatusOK, resp)
}

func (app *ProjectAPI) UpdatePeopleOfProject(c *gin.Context) {
	var resp = entities.NewResponse()

	projectID := c.Param("id")
	var b AddProjectPeopleRequestBody
	if err := c.BindJSON(&b); err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.String(http.StatusBadRequest, resp.String())
		return
	}

	project, esReturn, err := app.projectStore.Get(nil, fmt.Sprintf("_id:%s", projectID))
	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.String(http.StatusBadRequest, resp.String())
		return
	}

	newPeople := b.People
	mapUserRoles := make(map[string]ProjectPerson)
	for _, person := range newPeople {
		person1, found := mapUserRoles[person.ID]
		if !found {
			mapUserRoles[person.ID] = person
		} else {
			roles1 := person1.Roles
			for _, role := range person.Roles {
				if _, in := utils.FindInSlice(roles1, role); !in {
					roles1 = append(roles1, role)
				}
			}
			person1.Roles = roles1
			mapUserRoles[person.ID] = person1
		}
	}

	newPeople = make([]ProjectPerson, 0)
	for _, person := range mapUserRoles {
		newPeople = append(newPeople, person)
	}

	project.People = newPeople
	project.retrieveRolesMapFromPeople()

	if !project.IsValidProjectRole() {
		resp.ErrorCode = constants.ServerInvalidData
		c.String(http.StatusBadRequest, resp.String())
		return
	}

	hit := esReturn.Hits.Hits[0]
	update := ProjectUpdateRequest{
		ID:      hit.ID,
		Index:   hit.Index,
		Project: *project,
	}
	ctx := context.TODO()
	app.projectStore.Persist(ctx, &update)

	resp.Data = project.People
	c.JSON(http.StatusOK, resp)
}

func (app *ProjectAPI) DeleteProject(c *gin.Context) {
	resp := entities.NewResponse()

	projectID := c.Param(constants.ParamID)
	if projectID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.projectStore.Delete(Project{
		ID: projectID,
	})
	if err != nil {
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}
