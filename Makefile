VERSION ?= 0.1.0
NS      ?= hello

.PHONY: help build import deploy status logs test clean

help:    ## แสดงคำสั่งทั้งหมด
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n",$$1,$$2}'

build:   ## build docker image
	docker build --build-arg VERSION=$(VERSION) -t hello-devops:$(VERSION) .

import:  ## ส่ง image เข้า k3s
	docker save hello-devops:$(VERSION) | sudo k3s ctr images import -

deploy:  ## deploy ขึ้น cluster
	kubectl apply -f k8s/00-namespace.yaml
	kubectl apply -f k8s/
	kubectl -n $(NS) rollout status deploy/hello

status:  ## ดูสถานะ
	@kubectl -n $(NS) get deploy,pods,svc,ingress,endpoints

logs:    ## ดู log
	kubectl -n $(NS) logs -l app.kubernetes.io/name=hello --prefix --tail=100 -f

clean:   ## ลบทุกอย่างออกจาก cluster
	kubectl delete namespace $(NS) --ignore-not-found
