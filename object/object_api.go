package object

import (
	"fmt"
	"net/http"
	"time"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
	"vindr-lab-api/mw"
	"vindr-lab-api/utils"

	"github.com/bsm/redislock"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ObjectAPI struct {
	objectStore *ObjectES
	lockerRedis *redislock.Client
	logger      *zap.Logger
}

func NewObjectAPI(studyStore *ObjectES, lockerRedis *redislock.Client, logger *zap.Logger) (app *ObjectAPI) {
	app = &ObjectAPI{
		objectStore: studyStore,
		lockerRedis: lockerRedis,
		logger:      logger,
	}
	return app
}

func (app *ObjectAPI) InitRoute(engine *gin.Engine, path string) {
	group := engine.Group(path, mw.WrapAuthInfo(app.logger))
	group.GET("", mw.ValidPerms(path, mw.PERM_R), app.FetchObject)
	group.POST("", mw.ValidPerms(path, mw.PERM_C), app.CreateObject)
	group.PUT("/:id", mw.ValidPerms(path, mw.PERM_U), app.UpdateObject)
	group.DELETE("/:id", mw.ValidPerms(path, mw.PERM_D), app.DeleteObject)
}

func (app *ObjectAPI) FetchObject(c *gin.Context) {
	resp := entities.NewResponse()

	queries, queryStr, from, size, sort, aggs := utils.ConvertGinRequestToParams(c)

	objects := make([]Object, 0)
	objects, esReturn, err := app.objectStore.GetSlice(queries, queryStr, from, size, sort, aggs)

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

	resp.Data = objects
	resp.Count = esReturn.Hits.Total.Value

	c.JSON(http.StatusOK, resp)
}

func (app *ObjectAPI) CreateObject(c *gin.Context) {
	resp := entities.NewResponse()

	var objectBig ObjectBig
	err := c.ShouldBindJSON(&objectBig)

	if err != nil {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	app.QueueObject(objectBig)

	c.JSON(http.StatusOK, resp)
}

var queue *goconcurrentqueue.FIFO

func (app *ObjectAPI) DequeueObjects() {
	for true {
		if queue != nil && queue.GetLen() > 0 {

			objectBig, err := queue.Dequeue()
			if err == nil && objectBig != nil {
				err = ProcessCreateObject(*app.objectStore, app.lockerRedis, objectBig.(ObjectBig))
				if err != nil {
					utils.LogError(err)
				}
			}
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func (app *ObjectAPI) QueueObject(objectBig ObjectBig) {
	if queue == nil {
		queue = goconcurrentqueue.NewFIFO()
	}
	queue.Enqueue(objectBig)
}

func ProcessCreateObject(objectStore ObjectES, lockerRedis *redislock.Client, objectBig ObjectBig) error {
	now := time.Now().UnixNano() / int64(time.Millisecond)

	projectID := objectBig.ProjectID
	studyID := objectBig.StudyID

	sliceStudyInstanceUIDs := *objectBig.ListStudyInstanceUID
	sliceSeriesInstanceUIDs := *objectBig.ListSeriesInstanceUID
	sliceSOPInstanceUIDs := *objectBig.ListSOPInstanceUID

	objects := make([]Object, 0)
	for _, studyInstanceUID := range sliceStudyInstanceUIDs {
		_, esReturn, err := objectStore.GetSlice(nil,
			fmt.Sprintf("project_id.keyword:%s AND meta.study_instance_uid.keyword:%s AND type.keyword:%s",
				projectID, studyInstanceUID, constants.ObjectTypeStudy),
			0, 0, "", nil)
		if err != nil {
			utils.LogError(err)
		}

		if esReturn.Hits.Total.Value == 0 {
			objects = append(objects, Object{
				ID:        uuid.New().String(),
				Created:   now,
				Type:      constants.ObjectTypeStudy,
				ProjectID: projectID,
				StudyID:   studyID,
				Meta: &entities.MetaData{
					StudyInstanceUID: studyInstanceUID,
				},
			})
		}
	}

	for _, studyInstanceUID := range sliceStudyInstanceUIDs {
		for _, seriesInstanceUID := range sliceSeriesInstanceUIDs {

			_, esReturn, err := objectStore.GetSlice(nil,
				fmt.Sprintf("project_id.keyword:%s AND meta.series_instance_uid.keyword:%s AND type.keyword:%s",
					projectID, seriesInstanceUID, constants.ObjectTypeSeries),
				0, 0, "", nil)
			if err != nil {
				utils.LogError(err)
			}

			if esReturn.Hits.Total.Value == 0 {
				objects = append(objects, Object{
					ID:        uuid.New().String(),
					Created:   now,
					Type:      constants.ObjectTypeSeries,
					ProjectID: projectID,
					StudyID:   studyID,
					Meta: &entities.MetaData{
						StudyInstanceUID:  studyInstanceUID,
						SeriesInstanceUID: seriesInstanceUID,
					},
				})
			}
		}
	}

	for _, studyInstanceUID := range sliceStudyInstanceUIDs {
		for _, seriesInstanceUID := range sliceSeriesInstanceUIDs {
			for _, sopInstanceUID := range sliceSOPInstanceUIDs {

				_, esReturn, err := objectStore.GetSlice(nil,
					fmt.Sprintf("project_id.keyword:%s AND meta.sop_instance_uid.keyword:%s AND type.keyword:%s",
						projectID, sopInstanceUID, constants.ObjectTypeImage),
					0, 0, "", nil)
				if err != nil {
					utils.LogError(err)
				}

				if esReturn.Hits.Total.Value == 0 {
					objects = append(objects, Object{
						ID:        uuid.New().String(),
						Created:   now,
						Type:      constants.ObjectTypeImage,
						ProjectID: projectID,
						StudyID:   studyID,
						Meta: &entities.MetaData{
							StudyInstanceUID:  studyInstanceUID,
							SeriesInstanceUID: seriesInstanceUID,
							SOPInstanceUID:    sopInstanceUID,
						},
					})
				}
			}
		}
	}

	if len(objects) > 0 {
		err := objectStore.Bulk(objects)
		if err != nil {
			utils.LogError(err)
			return err
		}
	}

	return nil
}

func (app *ObjectAPI) UpdateObject(c *gin.Context) {
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
	app.objectStore.Update(Object{ID: antnID}, updateMap)
	fmt.Println(updateMap)

	c.JSON(http.StatusOK, resp)
}

func (app *ObjectAPI) DeleteObject(c *gin.Context) {
	resp := entities.NewResponse()

	objectID := c.Param(constants.ParamID)
	if objectID == "" {
		resp.ErrorCode = constants.ServerInvalidData
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	err := app.objectStore.Delete(nil, fmt.Sprintf("_id:%s", objectID))
	if err != nil {
		fmt.Println(err)
		resp.ErrorCode = constants.ServerError
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}
