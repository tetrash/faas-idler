TAG?=latest-dev
.PHONY: build

build:
	docker build -t openfaas/faas-idler:${TAG} .
push:
	docker push openfaas/faas-idler:${TAG}
