# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION=$(shell cat VERSION)
H3_VERSION=1.1
REGISTRY_NAME=carvicsforth
BASE_TAG=$(REGISTRY_NAME)/h3:$(H3_VERSION)
IMAGE_TAG=$(REGISTRY_NAME)/csi-h3:$(VERSION)

.PHONY: all h3-plugin clean h3-container

all: plugin container push

plugin:
	go mod download
	CGO_ENABLED=0 GOOS=linux go build -a -gcflags=-trimpath=$(go env GOPATH) -asmflags=-trimpath=$(go env GOPATH) -ldflags '-X github.com/CARV-ICS-FORTH/csi-h3/pkg/h3.DriverVersion=$(VERSION) -extldflags "-static"' -o _output/csi-h3-plugin ./cmd/csi-h3-plugin

container:
	docker build --build-arg BASE=$(BASE_TAG) -t $(IMAGE_TAG) -f ./cmd/csi-h3-plugin/Dockerfile .
	docker build --build-arg BASE=$(BASE_TAG)-dev -t $(IMAGE_TAG)-dev -f ./cmd/csi-h3-plugin/Dockerfile .

push:
	docker push $(IMAGE_TAG)
	docker push $(IMAGE_TAG)-dev

clean:
	go clean -r -x
	-rm -rf _output
