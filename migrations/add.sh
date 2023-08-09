#!/bin/sh -e

# Copyright 2023 Dima Krasner
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

last=`ls migrations/[0-9][0-9][0-9]_*.go | sort -n | tail -n 1 | cut -f 2 -d / | cut -f 1 -d _`
new=migrations/`printf "%03d" $(($last+1))`_$1.go

echo "Creating $new"

cat << EOF > $new
package migrations

import (
	"context"
	"database/sql"
)

func $1(ctx context.Context, db *sql.DB) error {
	// do stuff

	return nil
}
EOF
