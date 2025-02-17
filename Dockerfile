# Copyright 2023 - 2025 Dima Krasner
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# 	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.24-alpine AS build
RUN apk add --no-cache gcc musl-dev openssl
COPY go.mod /src/
COPY go.sum /src/
WORKDIR /src
RUN go mod download
COPY migrations /src/migrations
RUN go generate ./migrations
COPY . /src
RUN go vet ./...
RUN go test ./... -failfast -vet off -tags fts5
ARG TOOTIK_VERSION=?
RUN go build -ldflags "-X github.com/dimkr/tootik/buildinfo.Version=$TOOTIK_VERSION" -tags fts5 ./cmd/tootik

FROM alpine
RUN apk add --no-cache ca-certificates openssl
COPY --from=build /src/tootik /
COPY --from=build /src/LICENSE /
RUN adduser -D tootik
USER tootik
WORKDIR /tmp
ENTRYPOINT ["/tootik"]
