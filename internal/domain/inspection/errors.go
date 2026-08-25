package inspection

import "errors"

// ErrNotFound is returned both when no inspection matches the given ID
// and when it exists but belongs to a different user. The two cases are
// deliberately indistinguishable to a caller: a user must never be able
// to tell whether another user's inspection ID exists at all.
var ErrNotFound = errors.New("inspection not found")
