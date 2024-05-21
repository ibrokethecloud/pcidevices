/* This file was part of the google/gousb project, copied to this project
 * to get around private package issues.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2024 SUSE, LLC.
 *
 */

package usbid

import "time"

// LastUpdate stores the latest time that the library was updated.
//
// The baked-in data was last generated:
//   {{.Generated}}
var LastUpdate = time.Unix(0, {{.Generated.UnixNano}})

const usbIDListData = `{{printf "%s" .Data}}`