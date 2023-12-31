/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
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
 */

package miniocmd

import (
	"errors"
)

const (
	lockRESTVersion       = "v6" // Add Refresh API
	lockRESTVersionPrefix = SlashSeparator + lockRESTVersion
	lockRESTPrefix        = minioReservedBucketPath + "/lock"
)

const (
	lockRESTMethodHealth      = "/health"
	lockRESTMethodRefresh     = "/refresh"
	lockRESTMethodLock        = "/lock"
	lockRESTMethodRLock       = "/rlock"
	lockRESTMethodUnlock      = "/unlock"
	lockRESTMethodRUnlock     = "/runlock"
	lockRESTMethodForceUnlock = "/force-unlock"

	// lockRESTOwner represents owner UUID
	lockRESTOwner = "owner"

	// Unique ID of lock/unlock request.
	lockRESTUID = "uid"

	// Source contains the line number, function and file name of the code
	// on the client node that requested the lock.
	lockRESTSource = "source"

	// Quroum value to be saved along lock requester info, useful
	// in verifying stale locks
	lockRESTQuorum = "quorum"
)

var (
	errLockConflict       = errors.New("lock conflict")
	errLockNotInitialized = errors.New("lock not initialized")
	errLockNotFound       = errors.New("lock not found")
)
