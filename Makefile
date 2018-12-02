TAG?=latest-dev
.PHONY: build push ci-armhf-build ci-armhf-push ci-arm64-build ci-arm64-push

build:
	docker build -t openfaas/faas-idler:${TAG} .

push:
	docker push openfaas/faas-idler:${TAG}

ci-armhf-build:
	docker build -t openfaas/faas-idler:${TAG}-armhf . -f Dockerfile.armhf

ci-armhf-push:
	docker push openfaas/faas-idler:${TAG}-armhf

ci-arm64-build:
	docker build -t openfaas/faas-idler:${TAG}-arm64 . -f Dockerfile.arm64

ci-arm64-push:
	docker push openfaas/faas-idler:${TAG}-arm64
