package inspection

import "errors"

// ErrHiveNotFound is returned when the hive an inspection is being
// created under doesn't exist, doesn't belong to the caller (which also
// covers its apiary not belonging to the caller, since hive-service's own
// ownership check is transitive), or its ownership couldn't be
// confirmed. As with inspection.ErrNotFound, these cases are deliberately
// indistinguishable: a caller must not be able to tell whether another
// user's hive ID exists at all.
var ErrHiveNotFound = errors.New("hive not found")

// ErrImageNotFound is returned when an ID in CreateInput.Images or
// UpdateInput.Images doesn't belong to the caller, verified via a read
// against media-service (GET /api/v1/media?ids=) - whether because it
// doesn't exist, was deleted, or belongs to a different user, without
// distinguishing why, by the same non-leaking convention
// inspection.ErrNotFound already follows.
var ErrImageNotFound = errors.New("image not found")
