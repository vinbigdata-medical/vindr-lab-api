TAG := vindr/vinlab-api

build:
	docker build -t ${TAG} .

push-image:
	echo ${DOCKERHUB_PASSWORD} | docker login --username ${DOCKERHUB_USERNAME} --password-stdin
	docker push ${TAG}

test-local:
	go test -coverprofile=coverage.out ./...; \
	go tool cover -func=coverage.out
