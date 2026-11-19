#!/bin/sh -e

# Copyright 2026 Dima Krasner
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

# Builds the pg_lake image used by tests: PostgreSQL 18 with pg_lake and pgduck_server. The tag must match lakeImage
# in lake.go.
#
# Set NCORES to limit the number of parallel DuckDB compilation jobs (all cores by default), if the build runs out of
# memory.

PG_LAKE_VERSION=v3.6.2

src=`mktemp -d "${TMPDIR:-/tmp}/pg_lake.XXXXXX"`
trap 'rm -rf "$src"' EXIT

git clone --depth 1 --branch $PG_LAKE_VERSION https://github.com/Snowflake-Labs/pg_lake "$src"

# the upstream Dockerfile clones pg_lake again, at the ref in PG_LAKE_REF
sed -i.orig "s/^ENV PG_LAKE_REF=.*/ENV PG_LAKE_REF=$PG_LAKE_VERSION/" "$src/docker/Dockerfile"
grep -q "^ENV PG_LAKE_REF=$PG_LAKE_VERSION\$" "$src/docker/Dockerfile"

# the DuckDB build uses NCORES jobs
sed -i.orig "s/^FROM base AS pgduck_builder\$/&\\
ARG NCORES/" "$src/docker/Dockerfile"
grep -q "^ARG NCORES\$" "$src/docker/Dockerfile"

cat << 'DOCKERFILE' >> "$src/docker/Dockerfile"

FROM pg_lake_postgres AS tootik
ARG PG_MAJOR=18
COPY --from=pgduck_server --chown=postgres:postgres /home/postgres/pgsql-${PG_MAJOR}/bin/ /home/postgres/pgsql-${PG_MAJOR}/bin/
COPY --from=pgduck_server --chown=postgres:postgres /home/postgres/pgsql-${PG_MAJOR}/lib/ /home/postgres/pgsql-${PG_MAJOR}/lib/
DOCKERFILE

docker build --build-arg PG_MAJOR=18 --build-arg NCORES --target tootik -t tootik-pg-lake:$PG_LAKE_VERSION "$src/docker"
