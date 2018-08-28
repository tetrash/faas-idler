TAG?=latest-dev
.PHONY: build

build:
	docker build -t openfaas/faas-idler:${TAG} .
push:
	docker push openfaas/faas-idler:${TAG}
ci-armhf-build:
	docker build -t openfaas/faas-idler:${TAG}-armhf . -f Dockerfile.armhf
ci-armhf-push:
	docker push openfaas/faas-idler:${TAG}-armhf
