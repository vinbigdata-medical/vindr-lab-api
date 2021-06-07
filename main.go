package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"vindr-lab-api/account"
	"vindr-lab-api/annotation"
	"vindr-lab-api/constants"
	"vindr-lab-api/helper"
	"vindr-lab-api/keycloak"
	"vindr-lab-api/label_group"
	"vindr-lab-api/object"
	"vindr-lab-api/project"
	"vindr-lab-api/session"
	"vindr-lab-api/stats"
	"vindr-lab-api/study"
	"vindr-lab-api/utils"

	"github.com/bsm/redislock"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func newLogger() *zap.Logger {
	env := viper.GetString("workspace.env")
	var logger *zap.Logger
	switch env {
	case "DEVELOPMENT":
		logger, _ = zap.NewDevelopment()
	default:
		logger, _ = zap.NewProduction()
	}
	return logger
}

func initConfigs(env string) {
	viper.AddConfigPath("conf")
	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "__")
	viper.SetEnvKeyReplacer(replacer)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
}

func getMapEnvVars() *map[string]string {
	ret := make(map[string]string)
	envsOS := os.Environ()
	for _, envOS := range envsOS {
		items := strings.Split(envOS, "=")
		if len(items) > 1 {
			ret[items[0]] = items[1]
		}
	}
	return &ret
}

func main() {

	envVars := getMapEnvVars()
	env := "development"
	if value, found := (*envVars)[constants.ENV]; found {
		env = value
	}
	utils.LogInfo(fmt.Sprintf("API is running in [%s] mode", env))
	initConfigs(env)

	route := gin.Default()
	logger := newLogger()

	route.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"POST", "PUT", "GET", "DELETE"},
		AllowHeaders:     []string{"Access-Control-Allow-Headers", "Origin", "Accept", "X-Requested-With", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	var esAddresses []string
	esSingleNode := viper.GetString("elasticsearch.uri")
	if esSingleNode != "" {
		esAddresses = []string{esSingleNode}
	} else {
		esAddresses = viper.GetStringSlice("elasticsearch.uris")
	}
	utils.LogInfo("%v", esAddresses)

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: esAddresses,
	})
	_, err = es.Info()
	if err != nil {
		panic("Cannot connect to ES")
	}

	clientRedis := redis.NewClient(&redis.Options{
		Network:    "tcp",
		Addr:       viper.GetString("redis.uri"),
		MaxRetries: 1000,
	})
	defer clientRedis.Close()

	// Create a new lock client.
	lockerRedis := redislock.New(clientRedis)

	orthancClient := study.NewStudyOrthanC(viper.GetString("orthanc.uri"))
	idGenerator := helper.NewIDGenerator(viper.GetString("id_generator.uri"))

	// route.Use(mw.WrapAuthInfo(logger))
	route.Static("/docs/", "vindr-lab-api.html")

	labelStore := annotation.NewLabelStore(es, viper.GetString("elasticsearch.label_index_prefix"), logger)
	antnStore := annotation.NewAnnotationStore(es, viper.GetString("elasticsearch.annotation_index_prefix"), "es_template_annotation", logger)
	studyStore := study.NewStudyStore(es, viper.GetString("elasticsearch.study_index_prefix"), logger)
	projectStore := project.NewProjectStore(es, viper.GetString("elasticsearch.project_index_prefix"), logger)
	sessionStore := session.NewSessionStore(es, viper.GetString("elasticsearch.session_index_alias"), logger)
	objectStore := object.NewObjectStore(es, viper.GetString("elasticsearch.object_index_prefix"), logger)
	labelExportStore := stats.NewLabelExportStore(es, viper.GetString("elasticsearch.label_export_index_prefix"), logger)
	labelGroupStore := label_group.NewLabelGroupStore(es, viper.GetString("elasticsearch.label_group_index_prefix"), logger)
	taskStore := study.NewTaskStore(es, viper.GetString("elasticsearch.task_index_prefix"), logger)

	//put template
	utils.LogError(antnStore.PutMapping())
	utils.LogError(antnStore.PutIndexTemplate())

	kc := &keycloak.KeycloakConfig{
		MasterRealm:   viper.GetString("keycloak.master_realm"),
		AdminUsername: viper.GetString("keycloak.admin_username"),
		AdminPassword: viper.GetString("keycloak.admin_password"),
		KeycloakURI:   viper.GetString("keycloak.uri"),
	}
	keycloakStore := account.NewKeycloakStore(kc, viper.GetString("keycloak.app_realm"))

	utils.LogInfo(viper.GetString("minio.uri"))
	minioClient, err := minio.New(
		viper.GetString("minio.uri"),
		&minio.Options{
			Creds: credentials.NewStaticV4(viper.GetString("minio.access_key_id"), viper.GetString("minio.secret_access_key"), ""),
		})

	if err != nil {
		panic("Cannot connect to MinIO")
	}
	minioStorage := stats.NewMinIOStorage(minioClient, viper.GetString("minio.bucket_name"))

	annotationAPI := annotation.NewAnnotationAPI(antnStore, labelStore, keycloakStore, logger)
	annotationAPI.InitRoute(route, "annotations")

	labelAPI := annotation.NewLabelAPI(labelStore, antnStore, projectStore, logger)
	labelAPI.InitRoute(route, "labels")

	studyAPI := study.NewStudyAPI(studyStore, taskStore, projectStore, objectStore, orthancClient, logger)
	studyAPI.InitRoute(route, "studies")

	projectAPI := project.NewProjectAPI(projectStore, logger)
	projectAPI.InitRoute(route, "projects")

	taskAPI := study.NewTaskAPI(taskStore, studyStore, projectStore, objectStore, antnStore, labelStore, idGenerator, logger)
	taskAPI.InitRoute(route, "tasks")

	objectAPI := object.NewObjectAPI(objectStore, lockerRedis, logger)
	objectAPI.InitRoute(route, "objects")
	go objectAPI.DequeueObjects()
	time.Sleep(1 * time.Millisecond)

	stats := stats.NewLabelExportAPI(labelExportStore, labelGroupStore, labelStore, projectStore, antnStore, objectStore, studyStore, taskStore,
		minioStorage, keycloakStore, logger)
	stats.InitRoute(route, "stats")

	sessionAPI := session.NewSessionAPI(sessionStore, logger)
	sessionAPI.InitRoute(route, "sessions")

	labelGroupAPI := label_group.NewLabelGroupAPI(labelGroupStore, labelStore, logger)
	labelGroupAPI.InitRoute(route, "label_groups")

	accountAPI := account.NewAccountAPI(keycloakStore, logger)
	accountAPI.InitRoute(route, "accounts")

	route.Run("0.0.0.0:" + viper.GetString("webserver.port"))
}
